package webhook

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"solstice/internal/events"
)

// jiraTime is Jira's date-time format in REST and webhooks.
const jiraTime = "2006-01-02T15:04:05.000-0700"

func jtime(t time.Time) string { return t.Format(jiraTime) }

func jtimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return jtime(*t)
}

// builder renders the parts of a payload, with "self" links when the
// public address is known.
type builder struct {
	base  string
	users map[int64]*events.User
}

func (b *builder) self(path string) string {
	if b.base == "" {
		return ""
	}
	return b.base + path
}

func (b *builder) put(m map[string]any, key, path string) {
	if s := b.self(path); s != "" {
		m[key] = s
	}
}

func (b *builder) user(u *events.User) any {
	if u == nil {
		return nil
	}
	m := map[string]any{"name": u.Username, "key": u.Key, "displayName": u.Name, "emailAddress": u.Email,
		"active": u.Active}
	b.put(m, "self", "/rest/api/2/user?username="+url.QueryEscape(u.Username))
	return m
}

func (b *builder) userID(id int64) any { return b.user(b.users[id]) }

func named(n *events.Named) any {
	if n == nil {
		return nil
	}
	return map[string]any{"id": n.ID, "name": n.Name}
}

func namedList(list []events.Named) []any {
	out := []any{}
	for _, n := range list {
		out = append(out, map[string]any{"id": n.ID, "name": n.Name})
	}
	return out
}

// statusCategory is Jira's status category for Solstice's.
func statusCategory(cat string) map[string]any {
	switch cat {
	case "doing":
		return map[string]any{"id": 4, "key": "indeterminate", "name": "In Progress", "colorName": "yellow"}
	case "done":
		return map[string]any{"id": 3, "key": "done", "name": "Done", "colorName": "green"}
	}
	return map[string]any{"id": 2, "key": "new", "name": "To Do", "colorName": "blue-gray"}
}

func intPtr(n *int64) any {
	if n == nil {
		return nil
	}
	return *n
}

func strPtr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// issue renders an issue as Jira's REST API does.
func (b *builder) issue(iss *events.Issue) map[string]any {
	id := strconv.FormatInt(iss.ID, 10)
	project := map[string]any{"id": strconv.FormatInt(iss.ProjectID, 10), "key": iss.ProjectKey, "name": iss.ProjectName}
	b.put(project, "self", "/rest/api/2/project/"+strconv.FormatInt(iss.ProjectID, 10))
	labels := iss.Labels
	if labels == nil {
		labels = []string{}
	}
	f := map[string]any{
		"summary":     iss.Summary,
		"description": strPtr(iss.Description),
		"environment": strPtr(iss.Environment),
		"issuetype":   map[string]any{"id": iss.Type.ID, "name": iss.Type.Name, "subtask": iss.Type.Subtask},
		"project":     project,
		"status": map[string]any{"id": iss.Status.ID, "name": iss.Status.Name,
			"statusCategory": statusCategory(iss.Status.Category)},
		"priority":             named(iss.Priority),
		"resolution":           named(iss.Resolution),
		"assignee":             b.user(iss.Assignee),
		"reporter":             b.user(iss.Reporter),
		"creator":              b.user(iss.Creator),
		"created":              jtime(iss.Created),
		"updated":              jtime(iss.Updated),
		"resolutiondate":       jtimePtr(iss.Resolved),
		"duedate":              strPtr(iss.Due),
		"labels":               labels,
		"components":           namedList(iss.Components),
		"fixVersions":          namedList(iss.FixVersions),
		"versions":             namedList(iss.Versions),
		"security":             named(iss.Security),
		"timeoriginalestimate": intPtr(iss.Original),
		"timeestimate":         intPtr(iss.Remaining),
		"timespent":            intPtr(iss.Spent),
		"watches":              map[string]any{"watchCount": len(iss.Watchers), "isWatching": false},
	}
	if iss.Parent != nil {
		f["parent"] = map[string]any{"id": strconv.FormatInt(iss.Parent.ID, 10), "key": iss.Parent.Key}
	}
	out := map[string]any{"id": id, "key": iss.Key, "fields": f}
	b.put(out, "self", "/rest/api/2/issue/"+id)
	return out
}

func (b *builder) comment(ctx context.Context, db *sql.DB, issueID int64, c *events.Comment) map[string]any {
	id := strconv.FormatInt(c.ID, 10)
	out := map[string]any{"id": id, "body": c.Body, "author": b.userID(c.AuthorID), "created": jtime(c.Created),
		"updated": jtime(c.Updated)}
	if c.UpdatedBy != 0 {
		out["updateAuthor"] = b.userID(c.UpdatedBy)
	} else {
		out["updateAuthor"] = b.userID(c.AuthorID)
	}
	b.put(out, "self", "/rest/api/2/issue/"+strconv.FormatInt(issueID, 10)+"/comment/"+id)
	switch {
	case c.Group != "":
		out["visibility"] = map[string]any{"type": "group", "value": c.Group}
	case c.RoleID != 0:
		var name string
		db.QueryRowContext(ctx, `SELECT name FROM project_roles WHERE id = ?`, c.RoleID).Scan(&name)
		out["visibility"] = map[string]any{"type": "role", "value": name}
	}
	return out
}

// jiraDuration writes seconds as Jira does: "1d 2h 30m" (8-hour days).
func jiraDuration(seconds int64) string {
	var parts []string
	for _, u := range []struct {
		n    int64
		unit string
	}{{8 * 3600 * 5, "w"}, {8 * 3600, "d"}, {3600, "h"}, {60, "m"}} {
		if seconds >= u.n {
			parts = append(parts, fmt.Sprintf("%d%s", seconds/u.n, u.unit))
			seconds %= u.n
		}
	}
	if len(parts) == 0 {
		return "0m"
	}
	return strings.Join(parts, " ")
}

