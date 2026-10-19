package api

// The shared statuses, for the administration: adding one, renaming it,
// ordering the list, saying where it is used and deleting it when nothing
// uses it any more. Statuses belong to no project — every project's
// workflows pick from the same catalogue — so reading and changing them is
// for global administrators, while a project administrator may add one for
// the workflows of their project.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/sqlq"
)

// statusUseWorkflows is every (workflow, status) pair of a status used by a
// workflow: a status of the workflow's list, an end of its transitions or
// its initial status. A caller wraps it in a subquery and puts the filter on
// the status outside it (u.status_id = …), so the same text answers for a
// column of an outer row and for a placeholder.
const statusUseWorkflows = `SELECT workflow_id, status_id FROM workflow_statuses
	UNION ALL SELECT workflow_id, from_status_id FROM workflow_transitions
	UNION ALL SELECT workflow_id, to_status_id FROM workflow_transitions
	UNION ALL SELECT id, initial_status_id FROM workflows`

// statusUseBoardColumns is the FROM clause of the board columns holding a
// status: their column and board joined in, filtered on expr, the SQL
// expression of the status id ("?" standing for one argument). Foreign keys
// give every row one column and one board, so counting rows counts columns.
func statusUseBoardColumns(expr string) string {
	return `board_column_statuses bs
		JOIN board_columns c ON c.id = bs.column_id
		JOIN boards b ON b.id = c.board_id WHERE bs.status_id = ` + expr
}

