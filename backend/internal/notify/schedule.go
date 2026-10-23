package notify

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"solstice/internal/i18n"
	"solstice/internal/mail"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// IssueLine is an issue in a list e-mail.
type IssueLine struct {
	Key      string
	Summary  string
	Status   string
	Priority string
	Assignee string
	Due      *string // YYYY-MM-DD
}

// FilterResult is what a saved filter finds for someone.
type FilterResult struct {
	Name   string
	Query  string
	Total  int
	Issues []IssueLine
}

// Searcher runs saved filters (the search engine lives in package api).
type Searcher interface {
	// RunFilter runs a filter as sub sees it, returning at most limit
	// issues; ok is false when sub may not see the filter.
	RunFilter(ctx context.Context, sub *perm.Subject, filterID int64, limit int) (res *FilterResult, ok bool, err error)
}

// MaxRows bounds the issues listed in a subscription e-mail.
const MaxRows = 200

// Subscription mails a filter's results on a schedule, to a user or to
// every member of a group.
type Subscription struct {
	ID        int64
	FilterID  int64
	OwnerID   int64
	UserID    int64 // or…
	GroupID   int64 // …a whole group
	GroupName string
	Frequency string // daily, weekly
	Days      []int  // ISO weekdays, for weekly
	Hour      int
	EmptyToo  bool // send even when nothing matches
	LastRun   *time.Time
	NextRun   time.Time
}

const subscriptionColumns = `s.id, s.filter_id, s.owner_id, COALESCE(s.user_id, 0), COALESCE(s.group_id, 0),
	COALESCE(g.name, ''), s.frequency, s.days, s.hour, s.empty_too, s.last_run_at, s.next_run_at
	FROM filter_subscriptions s LEFT JOIN user_groups g ON g.id = s.group_id`

func scanSubscription(sc interface{ Scan(...any) error }) (*Subscription, error) {
	var sub Subscription
	var days string
	var last sql.NullTime
	if err := sc.Scan(&sub.ID, &sub.FilterID, &sub.OwnerID, &sub.UserID, &sub.GroupID, &sub.GroupName, &sub.Frequency,
		&days, &sub.Hour, &sub.EmptyToo, &last, &sub.NextRun); err != nil {
		return nil, err
	}
	for d := range strings.SplitSeq(days, ",") {
		if n, err := strconv.Atoi(d); err == nil {
			sub.Days = append(sub.Days, n)
		}
	}
	sub.LastRun = sqlq.Time(last)
	return &sub, nil
}

// Subscriptions lists a filter's subscriptions.
func (s *Service) Subscriptions(ctx context.Context, filterID int64) ([]*Subscription, error) {
	var out []*Subscription
	err := sqlq.Each(ctx, s.db, `SELECT `+subscriptionColumns+` WHERE s.filter_id = ? ORDER BY s.id`, func(rows *sql.Rows) error {
		sub, err := scanSubscription(rows)
		out = append(out, sub)
		return err
	}, filterID)
	return out, err
}

// Subscription reads one subscription; sql.ErrNoRows if there is none.
func (s *Service) Subscription(ctx context.Context, id int64) (*Subscription, error) {
	return scanSubscription(s.db.QueryRowContext(ctx, `SELECT `+subscriptionColumns+` WHERE s.id = ?`, id))
}

func (sub *Subscription) next(now time.Time, tz *time.Location) time.Time {
	if sub.Frequency == "weekly" {
		return nextAt(now, tz, sub.Hour, sub.Days)
	}
	return nextAt(now, tz, sub.Hour, nil)
}

