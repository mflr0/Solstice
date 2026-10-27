package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"solstice/internal/audit"
	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/sqlq"
)

type boardInput struct {
	Name       *string `json:"name"`
	SubQuery   *string `json:"subQuery"`
	DoneCutoff *string `json:"doneCutoff"`
	Swimlanes  *string `json:"swimlanes"`
	Estimate   *string `json:"estimate"` // "", "timeoriginalestimate" or "customfield_<id>"
	Backlog    *bool   `json:"kanbanBacklog"`
	Columns    *[]struct {
		ID       int64    `json:"id"`
		Name     string   `json:"name"`
		Min      *int     `json:"min"`
		Max      *int     `json:"max"`
		Statuses []string `json:"statuses"`
	} `json:"columns"`
	QuickFilters *[]struct {
		ID          int64   `json:"id"`
		Name        string  `json:"name"`
		Query       string  `json:"query"`
		Description *string `json:"description"`
	} `json:"quickFilters"`
	Lanes *[]struct {
		ID      int64   `json:"id"`
		Name    string  `json:"name"`
		Query   *string `json:"query"`
		Default bool    `json:"default"`
	} `json:"lanes"`
}

var laneStrategies = map[string]bool{"none": true, "custom": true, "assignee": true, "epic": true, "parent": true}