// createStatus adds a status to the shared catalogue, or returns the one of
// the same name if there is one. Administrators of the project it is made
// for may, since they build that project's workflows.
func (s *Server) createStatus(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	var in struct {
		Name     string               `json:"name"`
		Category issue.StatusCategory `json:"category"`
		Project  string               `json:"project"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if !sub.Admin {
		projectID, err := s.projectByKey(ctx, in.Project)
		if errors.Is(err, errNotFound) {
			return errForbidden
		}
		if err != nil {
			return err
		}
		if !sub.CanBrowseProject(projectID) || !s.administers(sub, projectID) {
			return errForbidden
		}
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return fail(http.StatusBadRequest, i18n.StatusNameRequired)
	}
	if !in.Category.Valid() {
		return fail(http.StatusBadRequest, i18n.StatusCategoryInvalid)
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	for _, st := range cat.Statuses {
		if strings.EqualFold(st.Name, name) {
			return writeJSON(w, http.StatusOK, st)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Numeric ids, continuing Jira's sequence for custom statuses.
	id, err := nextCatalogID(ctx, tx, "statuses")
	if err != nil {
		return err
	}
	st := statusRef{ID: id, Name: sqlq.Truncate(name, varcharWidth), Category: in.Category}
	if _, err := tx.ExecContext(ctx, `INSERT INTO statuses (id, name, category, position)
		SELECT ?, ?, ?, COALESCE(MAX(position), 0) + 1 FROM statuses`, st.ID, st.Name, st.Category); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.forgetCatalogue()
	s.auditLog(r, audit.StatusCreate, audit.Target{Type: "status", ID: st.ID, Label: st.Name},
		map[string]any{"category": st.Category})
	return writeJSON(w, http.StatusCreated, st)
}

// statusForAdmin loads a status of the catalogue for a global administrator:
// statuses are shared by every project.
func (s *Server) statusForAdmin(r *http.Request) (statusRef, error) {
	sub, err := s.subject(r)
	if err != nil {
		return statusRef{}, err
	}
	if !sub.Admin {
		return statusRef{}, errForbidden
	}
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return statusRef{}, err
	}
	st, ok := cat.status[r.PathValue("id")]
	if !ok {
		return statusRef{}, errNotFound
	}
	return st, nil
}

// updateStatus renames a status or changes its category, for every project.
func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	st, err := s.statusForAdmin(r)
	if err != nil {
		return err
	}
	before := st
	var in struct {
		Name     *string               `json:"name"`
		Category *issue.StatusCategory `json:"category"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return fail(http.StatusBadRequest, i18n.StatusNameRequired)
		}
		cat, err := s.catalogue(ctx)
		if err != nil {
			return err
		}
		for _, other := range cat.Statuses {
			if other.ID != st.ID && strings.EqualFold(other.Name, name) {
				return failf(http.StatusConflict, i18n.StatusNameTaken, other.Name)
			}
		}
		st.Name = sqlq.Truncate(name, varcharWidth)
	}
	if in.Category != nil {
		if !in.Category.Valid() {
			return fail(http.StatusBadRequest, i18n.StatusCategoryInvalid)
		}
		st.Category = *in.Category
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE statuses SET name = ?, category = ? WHERE id = ?`,
		st.Name, st.Category, st.ID); err != nil {
		return err
	}
	s.forgetCatalogue()
	if st != before {
		s.auditLog(r, audit.StatusUpdate, audit.Target{Type: "status", ID: st.ID, Label: st.Name}, map[string]any{
			"from": map[string]any{"name": before.Name, "category": before.Category},
			"to":   map[string]any{"name": st.Name, "category": st.Category}})
	}
	return writeJSON(w, http.StatusOK, st)
}

// deleteStatus removes a status nothing uses any more: no issue, workflow or
// board column. History keeps its name.
func (s *Server) deleteStatus(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	st, err := s.statusForAdmin(r)
	if err != nil {
		return err
	}
	var issues, workflows, columns int
	if err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM issues WHERE status_id = ?),
		(SELECT COUNT(DISTINCT u.workflow_id) FROM (`+statusUseWorkflows+`) u WHERE u.status_id = ?),
		(SELECT COUNT(*) FROM `+statusUseBoardColumns("?")+`)`,
		st.ID, st.ID, st.ID).Scan(&issues, &workflows, &columns); err != nil {
		return err
	}
	if issues+workflows+columns > 0 {
		return &httpError{status: http.StatusConflict, extra: map[string]any{
			"usage":  map[string]int{"issues": issues, "workflows": workflows, "boardColumns": columns},
			"status": st,
		}, msg: i18n.M(i18n.StatusInUse)}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM statuses WHERE id = ?`, st.ID); err != nil {
		return err
	}
	s.forgetCatalogue()
	s.auditLog(r, audit.StatusDelete, audit.Target{Type: "status", ID: st.ID, Label: st.Name}, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type statusAdminOut struct {
	statusRef
	Position int            `json:"position"`
	Usage    map[string]int `json:"usage"`
}

func (s *Server) statusList(ctx context.Context) ([]statusAdminOut, error) {
	out := []statusAdminOut{}
	err := sqlq.Each(ctx, s.db, `SELECT s.id, s.name, s.category, s.position,
		(SELECT COUNT(*) FROM issues i WHERE i.status_id = s.id),
		(SELECT COUNT(DISTINCT u.workflow_id) FROM (`+statusUseWorkflows+`) u WHERE u.status_id = s.id),
		(SELECT COUNT(*) FROM `+statusUseBoardColumns("s.id")+`)
		FROM statuses s ORDER BY s.position, s.name`, func(rows *sql.Rows) error {
		var st statusAdminOut
		var issues, workflows, columns int
		if err := rows.Scan(&st.ID, &st.Name, &st.Category, &st.Position, &issues, &workflows, &columns); err != nil {
			return err
		}
		st.Usage = map[string]int{"issues": issues, "workflows": workflows, "boardColumns": columns}
		out = append(out, st)
		return nil
	})
	return out, err
}

func (s *Server) adminStatuses(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	out, err := s.statusList(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// statusUsageOut tells where a status is used: the workflows (with their
// project), the board columns and, per project, the issues in it.
type statusUsageOut struct {
	Workflows []statusWorkflow `json:"workflows"`
	Boards    []statusBoard    `json:"boards"`
	Projects  []statusProject  `json:"projects"`
}

type statusWorkflow struct {
	ID      int64        `json:"id"`
	Name    string       `json:"name"`
	Project projectBrief `json:"project"`
}

type statusBoard struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Column string `json:"column"`
}

type statusProject struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Issues int    `json:"issues"`
}

func (s *Server) adminStatusUsage(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ctx := r.Context()
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	id := r.PathValue("id")
	if _, ok := cat.status[id]; !ok {
		return errNotFound
	}
	out := statusUsageOut{Workflows: []statusWorkflow{}, Boards: []statusBoard{}, Projects: []statusProject{}}
	if err := sqlq.Each(ctx, s.db, `SELECT w.id, w.name, p.id, p.pkey, p.name FROM workflows w
		JOIN projects p ON p.id = w.project_id WHERE w.id IN (
			SELECT u.workflow_id FROM (`+statusUseWorkflows+`) u WHERE u.status_id = ?)
		ORDER BY p.pkey, w.name`, func(rows *sql.Rows) error {
		var w statusWorkflow
		if err := rows.Scan(&w.ID, &w.Name, &w.Project.ID, &w.Project.Key, &w.Project.Name); err != nil {
			return err
		}
		out.Workflows = append(out.Workflows, w)
		return nil
	}, id); err != nil {
		return err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT b.id, b.name, c.name FROM `+statusUseBoardColumns("?")+`
		ORDER BY b.name, c.position`, func(rows *sql.Rows) error {
		var b statusBoard
		if err := rows.Scan(&b.ID, &b.Name, &b.Column); err != nil {
			return err
		}
		out.Boards = append(out.Boards, b)
		return nil
	}, id); err != nil {
		return err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT p.pkey, p.name, COUNT(*) FROM issues i
		JOIN projects p ON p.id = i.project_id WHERE i.status_id = ? GROUP BY p.id, p.pkey, p.name
		ORDER BY p.pkey`, func(rows *sql.Rows) error {
		var p statusProject
		if err := rows.Scan(&p.Key, &p.Name, &p.Issues); err != nil {
			return err
		}
		out.Projects = append(out.Projects, p)
		return nil
	}, id); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) orderStatuses(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ids, err := readOrder(r)
	if err != nil {
		return err
	}
	ctx := r.Context()
	list, err := s.statusList(ctx)
	if err != nil {
		return err
	}
	if err := sameIDs(list, ids, func(st statusAdminOut) string { return st.ID }); err != nil {
		return err
	}
	if err := s.setPositions(ctx, "statuses", ids); err != nil {
		return err
	}
	s.forgetCatalogue()
	s.auditLog(r, audit.StatusOrder, audit.Target{Type: "status"}, map[string]any{"ids": ids})
	out, err := s.statusList(ctx)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}
