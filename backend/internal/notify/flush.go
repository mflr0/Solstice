package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/mail"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// FlushDue writes the batches whose time has come into the outbox.
func (s *Service) FlushDue(ctx context.Context) error {
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

// recipientOf loads an active user with an address, or nil.
func (s *Service) recipientOf(ctx context.Context, userID int64) (*recipient, error) {
	users, err := events.LoadUsers(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}
	u := users[userID]
	if u == nil || !u.Active || !mail.ValidAddress(u.Email) {
		return nil, nil
	}
	sub, err := s.perms.Subject(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	locale := u.Locale
	if !i18n.Supported(locale) {
		locale = s.DefaultLocale()
	}
	if !i18n.Supported(locale) {
		locale = i18n.Default
	}
	return &recipient{user: u, locale: locale, sub: sub}, nil
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

// keyInText finds issue keys in history text ("This issue blocks WEB-12").
var keyInText = regexp.MustCompile(`\b[A-Z][A-Z0-9_]+-\d+\b`)

// relationFields name other issues; they are shown only when the recipient
// sees every issue they name.
var relationFields = map[string]bool{"link": true, "parent": true, "epic link": true, "epic child": true}

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
	loc := rc.locale
	p := &page{Lang: loc}

	// Issues in order of first appearance.
	var order []int64
	byIssue := map[int64][]storedItem{}
	for _, it := range items {
		if _, ok := byIssue[it.issueID]; !ok {
			order = append(order, it.issueID)
		}
		byIssue[it.issueID] = append(byIssue[it.issueID], it)
	}
	seen := map[string]bool{}
	canSeeKey := func(key string) bool {
		if v, ok := seen[key]; ok {
			return v
		}
		var access perm.Issue
		var level sql.NullInt64
		var reporter, assignee sql.NullInt64
		project, num, _ := strings.Cut(key, "-")
		err := s.db.QueryRowContext(ctx, `SELECT i.project_id, i.reporter_id, i.assignee_id, i.security_level_id
			FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.pkey = ? AND i.num = ?`, project, num).
			Scan(&access.ProjectID, &reporter, &assignee, &level)
		access.ReporterID, access.AssigneeID, access.SecurityLevel = reporter.Int64, assignee.Int64, level.Int64
		v := err == nil && rc.sub.CanSee(access)
		seen[key] = v
		return v
	}

	var reasons []string
	var project string
	var threadIssue int64
	created := false
	actors := map[int64]*events.User{}
	for _, id := range order {
		group := byIssue[id]
		cur, err := events.LoadIssue(ctx, s.db, id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		var kept []storedItem
		for _, it := range group {
			// Still allowed to see the issue? A deleted issue is judged as it
			// was; the comment, by its restriction.
			access := it.Access
			if cur != nil {
				access = cur.Access
			}
			if !rc.sub.CanSee(access) {
				continue
			}
			if it.Comment != nil && !rc.sub.SeesRestricted(access.ProjectID, it.Comment.Group, it.Comment.RoleID) {
				continue
			}
			kept = append(kept, it)
		}
		if len(kept) == 0 {
			continue
		}
		var actorIDs []int64
		for _, it := range kept {
			actorIDs = append(actorIDs, it.ActorID)
		}
		users, err := events.LoadUsers(ctx, s.db, actorIDs...)
		if err != nil {
			return nil, err
		}
		for k, v := range users {
			actors[k] = v
		}
		sec := section{Key: kept[0].Key, Summary: kept[0].Summary}
		if cur != nil {
			sec.Key, sec.Summary = cur.Key, cur.Summary
			sec.URL = s.issueURL(cur.Key)
			sec.Link = i18n.M(i18n.NotifyOpenIssue).In(loc)
			sec.Meta = issueMeta(loc, cur)
		}
		for _, it := range kept {
			if it.Kind == events.IssueCreated {
				created = true
			}
			en := entry{Headline: headline(loc, it, actorName(loc, actors[it.ActorID]), cur, userID),
				When: formatTime(loc, it.At.In(s.zone()))}
			for _, l := range it.Lines {
				if l.Field == "" {
					continue
				}
				f := strings.ToLower(l.Field)
				if relationFields[f] {
					ok := true
					for _, k := range keyInText.FindAllString(l.From+" "+l.To, -1) {
						ok = ok && canSeeKey(k)
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
				continue // only lines this recipient may not see
			}
			sec.Entries = append(sec.Entries, en)
			reasons = append(reasons, it.Reason)
			project = it.Project
		}
		if len(sec.Entries) == 0 {
			continue
		}
		p.Sections = append(p.Sections, sec)
		threadIssue = id
	}
	if len(p.Sections) == 0 {
		return nil, nil
	}

	msg := &mail.Message{UserID: userID, To: rc.user.Email, ToName: rc.user.Name, MessageID: mail.NewMessageID(cfg.Domain())}
	if digest {
		msg.Kind = "digest"
		p.Heading = i18n.M(i18n.NotifyDigestSubject, formatDate(loc, s.Now().In(s.zone()))).In(loc)
		msg.Subject = p.Heading
		s.footer(p, i18n.M(i18n.NotifyWhyDigest).In(loc))
	} else {
		msg.Kind = "notification"
		msg.Subject = "[" + p.Sections[0].Key + "] " + p.Sections[0].Summary
		// One thread per issue: the creation e-mail starts it, the others
		// answer it.
		root := "<issue." + strconv.FormatInt(threadIssue, 10) + "@" + cfg.Domain() + ">"
		msg.InReplyTo = root
		if created {
			msg.MessageID, msg.InReplyTo = root, ""
		}
		s.footer(p, why(loc, strongest(reasons), project))
	}
	p.Title = msg.Subject
	if msg.HTML, err = p.html(); err != nil {
		return nil, err
	}
	msg.Text = p.text()
	return msg, nil
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
	if !i18n.Supported(locale) {
		locale = i18n.Default
	}
	p := &page{Lang: locale, Title: i18n.M(i18n.MailTestSubject).In(locale)}
	p.Heading = p.Title
	p.Intro = []string{i18n.M(i18n.MailTestBody).In(locale),
		i18n.M(i18n.MailTestSettings, cfg.Host, cfg.From).In(locale)}
	p.Why = i18n.M(i18n.MailTestWhy).In(locale)
	html, _ := p.html()
	return mail.Message{Kind: "test", To: to, Subject: p.Title, Text: p.text(), HTML: html,
		MessageID: mail.NewMessageID(cfg.Domain())}
}
