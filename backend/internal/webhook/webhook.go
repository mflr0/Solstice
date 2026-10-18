// Package webhook POSTs what happens in Solstice to other systems, in the
// shape of Jira Server's webhooks (webhookEvent, timestamp, user, issue with
// its fields, changelog, comment…), so receivers written for Jira keep
// working.
//
// Deliveries are queued and sent by a background worker with retries. The
// target is checked when the connection is made, after name resolution: the
// link-local range (cloud metadata services live there) is always refused,
// whatever the name resolves to. Private networks and the local host are
// allowed, since self-hosted integrations are usually internal.
package webhook

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/secret"
)

// Event names, as Jira calls them.
const (
	IssueCreated   = "jira:issue_created"
	IssueUpdated   = "jira:issue_updated"
	IssueDeleted   = "jira:issue_deleted"
	CommentCreated = "comment_created"
	CommentUpdated = "comment_updated"
	CommentDeleted = "comment_deleted"
	WorklogCreated = "worklog_created"
	WorklogUpdated = "worklog_updated"
	WorklogDeleted = "worklog_deleted"
	SprintCreated  = "sprint_created"
	SprintStarted  = "sprint_started"
	SprintUpdated  = "sprint_updated"
	SprintClosed   = "sprint_closed"
	SprintDeleted  = "sprint_deleted"
	BoardCreated   = "board_created"
	BoardUpdated   = "board_updated"
	BoardDeleted   = "board_deleted"
	// TestEvent is what the administrator's test sends.
	TestEvent = "solstice:test"
)

// Events lists the events Solstice sends, in display order.
var Events = []string{IssueCreated, IssueUpdated, IssueDeleted, CommentCreated, CommentUpdated, CommentDeleted,
	WorklogCreated, WorklogUpdated, WorklogDeleted, SprintCreated, SprintStarted, SprintUpdated, SprintClosed,
	SprintDeleted, BoardCreated, BoardUpdated, BoardDeleted}

// namesOf gives the webhook events an internal event fires. Comments and
// work logs also count as issue updates, as in Jira.
func namesOf(k events.Kind) []string {
	switch k {
	case events.IssueCreated:
		return []string{IssueCreated}
	case events.IssueUpdated:
		return []string{IssueUpdated}
	case events.IssueDeleted:
		return []string{IssueDeleted}
	case events.CommentCreated:
		return []string{CommentCreated, IssueUpdated}
	case events.CommentUpdated:
		return []string{CommentUpdated, IssueUpdated}
	case events.CommentDeleted:
		return []string{CommentDeleted, IssueUpdated}
	case events.WorklogCreated:
		return []string{WorklogCreated, IssueUpdated}
	case events.WorklogUpdated:
		return []string{WorklogUpdated, IssueUpdated}
	case events.WorklogDeleted:
		return []string{WorklogDeleted, IssueUpdated}
	case events.SprintCreated:
		return []string{SprintCreated}
	case events.SprintStarted:
		return []string{SprintStarted}
	case events.SprintUpdated:
		return []string{SprintUpdated}
	case events.SprintClosed:
		return []string{SprintClosed}
	case events.SprintDeleted:
		return []string{SprintDeleted}
	case events.BoardCreated:
		return []string{BoardCreated}
	case events.BoardUpdated:
		return []string{BoardUpdated}
	case events.BoardDeleted:
		return []string{BoardDeleted}
	}
	return nil
}

// Webhook is a configured webhook.
type Webhook struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	Events       []string  `json:"events"`
	JQL          string    `json:"jql"`
	HasSecret    bool      `json:"hasSecret"`
	Enabled      bool      `json:"enabled"`
	ExcludeBody  bool      `json:"excludeBody"`
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
	LastDelivery *Delivery `json:"lastDelivery"`

	sealed string // the secret, sealed
}

func (w *Webhook) listens(name string) bool { return w.Enabled && slices.Contains(w.Events, name) }

// Matcher tells whether an issue matches a JQL query.
type Matcher func(ctx context.Context, jql string, issueID int64) (bool, error)

// Service stores webhooks and delivers their events.
type Service struct {
	db     *sql.DB
	log    *slog.Logger
	box    func() (*secret.Box, error)
	match  Matcher
	client *http.Client
	wake   chan struct{}

	// BaseURL is the public address, for "self" links; empty leaves them out.
	BaseURL func() string
	// DefaultLocale is the instance's language, for the names in payloads.
	DefaultLocale func() string
	// Now is the clock (tests move it).
	Now func() time.Time
	// Paused, when set and true, makes DeliverDue deliver nothing: the
	// server sets it while a replacing import rewrites the database.
	Paused func() bool

	mu      sync.Mutex
	cache   []*Webhook
	cacheAt time.Time
}

// secretLabel binds sealed secrets to webhooks (package secret).
const secretLabel = "webhook.secret"

// New makes the service: box opens secrets, match evaluates JQL filters.
func New(db *sql.DB, log *slog.Logger, box func() (*secret.Box, error), match Matcher) *Service {
	return &Service{db: db, log: log, box: box, match: match, client: guardedClient(), wake: make(chan struct{}, 1),
		BaseURL: func() string { return "" }, DefaultLocale: func() string { return i18n.Default }, Now: time.Now}
}

const columns = `id, name, url, events, COALESCE(jql, ''), COALESCE(secret, ''), enabled, exclude_body, created_at, updated_at`

