package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"solstice/internal/issue"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

type historyItem struct {
	Field     string  `json:"field"`
	FieldType string  `json:"fieldType"`
	From      *string `json:"from"`
	To        *string `json:"to"`
	// Raw values (ids) and display strings apart, for Jira's changelog.
	fromValue, fromString, toValue, toString *string
}

type historyEntry struct {
	ID       int64         `json:"id"`
	Author   *userRef      `json:"author"`
	Created  time.Time     `json:"created"`
	Items    []historyItem `json:"items"`
	authorID sql.NullInt64
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	iss, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	entries, err := s.issueHistory(r.Context(), sub, iss)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, entries)
}

// issueHistory returns the history of an issue, oldest first, without what
// the user may not read. Fields whose old and new values may reveal
// restricted information are not shown to everyone. (Jira hides nothing
// here; Solstice errs on the safe side only for the security level, whose
// ids mean nothing to readers anyway.)
func (s *Server) issueHistory(ctx context.Context, sub *perm.Subject, iss *issueRow) ([]historyEntry, error) {
	entries := []historyEntry{}
	index := map[int64]int{}
	var authors []int64
	err := sqlq.Each(ctx, s.db, `SELECT g.id, g.author_id, g.created_at, i.field_type, i.field,
		i.old_value, i.old_string, i.new_value, i.new_string
		FROM change_groups g JOIN change_items i ON i.group_id = g.id
		WHERE g.issue_id = ? ORDER BY g.created_at, g.id, i.id`, func(rows *sql.Rows) error {
		var id int64
		var author sql.NullInt64
		var created time.Time
		var it historyItem
		var fromValue, fromString, toValue, toString sql.NullString
		if err := rows.Scan(&id, &author, &created, &it.FieldType, &it.Field, &fromValue, &fromString, &toValue,
			&toString); err != nil {
			return err
		}
		if it.Field == "security" {
			return nil
		}
		it.fromValue, it.fromString, it.toValue, it.toString = sqlq.String(fromValue), sqlq.String(fromString),
			sqlq.String(toValue), sqlq.String(toString)
		it.From, it.To = it.fromString, it.toString
		if it.From == nil {
			it.From = it.fromValue
		}
		if it.To == nil {
			it.To = it.toValue
		}
		i, ok := index[id]
		if !ok {
			i = len(entries)
			index[id] = i
			entries = append(entries, historyEntry{ID: id, Created: created, authorID: author})
			authors = append(authors, author.Int64)
		}
		entries[i].Items = append(entries[i].Items, it)
		return nil
	}, iss.ID)
	if err != nil {
		return nil, err
	}
	entries = s.hideRestrictedHistory(ctx, sub, entries)
	users, err := s.users(ctx, authors...)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].Author = refOf(users, entries[i].authorID)
	}
	return entries, nil
}

// History lines that name other issues: "This issue blocks RH-12".
var relationFields = map[string]bool{
	"link": true, "parent": true, "parent issue": true, "parent link": true, "epic link": true, "epic child": true,
}

// Work log lines identify individual work logs, which may be restricted.
var worklogFields = map[string]bool{"worklogid": true, "worklogtimespent": true}

// hideRestrictedHistory drops history lines that would reveal an issue the
// user may not see, or an individual (possibly restricted) work log.
func (s *Server) hideRestrictedHistory(ctx context.Context, sub *perm.Subject, entries []historyEntry) []historyEntry {
	visible := map[string]bool{}
	canSee := func(key string) bool {
		if v, ok := visible[key]; ok {
			return v
		}
		iss, err := s.findIssue(ctx, key)
		// Text that only looks like a key names no issue to hide.
		v := errors.Is(err, errNotFound) || err == nil && sub.CanSee(iss.perm())
		visible[key] = v
		return v
	}
	out := entries[:0]
	for _, e := range entries {
		items := e.Items[:0]
		for _, it := range e.Items {
			field := strings.ToLower(it.Field)
			if worklogFields[field] {
				continue
			}
			if relationFields[field] {
				ok := true
				for _, text := range []*string{it.From, it.To} {
					if text == nil {
						continue
					}
					for _, key := range issue.KeysIn(*text) {
						ok = ok && canSee(key)
					}
				}
				if !ok {
					continue
				}
			}
			items = append(items, it)
		}
		if len(items) > 0 {
			e.Items = items
			out = append(out, e)
		}
	}
	return out
}
