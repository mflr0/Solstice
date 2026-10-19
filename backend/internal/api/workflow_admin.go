package api

// Editing workflows. A workflow belongs to one project — there are no
// schemes to share it — and lists its statuses and the transitions between
// them. Project administrators edit their project's workflows; statuses are
// a shared catalogue they may add to.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

type workflowTransition struct {
	ID   int64      `json:"id"`
	Name string     `json:"name"`
	From *statusRef `json:"from"` // nil: from any status
	To   statusRef  `json:"to"`
}

type workflowOut struct {
	ID          int64                `json:"id"`
	Name        string               `json:"name"`
	Default     bool                 `json:"default"`
	Initial     statusRef            `json:"initial"`
	Statuses    []statusRef          `json:"statuses"`
	Transitions []workflowTransition `json:"transitions"`
	IssueTypes  []typeRef            `json:"issueTypes"` // the types following it; the default takes the others
	Usage       map[string]int       `json:"usage"`      // issues per status
}

// projectTypes lists the issue types a project uses: those configured for
// it, or every type when none is.
func (s *Server) projectTypes(ctx context.Context, cat *catalog, projectID int64) ([]typeRef, error) {
	var out []typeRef
	err := sqlq.Each(ctx, s.db, `SELECT issue_type_id FROM project_issue_types WHERE project_id = ?`, func(rows *sql.Rows) error {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		out = append(out, cat.typeOf(id))
		return nil
	}, projectID)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		out = slices.Clone(cat.IssueTypes)
	}
	return out, nil
}

// typeWorkflows maps the project's issue types explicitly given another
// workflow than the default.
func (s *Server) typeWorkflows(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, projectID int64) (map[string]int64, error) {
	out := map[string]int64{}
	rows, err := q.QueryContext(ctx, `SELECT issue_type_id, workflow_id FROM workflow_issue_types WHERE project_id = ?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var typ string
		var wf int64
		if err := rows.Scan(&typ, &wf); err != nil {
			return nil, err
		}
		out[typ] = wf
	}
	return out, rows.Err()
}

func (s *Server) projectWorkflows(ctx context.Context, cat *catalog, projectID int64) ([]*workflowOut, error) {
	list := []*workflowOut{}
	byID := map[int64]*workflowOut{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name, is_default, initial_status_id FROM workflows WHERE project_id = ?
		ORDER BY is_default DESC, name, id`, func(rows *sql.Rows) error {
		var w workflowOut
		var initial string
		if err := rows.Scan(&w.ID, &w.Name, &w.Default, &initial); err != nil {
			return err
		}
		w.Initial = cat.statusOf(initial)
		w.Statuses, w.Transitions, w.IssueTypes = []statusRef{}, []workflowTransition{}, []typeRef{}
		w.Usage = map[string]int{}
		list = append(list, &w)
		byID[w.ID] = &w
		return nil
	}, projectID); err != nil {
		return nil, err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT ws.workflow_id, ws.status_id FROM workflow_statuses ws
		JOIN workflows w ON w.id = ws.workflow_id WHERE w.project_id = ? ORDER BY ws.position`, func(rows *sql.Rows) error {
		var wf int64
		var st string
		if err := rows.Scan(&wf, &st); err != nil {
			return err
		}
		byID[wf].Statuses = append(byID[wf].Statuses, cat.statusOf(st))
		return nil
	}, projectID); err != nil {
		return nil, err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT t.workflow_id, t.id, t.name, t.from_status_id, t.to_status_id
		FROM workflow_transitions t JOIN workflows w ON w.id = t.workflow_id WHERE w.project_id = ?
		ORDER BY t.position, t.id`, func(rows *sql.Rows) error {
		var wf int64
		var t workflowTransition
		var from sql.NullString
		var to string
		if err := rows.Scan(&wf, &t.ID, &t.Name, &from, &to); err != nil {
			return err
		}
		if from.Valid {
			st := cat.statusOf(from.String)
			t.From = &st
		}
		t.To = cat.statusOf(to)
		byID[wf].Transitions = append(byID[wf].Transitions, t)
		return nil
	}, projectID); err != nil {
		return nil, err
	}

	explicit, err := s.typeWorkflows(ctx, s.db, projectID)
	if err != nil {
		return nil, err
	}
	var def *workflowOut
	for _, w := range list {
		if w.Default {
			def = w
		}
	}
	follows := func(typ string) *workflowOut {
		if w, ok := byID[explicit[typ]]; ok {
			return w
		}
		return def
	}
	types, err := s.projectTypes(ctx, cat, projectID)
	if err != nil {
		return nil, err
	}
	for _, t := range types {
		if w := follows(t.ID); w != nil {
			w.IssueTypes = append(w.IssueTypes, t)
		}
	}
	err = sqlq.Each(ctx, s.db, `SELECT issue_type_id, status_id, COUNT(*) FROM issues WHERE project_id = ?
		GROUP BY issue_type_id, status_id`, func(rows *sql.Rows) error {
		var typ, st string
		var n int
		if err := rows.Scan(&typ, &st, &n); err != nil {
			return err
		}
		if w := follows(typ); w != nil {
			w.Usage[st] += n
		}
		return nil
	}, projectID)
	return list, err
}

