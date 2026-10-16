// Package notify e-mails people about what happens to the issues they
// follow, as Jira does: watchers, reporter and assignee by default, plus
// whoever is mentioned and the groups or roles a project adds.
//
// Who gets what is decided twice: when the change happens, and again when
// the e-mail is written, so nobody receives an issue or a restricted comment
// they can no longer see. Changes are batched: those made to one issue
// within a couple of minutes go out as one e-mail (or once a day, for those
// who chose a digest).
package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"solstice/internal/events"
	"solstice/internal/mail"
	"solstice/internal/perm"
)

// Service turns events into e-mails.
type Service struct {
	db     *sql.DB
	perms  *perm.Store
	outbox *mail.Outbox
	log    *slog.Logger
	tz     *time.Location
	config func(context.Context) (mail.Config, error)
	search Searcher

	// BaseURL is the public address for links; empty leaves links out.
	BaseURL func() string
	// Zone is the time zone of hours (digests, reminders, subscriptions);
	// by default the one given to New. DefaultLocale is the language of
	// e-mails to people who chose none (by default English).
	Zone          func() *time.Location
	DefaultLocale func() string
	// Window is how long changes to an issue wait for more before being
	// mailed; MaxWait bounds the wait when changes keep coming.
	Window, MaxWait time.Duration
	// Now is the clock (tests move it).
	Now func() time.Time

	mu        sync.Mutex
	enabled   bool
	enabledAt time.Time
}

// New makes the service. config reads the SMTP settings; search runs saved
// filters for subscriptions.
func New(db *sql.DB, perms *perm.Store, outbox *mail.Outbox, tz *time.Location, log *slog.Logger,
	config func(context.Context) (mail.Config, error), search Searcher) *Service {
	return &Service{db: db, perms: perms, outbox: outbox, log: log, tz: tz, config: config, search: search,
		BaseURL: func() string { return "" }, Window: 2 * time.Minute, MaxWait: 10 * time.Minute, Now: time.Now,
		Zone: func() *time.Location { return tz }, DefaultLocale: func() string { return "" }}
}

// zone is the time zone in effect.
func (s *Service) zone() *time.Location {
	if tz := s.Zone(); tz != nil {
		return tz
	}
	return time.Local
}

// mailEnabled tells whether e-mail is on, from a short-lived cache: it is
// asked for every event.
func (s *Service) mailEnabled(ctx context.Context) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.enabledAt) < 15*time.Second {
		return s.enabled
	}
	cfg, err := s.config(ctx)
	if err != nil {
		s.log.Warn("reading the mail settings failed", "err", err)
	}
	s.enabled, s.enabledAt = err == nil && cfg.Enabled, time.Now()
	return s.enabled
}

// Invalidate forgets the cached settings (after an administrator saved them).
func (s *Service) Invalidate() {
	s.mu.Lock()
	s.enabledAt = time.Time{}
	s.mu.Unlock()
}

// Wants: issue events, when e-mail is on.
func (s *Service) Wants(e *events.Event) bool {
	switch e.Kind {
	case events.IssueCreated, events.IssueUpdated, events.IssueDeleted, events.CommentCreated, events.CommentUpdated:
		return s.mailEnabled(context.Background())
	}
	return false
}

// Reasons someone receives a notification, strongest first.
const (
	reasonAssignee  = "assignee"
	reasonReporter  = "reporter"
	reasonMentioned = "mentioned"
	reasonWatcher   = "watcher"
	reasonProject   = "project"
)

var reasonRank = map[string]int{reasonAssignee: 0, reasonReporter: 1, reasonMentioned: 2, reasonWatcher: 3, reasonProject: 4}