func scanWebhook(sc interface{ Scan(...any) error }) (*Webhook, error) {
	var w Webhook
	var evs string
	if err := sc.Scan(&w.ID, &w.Name, &w.URL, &evs, &w.JQL, &w.sealed, &w.Enabled, &w.ExcludeBody, &w.Created,
		&w.Updated); err != nil {
		return nil, err
	}
	w.Events = []string{}
	for e := range strings.SplitSeq(evs, ",") {
		if e != "" {
			w.Events = append(w.Events, e)
		}
	}
	w.HasSecret = w.sealed != ""
	return &w, nil
}

// List returns every webhook, with its last delivery.
func (s *Service) List(ctx context.Context) ([]*Webhook, error) {
	out := []*Webhook{}
	rows, err := s.db.QueryContext(ctx, `SELECT `+columns+` FROM webhooks ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, w)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, w := range out {
		d, err := s.Deliveries(ctx, w.ID, 1)
		if err != nil {
			return nil, err
		}
		if len(d) > 0 {
			w.LastDelivery = &d[0]
		}
	}
	return out, nil
}

// Get returns a webhook; sql.ErrNoRows if there is none.
func (s *Service) Get(ctx context.Context, id int64) (*Webhook, error) {
	w, err := scanWebhook(s.db.QueryRowContext(ctx, `SELECT `+columns+` FROM webhooks WHERE id = ?`, id))
	if err != nil {
		return nil, err
	}
	if d, err := s.Deliveries(ctx, id, 1); err == nil && len(d) > 0 {
		w.LastDelivery = &d[0]
	}
	return w, nil
}

// Save creates (ID 0) or updates a webhook checked by the caller. The secret
// is left alone when nil, removed when empty.
func (s *Service) Save(ctx context.Context, w *Webhook, secretText *string, by int64) error {
	sealed := any(nil)
	keep := secretText == nil
	if secretText != nil && *secretText != "" {
		box, err := s.box()
		if err != nil {
			return err
		}
		v, err := box.SealString(secretLabel, *secretText)
		if err != nil {
			return err
		}
		sealed = v
	}
	var jql any
	if strings.TrimSpace(w.JQL) != "" {
		jql = strings.TrimSpace(w.JQL)
	}
	now := s.Now().UTC()
	evs := strings.Join(w.Events, ",")
	if w.ID == 0 {
		var creator any
		if by != 0 {
			creator = by
		}
		res, err := s.db.ExecContext(ctx, `INSERT INTO webhooks (name, url, events, jql, secret, enabled, exclude_body,
			created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			w.Name, w.URL, evs, jql, sealed, w.Enabled, w.ExcludeBody, creator, now, now)
		if err != nil {
			return err
		}
		w.ID, err = res.LastInsertId()
		s.invalidate()
		return err
	}
	q := `UPDATE webhooks SET name = ?, url = ?, events = ?, jql = ?, enabled = ?, exclude_body = ?, updated_at = ?`
	args := []any{w.Name, w.URL, evs, jql, w.Enabled, w.ExcludeBody, now}
	if !keep {
		q += `, secret = ?`
		args = append(args, sealed)
	}
	_, err := s.db.ExecContext(ctx, q+` WHERE id = ?`, append(args, w.ID)...)
	s.invalidate()
	return err
}

// Delete removes a webhook and its deliveries.
func (s *Service) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	s.invalidate()
	return err
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

// hooks returns the webhooks from a short-lived cache: they are looked at
// for every event.
func (s *Service) hooks(ctx context.Context) []*Webhook {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache != nil && time.Since(s.cacheAt) < 30*time.Second {
		return s.cache
	}
	var list []*Webhook
	rows, err := s.db.QueryContext(ctx, `SELECT `+columns+` FROM webhooks WHERE enabled`)
	if err != nil {
		s.log.Error("reading webhooks failed", "err", err)
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			s.log.Error("reading webhooks failed", "err", err)
			return nil
		}
		list = append(list, w)
	}
	s.cache, s.cacheAt = list, time.Now()
	if s.cache == nil {
		s.cache = []*Webhook{}
	}
	return s.cache
}

// Delivery is one POST of an event to a webhook, as its log shows it.
type Delivery struct {
	ID          int64      `json:"id"`
	Event       string     `json:"event"`
	Status      string     `json:"status"` // pending, sent, failed
	StatusCode  *int       `json:"statusCode"`
	DurationMs  *int       `json:"durationMs"`
	Attempts    int        `json:"attempts"`
	Error       *string    `json:"error"`
	Created     time.Time  `json:"created"`
	Attempted   *time.Time `json:"attempted"`
	NextAttempt *time.Time `json:"nextAttempt"`
}

// Deliveries returns a webhook's latest deliveries, newest first.
func (s *Service) Deliveries(ctx context.Context, webhookID int64, limit int) ([]Delivery, error) {
	out := []Delivery{}
	rows, err := s.db.QueryContext(ctx, `SELECT id, event, status, status_code, duration_ms, attempts, error, created_at,
		attempted_at, next_attempt_at FROM webhook_deliveries WHERE webhook_id = ? ORDER BY id DESC LIMIT ?`,
		webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d Delivery
		var code, dur sql.NullInt64
		var errText sql.NullString
		var attempted sql.NullTime
		var next time.Time
		if err := rows.Scan(&d.ID, &d.Event, &d.Status, &code, &dur, &d.Attempts, &errText, &d.Created, &attempted,
			&next); err != nil {
			return nil, err
		}
		if code.Valid {
			c := int(code.Int64)
			d.StatusCode = &c
		}
		if dur.Valid {
			v := int(dur.Int64)
			d.DurationMs = &v
		}
		if errText.Valid {
			d.Error = &errText.String
		}
		if attempted.Valid {
			d.Attempted = &attempted.Time
		}
		if d.Status == "pending" {
			d.NextAttempt = &next
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
