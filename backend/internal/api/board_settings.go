package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"solstice/internal/audit"
	"solstice/internal/events"
	"solstice/internal/i18n"
)

type boardInput struct {
	Name       *string `json:"name"`
	SubQuery   *string `json:"subQuery"`
	DoneCutoff *string `json:"doneCutoff"`
	Swimlanes  *string `json:"swimlanes"`
	Estimate   *string `json:"estimate"` // "", "timeoriginalestimate" or "customfield_<id>"
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
				return failf(http.StatusBadRequest, "%s : %s", what, he.msg)
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
			return fail(http.StatusBadRequest, "Le tableau a besoin d'un nom.")
		}
		sets, args = append(sets, "name = ?"), append(args, truncateRunes(n, 255))
	}
	if in.SubQuery != nil {
		var v any
		if q := strings.TrimSpace(*in.SubQuery); q != "" {
			if err := checkQuery(i18n.M("Sous-filtre"), q); err != nil {
				return err
			}
			v = q
		}
		sets, args = append(sets, "sub_query = ?"), append(args, v)
	}
	if in.DoneCutoff != nil {
		var v any
		if c := strings.TrimSpace(*in.DoneCutoff); c != "" {
			if !cutoffPattern.MatchString(c) {
				return fail(http.StatusBadRequest, "Délai invalide : un nombre suivi de d, w ou m (par exemple 2w).")
			}
			v = c
		}
		sets, args = append(sets, "done_cutoff = ?"), append(args, v)
	}
	if in.Swimlanes != nil {
		if !laneStrategies[*in.Swimlanes] {
			return fail(http.StatusBadRequest, "Couloirs : none, custom, assignee, epic ou parent.")
		}
		sets, args = append(sets, "swimlanes = ?"), append(args, *in.Swimlanes)
	}
	if in.Estimate != nil {
		var v any
		if e := *in.Estimate; e != "" {
			if e != "timeoriginalestimate" && !customFieldRef.MatchString(e) {
				return fail(http.StatusBadRequest, "Estimation : timeoriginalestimate ou customfield_<id>.")
			}
			v = e
		}
		sets, args = append(sets, "estimate = ?"), append(args, v)
	}
	if in.Columns != nil {
		if len(*in.Columns) == 0 {
			return fail(http.StatusBadRequest, "Un tableau a au moins une colonne.")
		}
		seen := map[string]bool{}
		for _, c := range *in.Columns {
			if strings.TrimSpace(c.Name) == "" {
				return fail(http.StatusBadRequest, "Chaque colonne a un nom.")
			}
			if c.Min != nil && c.Max != nil && *c.Min > *c.Max {
				return failf(http.StatusBadRequest, "Colonne « %s » : le minimum dépasse le maximum.", c.Name)
			}
			for _, st := range c.Statuses {
				if _, ok := cat.status[st]; !ok {
					return failf(http.StatusBadRequest, "Statut inconnu : %s.", st)
				}
				if seen[st] {
					return failf(http.StatusBadRequest, "Le statut « %s » est dans deux colonnes.", cat.statusOf(st).Name)
				}
				seen[st] = true
			}
		}
	}
	if in.QuickFilters != nil {
		for _, q := range *in.QuickFilters {
			if strings.TrimSpace(q.Name) == "" {
				return fail(http.StatusBadRequest, "Chaque filtre rapide a un nom.")
			}
			if err := checkQuery(i18n.M("Filtre rapide « %s »", q.Name), q.Query); err != nil {
				return err
			}
		}
	}
	if in.Lanes != nil {
		for _, l := range *in.Lanes {
			if strings.TrimSpace(l.Name) == "" {
				return fail(http.StatusBadRequest, "Chaque couloir a un nom.")
			}
			if l.Query != nil && strings.TrimSpace(*l.Query) != "" {
				if err := checkQuery(i18n.M("Couloir « %s »", l.Name), *l.Query); err != nil {
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
			q += ` AND id NOT IN (` + marks(len(keep)) + `)`
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
					return fail(http.StatusBadRequest, "Colonne inconnue sur ce tableau.")
				}
				if _, err := tx.ExecContext(ctx, `UPDATE board_columns SET name = ?, position = ?, min_issues = ?,
					max_issues = ? WHERE id = ?`, truncateRunes(strings.TrimSpace(c.Name), 255), pos, c.Min, c.Max, id); err != nil {
					return err
				}
			} else {
				res, err := tx.ExecContext(ctx, `INSERT INTO board_columns (board_id, name, position, min_issues, max_issues)
					VALUES (?, ?, ?, ?, ?)`, b.ID, truncateRunes(strings.TrimSpace(c.Name), 255), pos, c.Min, c.Max)
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
				desc = truncateRunes(*q.Description, 255)
			}
			if q.ID != 0 {
				_, err = tx.ExecContext(ctx, `UPDATE board_quick_filters SET name = ?, query = ?, description = ?, position = ?
					WHERE id = ? AND board_id = ?`, truncateRunes(q.Name, 255), q.Query, desc, pos, q.ID, b.ID)
			} else {
				_, err = tx.ExecContext(ctx, `INSERT INTO board_quick_filters (board_id, name, query, description, position)
					VALUES (?, ?, ?, ?, ?)`, b.ID, truncateRunes(q.Name, 255), q.Query, desc, pos)
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
			if l.Query != nil && strings.TrimSpace(*l.Query) != "" {
				q = *l.Query
			}
			if l.ID != 0 {
				_, err = tx.ExecContext(ctx, `UPDATE board_swimlanes SET name = ?, query = ?, position = ?, is_default = ?
					WHERE id = ? AND board_id = ?`, truncateRunes(l.Name, 255), q, pos, l.Default, l.ID, b.ID)
			} else {
				_, err = tx.ExecContext(ctx, `INSERT INTO board_swimlanes (board_id, name, query, position, is_default)
					VALUES (?, ?, ?, ?, ?)`, b.ID, truncateRunes(l.Name, 255), q, pos, l.Default)
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
	// The board's saved filter stays: it may be used on its own.
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM boards WHERE id = ?`, b.ID); err != nil {
		return err
	}
	s.emit(events.Event{Kind: events.BoardDeleted, ActorID: sub.UserID,
		Board: &events.Board{ID: b.ID, Name: b.Name, Kind: b.Kind}})
	s.auditBoard(r, audit.BoardDelete, b.ID, b.Name, map[string]any{"kind": b.Kind})
	w.WriteHeader(http.StatusNoContent)
	return nil
}