// line is one change shown in an e-mail.
type line struct {
	Group string `json:"group"`
	Field string `json:"field"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
}

type itemComment struct {
	ID     int64  `json:"id"`
	Body   string `json:"body"`
	Group  string `json:"group,omitempty"`
	RoleID int64  `json:"role,omitempty"`
	Edited bool   `json:"edited,omitempty"`
}

// item is one event waiting in a batch, as one recipient will see it.
type item struct {
	Kind      events.Kind  `json:"kind"`
	At        time.Time    `json:"at"`
	ActorID   int64        `json:"actor"`
	Lines     []line       `json:"lines,omitempty"`
	Comment   *itemComment `json:"comment,omitempty"`
	Mentioned bool         `json:"mentioned,omitempty"`
	Reason    string       `json:"reason"`
	Project   string       `json:"project,omitempty"` // the project's key, for reasonProject
	// The issue as it was, for when it is gone by the time of writing, and
	// who could see it then.
	Key     string     `json:"key"`
	Summary string     `json:"summary"`
	Access  perm.Issue `json:"access"`
}

// Fields that never appear in e-mails: their values mean nothing to readers
// or may reveal what they cannot see (as in the issue's history).
var hiddenFields = map[string]bool{"security": true, "rank": true, "worklogid": true, "timespent": true,
	"timeestimate": true, "timeoriginalestimate": true}

// groupOf files a history field under an event group.
func groupOf(field string) string {
	switch strings.ToLower(field) {
	case "status", "resolution":
		return GroupStatus
	case "assignee":
		return GroupAssigned
	case "attachment":
		return GroupAttachments
	}
	return GroupUpdated
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// linesOf describes an event's changes.
func linesOf(e *events.Event) []line {
	switch e.Kind {
	case events.IssueCreated:
		return []line{{Group: GroupCreated}}
	case events.IssueDeleted:
		return []line{{Group: GroupDeleted}}
	case events.CommentCreated, events.CommentUpdated:
		return []line{{Group: GroupCommented}}
	}
	var out []line
	for _, c := range e.Changes {
		if hiddenFields[strings.ToLower(c.Field)] {
			continue
		}
		from, to := deref(c.FromString), deref(c.ToString)
		if c.FromString == nil {
			from = deref(c.From)
		}
		if c.ToString == nil {
			to = deref(c.To)
		}
		out = append(out, line{Group: groupOf(c.Field), Field: c.Field, From: from, To: to})
	}
	return out
}

var mentionPattern = regexp.MustCompile(`\[~([^\]\s]{1,255})\]`)

// mentionNames lists the names mentioned in text ([~name]) that are not
// already mentioned in before.
func mentionNames(text, before string) []string {
	old := map[string]bool{}
	for _, m := range mentionPattern.FindAllStringSubmatch(before, -1) {
		old[m[1]] = true
	}
	var out []string
	for _, m := range mentionPattern.FindAllStringSubmatch(text, -1) {
		if !old[m[1]] && !slices.Contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}

// mentioned returns the users newly mentioned by an event: in a new or
// edited comment, or in the description of a new or changed issue.
func (s *Service) mentioned(ctx context.Context, e *events.Event) (map[int64]bool, error) {
	var names []string
	switch {
	case e.Comment != nil && (e.Kind == events.CommentCreated || e.Kind == events.CommentUpdated):
		names = mentionNames(e.Comment.Body, e.Comment.OldBody)
	case e.Kind == events.IssueCreated && e.Issue.Description != nil:
		names = mentionNames(*e.Issue.Description, "")
	case e.Kind == events.IssueUpdated:
		for _, c := range e.Changes {
			if strings.EqualFold(c.Field, "description") {
				names = mentionNames(deref(c.ToString), deref(c.FromString))
			}
		}
	}
	out := map[int64]bool{}
	if len(names) == 0 {
		return out, nil
	}
	var args []any
	for _, n := range names {
		args = append(args, strings.ToLower(n))
	}
	for _, n := range names {
		args = append(args, n)
	}
	err := scan(ctx, s.db, `SELECT id FROM users WHERE username IN (`+marks(len(names))+`) OR user_key IN (`+
		marks(len(names))+`)`, func(rows *sql.Rows) error {
		var id int64
		err := rows.Scan(&id)
		out[id] = true
		return err
	}, args...)
	return out, err
}

// Handle works out who hears about an event and files it in their batches.
func (s *Service) Handle(ctx context.Context, e *events.Event) error {
	iss := e.Issue
	if iss == nil {
		return nil // deleted before we got to it
	}
	ps, err := s.ProjectSettings(ctx, iss.ProjectID)
	if err != nil || !ps.Enabled {
		return err
	}
	lines := linesOf(e)
	if len(lines) == 0 {
		return nil
	}
	var groups []string
	for _, l := range lines {
		if !slices.Contains(groups, l.Group) {
			groups = append(groups, l.Group)
		}
	}
	mentioned, err := s.mentioned(ctx, e)
	if err != nil {
		return err
	}

	// Candidates and why they would hear about it.
	reasons := map[int64]string{}
	add := func(id int64, reason string) {
		if id == 0 {
			return
		}
		if r, ok := reasons[id]; !ok || reasonRank[reason] < reasonRank[r] {
			reasons[id] = reason
		}
	}
	for _, id := range iss.Watchers {
		add(id, reasonWatcher)
	}
	if iss.Reporter != nil {
		add(iss.Reporter.ID, reasonReporter)
	}
	if iss.Assignee != nil {
		add(iss.Assignee.ID, reasonAssignee)
	}
	for id := range mentioned {
		add(id, reasonMentioned)
	}
	extra, err := s.extraMembers(ctx, iss.ProjectID, ps.Extra, groups)
	if err != nil {
		return err
	}
	for id := range extra {
		add(id, reasonProject)
	}
	if len(reasons) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(reasons))
	for id := range reasons {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	users, err := events.LoadUsers(ctx, s.db, ids...)
	if err != nil {
		return err
	}
	prefs, err := s.prefsOf(ctx, ids...)
	if err != nil {
		return err
	}

	access := iss.Access
	if e.Kind == events.IssueDeleted {
		access = e.Access
	}
	var comment *itemComment
	if e.Comment != nil {
		comment = &itemComment{ID: e.Comment.ID, Body: e.Comment.Body, Group: e.Comment.Group, RoleID: e.Comment.RoleID,
			Edited: e.Kind == events.CommentUpdated}
	}
	for _, id := range ids {
		u, p := users[id], prefs[id]
		if u == nil || !u.Active || !mail.ValidAddress(u.Email) || p.Delivery == None {
			continue
		}
		if id == e.ActorID && !p.OwnChanges {
			continue
		}
		sub, err := s.perms.Subject(ctx, id, false)
		if err != nil {
			return err
		}
		if !sub.CanSee(access) {
			continue
		}
		if comment != nil && !sub.SeesRestricted(iss.ProjectID, comment.Group, comment.RoleID) {
			continue
		}
		var keep []line
		for _, l := range lines {
			if p.wants(l.Group) {
				keep = append(keep, l)
			}
		}
		isMentioned := mentioned[id] && p.wants(GroupMentioned)
		if len(keep) == 0 && !isMentioned {
			continue
		}
		it := item{Kind: e.Kind, At: e.At, ActorID: e.ActorID, Lines: keep, Mentioned: isMentioned, Reason: reasons[id],
			Project: iss.ProjectKey, Key: iss.Key, Summary: iss.Summary, Access: access}
		if comment != nil && (slices.ContainsFunc(keep, func(l line) bool { return l.Group == GroupCommented }) || isMentioned) {
			it.Comment = comment
		}
		if err := s.enqueue(ctx, id, p, iss.ID, it); err != nil {
			return err
		}
	}
	return nil
}

// enqueue files an item in the user's open batch for the issue (or their
// digest), creating it if needed. A batch is sent Window after its last
// change, but no later than MaxWait after its first.
func (s *Service) enqueue(ctx context.Context, userID int64, p Prefs, issueID int64, it item) error {
	data, err := json.Marshal(it)
	if err != nil {
		return err
	}
	now := s.Now().UTC()
	digest := p.Delivery == Digest
	batchIssue := issueID
	sendAt := now.Add(s.Window)
	if digest {
		batchIssue = 0
		sendAt = nextAt(now, s.zone(), p.DigestHour, nil)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO notification_batches (user_id, digest, issue_id, first_at, send_at)
		VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE
		send_at = IF(digest, send_at, LEAST(VALUES(send_at), first_at + INTERVAL ? MICROSECOND))`,
		userID, digest, batchIssue, now, sendAt, s.MaxWait.Microseconds()); err != nil {
		return err
	}
	var batch int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM notification_batches WHERE user_id = ? AND digest = ? AND issue_id = ?`,
		userID, digest, batchIssue).Scan(&batch); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO notification_items (batch_id, issue_id, data, created_at) VALUES (?, ?, ?, ?)`,
		batch, issueID, data, now); err != nil {
		return err
	}
	return tx.Commit()
}

// Pending counts the batches waiting to be written.
func (s *Service) Pending(ctx context.Context) (immediate, digests int, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(NOT digest), 0), COALESCE(SUM(digest), 0)
		FROM notification_batches`).Scan(&immediate, &digests)
	return immediate, digests, err
}

// Run writes due batches, subscriptions and reminders until ctx ends.
func (s *Service) Run(ctx context.Context) {
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	var scheduled time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if err := s.FlushDue(ctx); err != nil && ctx.Err() == nil {
			s.log.Error("writing notifications failed", "err", err)
		}
		if time.Since(scheduled) >= time.Minute {
			scheduled = time.Now()
			if err := s.RunSchedules(ctx); err != nil && ctx.Err() == nil {
				s.log.Error("scheduled e-mails failed", "err", err)
			}
		}
	}
}
