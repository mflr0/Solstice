package api

// Jira Software's Agile REST API (/rest/agile/1.0): boards, sprints, the
// backlog, ranking and epics. A board is visible exactly when its filter
// is, as in the interface.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"solstice/internal/i18n"
	"solstice/internal/perm"
)

// agilePage is the Agile API's paging envelope for boards and sprints.
func agilePage[T any](list []T, startAt, maxResults int) map[string]any {
	page := list[min(startAt, len(list)):min(startAt+maxResults, len(list))]
	values := make([]any, 0, len(page))
	for _, v := range page {
		values = append(values, v)
	}
	return map[string]any{"maxResults": maxResults, "startAt": startAt, "total": len(list),
		"isLast": startAt+len(page) >= len(list), "values": values}
}

func (o *restOut) agileBoard(b *boardSummary) map[string]any {
	out := map[string]any{"id": b.ID, "self": o.base + "/rest/agile/1.0/board/" + o.id(b.ID), "name": b.Name,
		"type": b.Kind}
	if len(b.Projects) > 0 {
		p := b.Projects[0]
		out["location"] = map[string]any{"projectId": p.ID, "projectKey": p.Key, "projectName": p.Name,
			"displayName": p.Name + " (" + p.Key + ")", "name": p.Name + " (" + p.Key + ")",
			"projectTypeKey": "software", "avatarURI": o.base + "/secure/projectavatar?ownerId=" + p.Key + "&size=small"}
	}
	return out
}

func (s *Server) agileBoards(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	target := "/api/boards"
	if v := q.Get("projectKeyOrId"); v != "" {
		id, err := s.restProjectID(r.Context(), v)
		if err != nil {
			return s.writeAgilePage(w, r, nil)
		}
		var key string
		if err := s.db.QueryRowContext(r.Context(), `SELECT pkey FROM projects WHERE id = ?`, id).Scan(&key); err != nil {
			return err
		}
		target += "?project=" + key
	}
	var list []*boardSummary
	if err := s.uiCall(r, s.listBoards, http.MethodGet, target, nil, nil, &list); err != nil {
		return err
	}
	kind, name := strings.ToLower(q.Get("type")), strings.ToLower(q.Get("name"))
	list = slices.DeleteFunc(list, func(b *boardSummary) bool {
		return (kind != "" && b.Kind != kind) || (name != "" && !strings.Contains(strings.ToLower(b.Name), name))
	})
	return s.writeAgilePage(w, r, list)
}

func (s *Server) writeAgilePage(w http.ResponseWriter, r *http.Request, list []*boardSummary) error {
	o := s.restOut(r)
	out := make([]map[string]any, 0, len(list))
	for _, b := range list {
		out = append(out, o.agileBoard(b))
	}
	startAt, maxResults := paging(r, 50, 50)
	return writeJSON(w, http.StatusOK, agilePage(out, startAt, maxResults))
}

// pathBoard loads the board of the path, if the user sees it, with the
// projects of it they see.
func (s *Server) pathBoard(r *http.Request, sub *perm.Subject) (*boardRow, *boardSummary, error) {
	b, err := s.board(r, sub)
	if err != nil {
		return nil, nil, err
	}
	projects, err := s.boardProjects(r.Context(), b.ID)
	if err != nil {
		return nil, nil, err
	}
	sum := &boardSummary{ID: b.ID, Name: b.Name, Kind: b.Kind, Projects: []projectBrief{}}
	for _, p := range projects {
		if sub.CanBrowseProject(p.ID) {
			sum.Projects = append(sum.Projects, p)
		}
	}
	return b, sum, nil
}

func (s *Server) agileBoard(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	_, sum, err := s.pathBoard(r, sub)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, s.restOut(r).agileBoard(sum))
}