func (s *Server) listWorkflows(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	projectID, err := s.projectByKey(ctx, r.PathValue("key"))
	if err != nil || (!sub.Admin && !sub.CanBrowseProject(projectID)) {
		return errNotFound
	}
	return s.writeWorkflows(w, r, sub, projectID)
}

func (s *Server) writeWorkflows(w http.ResponseWriter, r *http.Request, sub *perm.Subject, projectID int64) error {
	ctx := r.Context()
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	list, err := s.projectWorkflows(ctx, cat, projectID)
	if err != nil {
		return err
	}
	types, err := s.projectTypes(ctx, cat, projectID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{
		"workflows": list, "issueTypes": types, "statuses": cat.Statuses,
		"can": map[string]bool{"edit": s.administers(sub, projectID)},
	})
}

func (s *Server) administers(sub *perm.Subject, projectID int64) bool {
	return sub.Admin || sub.Can(perm.Admin, projectID, nil)
}

// workflowForEdit loads a workflow the user administers.
func (s *Server) workflowForEdit(r *http.Request, sub *perm.Subject) (id, projectID int64, isDefault bool, err error) {
	if id, err = pathID(r, "id"); err != nil {
		return 0, 0, false, err
	}
	err = s.db.QueryRowContext(r.Context(), `SELECT project_id, is_default FROM workflows WHERE id = ?`, id).
		Scan(&projectID, &isDefault)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !sub.Admin && !sub.CanBrowseProject(projectID)) {
		return 0, 0, false, errNotFound
	}
	if err != nil {
		return 0, 0, false, err
	}
	if !s.administers(sub, projectID) {
		return 0, 0, false, errForbidden
	}
	return id, projectID, isDefault, nil
}

type workflowInput struct {
	Name        string   `json:"name"`
	Initial     string   `json:"initial"`
	Statuses    []string `json:"statuses"`
	Transitions []struct {
		ID   int64   `json:"id"`
		Name string  `json:"name"`
		From *string `json:"from"`
		To   string  `json:"to"`
	} `json:"transitions"`
	IssueTypes []string          `json:"issueTypes"` // ignored for the default workflow
	Default    bool              `json:"default"`    // make this the project's default
	Remap      map[string]string `json:"remap"`      // removed status → status its issues move to
	// For issue types leaving this workflow (they go back to the default):
	// their issues' statuses the default lacks → a status of the default.
	RemapDefault map[string]string `json:"remapDefault"`
}

type strandedStatus struct {
	Status statusRef `json:"status"`
	Count  int       `json:"count"`
}

// stranded counts the project's issues matching cond whose status is not in
// keep.
func (s *Server) stranded(ctx context.Context, cat *catalog, projectID int64, cond string, args []any,
	keep map[string]bool) ([]strandedStatus, error) {
	var out []strandedStatus
	err := sqlq.Each(ctx, s.db, `SELECT status_id, COUNT(*) FROM issues WHERE project_id = ? AND `+cond+`
		GROUP BY status_id ORDER BY status_id`, func(rows *sql.Rows) error {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return err
		}
		if !keep[st] {
			out = append(out, strandedStatus{cat.statusOf(st), n})
		}
		return nil
	}, append([]any{projectID}, args...)...)
	return out, err
}