func (b *builder) worklog(issueID int64, w *events.Worklog) map[string]any {
	id := strconv.FormatInt(w.ID, 10)
	out := map[string]any{"id": id, "issueId": strconv.FormatInt(issueID, 10), "author": b.userID(w.AuthorID),
		"updateAuthor": b.userID(w.AuthorID), "comment": w.Body, "created": jtime(w.Created),
		"updated": jtime(w.Created), "started": jtime(w.Started), "timeSpent": jiraDuration(w.Seconds),
		"timeSpentSeconds": w.Seconds}
	b.put(out, "self", "/rest/api/2/issue/"+strconv.FormatInt(issueID, 10)+"/worklog/"+id)
	return out
}

func (b *builder) sprint(sp *events.Sprint) map[string]any {
	out := map[string]any{"id": sp.ID, "state": sp.State, "name": sp.Name, "originBoardId": sp.BoardID,
		"goal": strPtr(sp.Goal), "startDate": jtimePtr(sp.Start), "endDate": jtimePtr(sp.End),
		"completeDate": jtimePtr(sp.Completed)}
	b.put(out, "self", "/rest/agile/1.0/sprint/"+strconv.FormatInt(sp.ID, 10))
	return out
}

func (b *builder) board(bd *events.Board) map[string]any {
	out := map[string]any{"id": bd.ID, "name": bd.Name, "type": bd.Kind}
	b.put(out, "self", "/rest/agile/1.0/board/"+strconv.FormatInt(bd.ID, 10))
	return out
}

// issueEventType is Jira's issue_event_type_name for an event.
func issueEventType(e *events.Event) string {
	switch e.Kind {
	case events.IssueCreated:
		return "issue_created"
	case events.IssueDeleted:
		return "issue_deleted"
	case events.CommentCreated:
		return "issue_commented"
	case events.CommentUpdated:
		return "issue_comment_edited"
	case events.CommentDeleted:
		return "issue_comment_deleted"
	case events.WorklogCreated:
		return "issue_work_logged"
	}
	assigned, status := false, false
	for _, c := range e.Changes {
		switch strings.ToLower(c.Field) {
		case "resolution":
			if c.To != nil && c.From == nil {
				return "issue_resolved"
			}
			if c.From != nil && c.To == nil {
				return "issue_reopened"
			}
		case "status":
			status = true
		case "assignee":
			assigned = true
		}
	}
	switch {
	case status:
		return "issue_generic"
	case assigned && len(e.Changes) == 1:
		return "issue_assigned"
	}
	return "issue_updated"
}

// payload renders an event for one webhook event name.
func (s *Service) payload(ctx context.Context, e *events.Event, name string) (map[string]any, error) {
	ids := []int64{e.ActorID}
	if e.Comment != nil {
		ids = append(ids, e.Comment.AuthorID, e.Comment.UpdatedBy)
	}
	if e.Worklog != nil {
		ids = append(ids, e.Worklog.AuthorID)
	}
	users, err := events.LoadUsers(ctx, s.db, ids...)
	if err != nil {
		return nil, err
	}
	b := &builder{base: strings.TrimSuffix(s.BaseURL(), "/"), users: users}
	p := map[string]any{"timestamp": e.At.UnixMilli(), "webhookEvent": name}
	if e.ActorID != 0 {
		p["user"] = b.userID(e.ActorID)
	}
	if e.Issue != nil {
		p["issue"] = b.issue(e.Issue)
		if name == IssueCreated || name == IssueUpdated || name == IssueDeleted {
			p["issue_event_type_name"] = issueEventType(e)
		}
	}
	if len(e.Changes) > 0 && name == IssueUpdated {
		items := []any{}
		for _, c := range e.Changes {
			items = append(items, map[string]any{"field": c.Field, "fieldtype": "jira", "from": strPtr(c.From),
				"fromString": strPtr(c.FromString), "to": strPtr(c.To), "toString": strPtr(c.ToString)})
		}
		p["changelog"] = map[string]any{"items": items}
	}
	if e.Comment != nil {
		p["comment"] = b.comment(ctx, s.db, e.IssueID, e.Comment)
	}
	if e.Worklog != nil {
		p["worklog"] = b.worklog(e.IssueID, e.Worklog)
	}
	if e.Sprint != nil {
		p["sprint"] = b.sprint(e.Sprint)
	}
	if e.Board != nil {
		p["board"] = b.board(e.Board)
	}
	return p, nil
}

// expandURL fills in Jira's URL variables (${issue.key}…).
func expandURL(raw string, e *events.Event, actor *events.User) string {
	return urlVariables.ReplaceAllStringFunc(raw, func(v string) string {
		var val string
		switch v[2 : len(v)-1] {
		case "issue.id":
			val = strconv.FormatInt(e.IssueID, 10)
		case "issue.key":
			val = e.Key
		case "project.id":
			if e.Issue != nil {
				val = strconv.FormatInt(e.Issue.ProjectID, 10)
			}
		case "project.key":
			if e.Issue != nil {
				val = e.Issue.ProjectKey
			}
		case "sprint.id":
			if e.Sprint != nil {
				val = strconv.FormatInt(e.Sprint.ID, 10)
			}
		case "board.id":
			if e.Board != nil {
				val = strconv.FormatInt(e.Board.ID, 10)
			} else if e.Sprint != nil {
				val = strconv.FormatInt(e.Sprint.BoardID, 10)
			}
		case "modifiedUser.name":
			if actor != nil {
				val = actor.Username
			}
		case "modifiedUser.key":
			if actor != nil {
				val = actor.Key
			}
		default:
			return v
		}
		return url.PathEscape(val)
	})
}