// agileBoardConfiguration describes a board's columns, filter and
// estimation, from the interface's view of the board.
func (s *Server) agileBoardConfiguration(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, sum, err := s.pathBoard(r, sub)
	if err != nil {
		return err
	}
	var view struct {
		Board struct {
			FilterID      int64   `json:"filterId"`
			EstimateField *string `json:"estimateField"`
		} `json:"board"`
		Columns        []columnOut `json:"columns"`
		EstimateFields []namedRef  `json:"estimateFields"`
	}
	id := strconv.FormatInt(b.ID, 10)
	if err := s.uiCall(r, s.getBoard, http.MethodGet, "/api/boards/"+id, map[string]string{"id": id}, nil, &view); err != nil {
		return err
	}
	o := s.restOut(r)
	columns := []any{}
	for _, c := range view.Columns {
		statuses := []any{}
		for _, st := range c.Statuses {
			statuses = append(statuses, map[string]any{"id": st.ID, "self": o.api + "/status/" + st.ID})
		}
		col := map[string]any{"name": c.Name, "statuses": statuses}
		if c.Min != nil {
			col["min"] = *c.Min
		}
		if c.Max != nil {
			col["max"] = *c.Max
		}
		columns = append(columns, col)
	}
	out := o.agileBoard(sum)
	out["filter"] = map[string]any{"id": o.id(view.Board.FilterID), "self": o.api + "/filter/" + o.id(view.Board.FilterID)}
	out["columnConfig"] = map[string]any{"columns": columns, "constraintType": "issueCount"}
	out["estimation"] = map[string]any{"type": "none"}
	if f := view.Board.EstimateField; f != nil {
		name := *f
		for _, ef := range view.EstimateFields {
			if ef.ID == *f {
				name = ef.Name
			}
		}
		if *f == "timeoriginalestimate" {
			name = o.msg(systemFieldName("timeoriginalestimate"))
		}
		out["estimation"] = map[string]any{"type": "field", "field": map[string]any{"fieldId": *f, "displayName": name}}
	}
	fields, err := s.customFields(r.Context())
	if err != nil {
		return err
	}
	if rank, ok := fieldOfKind(fields, "gh-lexo-rank"); ok {
		out["ranking"] = map[string]any{"rankCustomFieldId": rank.ID}
	}
	return writeJSON(w, http.StatusOK, out)
}

func systemFieldName(id string) i18n.Message {
	f, _ := systemField(id)
	return f.name
}

// agileBoardSprints lists a Scrum board's sprints, optionally of some states
// (state=active,future).
func (s *Server) agileBoardSprints(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, _, err := s.pathBoard(r, sub)
	if err != nil {
		return err
	}
	if b.Kind != "scrum" {
		return fail(http.StatusBadRequest, "Un tableau Kanban n'a pas de sprints.")
	}
	var states []string
	for v := range strings.SplitSeq(r.URL.Query().Get("state"), ",") {
		if v = strings.ToLower(strings.TrimSpace(v)); v == "active" || v == "future" || v == "closed" {
			states = append(states, "'"+v+"'")
		}
	}
	where := "TRUE"
	if len(states) > 0 {
		where = "state IN (" + strings.Join(states, ", ") + ")"
	}
	sprints, err := s.boardSprints(r.Context(), b.ID, where)
	if err != nil {
		return err
	}
	// Oldest first, as Jira: closed, then active, then future.
	order := map[string]int{"closed": 0, "active": 1, "future": 2}
	slices.SortStableFunc(sprints, func(a, b sprintOut) int { return order[a.State] - order[b.State] })
	o := s.restOut(r)
	out := make([]map[string]any, 0, len(sprints))
	for _, sp := range sprints {
		out = append(out, o.agileSprint(sp, b.ID))
	}
	startAt, maxResults := paging(r, 50, 50)
	return writeJSON(w, http.StatusOK, agilePage(out, startAt, maxResults))
}

// agileIssues answers a page of issues matching a condition (aliases i, p,
// pr), narrowed by the request's jql, in board order.
func (s *Server) agileIssues(w http.ResponseWriter, r *http.Request, sub *perm.Subject, where string, args []any) error {
	ctx := r.Context()
	q := r.URL.Query()
	if src := strings.TrimSpace(q.Get("jql")); src != "" {
		cond, cargs, _, err := s.compileJQL(ctx, sub, src)
		if err != nil {
			return err
		}
		where += " AND (" + cond + ")"
		args = append(args, cargs...)
	}
	startAt, maxResults := paging(r, 50, 1000)
	rows, total, err := s.searchRows(ctx, where, args, "i.rank_key IS NULL, i.rank_key", startAt, maxResults)
	if err != nil {
		return err
	}
	issues, _, err := s.jiraIssues(ctx, s.restOut(r), sub, rows, parseFieldSelection(q["fields"], "*navigable"),
		expandSet(q["expand"]...), true)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"expand": "schema,names", "startAt": startAt,
		"maxResults": maxResults, "total": total, "issues": issues})
}

// agileBoardIssues lists the issues of a board: its filter's, in its columns.
func (s *Server) agileBoardIssues(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, _, err := s.pathBoard(r, sub)
	if err != nil {
		return err
	}
	where, args, err := s.boardWhere(r.Context(), sub, b, nil)
	if err != nil {
		return err
	}
	return s.agileIssues(w, r, sub, where, args)
}

