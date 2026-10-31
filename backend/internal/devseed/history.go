package devseed

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"
)

// The API dates everything now, so the boards' reports would have no
// history. history moves what the seed just did through the API back to the
// days of its timeline: only rows written since it started, of the issues
// and sprints the seed made.
type history struct {
	conn  *sql.DB
	since time.Time // rows written at or after this are the seed's, still undated
	day0  time.Time // today, midnight UTC
	seq   int
}

func newHistory(conn *sql.DB) *history {
	now := time.Now().UTC()
	return &history{conn: conn, since: now.Truncate(time.Millisecond),
		day0: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)}
}

// at is a moment of a day relative to today, in the morning; each call is a
// minute after the previous one, so the events of a day keep their order.
func (h *history) at(day int) time.Time {
	h.seq++
	return h.day0.AddDate(0, 0, day).Add(9*time.Hour + time.Duration(h.seq)*time.Minute)
}

// issueID finds an issue by key.
func (h *history) issueID(ctx context.Context, key string) (int64, error) {
	pkey, num, _ := strings.Cut(key, "-")
	n, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return 0, err
	}
	var id int64
	err = h.conn.QueryRowContext(ctx, `SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = ? AND i.num = ?`, pkey, n).Scan(&id)
	return id, err
}

// created dates an issue the seed just made.
func (h *history) created(ctx context.Context, key string, at time.Time) error {
	id, err := h.issueID(ctx, key)
	if err != nil {
		return err
	}
	_, err = h.conn.ExecContext(ctx, `UPDATE issues SET created_at = ?, updated_at = ? WHERE id = ?`, at, at, id)
	return err
}

// changed dates the issue's last changes, those not dated yet.
func (h *history) changed(ctx context.Context, key string, at time.Time) error {
	id, err := h.issueID(ctx, key)
	if err != nil {
		return err
	}
	for _, q := range []string{
		`UPDATE change_groups SET created_at = ? WHERE issue_id = ? AND created_at >= ?`,
		`UPDATE issues SET updated_at = ? WHERE id = ? AND updated_at >= ?`,
		`UPDATE issues SET resolved_at = ? WHERE id = ? AND resolved_at >= ?`,
	} {
		if _, err := h.conn.ExecContext(ctx, q, at, id, h.since); err != nil {
			return err
		}
	}
	return nil
}

// sprint dates a sprint: its start and end, and its completion when closed.
func (h *history) sprint(ctx context.Context, id int64, start, end time.Time, completed *time.Time) error {
	_, err := h.conn.ExecContext(ctx, `UPDATE sprints SET start_at = ?, end_at = ?, completed_at = ? WHERE id = ?`,
		start, end, completed, id)
	return err
}
