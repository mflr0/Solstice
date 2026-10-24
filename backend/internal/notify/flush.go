package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/mail"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// FlushDue writes the batches whose time has come into the outbox.
func (s *Service) FlushDue(ctx context.Context) error {
	if s.Paused != nil && s.Paused() {
		return nil
	}
	var ids []int64
	if err := sqlq.Each(ctx, s.db, `SELECT id FROM notification_batches WHERE send_at <= ? ORDER BY send_at, id LIMIT 200`,
		func(rows *sql.Rows) error {
			var id int64
			err := rows.Scan(&id)
			ids = append(ids, id)
			return err
		}, s.Now().UTC()); err != nil {
		return err
	}
	wrote := false
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		ok, err := s.flushBatch(ctx, id)
		if err != nil {
			return err
		}
		wrote = wrote || ok
	}
	if wrote {
		s.outbox.Wake()
	}
	return nil
}

type storedItem struct {
	issueID int64
	item
}

// flushBatch turns one batch into an e-mail (or nothing, when nothing in it
// may be shown any more) and deletes it, in one transaction.
func (s *Service) flushBatch(ctx context.Context, id int64) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var userID int64
	var digest bool
	err = tx.QueryRowContext(ctx, `SELECT user_id, digest FROM notification_batches WHERE id = ? FOR UPDATE`, id).
		Scan(&userID, &digest)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil // another worker took it
	}
	if err != nil {
		return false, err
	}
	var items []storedItem
	if err := sqlq.Each(ctx, tx, `SELECT issue_id, data FROM notification_items WHERE batch_id = ? ORDER BY id`,
		func(rows *sql.Rows) error {
			var it storedItem
			var data string
			if err := rows.Scan(&it.issueID, &data); err != nil {
				return err
			}
			if err := json.Unmarshal([]byte(data), &it.item); err != nil {
				s.log.Warn("unreadable notification item dropped", "batch", id, "err", err)
				return nil
			}
			items = append(items, it)
			return nil
		}, id); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM notification_batches WHERE id = ?`, id); err != nil {
		return false, err
	}
	msg, err := s.renderBatch(ctx, userID, digest, items)
	if err != nil {
		return false, err
	}
	if msg != nil {
		if err := s.outbox.Enqueue(ctx, tx, *msg); err != nil {
			return false, err
		}
	}
	return msg != nil, tx.Commit()
}

// recipient is who an e-mail is written for.
type recipient struct {
	user   *events.User
	locale string
	sub    *perm.Subject
}

// eligible tells whether an e-mail may be written to u: an active user with
// a usable address.
func eligible(u *events.User) bool {
	return u != nil && u.Active && mail.ValidAddress(u.Email)
}

// recipientOf loads an active user with an address, or nil.
func (s *Service) recipientOf(ctx context.Context, userID int64) (*recipient, error) {
	users, err := events.LoadUsers(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	u := users[userID]
	if !eligible(u) {
		return nil, nil
	}
	sub, err := s.perms.Subject(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	return &recipient{user: u, locale: i18n.Pick(u.Locale, s.DefaultLocale()), sub: sub}, nil
}

func (s *Service) url(path string) string {
	base := strings.TrimSuffix(s.BaseURL(), "/")
	if base == "" {
		return ""
	}
	return base + path
}

func (s *Service) issueURL(key string) string { return s.url("/browse/" + key) }

// footer fills in why the e-mail was sent and where to change it.
func (s *Service) footer(p *page, why string) {
	p.Why = why
	if u := s.url("/settings/notifications"); u != "" {
		p.Prefs, p.PrefsURL = i18n.M(i18n.NotifyManage).In(p.Lang), u
	} else {
		p.Prefs = i18n.M(i18n.NotifyChangeInPreferences).In(p.Lang)
	}
}

// relationFields name other issues; they are shown only when the recipient
// sees every issue they name.
var relationFields = map[string]bool{"link": true, "parent": true, "epic link": true, "epic child": true}

// keyAccess tells whether a recipient sees the issue a key names, asking the
// database once per key.
type keyAccess struct {
	db   *sql.DB
	sub  *perm.Subject
	seen map[string]bool
}

func (k *keyAccess) canSee(ctx context.Context, key string) bool {
	if v, ok := k.seen[key]; ok {
		return v
	}
	var access perm.Issue
	var level sql.NullInt64
	var reporter, assignee sql.NullInt64
	project, num, _ := strings.Cut(key, "-")
	const columns = `SELECT i.project_id, i.reporter_id, i.assignee_id, i.security_level_id FROM issues i `
	err := k.db.QueryRowContext(ctx, columns+`JOIN projects p ON p.id = i.project_id WHERE `+sqlq.ProjectKey("p", "?")+` AND i.num = ?`,
		project, project, num).Scan(&access.ProjectID, &reporter, &assignee, &level)
	if errors.Is(err, sql.ErrNoRows) {
		// The key of an issue moved to another project still names it.
		err = k.db.QueryRowContext(ctx, columns+`JOIN moved_issue_keys k ON k.issue_id = i.id WHERE k.old_key = ?`, key).
			Scan(&access.ProjectID, &reporter, &assignee, &level)
	}
	access.ReporterID, access.AssigneeID, access.SecurityLevel = reporter.Int64, assignee.Int64, level.Int64
	// Text that only looks like a key names no issue to hide.
	v := errors.Is(err, sql.ErrNoRows) || err == nil && k.sub.CanSee(access)
	k.seen[key] = v
	return v
}

// batchFacts is what an e-mail's subject, thread and footer depend on.
type batchFacts struct {
	reasons     []string // why, for each entry shown
	project     string   // the project of the last entry shown
	created     bool     // an issue's creation is among the items
	threadIssue int64    // the last issue shown
	actor       string   // who made the last entry shown
}

// renderBatch writes a batch for its recipient, checking again what they may
// see now. It returns nil when nothing is left to say.
func (s *Service) renderBatch(ctx context.Context, userID int64, digest bool, items []storedItem) (*mail.Message, error) {
	rc, err := s.recipientOf(ctx, userID)
	if err != nil || rc == nil {
		return nil, err
	}
	cfg, err := s.config(ctx)
	if err != nil {
		return nil, err
	}
	p := &page{Lang: rc.locale}
	keys := &keyAccess{db: s.db, sub: rc.sub, seen: map[string]bool{}}
	var facts batchFacts
	order, byIssue := groupByIssue(items)
	for _, id := range order {
		sec, f, err := s.sectionFor(ctx, rc, keys, id, byIssue[id])
		if err != nil {
			return nil, err
		}
		facts.reasons = append(facts.reasons, f.reasons...)
		if len(f.reasons) > 0 {
			facts.project = f.project
		}
		facts.created = facts.created || f.created
		if len(sec.Entries) == 0 {
			continue
		}
		facts.actor = f.actor
		p.Sections = append(p.Sections, sec)
		facts.threadIssue = id
	}
	if len(p.Sections) == 0 {
		return nil, nil
	}
	return s.compose(cfg, rc, digest, p, facts), nil
}

// groupByIssue groups items by issue, the issues in order of first appearance.
func groupByIssue(items []storedItem) ([]int64, map[int64][]storedItem) {
	var order []int64
	byIssue := map[int64][]storedItem{}
	for _, it := range items {
		if _, ok := byIssue[it.issueID]; !ok {
			order = append(order, it.issueID)
		}
		byIssue[it.issueID] = append(byIssue[it.issueID], it)
	}
	return order, byIssue
}

// sectionFor writes what happened to one issue, with the facts of the entries
// it shows. The section has no entries when the recipient may see none.
func (s *Service) sectionFor(ctx context.Context, rc *recipient, keys *keyAccess, id int64, group []storedItem) (section, batchFacts, error) {
	var facts batchFacts
	cur, err := events.LoadIssue(ctx, s.db, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return section{}, facts, err
	}
	kept, err := s.visibleItems(ctx, rc.sub, cur, group)
	if err != nil || len(kept) == 0 {
		return section{}, facts, err
	}
	var actorIDs []int64
	for _, it := range kept {
		actorIDs = append(actorIDs, it.ActorID)
	}
	actors, err := events.LoadUsers(ctx, s.db, actorIDs...)
	if err != nil {
		return section{}, facts, err
	}
	loc := rc.locale
	sec := section{Key: kept[0].Key, Summary: kept[0].Summary}
	if cur != nil {
		sec.Key, sec.Summary = cur.Key, cur.Summary
		sec.URL = s.issueURL(cur.Key)
		sec.Link = i18n.M(i18n.NotifyOpenIssue).In(loc)
		sec.Meta = issueMeta(loc, cur)
	}
	for _, it := range kept {
		if it.Kind == events.IssueCreated {
			facts.created = true
		}
		en, ok := s.entryFor(ctx, rc, keys, it, actors[it.ActorID], cur)
		if !ok {
			continue
		}
		sec.Entries = append(sec.Entries, en)
		facts.reasons = append(facts.reasons, it.Reason)
		facts.project = it.Project
		facts.actor = actorName(loc, actors[it.ActorID])
	}
	return sec, facts, nil
}

// visibleItems keeps the items the recipient may still see. A deleted issue
// (cur nil) is judged as it was; a comment, by its restriction as it is now.
func (s *Service) visibleItems(ctx context.Context, sub *perm.Subject, cur *events.Issue, items []storedItem) ([]storedItem, error) {
	var kept []storedItem
	for _, it := range items {
		access := it.Access
		if cur != nil {
			access = cur.Access
		}
		if !sub.CanSee(access) {
			continue
		}
		if it.Comment != nil {
			// A move gives a comment restricted to a role another restriction.
			group, role, project := it.Comment.Group, it.Comment.RoleID, it.Access.ProjectID
			var g sql.NullString
			var r sql.NullInt64
			err := s.db.QueryRowContext(ctx, `SELECT visibility_group, visibility_role_id FROM comments WHERE id = ?`,
				it.Comment.ID).Scan(&g, &r)
			switch {
			case err == nil:
				group, role, project = g.String, r.Int64, access.ProjectID
			case !errors.Is(err, sql.ErrNoRows):
				return nil, err
			}
			if !sub.SeesRestricted(project, group, role) {
				continue
			}
		}
		kept = append(kept, it)
	}
	return kept, nil
}

// entryFor writes one item for the recipient. It reports false when nothing
// of an update is left once the lines they may not see are dropped.
func (s *Service) entryFor(ctx context.Context, rc *recipient, keys *keyAccess, it storedItem, actor *events.User, cur *events.Issue) (entry, bool) {
	loc := rc.locale
	en := entry{Headline: headline(loc, it, actorName(loc, actor), cur, rc.user.ID),
		When: formatTime(loc, it.At.In(s.zone()))}
	for _, l := range it.Lines {
		if l.Field == "" {
			continue
		}
		f := strings.ToLower(l.Field)
		if relationFields[f] {
			ok := true
			for _, k := range issue.KeysIn(l.From + " " + l.To) {
				ok = ok && keys.canSee(ctx, k)
			}
			if !ok {
				continue
			}
		}
		row := changeRow{Field: fieldLabel(loc, l.Field), From: l.From, To: l.To}
		switch f {
		case "description", "environment":
			row.From, row.To = "", excerpt(l.To, 600)
		case "duedate":
			row.From, row.To = formatDay(loc, l.From), formatDay(loc, l.To)
		}
		if row.To == "" {
			row.To = i18n.M(i18n.NotifyEmptyValue).In(loc)
		}
		en.Changes = append(en.Changes, row)
	}
	if it.Comment != nil {
		en.Text = excerpt(it.Comment.Body, 4000)
		if it.Comment.Group != "" || it.Comment.RoleID != 0 {
			en.Note = i18n.M(i18n.NotifyRestrictedComment).In(loc)
		}
	}
	if it.Kind == events.IssueCreated && cur != nil && cur.Description != nil {
		en.Text = excerpt(*cur.Description, 1500)
	}
	if it.Kind == events.IssueUpdated && len(en.Changes) == 0 && !it.Mentioned {
		return entry{}, false // only lines this recipient may not see
	}
	return en, true
}

// compose turns the page into the e-mail: subject, thread and footer.
func (s *Service) compose(cfg mail.Config, rc *recipient, digest bool, p *page, facts batchFacts) *mail.Message {
	loc := rc.locale
	msg := &mail.Message{UserID: rc.user.ID, To: rc.user.Email, ToName: rc.user.Name, MessageID: mail.NewMessageID(cfg.Domain())}
	if digest {
		msg.Kind = "digest"
		p.Heading = i18n.M(i18n.NotifyDigestSubject, formatDate(loc, s.Now().In(s.zone()))).In(loc)
		msg.Subject = p.Heading
		s.footer(p, i18n.M(i18n.NotifyWhyDigest).In(loc))
	} else {
		msg.Kind = "notification"
		if len(p.Sections) > 1 {
			// A bulk change: one e-mail for several issues, in no issue's
			// thread.
			p.Heading = i18n.M(i18n.NotifyBulkSubject, facts.actor, len(p.Sections)).In(loc)
			msg.Subject = p.Heading
		} else {
			msg.Subject = "[" + p.Sections[0].Key + "] " + p.Sections[0].Summary
			// One thread per issue: the creation e-mail starts it, the
			// others answer it.
			root := "<issue." + strconv.FormatInt(facts.threadIssue, 10) + "@" + cfg.Domain() + ">"
			msg.InReplyTo = root
			if facts.created {
				msg.MessageID, msg.InReplyTo = root, ""
			}
		}
		s.footer(p, why(loc, strongest(facts.reasons), facts.project))
	}
	p.Title = msg.Subject
	msg.HTML = p.html()
	msg.Text = p.text()
	return msg
}

func strongest(reasons []string) string {
	best := reasonProject
	for _, r := range reasons {
		if reasonRank[r] < reasonRank[best] {
			best = r
		}
	}
	return best
}

// why explains in the footer why the e-mail came.
func why(loc, reason, project string) string {
	switch reason {
	case reasonAssignee:
		return i18n.M(i18n.NotifyWhyAssignee).In(loc)
	case reasonReporter:
		return i18n.M(i18n.NotifyWhyReporter).In(loc)
	case reasonMentioned:
		return i18n.M(i18n.NotifyWhyMention).In(loc)
	case reasonWatcher:
		return i18n.M(i18n.NotifyWhyWatcher).In(loc)
	}
	return i18n.M(i18n.NotifyWhyProject, project).In(loc)
}

func actorName(loc string, u *events.User) string {
	if u == nil {
		return i18n.M(i18n.NotifySomeone).In(loc)
	}
	return u.Name
}

// issueMeta is the line under an issue's title: type, status, priority,
// assignee.
func issueMeta(loc string, iss *events.Issue) string {
	parts := []string{iss.Type.Name, iss.Status.Name}
	if iss.Priority != nil {
		parts = append(parts, iss.Priority.Name)
	}
	if iss.Assignee != nil {
		parts = append(parts, i18n.M(i18n.NotifyAssignedTo, iss.Assignee.Name).In(loc))
	} else {
		parts = append(parts, i18n.M(i18n.NotifyUnassigned).In(loc))
	}
	return strings.Join(parts, " · ")
}

// headline says what happened, from the recipient's point of view.
func headline(loc string, it storedItem, actor string, cur *events.Issue, me int64) string {
	switch it.Kind {
	case events.IssueCreated:
		if it.Mentioned {
			return i18n.M(i18n.NotifyMentionDescription, actor).In(loc)
		}
		return i18n.M(i18n.NotifyCreated, actor).In(loc)
	case events.IssueDeleted:
		return i18n.M(i18n.NotifyDeleted, actor).In(loc)
	case events.CommentCreated, events.CommentUpdated:
		if it.Mentioned {
			return i18n.M(i18n.NotifyMentionComment, actor).In(loc)
		}
		if it.Kind == events.CommentUpdated {
			return i18n.M(i18n.NotifyCommentEdited, actor).In(loc)
		}
		return i18n.M(i18n.NotifyCommented, actor).In(loc)
	}
	var resolved, reopened, assignedToMe bool
	for _, l := range it.Lines {
		switch strings.ToLower(l.Field) {
		case "resolution":
			resolved, reopened = l.From == "" && l.To != "", l.From != "" && l.To == ""
		case "assignee":
			assignedToMe = cur != nil && cur.Assignee != nil && cur.Assignee.ID == me && l.To == cur.Assignee.Name
		}
	}
	switch {
	case it.Mentioned:
		return i18n.M(i18n.NotifyMentionDescription, actor).In(loc)
	case assignedToMe:
		return i18n.M(i18n.NotifyAssignedToYou, actor).In(loc)
	case resolved:
		return i18n.M(i18n.NotifyResolved, actor).In(loc)
	case reopened:
		return i18n.M(i18n.NotifyReopened, actor).In(loc)
	}
	return i18n.M(i18n.NotifyUpdated, actor).In(loc)
}

// TestMessage is the administrator's test e-mail.
func TestMessage(cfg mail.Config, to, locale string) mail.Message {
	locale = i18n.Pick(locale)
	p := &page{Lang: locale, Title: i18n.M(i18n.MailTestSubject).In(locale)}
	p.Heading = p.Title
	p.Intro = []string{i18n.M(i18n.MailTestBody).In(locale),
		i18n.M(i18n.MailTestSettings, cfg.Host, cfg.From).In(locale)}
	p.Why = i18n.M(i18n.MailTestWhy).In(locale)
	return mail.Message{Kind: "test", To: to, Subject: p.Title, Text: p.text(), HTML: p.html(),
		MessageID: mail.NewMessageID(cfg.Domain())}
}