// agileBacklog lists a board's backlog as the interface shows it: issues
// that are neither done, sub-tasks, nor in an open sprint of the board.
func (s *Server) agileBacklog(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	b, _, err := s.pathBoard(r, sub)
	if err != nil {
		return err
	}
	where, args, err := s.boardWhere(r.Context(), sub, b, nil)
	if err != nil {
		return err
	}
	where += ` AND i.parent_id IS NULL AND i.status_id NOT IN (SELECT id FROM statuses WHERE category = 'done')
		AND NOT EXISTS (SELECT 1 FROM sprint_issues si JOIN sprints sp ON sp.id = si.sprint_id
			WHERE si.issue_id = i.id AND sp.board_id = ? AND sp.state <> 'closed')`
	return s.agileIssues(w, r, sub, where, append(args, b.ID))
}

// pathSprint loads the sprint of the path, if the user sees its board.
func (s *Server) pathSprint(r *http.Request, sub *perm.Subject) (sprintOut, int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return sprintOut{}, 0, errNotFound
	}
	var board sql.NullInt64
	err = s.db.QueryRowContext(r.Context(), `SELECT board_id FROM sprints WHERE id = ?`, id).Scan(&board)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !board.Valid) {
		return sprintOut{}, 0, errNotFound
	}
	if err != nil {
		return sprintOut{}, 0, err
	}
	if _, err := s.boardByID(r.Context(), sub, board.Int64); err != nil {
		return sprintOut{}, 0, err
	}
	sprints, err := s.boardSprints(r.Context(), board.Int64, "id = "+strconv.FormatInt(id, 10))
	if err != nil || len(sprints) == 0 {
		return sprintOut{}, 0, errNotFound
	}
	return sprints[0], board.Int64, nil
}

func (s *Server) agileSprint(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	sp, board, err := s.pathSprint(r, sub)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, s.restOut(r).agileSprint(sp, board))
}

// agileSprintIssues lists the issues in a sprint that the user sees.
func (s *Server) agileSprintIssues(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	sp, _, err := s.pathSprint(r, sub)
	if err != nil {
		return err
	}
	vis, args := sub.VisibleSQL("i")
	return s.agileIssues(w, r, sub, vis+` AND EXISTS (SELECT 1 FROM sprint_issues si WHERE si.issue_id = i.id
		AND si.sprint_id = ?)`, append(args, sp.ID))
}

// agileIssueList reads the issues of a request body ({"issues": [keys or
// ids]}), all of which the user must see and may edit.
func (s *Server) agileIssueList(ctx context.Context, sub *perm.Subject, raw []string) ([]*issueRow, error) {
	if len(raw) == 0 || len(raw) > 50 {
		return nil, fieldError("issues", fail(http.StatusBadRequest, "Entre 1 et 50 tickets à la fois."))
	}
	var out []*issueRow
	for _, v := range raw {
		r := ref{key: v}
		if _, err := strconv.ParseInt(v, 10, 64); err == nil {
			r = ref{id: v}
		}
		iss, err := s.refIssue(ctx, sub, r)
		if err != nil {
			return nil, fieldError("issues", err)
		}
		pi := iss.perm()
		if !sub.Can(perm.Edit, iss.ProjectID, &pi) {
			return nil, errForbidden
		}
		out = append(out, iss)
	}
	return out, nil
}

type agileMoveInput struct {
	Issues          []string `json:"issues"`
	RankBeforeIssue string   `json:"rankBeforeIssue"`
	RankAfterIssue  string   `json:"rankAfterIssue"`
}

