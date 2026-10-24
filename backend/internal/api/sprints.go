package api

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
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

type sprintRef struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"` // future, active, closed
	Board int64  `json:"board,omitempty"`
}

type sprintOut struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	State     string     `json:"state"`
	Goal      *string    `json:"goal"`
	Start     *time.Time `json:"start"`
	End       *time.Time `json:"end"`
	Completed *time.Time `json:"completed"`
}

const sprintColumns = `id, name, state, goal, start_at, end_at, completed_at`

func scanSprint(sc scanner) (sprintOut, error) {
	var sp sprintOut
	var goal sql.NullString
	var start, end, done sql.NullTime
	if err := sc.Scan(&sp.ID, &sp.Name, &sp.State, &goal, &start, &end, &done); err != nil {
		return sp, err
	}
	sp.Goal = sqlq.String(goal)
	sp.Start, sp.End, sp.Completed = sqlq.Time(start), sqlq.Time(end), sqlq.Time(done)
	return sp, nil
}

// boardSprints lists the sprints of a board in the given states, or in any
// state for none, the active one first.
func (s *Server) boardSprints(ctx context.Context, board int64, states ...string) ([]sprintOut, error) {
	q := `SELECT ` + sprintColumns + ` FROM sprints WHERE board_id = ?`
	args := []any{board}
	if len(states) > 0 {
		q += ` AND state IN (` + sqlq.Marks(len(states)) + `)`
		for _, st := range states {
			args = append(args, st)
		}
	}
	out := []sprintOut{}
	err := sqlq.Each(ctx, s.db, q+` ORDER BY state = 'active' DESC, position, id`, func(rows *sql.Rows) error {
		sp, err := scanSprint(rows)
		if err != nil {
			return err
		}
		out = append(out, sp)
		return nil
	}, args...)
	return out, err
}

