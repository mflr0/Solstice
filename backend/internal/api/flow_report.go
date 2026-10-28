package api

import (
	"database/sql"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"solstice/internal/report"
	"solstice/internal/sqlq"
)

const (
	flowDefaultDays = 30
	flowMaxDays     = 366
)

type flowDay struct {
	Date   string `json:"date"`
	Counts []int  `json:"counts"`
}

// flowReport is a board's cumulative flow: for each day of the period
// (?days=, default 30, ending today in the ?tz= zone), how many of its issues
// sat in each column at the end of the day. The issues are those the board's
// filter and Kanban sub-filter match today and the viewer may see; the
// columns, the board's as they are now.
func (s *Server) flowReport(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	loc, err := s.columnZone(r)
	if err != nil {
		return err
	}
	days := flowDefaultDays
	if n, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil {
		days = min(max(n, 1), flowMaxDays)
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	cols, err := s.boardColumns(ctx, cat, b.ID)
	if err != nil {
		return err
	}
	column := map[string]int{}
	for i, c := range cols {
		for _, st := range c.Statuses {
			column[st.ID] = i
		}
	}

	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	first := today.AddDate(0, 0, 1-days)
	ends := make([]time.Time, days)
	for k := range ends {
		ends[k] = first.AddDate(0, 0, k+1)
	}

	vis, args := sub.VisibleSQL("i")
	where := []string{vis}
	queries := []string{b.Query}
	if b.Kind == "kanban" && b.SubQuery.Valid {
		queries = append(queries, b.SubQuery.String)
	}
	for _, q := range queries {
		c, a, err := s.queryCondition(ctx, sub, q)
		if err != nil {
			return err
		}
		where = append(where, "("+c+")")
		args = append(args, a...)
	}
	issues := map[int64]*report.FlowIssue{}
	if err := sqlq.Each(ctx, s.db, `SELECT i.id, i.status_id, i.created_at FROM issues i
		JOIN projects p ON p.id = i.project_id WHERE `+strings.Join(where, " AND "), func(rows *sql.Rows) error {
		var id int64
		var is report.FlowIssue
		if err := rows.Scan(&id, &is.Status, &is.Created); err != nil {
			return err
		}
		issues[id] = &is
		return nil
	}, args...); err != nil {
		return err
	}

	ids := make([]any, 0, len(issues))
	for id := range issues {
		ids = append(ids, id)
	}
	for chunk := range slices.Chunk(ids, 500) {
		err := sqlq.Each(ctx, s.db, `SELECT g.issue_id, g.created_at, ci.id, ci.old_value, ci.new_value
			FROM change_groups g JOIN change_items ci ON ci.group_id = g.id
			WHERE g.issue_id IN (`+sqlq.Marks(len(chunk))+`) AND g.created_at >= ?
			AND ci.field_type = 'jira' AND ci.field = 'status'`, func(rows *sql.Rows) error {
			var id int64
			c := report.Change{Field: report.FieldStatus}
			var old, next sql.NullString
			if err := rows.Scan(&id, &c.At, &c.Seq, &old, &next); err != nil {
				return err
			}
			c.Old, c.New = old.String, next.String
			issues[id].Changes = append(issues[id].Changes, c)
			return nil
		}, append(chunk, ends[0])...)
		if err != nil {
			return err
		}
	}

	list := make([]report.FlowIssue, 0, len(issues))
	for _, is := range issues {
		list = append(list, *is)
	}
	counts := report.Flow(list, column, len(cols), ends)
	out := make([]flowDay, days)
	for k := range out {
		out[k] = flowDay{Date: first.AddDate(0, 0, k).Format(time.DateOnly), Counts: counts[k]}
	}
	if cols == nil {
		cols = []columnOut{}
	}
	return writeJSON(w, http.StatusOK, map[string]any{"columns": cols, "days": out})
}
