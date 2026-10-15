package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"solstice/internal/events"
)

// MaxAttempts is how many times a delivery is tried before giving up
// (about three hours with the delays of backoff).
const MaxAttempts = 6

// keep is how many deliveries each webhook's log keeps.
const keep = 100

// urlVariables are Jira's placeholders in webhook addresses.
var urlVariables = regexp.MustCompile(`\$\{[A-Za-z.]+\}`)

// Wants: events some enabled webhook listens to.
func (s *Service) Wants(e *events.Event) bool {
	names := namesOf(e.Kind)
	if len(names) == 0 {
		return false
	}
	for _, w := range s.hooks(context.Background()) {
		for _, n := range names {
			if w.listens(n) {
				return true
			}
		}
	}
	return false
}

// MatchBeforeDelete evaluates, before an issue is deleted, the JQL filters
// of the webhooks that announce deletions.
func (s *Service) MatchBeforeDelete(ctx context.Context, issueID int64) map[int64]bool {
	out := map[int64]bool{}
	for _, w := range s.hooks(ctx) {
		if w.listens(IssueDeleted) && w.JQL != "" {
			ok, err := s.match(ctx, w.JQL, issueID)
			if err != nil {
				s.log.Warn("webhook JQL failed", "webhook", w.ID, "err", err)
			}
			out[w.ID] = ok
		}
	}
	return out
}

// matches tells whether an event's issue passes a webhook's JQL filter.
// Events without an issue (sprints, boards) always pass.
func (s *Service) matches(ctx context.Context, w *Webhook, e *events.Event) bool {
	if w.JQL == "" || e.IssueID == 0 {
		return true
	}
	if e.Kind == events.IssueDeleted {
		return e.HookMatches[w.ID]
	}
	if e.Issue == nil {
		return false
	}
	ok, err := s.match(ctx, w.JQL, e.IssueID)
	if err != nil {
		s.log.Warn("webhook JQL failed", "webhook", w.ID, "err", err)
	}
	return ok
}

// Handle queues the deliveries of an event.
func (s *Service) Handle(ctx context.Context, e *events.Event) error {
	if e.IssueID != 0 && e.Issue == nil {
		return nil // the issue vanished before we got to it
	}
	var actor *events.User
	if e.ActorID != 0 {
		users, err := events.LoadUsers(ctx, s.db, e.ActorID)
		if err != nil {
			return err
		}
		actor = users[e.ActorID]
	}
	bodies := map[string][]byte{}
	queued := false
	now := s.Now().UTC()
	for _, w := range s.hooks(ctx) {
		for _, name := range namesOf(e.Kind) {
			if !w.listens(name) || !s.matches(ctx, w, e) {
				continue
			}
			body, ok := bodies[name]
			if !ok {
				p, err := s.payload(ctx, e, name)
				if err != nil {
					return err
				}
				if body, err = json.Marshal(p); err != nil {
					return err
				}
				bodies[name] = body
			}
			if w.ExcludeBody {
				body = nil
			}
			if _, err := s.db.ExecContext(ctx, `INSERT INTO webhook_deliveries (webhook_id, event, url, payload,
				next_attempt_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`, w.ID, name, expandURL(w.URL, e, actor),
				string(body), now, now); err != nil {
				return err
			}
			queued = true
		}
	}
	if queued {
		s.Wake()
	}
	return nil
}

// Wake makes the worker deliver now rather than at its next tick.
func (s *Service) Wake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// backoff is the delay before the next try after n failed attempts.
func backoff(n int) time.Duration {
	switch n {
	case 1:
		return 30 * time.Second
	case 2:
		return 2 * time.Minute
	case 3:
		return 10 * time.Minute
	case 4:
		return 30 * time.Minute
	}
	return 2 * time.Hour
}

// result is the outcome of one POST.
type result struct {
	code      int
	duration  time.Duration
	err       error
	permanent bool
}