// unmapped returns the stranded statuses remap does not send to a status in
// keep.
func unmapped(lost []strandedStatus, remap map[string]string, keep map[string]bool) []strandedStatus {
	var out []strandedStatus
	for _, l := range lost {
		if to, ok := remap[l.Status.ID]; !ok || !keep[to] {
			out = append(out, l)
		}
	}
	return out
}

// moveStranded moves the project's issues matching cond out of the stranded
// statuses, as remap says, recording each move in the issue's history.
func moveStranded(ctx context.Context, tx *sql.Tx, cat *catalog, projectID, author int64, cond string, args []any,
	lost []strandedStatus, remap map[string]string, now time.Time) error {
	for _, l := range lost {
		to := remap[l.Status.ID]
		issues, err := idsOf(ctx, tx, `SELECT id FROM issues WHERE project_id = ? AND status_id = ? AND `+cond,
			append([]any{projectID, l.Status.ID}, args...)...)
		if err != nil {
			return err
		}
		for _, issue := range issues {
			if _, err := tx.ExecContext(ctx, `UPDATE issues SET status_id = ?, updated_at = ? WHERE id = ?`,
				to, now, issue); err != nil {
				return err
			}
			if err := recordChanges(ctx, tx, issue, author, now, []change{{"status", l.Status.ID, l.Status.Name,
				to, cat.statusOf(to).Name}}); err != nil {
				return err
			}
		}
	}
	return nil
}

