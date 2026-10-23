package api

// Suggestions for search filters and query autocompletion: values the user
// can pick without knowing them by heart.

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"solstice/internal/issue"
	"solstice/internal/sqlq"
)

const maxSuggestions = 30

// labels lists the labels used on issues the user sees, most used first,
// optionally within a project and containing q.
func (s *Server) labels(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	vis, args := sub.VisibleSQL("i")
	where := vis + " AND l.field_id IS NULL"
	if key := r.URL.Query().Get("project"); key != "" {
		projectID, err := s.projectByKey(ctx, key)
		if err != nil {
			return writeJSON(w, http.StatusOK, []string{})
		}
		where += " AND i.project_id = ?"
		args = append(args, projectID)
	}
	if q != "" {
		// Labels are case-sensitive (binary collation): match case-insensitively.
		where += " AND LOWER(l.label) LIKE ?"
		args = append(args, "%"+sqlq.EscapeLike(strings.ToLower(q))+"%")
	}
	out := []string{}
	err = sqlq.Each(ctx, s.db, `SELECT l.label FROM labels l JOIN issues i ON i.id = l.issue_id
		JOIN projects p ON p.id = i.project_id WHERE `+where+` GROUP BY l.label ORDER BY COUNT(*) DESC, l.label LIMIT ?`,
		func(rows *sql.Rows) error {
			var label string
			if err := rows.Scan(&label); err != nil {
				return err
			}
			out = append(out, label)
			return nil
		}, append(args, maxSuggestions)...)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// sprints lists the sprints of the boards the user sees: the active ones
// first, then future ones, then the most recently closed, matching q.
func (s *Server) sprints(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	visible, err := s.visibleFilterIDs(ctx, sub)
	if err != nil {
		return err
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	type sprintSuggestion struct {
		sprintRef
		BoardName string `json:"boardName"`
	}
	out := []sprintSuggestion{}
	err = sqlq.Each(ctx, s.db, `SELECT sp.id, sp.name, sp.state, b.id, b.name, b.filter_id FROM sprints sp
		JOIN boards b ON b.id = sp.board_id WHERE LOWER(sp.name) LIKE ?
		ORDER BY sp.state = 'closed', sp.state <> 'active', COALESCE(sp.completed_at, sp.start_at) DESC, sp.id DESC`,
		func(rows *sql.Rows) error {
			var sp sprintSuggestion
			var filter int64
			if err := rows.Scan(&sp.ID, &sp.Name, &sp.State, &sp.Board, &sp.BoardName, &filter); err != nil {
				return err
			}
			if visible[filter] && len(out) < maxSuggestions {
				out = append(out, sp)
			}
			return nil
		}, "%"+sqlq.EscapeLike(q)+"%")
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// epics lists the epics the user sees, matching q (key or summary), most
// recently updated first.
func (s *Server) epics(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	var types []any
	for _, t := range cat.IssueTypes {
		if isEpicType(t) {
			types = append(types, t.ID)
		}
	}
	type epicOut struct {
		Key     string    `json:"key"`
		Summary string    `json:"summary"`
		Status  statusRef `json:"status"`
		Project string    `json:"project"`
	}
	out := []epicOut{}
	if len(types) == 0 {
		return writeJSON(w, http.StatusOK, out)
	}
	vis, args := sub.VisibleSQL("i")
	where := vis + " AND i.issue_type_id IN (" + sqlq.Marks(len(types)) + ")"
	args = append(args, types...)
	if key := r.URL.Query().Get("project"); key != "" {
		where += " AND " + sqlq.ProjectKey("p", "?")
		args = append(args, strings.ToUpper(key), strings.ToUpper(key))
	}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		where += " AND (CONCAT(p.pkey, '-', i.num) LIKE ? OR LOWER(i.summary) LIKE ?)"
		like := "%" + sqlq.EscapeLike(strings.ToLower(q)) + "%"
		args = append(args, strings.ToUpper(like), like)
	}
	err = sqlq.Each(ctx, s.db, `SELECT p.pkey, i.num, i.summary, i.status_id FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE `+where+` ORDER BY i.updated_at DESC LIMIT ?`, func(rows *sql.Rows) error {
		var e epicOut
		var num int64
		var status string
		if err := rows.Scan(&e.Project, &num, &e.Summary, &status); err != nil {
			return err
		}
		e.Key = e.Project + "-" + strconv.FormatInt(num, 10)
		e.Status = cat.statusOf(status)
		out = append(out, e)
		return nil
	}, append(args, maxSuggestions)...)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// jqlFields lists the custom fields, for query autocompletion: how JQL names
// each one, and its kind (the last part of its type: float, select,
// datepicker, userpicker…).
func (s *Server) jqlFields(w http.ResponseWriter, r *http.Request) error {
	if _, err := s.subject(r); err != nil {
		return err
	}
	type fieldOut struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Kind string `json:"kind"`
		JQL  string `json:"jql"`
	}
	out := []fieldOut{}
	err := sqlq.Each(r.Context(), s.db, `SELECT id, name, type_key FROM custom_fields ORDER BY name`, func(rows *sql.Rows) error {
		var f fieldOut
		var typeKey string
		if err := rows.Scan(&f.ID, &f.Name, &typeKey); err != nil {
			return err
		}
		f.Kind = issue.FieldKind(typeKey)
		f.JQL = `"` + strings.ReplaceAll(f.Name, `"`, `\"`) + `"`
		out = append(out, f)
		return nil
	})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

type projectPart struct {
	ID       int64        `json:"id"`
	Name     string       `json:"name"`
	Project  projectBrief `json:"project"`
	Released *bool        `json:"released,omitempty"`
	Archived *bool        `json:"archived,omitempty"`
}

// projectParts lists versions or components of the projects the user sees,
// matching q and optionally within one project.
func (s *Server) projectParts(table string) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()
		sub, err := s.subject(r)
		if err != nil {
			return err
		}
		cols, order := "x.archived, NULL", "x.name"
		if table == "versions" {
			cols, order = "x.archived, x.released", "x.released, COALESCE(x.release_date, '9999-12-31') DESC, x.name"
		}
		// Archived projects stay out of pickers.
		where, args := "LOWER(x.name) LIKE ? AND NOT p.archived", []any{"%" + sqlq.EscapeLike(strings.ToLower(r.URL.Query().Get("q"))) + "%"}
		if key := r.URL.Query().Get("project"); key != "" {
			where += " AND " + sqlq.ProjectKey("p", "?")
			args = append(args, strings.ToUpper(key), strings.ToUpper(key))
		}
		out := []projectPart{}
		err = sqlq.Each(ctx, s.db, `SELECT x.id, x.name, p.id, p.pkey, p.name, `+cols+` FROM `+table+` x
			JOIN projects p ON p.id = x.project_id WHERE `+where+` ORDER BY `+order, func(rows *sql.Rows) error {
			var part projectPart
			var archived, released sql.NullBool
			if err := rows.Scan(&part.ID, &part.Name, &part.Project.ID, &part.Project.Key, &part.Project.Name,
				&archived, &released); err != nil {
				return err
			}
			if !sub.CanBrowseProject(part.Project.ID) || len(out) >= 50 {
				return nil
			}
			if archived.Valid {
				part.Archived = &archived.Bool
			}
			if released.Valid {
				part.Released = &released.Bool
			}
			out = append(out, part)
			return nil
		}, args...)
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, out)
	}
}