// SaveSubscription creates (ID 0) or updates a subscription, checked by the
// caller, and plans its next run.
func (s *Service) SaveSubscription(ctx context.Context, sub *Subscription) error {
	now := s.Now().UTC()
	sub.NextRun = sub.next(now, s.zone())
	days := make([]string, 0, len(sub.Days))
	slices.Sort(sub.Days)
	for _, d := range slices.Compact(sub.Days) {
		days = append(days, strconv.Itoa(d))
	}
	var user, group any
	if sub.GroupID != 0 {
		group = sub.GroupID
	} else {
		user = sub.UserID
	}
	if sub.ID == 0 {
		res, err := s.db.ExecContext(ctx, `INSERT INTO filter_subscriptions (filter_id, owner_id, user_id, group_id,
			frequency, days, hour, empty_too, next_run_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sub.FilterID, sub.OwnerID, user, group, sub.Frequency, strings.Join(days, ","), sub.Hour, sub.EmptyToo,
			sub.NextRun, now)
		if err != nil {
			return err
		}
		sub.ID, err = res.LastInsertId()
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE filter_subscriptions SET user_id = ?, group_id = ?, frequency = ?, days = ?,
		hour = ?, empty_too = ?, next_run_at = ? WHERE id = ?`, user, group, sub.Frequency, strings.Join(days, ","),
		sub.Hour, sub.EmptyToo, sub.NextRun, sub.ID)
	return err
}

// DeleteSubscription removes a subscription.
func (s *Service) DeleteSubscription(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM filter_subscriptions WHERE id = ?`, id)
	return err
}

// RunSubscription mails a subscription's results now, whatever its
// schedule. It returns how many e-mails were written.
func (s *Service) RunSubscription(ctx context.Context, sub *Subscription) (int, error) {
	if _, err := s.db.ExecContext(ctx, `UPDATE filter_subscriptions SET last_run_at = ? WHERE id = ?`,
		s.Now().UTC(), sub.ID); err != nil {
		return 0, err
	}
	n, err := s.sendSubscription(ctx, sub)
	if n > 0 {
		s.outbox.Wake()
	}
	return n, err
}

// RunSchedules sends the subscriptions and due-date reminders whose time
// has come.
func (s *Service) RunSchedules(ctx context.Context) error {
	if s.Paused != nil && s.Paused() {
		return nil
	}
	if !s.mailEnabled(ctx) {
		return nil
	}
	errSub := s.runSubscriptions(ctx)
	errRem := s.runReminders(ctx)
	return errors.Join(errSub, errRem)
}

func (s *Service) runSubscriptions(ctx context.Context) error {
	now := s.Now().UTC()
	var due []*Subscription
	if err := sqlq.Each(ctx, s.db, `SELECT `+subscriptionColumns+` WHERE s.next_run_at <= ? ORDER BY s.next_run_at LIMIT 100`,
		func(rows *sql.Rows) error {
			sub, err := scanSubscription(rows)
			due = append(due, sub)
			return err
		}, now); err != nil {
		return err
	}
	wrote := 0
	for _, sub := range due {
		if ctx.Err() != nil {
			break
		}
		// Claim the run, so two workers never send it twice.
		res, err := s.db.ExecContext(ctx, `UPDATE filter_subscriptions SET next_run_at = ?, last_run_at = ?
			WHERE id = ? AND next_run_at = ?`, sub.next(now, s.zone()), now, sub.ID, sub.NextRun)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
		n, err := s.sendSubscription(ctx, sub)
		if err != nil {
			s.log.Error("filter subscription failed", "subscription", sub.ID, "err", err)
		}
		wrote += n
	}
	if wrote > 0 {
		s.outbox.Wake()
	}
	return nil
}

// sendSubscription writes the e-mails of a subscription, one per
// recipient, each with the results that recipient may see.
func (s *Service) sendSubscription(ctx context.Context, sub *Subscription) (int, error) {
	var users []int64
	if sub.GroupID != 0 {
		if err := sqlq.Each(ctx, s.db, `SELECT m.user_id FROM group_members m JOIN users u ON u.id = m.user_id
			WHERE m.group_id = ? AND u.active ORDER BY m.user_id`, func(rows *sql.Rows) error {
			var id int64
			err := rows.Scan(&id)
			users = append(users, id)
			return err
		}, sub.GroupID); err != nil {
			return 0, err
		}
	} else {
		users = []int64{sub.UserID}
	}
	cfg, err := s.config(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range users {
		rc, err := s.recipientOf(ctx, id)
		if err != nil {
			return n, err
		}
		if rc == nil {
			continue
		}
		res, ok, err := s.search.RunFilter(ctx, rc.sub, sub.FilterID, MaxRows)
		if err != nil {
			return n, err
		}
		if !ok || (res.Total == 0 && !sub.EmptyToo) {
			continue
		}
		loc := rc.locale
		p := &page{Lang: loc, Heading: i18n.M(i18n.SubscriptionSubject, res.Name).In(loc)}
		p.Title = p.Heading
		p.Intro = []string{i18n.M(i18n.SubscriptionCount, res.Total, formatTime(loc, s.Now().In(s.zone()))).In(loc)}
		t := s.issueTable(loc, res.Issues, true)
		t.Empty = i18n.M(i18n.SubscriptionNoMatch).In(loc)
		all := s.url("/issues?q=" + url.QueryEscape(res.Query))
		switch {
		case res.Total > len(res.Issues):
			t.More, t.MoreURL = i18n.M(i18n.SubscriptionTruncated, len(res.Issues)).In(loc), all
		case all != "":
			t.More, t.MoreURL = i18n.M(i18n.SubscriptionOpen).In(loc), all
		}
		p.Tables = []table{t}
		if sub.GroupID != 0 {
			s.footer(p, i18n.M(i18n.SubscriptionWhyGroup, sub.GroupName).In(loc))
		} else {
			s.footer(p, i18n.M(i18n.SubscriptionWhyUser).In(loc))
		}
		if err := s.write(ctx, cfg, "subscription", rc, p); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// issueTable lists issues: key, summary, status, priority, assignee, due.
func (s *Service) issueTable(loc string, issues []IssueLine, wide bool) table {
	t := table{Columns: []string{i18n.M(i18n.FieldNameKey).In(loc), fieldLabel(loc, "summary"), fieldLabel(loc, "status")}}
	if wide {
		t.Columns = append(t.Columns, fieldLabel(loc, "priority"), fieldLabel(loc, "assignee"))
	}
	t.Columns = append(t.Columns, fieldLabel(loc, "duedate"))
	for _, iss := range issues {
		r := tableRow{Key: iss.Key, URL: s.issueURL(iss.Key), Cells: []string{iss.Summary, iss.Status}}
		if wide {
			r.Cells = append(r.Cells, iss.Priority, iss.Assignee)
		}
		due := ""
		if iss.Due != nil {
			due = formatDay(loc, *iss.Due)
		}
		r.Cells = append(r.Cells, due)
		t.Rows = append(t.Rows, r)
	}
	return t
}

// write renders a page into the outbox.
func (s *Service) write(ctx context.Context, cfg mail.Config, kind string, rc *recipient, p *page) error {
	return s.outbox.Enqueue(ctx, s.db, mail.Message{Kind: kind, UserID: rc.user.ID, To: rc.user.Email,
		ToName: rc.user.Name, Subject: p.Title, Text: p.text(), HTML: p.html(), MessageID: mail.NewMessageID(cfg.Domain())})
}

// runReminders sends, once a day from the chosen hour, the list of their
// unresolved issues due today or overdue to those who asked for it.
func (s *Service) runReminders(ctx context.Context) error {
	now := s.Now()
	local := now.In(s.zone())
	today := local.Format(time.DateOnly)
	type who struct {
		id   int64
		hour int
	}
	var list []who
	if err := sqlq.Each(ctx, s.db, `SELECT p.user_id, p.reminder_hour FROM notification_prefs p JOIN users u ON u.id = p.user_id
		WHERE p.due_reminder AND u.active AND (p.reminder_sent IS NULL OR p.reminder_sent < ?)`, func(rows *sql.Rows) error {
		var w who
		err := rows.Scan(&w.id, &w.hour)
		list = append(list, w)
		return err
	}, today); err != nil {
		return err
	}
	wrote := false
	for _, w := range list {
		if local.Hour() < w.hour || ctx.Err() != nil {
			continue
		}
		res, err := s.db.ExecContext(ctx, `UPDATE notification_prefs SET reminder_sent = ?
			WHERE user_id = ? AND (reminder_sent IS NULL OR reminder_sent < ?)`, today, w.id, today)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			continue
		}
		ok, err := s.sendReminder(ctx, w.id, today)
		if err != nil {
			s.log.Error("due-date reminder failed", "user", w.id, "err", err)
		}
		wrote = wrote || ok
	}
	if wrote {
		s.outbox.Wake()
	}
	return nil
}

// sendReminder writes a user's due-date reminder, if anything is due.
func (s *Service) sendReminder(ctx context.Context, userID int64, today string) (bool, error) {
	rc, err := s.recipientOf(ctx, userID)
	if err != nil || rc == nil {
		return false, err
	}
	vis, args := rc.sub.VisibleSQL("i")
	var overdue, due []IssueLine
	err = sqlq.Each(ctx, s.db, `SELECT p.pkey, i.num, i.summary, i.due_date, st.name, COALESCE(pr.name, '')
		FROM issues i JOIN projects p ON p.id = i.project_id JOIN statuses st ON st.id = i.status_id
		LEFT JOIN priorities pr ON pr.id = i.priority_id
		WHERE i.assignee_id = ? AND i.resolution_id IS NULL AND st.category <> 'done'
		AND i.due_date IS NOT NULL AND i.due_date <= ? AND `+vis+`
		ORDER BY i.due_date, p.pkey, i.num LIMIT 200`, func(rows *sql.Rows) error {
		var key string
		var num int64
		var l IssueLine
		var d time.Time
		if err := rows.Scan(&key, &num, &l.Summary, &d, &l.Status, &l.Priority); err != nil {
			return err
		}
		l.Key = key + "-" + strconv.FormatInt(num, 10)
		day := d.Format(time.DateOnly)
		l.Due = &day
		if day < today {
			overdue = append(overdue, l)
		} else {
			due = append(due, l)
		}
		return nil
	}, append([]any{userID, today}, args...)...)
	if err != nil || len(overdue)+len(due) == 0 {
		return false, err
	}
	cfg, err := s.config(ctx)
	if err != nil {
		return false, err
	}
	loc := rc.locale
	p := &page{Lang: loc, Heading: i18n.M(i18n.ReminderSubject, formatDay(loc, today)).In(loc)}
	p.Title = p.Heading
	if len(overdue) > 0 {
		t := s.issueTable(loc, overdue, false)
		t.Title = i18n.M(i18n.ReminderOverdue).In(loc)
		p.Tables = append(p.Tables, t)
	}
	if len(due) > 0 {
		t := s.issueTable(loc, due, false)
		t.Title = i18n.M(i18n.ReminderDueToday).In(loc)
		p.Tables = append(p.Tables, t)
	}
	s.footer(p, i18n.M(i18n.ReminderWhy).In(loc))
	return true, s.write(ctx, cfg, "reminder", rc, p)
}