// defaultWorkflow returns the project's default workflow and its statuses.
func (s *Server) defaultWorkflow(ctx context.Context, cat *catalog, projectID int64) (id int64, name string,
	statuses []statusRef, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT id, name FROM workflows WHERE project_id = ? AND is_default LIMIT 1`, projectID).
		Scan(&id, &name)
	if err != nil {
		return 0, "", nil, err
	}
	err = sqlq.Each(ctx, s.db, `SELECT status_id FROM workflow_statuses WHERE workflow_id = ? ORDER BY position`,
		func(rows *sql.Rows) error {
			var st string
			if err := rows.Scan(&st); err != nil {
				return err
			}
			statuses = append(statuses, cat.statusOf(st))
			return nil
		}, id)
	return id, name, statuses, err
}

// toDefault checks the issues of types going back to the project's default
// workflow: the statuses it lacks (lost) and, when remap does not place them
// all, the body of the 409 answer (conflict).
func (s *Server) toDefault(ctx context.Context, cat *catalog, projectID int64, types []any,
	remap map[string]string) (lost []strandedStatus, cond string, conflict map[string]any, err error) {
	if len(types) == 0 {
		return nil, "FALSE", nil, nil
	}
	defID, defName, defStatuses, err := s.defaultWorkflow(ctx, cat, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "FALSE", nil, nil
	}
	if err != nil {
		return nil, "", nil, err
	}
	keep := map[string]bool{}
	for _, st := range defStatuses {
		keep[st.ID] = true
	}
	cond = `issue_type_id IN (` + sqlq.Marks(len(types)) + `)`
	if lost, err = s.stranded(ctx, cat, projectID, cond, types, keep); err != nil {
		return nil, "", nil, err
	}
	if missing := unmapped(lost, remap, keep); len(missing) > 0 {
		conflict = map[string]any{
			"strandedDefault": missing,
			"defaultWorkflow": map[string]any{"id": defID, "name": defName, "statuses": defStatuses},
		}
	}
	return lost, cond, conflict, nil
}

// updateWorkflow replaces a workflow's statuses, transitions and issue
// types. Issues sitting in a status the workflow no longer has must be moved:
// without a remap for that status the request is refused (409) with the
// counts, so the user can choose where they go.
func (s *Server) updateWorkflow(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	id, projectID, isDefault, err := s.workflowForEdit(r, sub)
	if err != nil {
		return err
	}
	var in workflowInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return fail(http.StatusBadRequest, i18n.WorkflowNameRequired)
	}
	if len(in.Statuses) == 0 {
		return fail(http.StatusBadRequest, i18n.WorkflowStatusRequired)
	}
	keep := map[string]bool{}
	for _, st := range in.Statuses {
		if _, ok := cat.status[st]; !ok {
			return failf(http.StatusBadRequest, i18n.StatusNameUnknown, st)
		}
		if keep[st] {
			return failf(http.StatusBadRequest, i18n.WorkflowStatusTwice, cat.statusOf(st).Name)
		}
		keep[st] = true
	}
	if !keep[in.Initial] {
		return fail(http.StatusBadRequest, i18n.WorkflowInitialStatus)
	}
	for _, t := range in.Transitions {
		if strings.TrimSpace(t.Name) == "" {
			return fail(http.StatusBadRequest, i18n.WorkflowTransitionNameRequired)
		}
		if !keep[t.To] || (t.From != nil && !keep[*t.From]) {
			return failf(http.StatusBadRequest, i18n.WorkflowTransitionStatus, t.Name)
		}
		if t.From != nil && *t.From == t.To {
			return failf(http.StatusBadRequest, i18n.WorkflowTransitionNoTarget, t.Name)
		}
	}
	types, err := s.projectTypes(ctx, cat, projectID)
	if err != nil {
		return err
	}
	becomesDefault := isDefault || in.Default
	if !becomesDefault {
		for _, typ := range in.IssueTypes {
			if !slices.ContainsFunc(types, func(t typeRef) bool { return t.ID == typ }) {
				return failf(http.StatusBadRequest, i18n.IssueTypeNotInProject, typ)
			}
		}
	}

	// The issue types following this workflow once saved, and those leaving
	// it for the default.
	explicit, err := s.typeWorkflows(ctx, s.db, projectID)
	if err != nil {
		return err
	}
	var leaving []any
	for typ, wf := range explicit {
		if wf == id {
			delete(explicit, typ)
			if !becomesDefault && !slices.Contains(in.IssueTypes, typ) {
				leaving = append(leaving, typ)
			}
		}
	}
	if !becomesDefault {
		for _, typ := range in.IssueTypes {
			explicit[typ] = id
		}
	}
	// Which issues follow the workflow once saved: for the default, those of
	// any type not given another workflow — types the project does not list
	// included, as issues moved in from other projects keep theirs.
	followCond, followArgs := "FALSE", []any{}
	if becomesDefault {
		followCond = "TRUE"
		var others []any
		for typ, wf := range explicit {
			if wf != id {
				others = append(others, typ)
			}
		}
		if len(others) > 0 {
			followCond, followArgs = `issue_type_id NOT IN (`+sqlq.Marks(len(others))+`)`, others
		}
	} else if len(in.IssueTypes) > 0 {
		for _, typ := range in.IssueTypes {
			followArgs = append(followArgs, typ)
		}
		followCond = `issue_type_id IN (` + sqlq.Marks(len(followArgs)) + `)`
	}

	// Issues left in a status the workflow drops, or that the default
	// workflow lacks for the types going back to it.
	lost, err := s.stranded(ctx, cat, projectID, followCond, followArgs, keep)
	if err != nil {
		return err
	}
	lostDefault, leavingCond, conflict, err := s.toDefault(ctx, cat, projectID, leaving, in.RemapDefault)
	if err != nil {
		return err
	}
	if missing := unmapped(lost, in.Remap, keep); len(missing) > 0 || conflict != nil {
		if conflict == nil {
			conflict = map[string]any{}
		}
		if missing == nil {
			missing = []strandedStatus{}
		}
		conflict["stranded"] = missing
		return &httpError{status: http.StatusConflict, extra: conflict,
			msg: i18n.M(i18n.WorkflowStatusesRemoved)}
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if in.Default && !isDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE workflows SET is_default = FALSE WHERE project_id = ?`, projectID); err != nil {
			return err
		}
	}
	if becomesDefault {
		// The default takes every type not explicitly mapped elsewhere.
		if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_issue_types WHERE workflow_id = ?`, id); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workflows SET name = ?, initial_status_id = ?, is_default = ? WHERE id = ?`,
		sqlq.Truncate(name, varcharWidth), in.Initial, becomesDefault, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_statuses WHERE workflow_id = ?`, id); err != nil {
		return err
	}
	for pos, st := range in.Statuses {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_statuses (workflow_id, status_id, position) VALUES (?, ?, ?)`,
			id, st, pos); err != nil {
			return err
		}
	}
	var keepIDs []any
	for _, t := range in.Transitions {
		if t.ID != 0 {
			var exists int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_transitions WHERE id = ? AND workflow_id = ?`,
				t.ID, id).Scan(&exists); err != nil {
				return err
			}
			if exists == 0 {
				return fail(http.StatusBadRequest, i18n.WorkflowTransitionUnknown)
			}
			keepIDs = append(keepIDs, t.ID)
		}
	}
	q := `DELETE FROM workflow_transitions WHERE workflow_id = ?`
	if len(keepIDs) > 0 {
		q += ` AND id NOT IN (` + sqlq.Marks(len(keepIDs)) + `)`
	}
	if _, err := tx.ExecContext(ctx, q, append([]any{id}, keepIDs...)...); err != nil {
		return err
	}
	for pos, t := range in.Transitions {
		var from any
		if t.From != nil {
			from = *t.From
		}
		tname := sqlq.Truncate(strings.TrimSpace(t.Name), varcharWidth)
		if t.ID != 0 {
			_, err = tx.ExecContext(ctx, `UPDATE workflow_transitions SET name = ?, from_status_id = ?, to_status_id = ?,
				position = ? WHERE id = ?`, tname, from, t.To, pos, t.ID)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO workflow_transitions (workflow_id, name, from_status_id, to_status_id,
				position) VALUES (?, ?, ?, ?, ?)`, id, tname, from, t.To, pos)
		}
		if err != nil {
			return err
		}
	}
	if !becomesDefault {
		if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_issue_types WHERE workflow_id = ?`, id); err != nil {
			return err
		}
		for _, typ := range in.IssueTypes {
			if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_issue_types (project_id, issue_type_id, workflow_id)
				VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE workflow_id = VALUES(workflow_id)`, projectID, typ, id); err != nil {
				return err
			}
		}
	}

	// Move the stranded issues, each with its history entry.
	if err := moveStranded(ctx, tx, cat, projectID, sub.UserID, followCond, followArgs, lost, in.Remap, now); err != nil {
		return err
	}
	if err := moveStranded(ctx, tx, cat, projectID, sub.UserID, leavingCond, leaving, lostDefault, in.RemapDefault,
		now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auditWorkflow(r, audit.WorkflowUpdate, id, projectID, name, map[string]any{"statuses": in.Statuses,
		"transitions": len(in.Transitions), "issueTypes": in.IssueTypes, "default": in.Default, "remap": in.Remap})
	return s.writeWorkflows(w, r, sub, projectID)
}

