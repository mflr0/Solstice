package devseed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"solstice/internal/issue"
)

// demoIssue is a demo issue: who reports it, who works on it, how far it got.
type demoIssue struct {
	reporter, assignee string
	summary            string
	status             issue.StatusCategory // where it ends; "" leaves it in the first status
	comment            string               // by the assignee
}

var issues = []demoIssue{
	{"alice", "alice", "Sign-in page: remember the last username", issue.Doing, "Working on it, the cookie is set."},
	{"alice", "", "Board columns overflow on small screens", "", ""},
	{"marie", "marie", "Traduire les libellés du tableau de bord", issue.Done, "C'est fait, à relire."},
	{"marie", "hans", "Export CSV : les dates sont en UTC", "", ""},
	{"hans", "hans", "Suche nach Umlauten findet nichts", issue.Doing, "Die Sortierung der Datenbank ist schuld."},
	{"hans", "giulia", "Attachment preview for PDF files", "", ""},
	{"giulia", "giulia", "Notifiche duplicate per i commenti", issue.Done, "Risolto con il raggruppamento."},
	{"giulia", "", "Keyboard shortcut to assign to me", "", ""},
	{"priya", "priya", "हिंदी में तारीख़ का प्रारूप", issue.Doing, "अंक अभी अंग्रेज़ी में दिखते हैं।"},
	{"priya", "tom", "Due dates in the past are not highlighted", "", ""},
	{"tom", "tom", "Upgrade the release checklist", issue.Done, "Checklist updated in the wiki."},
	{"tom", "alice", "Slow search on projects with many labels", "", ""},
}

// ensureIssues creates the demo issues the demo project lacks (by summary),
// each by its reporter. It returns how many it created.
func ensureIssues(ctx context.Context, conn *sql.DB, base string, admin *client, ids map[string]int64) (int, error) {
	var p struct {
		IssueTypes []struct {
			ID      string
			Subtask bool
		}
	}
	if err := admin.call("GET", "/api/projects/"+Project, nil, &p); err != nil {
		return 0, err
	}
	var typ string
	for _, t := range p.IssueTypes {
		if !t.Subtask {
			typ = t.ID
			break
		}
	}
	if typ == "" {
		return 0, errors.New("the project has no standard issue type")
	}
	clients := map[string]*client{}
	as := func(username string) (*client, error) {
		if c, ok := clients[username]; ok {
			return c, nil
		}
		c, err := login(ctx, base, username, UserPassword)
		clients[username] = c
		return c, err
	}
	created := 0
	for _, is := range issues {
		var exists int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues i JOIN projects p ON p.id = i.project_id
			WHERE p.pkey = ? AND i.summary = ?`, Project, is.summary).Scan(&exists); err != nil {
			return 0, err
		}
		if exists > 0 {
			continue
		}
		reporter, err := as(is.reporter)
		if err != nil {
			return 0, err
		}
		fields := map[string]any{"project": Project, "type": typ, "summary": is.summary}
		if is.assignee != "" {
			fields["assignee"] = ids[is.assignee]
		}
		var made struct{ Key string }
		if err := reporter.call("POST", "/api/issues", fields, &made); err != nil {
			return 0, fmt.Errorf("%q: %w", is.summary, err)
		}
		if is.status != "" {
			if err := moveTo(reporter, made.Key, is.status); err != nil {
				return 0, fmt.Errorf("%s: %w", made.Key, err)
			}
		}
		if is.comment != "" {
			assignee, err := as(is.assignee)
			if err != nil {
				return 0, err
			}
			if err := assignee.call("POST", "/api/issues/"+made.Key+"/comments",
				map[string]any{"body": is.comment}, nil); err != nil {
				return 0, fmt.Errorf("%s: %w", made.Key, err)
			}
		}
		created++
	}
	return created, nil
}

// moveTo moves an issue to the first status of a category.
func moveTo(c *client, key string, category issue.StatusCategory) error {
	var d struct {
		Statuses []struct{ ID, Category string }
	}
	if err := c.call("GET", "/api/issues/"+key, nil, &d); err != nil {
		return err
	}
	for _, s := range d.Statuses {
		if issue.StatusCategory(s.Category) == category {
			return c.call("PATCH", "/api/issues/"+key, map[string]any{"status": s.ID}, nil)
		}
	}
	return fmt.Errorf("no %s status", category)
}