// sprintByID loads a sprint of the board.
func (s *Server) sprintByID(ctx context.Context, board, id int64) (*sprintOut, error) {
	sp, err := scanSprint(s.db.QueryRowContext(ctx, `SELECT `+sprintColumns+` FROM sprints WHERE board_id = ? AND id = ?`,
		board, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sp, nil
}

func (s *Server) sprintForManage(r *http.Request, sub *perm.Subject) (*boardRow, *sprintOut, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, nil, errNotFound
	}
	var board sql.NullInt64
	if err := s.db.QueryRowContext(r.Context(), `SELECT board_id FROM sprints WHERE id = ?`, id).Scan(&board); err != nil {
		return nil, nil, errNotFound
	}
	b, err := s.boardByID(r.Context(), sub, board.Int64)
	if err != nil {
		return nil, nil, err
	}
	if ok, err := s.canManage(r.Context(), sub, b); err != nil {
		return nil, nil, err
	} else if !ok {
		return nil, nil, errForbidden
	}
	sp, err := s.sprintByID(r.Context(), b.ID, id)
	if err != nil {
		return nil, nil, err
	}
	return b, sp, nil
}

func (s *Server) createSprint(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	if b.Kind != "scrum" {
		return fail(http.StatusBadRequest, i18n.SprintKanban)
	}
	if ok, err := s.canManage(ctx, sub, b); err != nil {
		return err
	} else if !ok {
		return errForbidden
	}
	var in struct {
		Name string `json:"name"`
		Goal string `json:"goal"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	var count, position int64
	s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(position), 0) FROM sprints WHERE board_id = ?`, b.ID).
		Scan(&count, &position)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = b.Name + " Sprint " + strconv.FormatInt(count+1, 10)
	}
	goal := sqlq.NullIfBlank(in.Goal)
	res, err := s.db.ExecContext(ctx, `INSERT INTO sprints (board_id, name, goal, state, position) VALUES (?, ?, ?, 'future', ?)`,
		b.ID, sqlq.Truncate(name, sqlq.VarcharWidth), goal, position+1)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	s.emitSprint(ctx, events.SprintCreated, sub.UserID, b.ID, id)
	return writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) startSprint(w http.ResponseWriter, r *http.Request) error {
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
		return fail(http.StatusConflict, i18n.SprintStarted)
	}
	var in struct {
		Name *string    `json:"name"`
		Goal *string    `json:"goal"`
		End  *time.Time `json:"end"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	var active int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sprints WHERE board_id = ? AND state = 'active'`, b.ID).Scan(&active)
	if active > 0 {
		return fail(http.StatusConflict, i18n.SprintAlreadyActive)
	}
	start := time.Now().UTC()
	end := start.AddDate(0, 0, 14)
	if in.End != nil {
		if !in.End.After(start) {
			return fail(http.StatusBadRequest, i18n.SprintEndPast)
		}
		end = in.End.UTC()
	}
	sets := []string{"state = 'active'", "start_at = ?", "end_at = ?"}
	args := []any{start, end}
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		sets, args = append(sets, "name = ?"), append(args, sqlq.Truncate(strings.TrimSpace(*in.Name), sqlq.VarcharWidth))
	}
	if in.Goal != nil {
		sets, args = append(sets, "goal = ?"), append(args, sqlq.NullIfBlank(*in.Goal))
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE sprints SET `+strings.Join(sets, ", ")+` WHERE id = ?`,
		append(args, sp.ID)...); err != nil {
		return err
	}
	s.emitSprint(ctx, events.SprintStarted, sub.UserID, b.ID, sp.ID)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// completeSprint closes the sprint. Its unfinished issues go to another open
// sprint of the board (moveTo) or back to the backlog; the closed sprint
// stays in their history either way.
func (s *Server) completeSprint(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, sp, err := s.sprintForManage(r, sub)
	if err != nil {
		return err
	}
	if sp.State != "active" {
		return fail(http.StatusConflict, i18n.SprintNotActive)
	}
	var in struct {
		MoveTo int64 `json:"moveTo"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.MoveTo != 0 {
		var state string
		err := s.db.QueryRowContext(ctx, `SELECT state FROM sprints WHERE id = ? AND board_id = ?`, in.MoveTo, b.ID).Scan(&state)
		if err != nil || state != "future" || in.MoveTo == sp.ID {
			return fail(http.StatusBadRequest, i18n.SprintTargetInvalid)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE sprints SET state = 'closed', completed_at = ? WHERE id = ?`, now, sp.ID); err != nil {
		return err
	}
	moved := int64(0)
	if in.MoveTo != 0 {
		res, err := tx.ExecContext(ctx, `INSERT IGNORE INTO sprint_issues (sprint_id, issue_id)
			SELECT ?, si.issue_id FROM sprint_issues si JOIN issues i ON i.id = si.issue_id
			JOIN statuses st ON st.id = i.status_id WHERE si.sprint_id = ? AND st.category <> 'done'`, in.MoveTo, sp.ID)
		if err != nil {
			return err
		}
		moved, _ = res.RowsAffected()
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.emitSprint(ctx, events.SprintClosed, sub.UserID, b.ID, sp.ID)
	return writeJSON(w, http.StatusOK, map[string]int64{"moved": moved})
}

// Changing and deleting sprints, with the same rights as starting and
// completing them: those who manage the board.

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
		if utf8.RuneCountInString(name) > sqlq.VarcharWidth {
			return nil, failf(http.StatusBadRequest, i18n.FieldTooLong, sqlq.VarcharWidth)
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
	sp, err = s.sprintByID(ctx, b.ID, sp.ID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, sp)
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
	issueIDs, line, err := dropSprint(ctx, tx, sub.UserID, sp, time.Now().UTC())
	if err != nil {
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
// dropSprint deletes a future sprint in tx and records, on each of its
// issues, that it went back to the backlog. It returns those issues and the
// history line, for announceBackToBacklog once tx commits.
func dropSprint(ctx context.Context, tx *sql.Tx, by int64, sp *sprintOut, now time.Time) ([]int64, change, error) {
	line := change{field: "Sprint", oldValue: strconv.FormatInt(sp.ID, 10), oldString: sp.Name}
	var issueIDs []int64
	if err := sqlq.Each(ctx, tx, `SELECT issue_id FROM sprint_issues WHERE sprint_id = ? ORDER BY issue_id`,
		func(rows *sql.Rows) error {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			issueIDs = append(issueIDs, id)
			return nil
		}, sp.ID); err != nil {
		return nil, line, err
	}
	for _, id := range issueIDs {
		if err := recordChanges(ctx, tx, id, by, now, []change{line}); err != nil {
			return nil, line, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, now, id); err != nil {
			return nil, line, err
		}
	}
	// The sprint's issues leave with it (sprint_issues cascades).
	if _, err := tx.ExecContext(ctx, `DELETE FROM sprints WHERE id = ?`, sp.ID); err != nil {
		return nil, line, err
	}
	return issueIDs, line, nil
}

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