// createWorkflow adds a workflow to a project, a copy of another of its
// workflows (by default the default one). A project's first workflow
// becomes its default.
func (s *Server) createWorkflow(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	projectID, err := s.projectByKey(ctx, r.PathValue("key"))
	if err != nil || (!sub.Admin && !sub.CanBrowseProject(projectID)) {
		return errNotFound
	}
	if !s.administers(sub, projectID) {
		return errForbidden
	}
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
	err = s.db.QueryRowContext(ctx, `SELECT id, initial_status_id FROM workflows WHERE project_id = ?
		AND (id = ? OR (? = 0 AND is_default)) LIMIT 1`, projectID, in.CopyFrom, in.CopyFrom).Scan(&source, &initial)
	switch {
	case errors.Is(err, sql.ErrNoRows) && in.CopyFrom != 0:
		return fail(http.StatusBadRequest, i18n.WorkflowCopyNotFound)
	case errors.Is(err, sql.ErrNoRows):
		// A project without workflows: start from the statuses its issues use.
		if initial, err = s.initialStatus(ctx, projectID); err != nil {
			return err
		}
	case err != nil:
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var defaults int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflows WHERE project_id = ? AND is_default`, projectID).
		Scan(&defaults); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO workflows (project_id, name, is_default, initial_status_id) VALUES (?, ?, ?, ?)`,
		projectID, sqlq.Truncate(name, varcharWidth), defaults == 0, initial)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if source != 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_statuses (workflow_id, status_id, position)
			SELECT ?, status_id, position FROM workflow_statuses WHERE workflow_id = ?`, id, source); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_transitions (workflow_id, name, from_status_id, to_status_id,
			position) SELECT ?, name, from_status_id, to_status_id, position FROM workflow_transitions WHERE workflow_id = ?`,
			id, source); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_statuses (workflow_id, status_id, position)
			SELECT ?, s.id, s.position FROM statuses s WHERE s.id IN (SELECT status_id FROM issues WHERE project_id = ?)`,
			id, projectID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO workflow_statuses (workflow_id, status_id, position)
			VALUES (?, ?, 0)`, id, initial); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auditWorkflow(r, audit.WorkflowCreate, id, projectID, name, map[string]any{"copyFrom": in.CopyFrom})
	return writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

