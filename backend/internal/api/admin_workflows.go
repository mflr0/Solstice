package api

// Global workflows: workflows of no project, which global administrators
// manage. Projects never follow one, they get a copy. One global workflow
// is the default, so there is always one left.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"strings"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/issue"
)

type globalWorkflowOut struct {
	ID          int64                `json:"id"`
	Name        string               `json:"name"`
	Default     bool                 `json:"default"`
	Initial     statusRef            `json:"initial"`
	Statuses    []statusRef          `json:"statuses"`
	Transitions []workflowTransition `json:"transitions"`
}

// globalWorkflowSeeds are the global workflows of a new instance, the first
// one the default.
var globalWorkflowSeeds = []struct {
	name     i18n.Message
	statuses []templateStatus
}{
	{i18n.M(i18n.DefaultWorkflowSoftware), []templateStatus{{i18n.M(i18n.DefaultStatusToDo), issue.Todo},
		{i18n.M(i18n.DefaultStatusInProgress), issue.Doing}, {i18n.M(i18n.DefaultStatusInReview), issue.Doing},
		{i18n.M(i18n.DefaultStatusDone), issue.Done}}},
	{i18n.M(i18n.DefaultWorkflowSimple), []templateStatus{{i18n.M(i18n.DefaultStatusToDo), issue.Todo},
		{i18n.M(i18n.DefaultStatusInProgress), issue.Doing}, {i18n.M(i18n.DefaultStatusDone), issue.Done}}},
}

const globalWorkflowsLock = "solstice-global-workflows"

// seedGlobalWorkflows creates the global workflows, named in the instance's
// language, when there is none. The last one cannot be deleted, so this
// happens once. The lock keeps servers starting together from both seeding.
func (s *Server) seedGlobalWorkflows(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, 30)`, globalWorkflowsLock).Scan(&got); err != nil {
		return err
	}
	if got.Int64 != 1 {
		return errors.New("global workflows: lock not obtained")
	}
	defer func() {
		// A lock that cannot be released must not go back to the pool.
		var released sql.NullInt64
		if conn.QueryRowContext(context.WithoutCancel(ctx), `SELECT RELEASE_LOCK(?)`, globalWorkflowsLock).
			Scan(&released) != nil {
			conn.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflows WHERE project_id IS NULL`).Scan(&n); err != nil || n > 0 {
		return err
	}
	lang := s.defaultLanguage()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, wf := range globalWorkflowSeeds {
		if err := writeNewWorkflow(ctx, tx, nil, wf.name.In(lang), i == 0,
			templateStatuses(wf.statuses, lang)); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.forgetCatalogue()
	return nil
}

func (s *Server) globalWorkflows(ctx context.Context) ([]globalWorkflowOut, error) {
	cat, err := s.catalogue(ctx)
	if err != nil {
		return nil, err
	}
	list, _, err := s.loadWorkflows(ctx, cat, 0)
	if err != nil {
		return nil, err
	}
	out := make([]globalWorkflowOut, len(list))
	for i, w := range list {
		out[i] = globalWorkflowOut{w.ID, w.Name, w.Default, w.Initial, w.Statuses, w.Transitions}
	}
	return out, nil
}

func (s *Server) writeGlobalWorkflows(w http.ResponseWriter, r *http.Request) error {
	out, err := s.globalWorkflows(r.Context())
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) adminWorkflows(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	return s.writeGlobalWorkflows(w, r)
}

// globalWorkflowForEdit loads the global workflow of the path.
func (s *Server) globalWorkflowForEdit(r *http.Request) (id int64, isDefault bool, err error) {
	if err := requireAdmin(r); err != nil {
		return 0, false, err
	}
	if id, err = pathID(r, "id"); err != nil {
		return 0, false, err
	}
	err = s.db.QueryRowContext(r.Context(), `SELECT is_default FROM workflows WHERE id = ? AND project_id IS NULL`, id).
		Scan(&isDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, errNotFound
	}
	return id, isDefault, err
}

// createGlobalWorkflow adds a global workflow, a copy of another (by default
// the default one).
func (s *Server) createGlobalWorkflow(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ctx := r.Context()
	var in struct {
		Name     string `json:"name"`
		CopyFrom int64  `json:"copyFrom"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return fail(http.StatusBadRequest, i18n.WorkflowNameRequired)
	}
	var source int64
	var initial string
	err := s.db.QueryRowContext(ctx, `SELECT id, initial_status_id FROM workflows WHERE project_id IS NULL
		AND (id = ? OR (? = 0 AND is_default)) LIMIT 1`, in.CopyFrom, in.CopyFrom).Scan(&source, &initial)
	if errors.Is(err, sql.ErrNoRows) {
		return fail(http.StatusBadRequest, i18n.WorkflowGlobalCopyNotFound)
	}
	if err != nil {
		return err
	}
	id, err := s.copyWorkflow(ctx, nil, name, initial, source)
	if err != nil {
		return err
	}
	s.auditWorkflow(r, audit.WorkflowCreate, id, 0, name, map[string]any{"copyFrom": in.CopyFrom})
	return writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

// updateGlobalWorkflow replaces a global workflow's name, statuses,
// transitions and initial status, and makes it the default if asked. No
// issue follows it, so nothing has to move.
func (s *Server) updateGlobalWorkflow(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := &workflowUpdate{}
	var err error
	if u.id, u.isDefault, err = s.globalWorkflowForEdit(r); err != nil {
		return err
	}
	if err := readJSON(r, &u.in); err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	if err := u.check(cat); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := u.save(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	in := u.in
	s.auditWorkflow(r, audit.WorkflowUpdate, u.id, 0, u.name, map[string]any{"statuses": in.Statuses,
		"transitions": len(in.Transitions), "default": in.Default})
	return s.writeGlobalWorkflows(w, r)
}

// deleteGlobalWorkflow removes a global workflow other than the default,
// which also keeps the last one.
func (s *Server) deleteGlobalWorkflow(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id, isDefault, err := s.globalWorkflowForEdit(r)
	if err != nil {
		return err
	}
	if isDefault {
		return fail(http.StatusConflict, i18n.WorkflowDefaultDelete)
	}
	name, err := s.workflowName(ctx, id)
	if err != nil {
		return err
	}
	// Made the default meanwhile: refused as well.
	res, err := s.db.ExecContext(ctx, `DELETE FROM workflows WHERE id = ? AND project_id IS NULL AND NOT is_default`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		if err != nil {
			return err
		}
		return fail(http.StatusConflict, i18n.WorkflowDefaultDelete)
	}
	s.auditWorkflow(r, audit.WorkflowDelete, id, 0, name, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
