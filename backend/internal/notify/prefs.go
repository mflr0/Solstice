package notify

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"

	"solstice/internal/sqlq"
)

// Event groups: what a user can switch on or off.
const (
	GroupCreated     = "created"     // an issue was created
	GroupAssigned    = "assigned"    // an issue was (re)assigned
	GroupCommented   = "commented"   // a comment was added, edited or deleted
	GroupMentioned   = "mentioned"   // you were mentioned
	GroupStatus      = "status"      // status changed, resolved, reopened
	GroupUpdated     = "updated"     // other fields changed
	GroupAttachments = "attachments" // attachments added or removed
	GroupWorklogs    = "worklogs"    // work logged, a work log edited or deleted
	GroupDeleted     = "deleted"     // an issue was deleted
)

// Groups lists the event groups in display order.
var Groups = []string{GroupCreated, GroupAssigned, GroupCommented, GroupMentioned, GroupStatus, GroupUpdated,
	GroupAttachments, GroupWorklogs, GroupDeleted}

// Delivery modes.
const (
	Immediate = "immediate" // within a couple of minutes, changes to an issue batched
	Digest    = "digest"    // once a day, at DigestHour
	None      = "none"      // no notification e-mails
)

// Prefs is what a user wants by e-mail. Hours are in the server's time zone.
type Prefs struct {
	Delivery     string          `json:"delivery"`
	DigestHour   int             `json:"digestHour"`
	OwnChanges   bool            `json:"ownChanges"`
	Events       map[string]bool `json:"events"`
	DueReminder  bool            `json:"dueReminder"`
	ReminderHour int             `json:"reminderHour"`
}

// Defaults are the preferences of someone who never changed them.
func Defaults() Prefs {
	p := Prefs{Delivery: Immediate, DigestHour: 8, ReminderHour: 8, Events: map[string]bool{}}
	for _, g := range Groups {
		p.Events[g] = true
	}
	return p
}

// wants tells whether the user wants an event group.
func (p Prefs) wants(group string) bool { return p.Events[group] }

func (p Prefs) disabled() string {
	var off []string
	for _, g := range Groups {
		if !p.Events[g] {
			off = append(off, g)
		}
	}
	return strings.Join(off, ",")
}

func scanPrefs(delivery string, digestHour int, own bool, disabled string, reminder bool, reminderHour int) Prefs {
	p := Defaults()
	p.Delivery, p.DigestHour, p.OwnChanges, p.DueReminder, p.ReminderHour = delivery, digestHour, own, reminder, reminderHour
	for g := range strings.SplitSeq(disabled, ",") {
		if _, ok := p.Events[g]; ok {
			p.Events[g] = false
		}
	}
	return p
}

// Prefs reads a user's preferences.
func (s *Service) Prefs(ctx context.Context, userID int64) (Prefs, error) {
	all, err := s.prefsOf(ctx, userID)
	if err != nil {
		return Prefs{}, err
	}
	return all[userID], nil
}