// deleteWorkflow removes a workflow other than the default; its issue types
// go back to the default workflow. Issues in a status the default lacks must
// be moved: the request body gives remapDefault, or the answer is a 409 with
// the counts, as for updateWorkflow.
func (s *Server) deleteWorkflow(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	id, projectID, isDefault, err := s.workflowForEdit(r, sub)
	if err != nil {
		return err
	}
	if isDefault {
		return fail(http.StatusConflict, i18n.WorkflowDefaultDelete)
	}
	name := s.workflowName(ctx, id)
	var in struct {
		RemapDefault map[string]string `json:"remapDefault"`
	}
	if err := readOptionalJSON(r, &in); err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	explicit, err := s.typeWorkflows(ctx, s.db, projectID)
	if err != nil {
		return err
	}
	var leaving []any
	for typ, wf := range explicit {
		if wf == id {
			leaving = append(leaving, typ)
		}
	}
	lost, cond, conflict, err := s.toDefault(ctx, cat, projectID, leaving, in.RemapDefault)
	if err != nil {
		return err
	}
	if conflict != nil {
		conflict["stranded"] = []strandedStatus{}
		return &httpError{status: http.StatusConflict, extra: conflict,
			msg: i18n.M(i18n.WorkflowDefaultStatusesMissing)}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := moveStranded(ctx, tx, cat, projectID, sub.UserID, cond, leaving, lost, in.RemapDefault,
		time.Now().UTC()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workflows WHERE id = ?`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auditWorkflow(r, audit.WorkflowDelete, id, projectID, name, map[string]any{"remapDefault": in.RemapDefault})
	w.WriteHeader(http.StatusNoContent)
	return nil
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
		if err != nil || !sub.CanBrowseProject(projectID) || !s.administers(sub, projectID) {
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
	var next, position int64
	if err := tx.QueryRowContext(ctx, `SELECT GREATEST(COALESCE(MAX(CAST(id AS UNSIGNED)), 0) + 1, 10000),
		(SELECT COALESCE(MAX(position), 0) + 1 FROM statuses) FROM statuses WHERE id REGEXP '^[0-9]+$'`).
		Scan(&next, &position); err != nil {
		return err
	}
	st := statusRef{ID: strconv.FormatInt(next, 10), Name: sqlq.Truncate(name, varcharWidth), Category: in.Category}
	if _, err := tx.ExecContext(ctx, `INSERT INTO statuses (id, name, category, position) VALUES (?, ?, ?, ?)`,
		st.ID, st.Name, st.Category, position); err != nil {
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
		(SELECT COUNT(DISTINCT workflow_id) FROM (SELECT workflow_id FROM workflow_statuses WHERE status_id = ?
			UNION ALL SELECT workflow_id FROM workflow_transitions WHERE from_status_id = ? OR to_status_id = ?
			UNION ALL SELECT id FROM workflows WHERE initial_status_id = ?) u),
		(SELECT COUNT(*) FROM board_column_statuses WHERE status_id = ?)`,
		st.ID, st.ID, st.ID, st.ID, st.ID, st.ID).Scan(&issues, &workflows, &columns); err != nil {
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

// forgetCatalogue drops the cached catalogue after a change to it.
func (s *Server) forgetCatalogue() {
	s.catMu.Lock()
	s.catalog = nil
	s.catMu.Unlock()
}