// updateBoard changes a board's settings. Lists given replace the current
// ones: items with an id are kept and updated, new ones are added, missing
// ones removed.
func (s *Server) updateBoard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	if ok, err := s.canManage(ctx, sub, b); err != nil {
		return err
	} else if !ok {
		return errForbidden
	}
	var in boardInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	checkQuery := func(what i18n.Message, q string) error {
		if _, _, err := s.queryCondition(ctx, sub, q); err != nil {
			var he *httpError
			if errors.As(err, &he) {
				return failf(http.StatusBadRequest, i18n.BoardQueryInvalid, what, he.msg)
			}
			return err
		}
		return nil
	}

	sets := []string{"updated_at = ?"}
	args := []any{time.Now().UTC()}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" {
			return fail(http.StatusBadRequest, i18n.BoardNameRequired)
		}
		sets, args = append(sets, "name = ?"), append(args, sqlq.Truncate(n, sqlq.VarcharWidth))
	}
	if in.SubQuery != nil {
		q := strings.TrimSpace(*in.SubQuery)
		if q != "" {
			if err := checkQuery(i18n.M(i18n.BoardSubFilter), q); err != nil {
				return err
			}
		}
		sets, args = append(sets, "sub_query = ?"), append(args, sqlq.NullIf(q))
	}
	if in.DoneCutoff != nil {
		c := strings.TrimSpace(*in.DoneCutoff)
		if c != "" && !cutoffPattern.MatchString(c) {
			return fail(http.StatusBadRequest, i18n.BoardPeriodInvalid)
		}
		sets, args = append(sets, "done_cutoff = ?"), append(args, sqlq.NullIf(c))
	}
	if in.Swimlanes != nil {
		if !laneStrategies[*in.Swimlanes] {
			return fail(http.StatusBadRequest, i18n.BoardSwimlanesUnknown)
		}
		sets, args = append(sets, "swimlanes = ?"), append(args, *in.Swimlanes)
	}
	if in.Estimate != nil {
		e := *in.Estimate
		if e != "" && e != "timeoriginalestimate" && !customFieldRef.MatchString(e) {
			return fail(http.StatusBadRequest, i18n.BoardEstimationUnknown)
		}
		sets, args = append(sets, "estimate = ?"), append(args, sqlq.NullIf(e))
	}
	if in.Backlog != nil && *in.Backlog && b.Kind != "kanban" {
		return fail(http.StatusBadRequest, i18n.BoardBacklogColumns)
	}
	if in.Columns != nil {
		if len(*in.Columns) == 0 {
			return fail(http.StatusBadRequest, i18n.BoardColumnRequired)
		}
		seen := map[string]bool{}
		for _, c := range *in.Columns {
			if strings.TrimSpace(c.Name) == "" {
				return fail(http.StatusBadRequest, i18n.BoardColumnNameRequired)
			}
			if c.Min != nil && c.Max != nil && *c.Min > *c.Max {
				return failf(http.StatusBadRequest, i18n.BoardColumnLimits, c.Name)
			}
			for _, st := range c.Statuses {
				if _, ok := cat.status[st]; !ok {
					return failf(http.StatusBadRequest, i18n.StatusNameUnknown, st)
				}
				if seen[st] {
					return failf(http.StatusBadRequest, i18n.BoardStatusInTwoColumns, cat.statusOf(st).Name)
				}
				seen[st] = true
			}
		}
	}
	if in.QuickFilters != nil {
		for _, q := range *in.QuickFilters {
			if strings.TrimSpace(q.Name) == "" {
				return fail(http.StatusBadRequest, i18n.BoardQuickFilterNameRequired)
			}
			if err := checkQuery(i18n.M(i18n.BoardQuickFilterNamed, q.Name), q.Query); err != nil {
				return err
			}
		}
	}
	if in.Lanes != nil {
		for _, l := range *in.Lanes {
			if strings.TrimSpace(l.Name) == "" {
				return fail(http.StatusBadRequest, i18n.BoardSwimlaneNameRequired)
			}
			if l.Query != nil && strings.TrimSpace(*l.Query) != "" {
				if err := checkQuery(i18n.M(i18n.BoardSwimlaneNamed, l.Name), *l.Query); err != nil {
					return err
				}
			}
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE boards SET `+strings.Join(sets, ", ")+` WHERE id = ?`,
		append(args, b.ID)...); err != nil {
		return err
	}
	// keepOnly removes the board's rows of table whose id is not in keep.
	keepOnly := func(table string, keep []int64) error {
		q := `DELETE FROM ` + table + ` WHERE board_id = ?`
		a := []any{b.ID}
		if len(keep) > 0 {
			q += ` AND id NOT IN (` + sqlq.Marks(len(keep)) + `)`
			for _, id := range keep {
				a = append(a, id)
			}
		}
		_, err := tx.ExecContext(ctx, q, a...)
		return err
	}
	if in.Columns != nil {
		var keep []int64
		for _, c := range *in.Columns {
			if c.ID != 0 {
				keep = append(keep, c.ID)
			}
		}
		if err := keepOnly("board_columns", keep); err != nil {
			return err
		}
		for pos, c := range *in.Columns {
			id := c.ID
			if id != 0 {
				var exists int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM board_columns WHERE id = ? AND board_id = ?`,
					id, b.ID).Scan(&exists); err != nil {
					return err
				}
				if exists == 0 {
					return fail(http.StatusBadRequest, i18n.BoardColumnUnknown)
				}
				if _, err := tx.ExecContext(ctx, `UPDATE board_columns SET name = ?, position = ?, min_issues = ?,
					max_issues = ? WHERE id = ?`, sqlq.Truncate(strings.TrimSpace(c.Name), sqlq.VarcharWidth), pos, c.Min, c.Max, id); err != nil {
					return err
				}
			} else {
				res, err := tx.ExecContext(ctx, `INSERT INTO board_columns (board_id, name, position, min_issues, max_issues)
					VALUES (?, ?, ?, ?, ?)`, b.ID, sqlq.Truncate(strings.TrimSpace(c.Name), sqlq.VarcharWidth), pos, c.Min, c.Max)
				if err != nil {
					return err
				}
				id, _ = res.LastInsertId()
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM board_column_statuses WHERE column_id = ?`, id); err != nil {
				return err
			}
			for i, st := range c.Statuses {
				if _, err := tx.ExecContext(ctx, `INSERT INTO board_column_statuses (column_id, status_id, position)
					VALUES (?, ?, ?)`, id, st, i); err != nil {
					return err
				}
			}
		}
	}
	if in.Backlog != nil || (b.Backlog && in.Columns != nil) {
		on := b.Backlog
		if in.Backlog != nil {
			on = *in.Backlog
		}
		if on {
			// The first column goes to the backlog page: the board keeps the others.
			var columns, firstStatuses int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(c.id = (SELECT id FROM board_columns
				WHERE board_id = ? ORDER BY position, id LIMIT 1) AND EXISTS (SELECT 1 FROM board_column_statuses cs
				WHERE cs.column_id = c.id)), 0) FROM board_columns c WHERE c.board_id = ?`, b.ID, b.ID).
				Scan(&columns, &firstStatuses); err != nil {
				return err
			}
			if columns < 2 || firstStatuses == 0 {
				if in.Backlog != nil {
					return fail(http.StatusBadRequest, i18n.BoardBacklogColumns)
				}
				on = false // the columns no longer allow it
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE boards SET kanban_backlog = ? WHERE id = ?`, on, b.ID); err != nil {
			return err
		}
	}
	if in.QuickFilters != nil {
		var keep []int64
		for _, q := range *in.QuickFilters {
			if q.ID != 0 {
				keep = append(keep, q.ID)
			}
		}
		if err := keepOnly("board_quick_filters", keep); err != nil {
			return err
		}
		for pos, q := range *in.QuickFilters {
			var desc any
			if q.Description != nil && strings.TrimSpace(*q.Description) != "" {
				desc = sqlq.Truncate(*q.Description, sqlq.VarcharWidth)
			}
			if q.ID != 0 {
				_, err = tx.ExecContext(ctx, `UPDATE board_quick_filters SET name = ?, query = ?, description = ?, position = ?
					WHERE id = ? AND board_id = ?`, sqlq.Truncate(q.Name, sqlq.VarcharWidth), q.Query, desc, pos, q.ID, b.ID)
			} else {
				_, err = tx.ExecContext(ctx, `INSERT INTO board_quick_filters (board_id, name, query, description, position)
					VALUES (?, ?, ?, ?, ?)`, b.ID, sqlq.Truncate(q.Name, sqlq.VarcharWidth), q.Query, desc, pos)
			}
			if err != nil {
				return err
			}
		}
	}
	if in.Lanes != nil {
		var keep []int64
		for _, l := range *in.Lanes {
			if l.ID != 0 {
				keep = append(keep, l.ID)
			}
		}
		if err := keepOnly("board_swimlanes", keep); err != nil {
			return err
		}
		for pos, l := range *in.Lanes {
			var q any
			if l.Query != nil {
				q = sqlq.NullIfBlank(*l.Query)
			}
			if l.ID != 0 {
				_, err = tx.ExecContext(ctx, `UPDATE board_swimlanes SET name = ?, query = ?, position = ?, is_default = ?
					WHERE id = ? AND board_id = ?`, sqlq.Truncate(l.Name, sqlq.VarcharWidth), q, pos, l.Default, l.ID, b.ID)
			} else {
				_, err = tx.ExecContext(ctx, `INSERT INTO board_swimlanes (board_id, name, query, position, is_default)
					VALUES (?, ?, ?, ?, ?)`, b.ID, sqlq.Truncate(l.Name, sqlq.VarcharWidth), q, pos, l.Default)
			}
			if err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	board := &events.Board{ID: b.ID}
	if err := s.db.QueryRowContext(ctx, `SELECT name, kind FROM boards WHERE id = ?`, b.ID).
		Scan(&board.Name, &board.Kind); err == nil {
		s.emit(events.Event{Kind: events.BoardUpdated, ActorID: sub.UserID, Board: board})
	}
	s.auditBoard(r, audit.BoardUpdate, b.ID, b.Name, map[string]any{"changed": boardChanges(in)})
	return s.getBoard(w, r)
}

func (s *Server) deleteBoard(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	if ok, err := s.canManage(r.Context(), sub, b); err != nil {
		return err
	} else if !ok {
		return errForbidden
	}
	ctx := r.Context()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Its future sprints go, as if deleted one by one; its active sprint is
	// completed. Closed sprints stay, with no board, as the issues' history.
	var future, active []*sprintOut
	if err := sqlq.Each(ctx, tx, `SELECT `+sprintColumns+` FROM sprints WHERE board_id = ? AND state <> ?
		ORDER BY position, id FOR UPDATE`, func(rows *sql.Rows) error {
		sp, err := scanSprint(rows)
		if err != nil {
			return err
		}
		if sp.State == issue.SprintFuture {
			future = append(future, &sp)
		} else {
			active = append(active, &sp)
		}
		return nil
	}, b.ID, issue.SprintClosed); err != nil {
		return err
	}
	now := time.Now().UTC()
	type dropped struct {
		issueIDs []int64
		line     change
	}
	drops := make([]dropped, len(future))
	for i, sp := range future {
		if drops[i].issueIDs, drops[i].line, err = dropSprint(ctx, tx, sub.UserID, sp, now); err != nil {
			return err
		}
	}
	for _, sp := range active {
		if _, err := tx.ExecContext(ctx, `UPDATE sprints SET state = ?, completed_at = ? WHERE id = ?`, issue.SprintClosed, now, sp.ID); err != nil {
			return err
		}
		sp.State, sp.Completed = issue.SprintClosed, &now
	}
	// The board's saved filter stays: it may be used on its own.
	if _, err := tx.ExecContext(ctx, `DELETE FROM boards WHERE id = ?`, b.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	for i, sp := range future {
		s.announceBackToBacklog(ctx, sub.UserID, drops[i].issueIDs, drops[i].line)
		s.emit(events.Event{Kind: events.SprintDeleted, ActorID: sub.UserID, Sprint: sprintEvent(sp, b.ID)})
	}
	for _, sp := range active {
		s.emit(events.Event{Kind: events.SprintClosed, ActorID: sub.UserID, Sprint: sprintEvent(sp, b.ID)})
	}
	s.emit(events.Event{Kind: events.BoardDeleted, ActorID: sub.UserID,
		Board: &events.Board{ID: b.ID, Name: b.Name, Kind: b.Kind}})
	s.auditBoard(r, audit.BoardDelete, b.ID, b.Name,
		map[string]any{"kind": b.Kind, "deletedSprints": len(future), "completedSprints": len(active)})
	w.WriteHeader(http.StatusNoContent)
	return nil
}