// Facets: how many of a search's issues have each value of a field.
var facetColumns = map[string]string{
	"project": "p.pkey", "status": "i.status_id", "type": "i.issue_type_id", "priority": "i.priority_id",
	"resolution": "i.resolution_id", "assignee": "i.assignee_id", "reporter": "i.reporter_id",
}

// facets counts, for the search given as for /api/issues (jql or q,
// project), the issues per value of each requested field (fields=status,
// assignee,labels…). An empty value is counted under "".
func (s *Server) facets(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	params := r.URL.Query()
	where, args, _, err := s.searchCondition(ctx, sub, params)
	if err != nil {
		return err
	}
	from := ` FROM issues i JOIN projects p ON p.id = i.project_id WHERE ` + where
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return err
	}
	out := map[string]map[string]int{}
	for field := range strings.SplitSeq(params.Get("fields"), ",") {
		field = strings.TrimSpace(field)
		var q string
		switch {
		case field == "labels":
			q = `SELECT l.label, COUNT(DISTINCT i.id) FROM labels l JOIN issues i ON i.id = l.issue_id
				JOIN projects p ON p.id = i.project_id WHERE l.field_id IS NULL AND ` + where + ` GROUP BY l.label`
		case facetColumns[field] != "":
			col := facetColumns[field]
			q = `SELECT CAST(` + col + ` AS CHAR), COUNT(*)` + from + ` GROUP BY ` + col
		default:
			continue
		}
		counts := map[string]int{}
		if err := sqlq.Each(ctx, s.db, q, func(rows *sql.Rows) error {
			var v sql.NullString
			var n int
			if err := rows.Scan(&v, &n); err != nil {
				return err
			}
			counts[v.String] += n
			return nil
		}, args...); err != nil {
			return err
		}
		out[field] = counts
	}
	return writeJSON(w, http.StatusOK, map[string]any{"total": total, "facets": out})
}
