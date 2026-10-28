package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/perm"
	"solstice/internal/report"
	"solstice/internal/sqlq"
)

type reportIssue struct {
	issueSummary
	Committed     bool           `json:"committed"`
	Outcome       report.Outcome `json:"outcome"`
	StartEstimate *float64       `json:"startEstimate"`
	EndEstimate   *float64       `json:"endEstimate"`
}

type reportStep struct {
	At        time.Time `json:"at"`
	Issue     string    `json:"issue"`
	Remaining float64   `json:"remaining"`
}

// reportSprints lists the sprints of a board that have a report: the active
// one, then the closed ones, the last completed first.
func (s *Server) reportSprints(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	out := []sprintOut{}
	err = sqlq.Each(ctx, s.db, `SELECT `+sprintColumns+` FROM sprints WHERE board_id = ? AND state <> ?
		AND start_at IS NOT NULL ORDER BY state = ? DESC, COALESCE(completed_at, end_at) DESC, id DESC`,
		func(rows *sql.Rows) error {
			sp, err := scanSprint(rows)
			if err != nil {
				return err
			}
			out = append(out, sp)
			return nil
		}, b.ID, issue.SprintFuture, issue.SprintActive)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// sprintReport rebuilds a started sprint from the change log: its scope at
// start, what joined or left it, what got done, and its burndown. Issues the
// viewer can't browse are left out.
func (s *Server) sprintReport(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("sprint"), 10, 64)
	if err != nil {
		return errNotFound
	}
	sp, err := s.sprintByID(ctx, b.ID, id)
	if err != nil {
		return err
	}
	if sp.State == issue.SprintFuture || sp.Start == nil {
		return errNotFound
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	done, err := s.doneStatuses(ctx, b.ID)
	if err != nil {
		return err
	}
	end := sprintReportEnd(sp)
	rep, cards, err := s.replaySprint(ctx, sub, b, sp, end, cat, done)
	if err != nil {
		return err
	}

	byID := map[int64]report.Entry{}
	for _, e := range rep.Entries {
		byID[e.ID] = e
	}
	keys := map[int64]*card{}
	out := []reportIssue{}
	for _, c := range cards { // board order
		keys[c.ID] = c
		if e, ok := byID[c.ID]; ok {
			out = append(out, reportIssue{issueSummary: c.issueSummary, Committed: e.Committed, Outcome: e.Outcome,
				StartEstimate: e.StartEstimate, EndEstimate: e.EndEstimate})
		}
	}
	steps := make([]reportStep, len(rep.Steps))
	for i, st := range rep.Steps {
		steps[i] = reportStep{At: st.At, Issue: keys[st.Issue].Key, Remaining: st.Remaining}
	}
	t := rep.Totals
	return writeJSON(w, http.StatusOK, map[string]any{
		"sprint": sp, "end": end, "estimate": reportEstimate(b),
		"totals": map[string]float64{"committed": t.Committed, "added": t.Added, "removed": t.Removed,
			"completed": t.Completed, "notCompleted": t.NotCompleted},
		"startRemaining": rep.StartRemaining, "steps": steps, "issues": out,
	})
}

type velocitySprint struct {
	Sprint    sprintOut `json:"sprint"`
	Committed float64   `json:"committed"`
	Completed float64   `json:"completed"`
}

// velocityReport gives, for the board's last closed sprints (?count, 7 by
// default), the work committed at start and the work completed, oldest
// first. Issues the viewer can't browse are left out.
func (s *Server) velocityReport(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, err := s.board(r, sub)
	if err != nil {
		return err
	}
	count := 7
	if v := r.URL.Query().Get("count"); v != "" {
		if count, err = strconv.Atoi(v); err != nil || count < 1 || count > 20 {
			return failf(http.StatusBadRequest, i18n.RequestInvalid, "count must be 1 to 20")
		}
	}
	var sprints []sprintOut
	err = sqlq.Each(ctx, s.db, `SELECT `+sprintColumns+` FROM sprints WHERE board_id = ? AND state = ?
		AND start_at IS NOT NULL ORDER BY COALESCE(completed_at, end_at) DESC, id DESC LIMIT ?`,
		func(rows *sql.Rows) error {
			sp, err := scanSprint(rows)
			if err != nil {
				return err
			}
			sprints = append(sprints, sp)
			return nil
		}, b.ID, issue.SprintClosed, count)
	if err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	done, err := s.doneStatuses(ctx, b.ID)
	if err != nil {
		return err
	}
	out := make([]velocitySprint, len(sprints))
	for i, sp := range sprints {
		rep, _, err := s.replaySprint(ctx, sub, b, &sp, sprintReportEnd(&sp), cat, done)
		if err != nil {
			return err
		}
		out[len(sprints)-1-i] = velocitySprint{Sprint: sp, Committed: rep.Totals.Committed, Completed: rep.Totals.Completed}
	}
	return writeJSON(w, http.StatusOK, map[string]any{"estimate": reportEstimate(b), "sprints": out})
}

// sprintReportEnd is when a started sprint's report stops: its completion
// (else its planned end) once closed, now while active.
func sprintReportEnd(sp *sprintOut) time.Time {
	if sp.State == issue.SprintClosed {
		if sp.Completed != nil {
			return *sp.Completed
		}
		if sp.End != nil {
			return *sp.End
		}
	}
	return time.Now().UTC()
}

// reportEstimate is how the board's reports measure work: "points", "time",
// or "" when they count issues.
func reportEstimate(b *boardRow) string {
	if !b.Estimate.Valid {
		return ""
	}
	if b.Estimate.String == "timeoriginalestimate" {
		return "time"
	}
	return "points"
}

// doneStatuses are the statuses of the board's last column: an issue there
// counts as done in its reports.
func (s *Server) doneStatuses(ctx context.Context, board int64) (map[string]bool, error) {
	done := map[string]bool{}
	err := sqlq.Each(ctx, s.db, `SELECT cs.status_id FROM board_column_statuses cs
		JOIN board_columns c ON c.id = cs.column_id
		WHERE c.board_id = ? AND c.position = (SELECT MAX(position) FROM board_columns WHERE board_id = ?)`,
		func(rows *sql.Rows) error {
			var st string
			if err := rows.Scan(&st); err != nil {
				return err
			}
			done[st] = true
			return nil
		}, board, board)
	return done, err
}

// replaySprint replays a started sprint up to end over the issues the viewer
// can browse that were in it at some point, and returns those issues' cards
// in board order.
func (s *Server) replaySprint(ctx context.Context, sub *perm.Subject, b *boardRow, sp *sprintOut, end time.Time,
	cat *catalog, done map[string]bool) (report.Report, []*card, error) {
	// Issues in the sprint now, or that joined or left it since it started.
	candidates := map[int64]*report.Issue{}
	candidate := func(id int64) *report.Issue {
		if c, ok := candidates[id]; ok {
			return c
		}
		c := &report.Issue{ID: id}
		candidates[id] = c
		return c
	}
	if err := sqlq.Each(ctx, s.db, `SELECT issue_id FROM sprint_issues WHERE sprint_id = ?`, func(rows *sql.Rows) error {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		candidate(id).InSprint = true
		return nil
	}, sp.ID); err != nil {
		return report.Report{}, nil, err
	}
	like := "%" + strconv.FormatInt(sp.ID, 10) + "%"
	if err := sqlq.Each(ctx, s.db, `SELECT g.issue_id, g.created_at, ci.id, ci.old_value, ci.new_value
		FROM change_items ci JOIN change_groups g ON g.id = ci.group_id
		WHERE ci.field = 'Sprint' AND g.created_at > ? AND (ci.old_value LIKE ? OR ci.new_value LIKE ?)`,
		func(rows *sql.Rows) error {
			var id int64
			var c report.Change
			var old, next sql.NullString
			if err := rows.Scan(&id, &c.At, &c.Seq, &old, &next); err != nil {
				return err
			}
			c.Field, c.Old, c.New = report.FieldSprint, old.String, next.String
			is := candidate(id)
			is.Changes = append(is.Changes, c)
			return nil
		}, *sp.Start, like, like); err != nil {
		return report.Report{}, nil, err
	}

	ids := make([]any, 0, len(candidates))
	for id := range candidates {
		ids = append(ids, id)
	}
	var cards []*card
	if len(ids) > 0 {
		vis, args := sub.VisibleSQL("i")
		var err error
		cards, _, err = s.loadCards(ctx, b, vis+` AND i.id IN (`+sqlq.Marks(len(ids))+`)`, append(args, ids...), cat)
		if err != nil {
			return report.Report{}, nil, err
		}
	}
	if err := s.reportHistory(ctx, b, *sp.Start, cards, candidates); err != nil {
		return report.Report{}, nil, err
	}

	issues := make([]report.Issue, 0, len(cards))
	for _, c := range cards {
		is := candidates[c.ID]
		is.Status, is.Estimate = c.row.StatusID, c.Estimate
		issues = append(issues, *is)
	}
	rep := report.Build(report.Sprint{ID: sp.ID, Start: *sp.Start, End: end}, issues,
		func(st string) bool { return done[st] }, !b.Estimate.Valid)
	return rep, cards, nil
}

// reportHistory adds to the candidates the status and estimate changes of the
// visible ones since the sprint started. A custom field's changes are logged
// under its name, so those made before a rename are not found.
func (s *Server) reportHistory(ctx context.Context, b *boardRow, start time.Time, cards []*card,
	candidates map[int64]*report.Issue) error {
	if len(cards) == 0 {
		return nil
	}
	estType, estField, hours := "", "", false
	if b.Estimate.Valid {
		if b.Estimate.String == "timeoriginalestimate" {
			estType, estField, hours = "jira", "timeoriginalestimate", true
		} else if m := customFieldRef.FindStringSubmatch(b.Estimate.String); m != nil {
			err := s.db.QueryRowContext(ctx, `SELECT name FROM custom_fields WHERE id = ?`, m[1]).Scan(&estField)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			estType = "custom"
		}
	}
	ids := make([]any, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	for chunk := range slices.Chunk(ids, 500) {
		err := sqlq.Each(ctx, s.db, `SELECT g.issue_id, g.created_at, ci.id, ci.field_type, ci.field, ci.old_value, ci.old_string,
			ci.new_value, ci.new_string FROM change_groups g JOIN change_items ci ON ci.group_id = g.id
			WHERE g.issue_id IN (`+sqlq.Marks(len(chunk))+`) AND g.created_at > ?
			AND ((ci.field_type = 'jira' AND ci.field = 'status') OR (ci.field_type = ? AND ci.field = ?))`,
			func(rows *sql.Rows) error {
				var id int64
				var fieldType, field string
				var c report.Change
				var oldV, oldS, newV, newS sql.NullString
				if err := rows.Scan(&id, &c.At, &c.Seq, &fieldType, &field, &oldV, &oldS, &newV, &newS); err != nil {
					return err
				}
				switch {
				case fieldType == "jira" && field == "status":
					c.Field, c.Old, c.New = report.FieldStatus, oldV.String, newV.String
				case hours:
					c.Field, c.Old, c.New = report.FieldEstimate, secondsToHours(oldV.String), secondsToHours(newV.String)
				default:
					c.Field, c.Old, c.New = report.FieldEstimate, oldS.String, newS.String
				}
				candidates[id].Changes = append(candidates[id].Changes, c)
				return nil
			}, append(chunk, start, estType, estField)...)
		if err != nil {
			return err
		}
	}
	return nil
}

func secondsToHours(s string) string {
	v := report.ParseEstimate(s)
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v/3600, 'f', -1, 64)
}