// post sends a body to a webhook, signed when it has a secret.
func (s *Service) post(ctx context.Context, target, event string, delivery int64, body []byte, sealed string) result {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return result{err: err, permanent: true}
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return result{err: ErrScheme, permanent: true}
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "Solstice-Webhook/1")
	req.Header.Set("X-Solstice-Event", event)
	req.Header.Set("X-Solstice-Delivery", strconv.FormatInt(delivery, 10))
	if sealed != "" {
		box, err := s.box()
		if err != nil {
			return result{err: err}
		}
		key, err := box.Open(secretLabel, sealed)
		if err != nil {
			return result{err: err}
		}
		mac := hmac.New(sha256.New, key)
		mac.Write(body)
		req.Header.Set("X-Hub-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	start := time.Now()
	resp, err := s.client.Do(req)
	r := result{duration: time.Since(start)}
	if err != nil {
		var be *blockedError
		r.err, r.permanent = err, errors.As(err, &be)
		return r
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	r.code = resp.StatusCode
	switch {
	case r.code >= 200 && r.code < 300:
	case r.code >= 300 && r.code < 400:
		r.err, r.permanent = fmt.Errorf("redirect to %q not followed", resp.Header.Get("Location")), true
	case r.code == 408 || r.code == 425 || r.code == 429 || r.code >= 500:
		r.err = fmt.Errorf("HTTP %d", r.code)
	default:
		r.err, r.permanent = fmt.Errorf("HTTP %d", r.code), true
	}
	return r
}

// DeliverDue sends the deliveries whose time has come, up to a batch. It
// stops between deliveries when ctx ends.
func (s *Service) DeliverDue(ctx context.Context) (int, error) {
	raw := make([]byte, 16)
	rand.Read(raw)
	claim := hex.EncodeToString(raw)
	now := s.Now().UTC()
	if _, err := s.db.ExecContext(ctx, `UPDATE webhook_deliveries SET claim = ?, next_attempt_at = ?
		WHERE status = 'pending' AND next_attempt_at <= ? ORDER BY id LIMIT 50`, claim, now.Add(10*time.Minute), now); err != nil {
		return 0, err
	}
	type row struct {
		id, hook        int64
		event, url, sec string
		body            []byte
		attempts        int
	}
	var list []row
	rows, err := s.db.QueryContext(ctx, `SELECT d.id, d.webhook_id, d.event, d.url, d.payload, d.attempts,
		COALESCE(w.secret, '') FROM webhook_deliveries d JOIN webhooks w ON w.id = d.webhook_id
		WHERE d.claim = ? ORDER BY d.id`, claim)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var r row
		var body string
		if err := rows.Scan(&r.id, &r.hook, &r.event, &r.url, &body, &r.attempts, &r.sec); err != nil {
			rows.Close()
			return 0, err
		}
		r.body = []byte(body)
		list = append(list, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	bg := context.WithoutCancel(ctx)
	sent := 0
	touched := map[int64]bool{}
	for _, r := range list {
		if ctx.Err() != nil {
			break
		}
		res := s.post(bg, r.url, r.event, r.id, r.body, r.sec)
		s.record(bg, r.id, r.attempts+1, res)
		if res.err == nil {
			sent++
		}
		touched[r.hook] = true
	}
	// Left over at shutdown: back to the queue.
	if _, err := s.db.ExecContext(bg, `UPDATE webhook_deliveries SET claim = NULL, next_attempt_at = ?
		WHERE claim = ? AND status = 'pending'`, now, claim); err != nil {
		return sent, err
	}
	for hook := range touched {
		s.prune(bg, hook)
	}
	return sent, nil
}

// record stores the outcome of an attempt.
func (s *Service) record(ctx context.Context, id int64, attempts int, r result) {
	now := s.Now().UTC()
	status, next := "sent", now
	var errText, code any
	if r.err != nil {
		errText = truncate(r.err.Error(), 1000)
		status = "failed"
		if !r.permanent && attempts < MaxAttempts {
			status, next = "pending", now.Add(backoff(attempts))
		}
	}
	if r.code != 0 {
		code = r.code
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE webhook_deliveries SET status = ?, attempts = ?, status_code = ?,
		duration_ms = ?, error = ?, attempted_at = ?, next_attempt_at = ?, claim = NULL WHERE id = ?`,
		status, attempts, code, r.duration.Milliseconds(), errText, now, next, id); err != nil {
		s.log.Error("webhook delivery update failed", "id", id, "err", err)
	}
}

// prune keeps the last deliveries of a webhook.
func (s *Service) prune(ctx context.Context, hook int64) {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM webhook_deliveries WHERE webhook_id = ? AND status <> 'pending'
		AND id <= (SELECT id FROM (SELECT id FROM webhook_deliveries WHERE webhook_id = ? ORDER BY id DESC
		LIMIT 1 OFFSET ?) last)`, hook, hook, keep); err != nil {
		s.log.Warn("webhook log pruning failed", "webhook", hook, "err", err)
	}
}

// Run delivers until ctx ends: every few seconds, or as soon as something
// is queued.
func (s *Service) Run(ctx context.Context) {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-s.wake:
		}
		for {
			n, err := s.DeliverDue(ctx)
			if err != nil {
				s.log.Error("webhook deliveries failed", "err", err)
			}
			if n < 50 || ctx.Err() != nil {
				break
			}
		}
	}
}

// Test sends a test event to a webhook now and returns how it went. It
// appears in the delivery log, and is not retried.
func (s *Service) Test(ctx context.Context, id int64, actor int64) (*Delivery, error) {
	w, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	users, err := events.LoadUsers(ctx, s.db, actor)
	if err != nil {
		return nil, err
	}
	b := &builder{base: s.BaseURL(), locale: s.DefaultLocale(), users: users}
	now := s.Now().UTC()
	body, err := json.Marshal(map[string]any{"timestamp": now.UnixMilli(), "webhookEvent": TestEvent,
		"user": b.userID(actor), "webhook": map[string]any{"id": w.ID, "name": w.Name}})
	if err != nil {
		return nil, err
	}
	if w.ExcludeBody {
		body = nil
	}
	target := urlVariables.ReplaceAllString(w.URL, "")
	res, err := s.db.ExecContext(ctx, `INSERT INTO webhook_deliveries (webhook_id, event, url, payload, status, attempts,
		next_attempt_at, created_at) VALUES (?, ?, ?, ?, 'sending', 0, ?, ?)`, w.ID, TestEvent, target, string(body), now, now)
	if err != nil {
		return nil, err
	}
	did, _ := res.LastInsertId()
	r := s.post(ctx, target, TestEvent, did, body, w.sealed)
	r.permanent = true // a test is not retried
	s.record(context.WithoutCancel(ctx), did, 1, r)
	s.prune(context.WithoutCancel(ctx), w.ID)
	list, err := s.Deliveries(ctx, w.ID, 1)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
