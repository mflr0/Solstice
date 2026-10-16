package mail

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"time"
)

// MaxAttempts is how many times a message is tried before giving up (about
// two hours with the delays of backoff).
const MaxAttempts = 8

// Execer runs a statement: the database or a transaction.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Outbox stores messages to send and sends them.
type Outbox struct {
	db     *sql.DB
	log    *slog.Logger
	config func(context.Context) (Config, error)
	wake   chan struct{}

	// Now is the clock (tests move it).
	Now func() time.Time
	// Paused, when set and true, makes SendDue send nothing: the server
	// sets it while a replacing import rewrites the database.
	Paused func() bool
}

// NewOutbox makes an outbox; config reads the current SMTP settings.
func NewOutbox(db *sql.DB, log *slog.Logger, config func(context.Context) (Config, error)) *Outbox {
	return &Outbox{db: db, log: log, config: config, wake: make(chan struct{}, 1), Now: time.Now}
}

// Enqueue stores a message, in the caller's transaction if ex is one. Call
// Wake once committed.
func (o *Outbox) Enqueue(ctx context.Context, ex Execer, m Message) error {
	now := o.Now().UTC()
	var inReplyTo any
	if m.InReplyTo != "" {
		inReplyTo = m.InReplyTo
	}
	var user any
	if m.UserID != 0 {
		user = m.UserID
	}
	_, err := ex.ExecContext(ctx, `INSERT INTO mail_outbox (kind, user_id, to_addr, subject, text_body, html_body,
		message_id, in_reply_to, next_attempt_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Kind, user, m.To, truncate(headerValue(m.Subject), 900), m.Text, m.HTML, m.MessageID, inReplyTo, now, now)
	return err
}

// Wake makes the worker look at the outbox now rather than at its next tick.
func (o *Outbox) Wake() {
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

// backoff is the delay before the next try after n failed attempts.
func backoff(n int) time.Duration {
	d := time.Minute << max(0, n-1)
	return min(d, 2*time.Hour)
}

// SendDue sends the messages whose time has come, up to a batch. It stops
// between messages when ctx ends; a message being sent is finished.
func (o *Outbox) SendDue(ctx context.Context) (int, error) {
	if o.Paused != nil && o.Paused() {
		return 0, nil
	}
	cfg, err := o.config(ctx)
	if err != nil || !cfg.Enabled || cfg.Host == "" || cfg.From == "" {
		return 0, err
	}
	raw := make([]byte, 16)
	rand.Read(raw)
	claim := hex.EncodeToString(raw)
	now := o.Now().UTC()
	// Claimed rows are leased: if this process dies, they come back later.
	if _, err := o.db.ExecContext(ctx, `UPDATE mail_outbox SET claim = ?, next_attempt_at = ?
		WHERE status = 'pending' AND next_attempt_at <= ? ORDER BY id LIMIT 50`,
		claim, now.Add(15*time.Minute), now); err != nil {
		return 0, err
	}
	type row struct {
		id       int64
		m        Message
		attempts int
		inactive bool
	}
	var list []row
	rows, err := o.db.QueryContext(ctx, `SELECT o.id, o.kind, COALESCE(o.user_id, 0), o.to_addr, o.subject, o.text_body,
		o.html_body, o.message_id, COALESCE(o.in_reply_to, ''), o.attempts, COALESCE(NOT u.active, FALSE)
		FROM mail_outbox o LEFT JOIN users u ON u.id = o.user_id WHERE o.claim = ? ORDER BY o.id`, claim)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.m.Kind, &r.m.UserID, &r.m.To, &r.m.Subject, &r.m.Text, &r.m.HTML, &r.m.MessageID,
			&r.m.InReplyTo, &r.attempts, &r.inactive); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(list) == 0 {
		return 0, nil
	}

	// Writes after a send must happen even if shutdown began meanwhile.
	bg := context.WithoutCancel(ctx)
	s := &session{cfg: cfg}
	defer s.close()
	sent := 0
	for _, r := range list {
		if ctx.Err() != nil {
			break
		}
		if r.inactive {
			o.finish(bg, r.id, "failed", r.attempts, "recipient account deactivated", time.Time{})
			continue
		}
		data, err := r.m.Bytes(cfg.Sender(), o.Now())
		if err == nil {
			sendCtx, cancel := context.WithTimeout(bg, 2*time.Minute)
			err = s.send(sendCtx, cfg.From, r.m.To, data)
			cancel()
		}
		attempts := r.attempts + 1
		switch {
		case err == nil:
			sent++
			o.finish(bg, r.id, "sent", attempts, "", time.Time{})
		case Permanent(err) || attempts >= MaxAttempts:
			o.log.Warn("e-mail abandoned", "id", r.id, "to", r.m.To, "err", err)
			o.finish(bg, r.id, "failed", attempts, err.Error(), time.Time{})
		default:
			o.finish(bg, r.id, "pending", attempts, err.Error(), o.Now().UTC().Add(backoff(attempts)))
		}
	}
	// Whatever was left over (shutdown) goes back to the queue.
	_, err = o.db.ExecContext(bg, `UPDATE mail_outbox SET claim = NULL, next_attempt_at = ?
		WHERE claim = ? AND status = 'pending'`, now, claim)
	return sent, err
}

func (o *Outbox) finish(ctx context.Context, id int64, status string, attempts int, errText string, next time.Time) {
	var lastErr, sentAt any
	if errText != "" {
		lastErr = truncate(errText, 2000)
	}
	now := o.Now().UTC()
	if status == "sent" {
		sentAt = now
	}
	if next.IsZero() {
		next = now
	}
	if _, err := o.db.ExecContext(ctx, `UPDATE mail_outbox SET status = ?, attempts = ?, last_error = ?, sent_at = ?,
		next_attempt_at = ?, claim = NULL WHERE id = ?`, status, attempts, lastErr, sentAt, next, id); err != nil {
		o.log.Error("outbox update failed", "id", id, "err", err)
	}
}

// Run sends the outbox until ctx ends: every few seconds, or as soon as
// something is enqueued. Old messages are pruned along the way.
func (o *Outbox) Run(ctx context.Context) {
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	var pruned time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-o.wake:
		}
		for {
			n, err := o.SendDue(ctx)
			if err != nil {
				o.log.Error("sending e-mails failed", "err", err)
			}
			if n < 50 || ctx.Err() != nil {
				break
			}
		}
		if time.Since(pruned) > time.Hour {
			pruned = time.Now()
			now := o.Now().UTC()
			if _, err := o.db.ExecContext(ctx, `DELETE FROM mail_outbox WHERE (status = 'sent' AND created_at < ?)
				OR (status = 'failed' AND created_at < ?)`, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30)); err != nil {
				o.log.Warn("outbox pruning failed", "err", err)
			}
		}
	}
}

// Entry is an outbox message as administrators see it.
type Entry struct {
	ID          int64      `json:"id"`
	Kind        string     `json:"kind"`
	To          string     `json:"to"`
	Subject     string     `json:"subject"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	LastError   *string    `json:"lastError"`
	Created     time.Time  `json:"created"`
	NextAttempt *time.Time `json:"nextAttempt"`
	Sent        *time.Time `json:"sent"`
}

// Stats sums up the outbox.
type Stats struct {
	Pending       int        `json:"pending"`
	Failed        int        `json:"failed"`
	SentLastDay   int        `json:"sentLastDay"`
	OldestPending *time.Time `json:"oldestPending"`
	Recent        []Entry    `json:"recent"` // the last messages, newest first
	Errors        []Entry    `json:"errors"` // the last ones that failed at least once
}

// Stats reads the outbox's state.
func (o *Outbox) Stats(ctx context.Context) (*Stats, error) {
	st := &Stats{Recent: []Entry{}, Errors: []Entry{}}
	var oldest sql.NullTime
	err := o.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(status = 'pending'), 0), COALESCE(SUM(status = 'failed'), 0),
		COALESCE(SUM(status = 'sent' AND sent_at >= ?), 0), MIN(CASE WHEN status = 'pending' THEN created_at END)
		FROM mail_outbox`, o.Now().UTC().Add(-24*time.Hour)).Scan(&st.Pending, &st.Failed, &st.SentLastDay, &oldest)
	if err != nil {
		return nil, err
	}
	if oldest.Valid {
		st.OldestPending = &oldest.Time
	}
	for _, q := range []struct {
		where string
		out   *[]Entry
	}{{"TRUE", &st.Recent}, {"last_error IS NOT NULL", &st.Errors}} {
		rows, err := o.db.QueryContext(ctx, `SELECT id, kind, to_addr, subject, status, attempts, last_error, created_at,
			next_attempt_at, sent_at FROM mail_outbox WHERE `+q.where+` ORDER BY id DESC LIMIT 50`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var e Entry
			var lastErr sql.NullString
			var next, sent sql.NullTime
			if err := rows.Scan(&e.ID, &e.Kind, &e.To, &e.Subject, &e.Status, &e.Attempts, &lastErr, &e.Created, &next,
				&sent); err != nil {
				rows.Close()
				return nil, err
			}
			if lastErr.Valid {
				e.LastError = &lastErr.String
			}
			if e.Status == "pending" && next.Valid {
				e.NextAttempt = &next.Time
			}
			if sent.Valid {
				e.Sent = &sent.Time
			}
			*q.out = append(*q.out, e)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return st, nil
}

// Retry puts the messages that were given up on back in the queue.
func (o *Outbox) Retry(ctx context.Context) (int64, error) {
	res, err := o.db.ExecContext(ctx, `UPDATE mail_outbox SET status = 'pending', attempts = 0, next_attempt_at = ?,
		claim = NULL WHERE status = 'failed'`, o.Now().UTC())
	if err != nil {
		return 0, err
	}
	o.Wake()
	return res.RowsAffected()
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