// prefsOf reads several users' preferences; everyone gets an entry.
func (s *Service) prefsOf(ctx context.Context, ids ...int64) (map[int64]Prefs, error) {
	out := map[int64]Prefs{}
	var args []any
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			out[id] = Defaults()
			args = append(args, id)
		}
	}
	if len(args) == 0 {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, delivery, digest_hour, own_changes, disabled_events,
		due_reminder, reminder_hour FROM notification_prefs WHERE user_id IN (`+sqlq.Marks(len(args))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var delivery, disabled string
		var digestHour, reminderHour int
		var own, reminder bool
		if err := rows.Scan(&id, &delivery, &digestHour, &own, &disabled, &reminder, &reminderHour); err != nil {
			return nil, err
		}
		out[id] = scanPrefs(delivery, digestHour, own, disabled, reminder, reminderHour)
	}
	return out, rows.Err()
}

// SavePrefs writes a user's preferences (checked by the caller). Changes to
// e-mails already waiting follow: none drops them, a new digest hour moves
// the digest.
func (s *Service) SavePrefs(ctx context.Context, userID int64, p Prefs) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO notification_prefs (user_id, delivery, digest_hour, own_changes,
		disabled_events, due_reminder, reminder_hour, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE delivery = VALUES(delivery), digest_hour = VALUES(digest_hour),
		own_changes = VALUES(own_changes), disabled_events = VALUES(disabled_events),
		due_reminder = VALUES(due_reminder), reminder_hour = VALUES(reminder_hour), updated_at = VALUES(updated_at)`,
		userID, p.Delivery, p.DigestHour, p.OwnChanges, p.disabled(), p.DueReminder, p.ReminderHour,
		s.Now().UTC()); err != nil {
		return err
	}
	switch p.Delivery {
	case None:
		if _, err := tx.ExecContext(ctx, `DELETE FROM notification_batches WHERE user_id = ?`, userID); err != nil {
			return err
		}
	case Digest:
		if _, err := tx.ExecContext(ctx, `UPDATE notification_batches SET send_at = ? WHERE user_id = ? AND digest`,
			nextAt(s.Now(), s.zone(), p.DigestHour, nil), userID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ExtraRecipient is a group or project role that also receives an event
// group in a project.
type ExtraRecipient struct {
	Event string `json:"event"`
	Kind  string `json:"kind"` // group, role
	ID    int64  `json:"id"`
	Name  string `json:"name"`
}

// ProjectSettings are a project's notification overrides.
type ProjectSettings struct {
	Enabled bool             `json:"enabled"`
	Extra   []ExtraRecipient `json:"extra"`
}

// ProjectSettings reads a project's overrides.
func (s *Service) ProjectSettings(ctx context.Context, projectID int64) (*ProjectSettings, error) {
	ps := &ProjectSettings{Enabled: true, Extra: []ExtraRecipient{}}
	err := s.db.QueryRowContext(ctx, `SELECT enabled FROM project_notifications WHERE project_id = ?`, projectID).
		Scan(&ps.Enabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT r.event, r.group_id, r.role_id, COALESCE(g.name, pr.name, '')
		FROM project_notification_recipients r
		LEFT JOIN user_groups g ON g.id = r.group_id LEFT JOIN project_roles pr ON pr.id = r.role_id
		WHERE r.project_id = ? ORDER BY r.event, r.id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e ExtraRecipient
		var group, role sql.NullInt64
		if err := rows.Scan(&e.Event, &group, &role, &e.Name); err != nil {
			return nil, err
		}
		if group.Valid {
			e.Kind, e.ID = "group", group.Int64
		} else {
			e.Kind, e.ID = "role", role.Int64
		}
		ps.Extra = append(ps.Extra, e)
	}
	return ps, rows.Err()
}

// SaveProjectSettings writes a project's overrides (checked by the caller).
func (s *Service) SaveProjectSettings(ctx context.Context, projectID int64, ps ProjectSettings) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_notifications (project_id, enabled) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE enabled = VALUES(enabled)`, projectID, ps.Enabled); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_notification_recipients WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	for _, e := range ps.Extra {
		var group, role any
		if e.Kind == "group" {
			group = e.ID
		} else {
			role = e.ID
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_notification_recipients (project_id, event, group_id, role_id)
			VALUES (?, ?, ?, ?)`, projectID, e.Event, group, role); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// extraMembers returns the users a project adds for some event groups, with
// the groups each gets.
func (s *Service) extraMembers(ctx context.Context, projectID int64, extra []ExtraRecipient, groups []string) (map[int64]bool, error) {
	out := map[int64]bool{}
	for _, e := range extra {
		if !slices.Contains(groups, e.Event) {
			continue
		}
		var q string
		var args []any
		if e.Kind == "group" {
			q, args = `SELECT user_id FROM group_members WHERE group_id = ?`, []any{e.ID}
		} else {
			q = `SELECT user_id FROM project_role_members WHERE project_id = ? AND role_id = ? AND user_id IS NOT NULL
				UNION SELECT m.user_id FROM project_role_members r JOIN group_members m ON m.group_id = r.group_id
				WHERE r.project_id = ? AND r.role_id = ?`
			args = []any{projectID, e.ID, projectID, e.ID}
		}
		if err := sqlq.Each(ctx, s.db, q, func(rows *sql.Rows) error {
			var id int64
			err := rows.Scan(&id)
			out[id] = true
			return err
		}, args...); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// nextAt is the first time after now at hour o'clock in tz, on one of days
// (ISO weekdays, 1 = Monday) when days is not empty.
func nextAt(now time.Time, tz *time.Location, hour int, days []int) time.Time {
	local := now.In(tz)
	t := time.Date(local.Year(), local.Month(), local.Day(), hour, 0, 0, 0, tz)
	for i := 0; i < 8; i++ {
		if t.After(now) && (len(days) == 0 || slices.Contains(days, isoWeekday(t))) {
			return t.UTC()
		}
		t = time.Date(t.Year(), t.Month(), t.Day()+1, hour, 0, 0, 0, tz)
	}
	return t.UTC()
}

func isoWeekday(t time.Time) int {
	if d := int(t.Weekday()); d != 0 {
		return d
	}
	return 7
}
