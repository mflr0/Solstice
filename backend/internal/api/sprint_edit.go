package api

// Changing and deleting sprints, with the same rights as starting and
// completing them: those who manage the board.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"solstice/internal/audit"
	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/sqlq"
)

// sprintInput is a request to change a sprint; keys that are absent leave
// the field alone, null clears a goal or a date.
type sprintInput struct {
	Name  *string    `json:"name"`
	Goal  *string    `json:"goal"`
	Start *time.Time `json:"start"`
	End   *time.Time `json:"end"`
}

// sprintPatch is what changing a sprint comes to once checked: the columns
// to write, and the new values of the fields that changed, for the audit.
type sprintPatch struct {
	sets    []string
	args    []any
	changed map[string]any
}

// planSprintEdit checks a change of a sprint against its state. A future
// sprint takes any planned dates; an active one has begun, so it always keeps
// a start (not in the future) and an end. A completed sprint is history.
func planSprintEdit(sp *sprintOut, in sprintInput, given map[string]bool, now time.Time) (*sprintPatch, error) {
	if sp.State == "closed" {
		return nil, fail(http.StatusConflict, i18n.SprintClosedLocked)
	}
	p := &sprintPatch{changed: map[string]any{}}
	set := func(field, column string, v any) {
		p.sets, p.args = append(p.sets, column+" = ?"), append(p.args, v)
		p.changed[field] = v
	}
	if given["name"] {
		if in.Name == nil || strings.TrimSpace(*in.Name) == "" {
			return nil, fail(http.StatusBadRequest, i18n.SprintNameRequired)
		}
		name := strings.TrimSpace(*in.Name)
		if utf8.RuneCountInString(name) > varcharWidth {
			return nil, failf(http.StatusBadRequest, i18n.FieldTooLong, varcharWidth)
		}
		if name != sp.Name {
			set("name", "name", name)
		}
	}
	if given["goal"] {
		var goal any
		if in.Goal != nil {
			goal = sqlq.NullIfBlank(*in.Goal)
		}
		if goal != derefString(sp.Goal) {
			set("goal", "goal", goal)
		}
	}
	start, end := sp.Start, sp.End
	if given["start"] {
		start = utcPtr(in.Start)
	}
	if given["end"] {
		end = utcPtr(in.End)
	}
	if sp.State == "active" {
		if start == nil || end == nil {
			return nil, fail(http.StatusBadRequest, i18n.SprintDatesOrder)
		}
		if start.After(now) {
			return nil, fail(http.StatusBadRequest, i18n.SprintStartFuture)
		}
	}
	if start != nil && end != nil && !end.After(*start) {
		return nil, fail(http.StatusBadRequest, i18n.SprintDatesOrder)
	}
	for _, d := range []struct {
		field string
		given bool
		from  *time.Time
		to    *time.Time
	}{{"start", given["start"], sp.Start, start}, {"end", given["end"], sp.End, end}} {
		if d.given && !sameInstant(d.from, d.to) {
			var v any
			if d.to != nil {
				v = *d.to
			}
			set(d.field, d.field+"_at", v)
		}
	}
	return p, nil
}

func derefString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func sameInstant(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// editSprint renames a sprint, changes its goal and its dates.
func (s *Server) editSprint(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, sp, err := s.sprintForManage(r, sub)
	if err != nil {
		return err
	}
	var in sprintInput
	keys, err := readJSONFields(r, &in)
	if err != nil {
		return err
	}
	given := map[string]bool{}
	for k := range keys {
		given[k] = true
	}
	p, err := planSprintEdit(sp, in, given, time.Now().UTC())
	if err != nil {
		return err
	}
	if len(p.sets) > 0 {
		if _, err := s.db.ExecContext(ctx, `UPDATE sprints SET `+strings.Join(p.sets, ", ")+` WHERE id = ?`,
			append(p.args, sp.ID)...); err != nil {
			return err
		}
		s.emitSprint(ctx, events.SprintUpdated, sub.UserID, b.ID, sp.ID)
		s.auditSprint(r, audit.SprintUpdate, sp.ID, sp.Name, map[string]any{"board": b.ID, "changed": p.changed})
	}
	list, err := s.boardSprints(ctx, b.ID, "id = "+strconv.FormatInt(sp.ID, 10))
	if err != nil || len(list) == 0 {
		return errNotFound
	}
	return writeJSON(w, http.StatusOK, list[0])
}

// deleteSprint deletes a sprint that has not started. Its issues go back to
// the backlog; each one's history says so.
func (s *Server) deleteSprint(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, sp, err := s.sprintForManage(r, sub)
	if err != nil {
		return err
	}
	if sp.State != "future" {
		return fail(http.StatusConflict, i18n.SprintDeleteNotFuture)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var issueIDs []int64
	rows, err := tx.QueryContext(ctx, `SELECT issue_id FROM sprint_issues WHERE sprint_id = ? ORDER BY issue_id`, sp.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		issueIDs = append(issueIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	now := time.Now().UTC()
	line := change{field: "Sprint", oldValue: strconv.FormatInt(sp.ID, 10), oldString: sp.Name}
	for _, id := range issueIDs {
		if err := recordChanges(ctx, tx, id, sub.UserID, now, []change{line}); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, now, id); err != nil {
			return err
		}
	}
	// The sprint's issues leave with it (sprint_issues cascades).
	if _, err := tx.ExecContext(ctx, `DELETE FROM sprints WHERE id = ?`, sp.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.announceBackToBacklog(ctx, sub.UserID, issueIDs, line)
	s.emit(events.Event{Kind: events.SprintDeleted, ActorID: sub.UserID, Sprint: sprintEvent(sp, b.ID)})
	s.auditSprint(r, audit.SprintDelete, sp.ID, sp.Name, map[string]any{"board": b.ID, "issues": len(issueIDs)})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// announceBackToBacklog tells the pages and the webhooks that issues left a
// sprint, with their new history line.
func (s *Server) announceBackToBacklog(ctx context.Context, by int64, issueIDs []int64, line change) {
	for _, id := range issueIDs {
		iss, err := s.findIssueByID(ctx, id)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				s.log.Warn("sprint deletion event lost", "issue", id, "err", err)
			}
			continue
		}
		s.changed(iss, by, "issue", []change{line})
	}
}