// agileMoveToSprint puts issues in an open sprint, as dragging them on the
// board does.
func (s *Server) agileMoveToSprint(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	sp, _, err := s.pathSprint(r, sub)
	if err != nil {
		return err
	}
	b, err := s.sprintBoard(ctx, sub, sp.ID)
	if err != nil {
		return err
	}
	var in agileMoveInput
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	issues, err := s.agileIssueList(ctx, sub, in.Issues)
	if err != nil {
		return err
	}
	for _, iss := range issues {
		if err := s.setSprint(ctx, sub, b, iss, sp.ID); err != nil {
			return err
		}
		s.publish(iss, "issue", sub.UserID)
	}
	if in.RankBeforeIssue != "" || in.RankAfterIssue != "" {
		if err := s.rankIssues(ctx, sub, issues, in.RankBeforeIssue, in.RankAfterIssue); err != nil {
			return err
		}
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// agileMoveToBacklog takes issues out of their open sprint.
func (s *Server) agileMoveToBacklog(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	var in agileMoveInput
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	issues, err := s.agileIssueList(ctx, sub, in.Issues)
	if err != nil {
		return err
	}
	for _, iss := range issues {
		if err := s.setSprint(ctx, sub, &boardRow{}, iss, 0); err != nil {
			return err
		}
		s.publish(iss, "issue", sub.UserID)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// agileRank moves issues, in the given order, just before or just after
// another one in the board order.
func (s *Server) agileRank(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	var in agileMoveInput
	if err := readRESTJSON(r, &in); err != nil {
		return err
	}
	if (in.RankBeforeIssue == "") == (in.RankAfterIssue == "") {
		return fail(http.StatusBadRequest, "Indiquez rankBeforeIssue ou rankAfterIssue.")
	}
	issues, err := s.agileIssueList(ctx, sub, in.Issues)
	if err != nil {
		return err
	}
	if err := s.rankIssues(ctx, sub, issues, in.RankBeforeIssue, in.RankAfterIssue); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// rankIssues gives issues consecutive ranks just before (before) or just
// after (after) another issue, which the user must see.
func (s *Server) rankIssues(ctx context.Context, sub *perm.Subject, issues []*issueRow, before, after string) error {
	anchorKey := before + after
	r := ref{key: anchorKey}
	if _, err := strconv.ParseInt(anchorKey, 10, 64); err == nil {
		r = ref{id: anchorKey}
	}
	anchor, err := s.refIssue(ctx, sub, r)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(issues, func(iss *issueRow) bool { return iss.ID == anchor.ID }) {
		return fail(http.StatusBadRequest, "Un ticket ne peut pas être classé par rapport à lui-même.")
	}
	var lower, upper string
	switch {
	case !anchor.Rank.Valid:
		// An unranked issue sorts last: go to the end.
		err = s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(rank_key), '') FROM issues`).Scan(&lower)
	case before != "":
		upper = anchor.Rank.String
		err = s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(rank_key), '') FROM issues WHERE rank_key < ?`, upper).
			Scan(&lower)
	default:
		lower = anchor.Rank.String
		err = s.db.QueryRowContext(ctx, `SELECT COALESCE(MIN(rank_key), '') FROM issues WHERE rank_key > ?`, lower).
			Scan(&upper)
	}
	if err != nil {
		return err
	}
	for _, iss := range issues {
		rank := rankBetween(lower, upper)
		if _, err := s.db.ExecContext(ctx, `UPDATE issues SET rank_key = ? WHERE id = ?`, rank, iss.ID); err != nil {
			return err
		}
		lower = rank
		s.publish(iss, "issue", sub.UserID)
	}
	return nil
}

// pathEpic loads the epic of the path (key or id), if the user sees it.
func (s *Server) pathEpic(r *http.Request, sub *perm.Subject) (*issueRow, *catalog, error) {
	iss, err := s.restIssue(r, sub)
	if err != nil {
		return nil, nil, err
	}
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return nil, nil, err
	}
	if !isEpicType(cat.typeOf(iss.TypeID)) {
		return nil, nil, failf(http.StatusNotFound, "%s n'est pas une épopée.", iss.Key())
	}
	return iss, cat, nil
}

func (s *Server) agileEpic(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	epic, cat, err := s.pathEpic(r, sub)
	if err != nil {
		return err
	}
	// The epic's name is its "Epic Name" field, if it has one.
	name := ""
	fields, err := s.customFields(r.Context())
	if err != nil {
		return err
	}
	if f, ok := fieldOfKind(fields, "gh-epic-label"); ok {
		s.db.QueryRowContext(r.Context(), `SELECT COALESCE(string_value, '') FROM custom_field_values
			WHERE issue_id = ? AND field_id = ? LIMIT 1`, epic.ID, f.ID).Scan(&name)
	}
	return writeJSON(w, http.StatusOK, s.restOut(r).agileEpic(cat, epic, name))
}

// agileEpicIssues lists the issues of an epic that the user sees.
func (s *Server) agileEpicIssues(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	epic, _, err := s.pathEpic(r, sub)
	if err != nil {
		return err
	}
	vis, args := sub.VisibleSQL("i")
	return s.agileIssues(w, r, sub, vis+" AND i.epic_id = ?", append(args, epic.ID))
}
