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
	"strings"
	"time"

	"solstice/internal/audit"
	"solstice/internal/i18n"
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
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	projectID, err := s.browsableProject(r)
	if err != nil {
		return err
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

// transitionInput is a transition as an update sends it; ID is 0 for a new
// one.
type transitionInput struct {
	ID   int64   `json:"id"`
	Name string  `json:"name"`
	From *string `json:"from"`
	To   string  `json:"to"`
}

type workflowInput struct {
	Name        string            `json:"name"`
	Initial     string            `json:"initial"`
	Statuses    []string          `json:"statuses"`
	Transitions []transitionInput `json:"transitions"`
	IssueTypes  []string          `json:"issueTypes"` // ignored for the default workflow
	Default     bool              `json:"default"`    // make this the project's default
	Remap       map[string]string `json:"remap"`      // removed status → status its issues move to
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

// workflowPlan is what an update of a workflow does, worked out before
// anything is written.
type workflowPlan struct {
	name           string
	keep           map[string]bool // the statuses the workflow keeps
	becomesDefault bool
	// The issues that follow the workflow once saved.
	followCond string
	followArgs []any
	// The issue types going back to the default workflow, and their issues.
	leaving     []any
	leavingCond string
	// The statuses the issues following the workflow, or going back to the
	// default, are in and lose.
	lost, lostDefault []strandedStatus
}

// checkWorkflowInput checks the name, statuses, initial status and
// transitions of an update, and returns the trimmed name and the statuses
// the workflow keeps.
func checkWorkflowInput(cat *catalog, in workflowInput) (string, map[string]bool, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", nil, fail(http.StatusBadRequest, i18n.WorkflowNameRequired)
	}
	if len(in.Statuses) == 0 {
		return "", nil, fail(http.StatusBadRequest, i18n.WorkflowStatusRequired)
	}
	keep := map[string]bool{}
	for _, st := range in.Statuses {
		if _, ok := cat.status[st]; !ok {
			return "", nil, failf(http.StatusBadRequest, i18n.StatusNameUnknown, st)
		}
		if keep[st] {
			return "", nil, failf(http.StatusBadRequest, i18n.WorkflowStatusTwice, cat.statusOf(st).Name)
		}
		keep[st] = true
	}
	if !keep[in.Initial] {
		return "", nil, fail(http.StatusBadRequest, i18n.WorkflowInitialStatus)
	}
	for _, t := range in.Transitions {
		if strings.TrimSpace(t.Name) == "" {
			return "", nil, fail(http.StatusBadRequest, i18n.WorkflowTransitionNameRequired)
		}
		if !keep[t.To] || (t.From != nil && !keep[*t.From]) {
			return "", nil, failf(http.StatusBadRequest, i18n.WorkflowTransitionStatus, t.Name)
		}
		if t.From != nil && *t.From == t.To {
			return "", nil, failf(http.StatusBadRequest, i18n.WorkflowTransitionNoTarget, t.Name)
		}
	}
	return name, keep, nil
}

// typesOn returns the issue types explicitly given workflow id.
func typesOn(explicit map[string]int64, id int64) []string {
	var out []string
	for typ, wf := range explicit {
		if wf == id {
			out = append(out, typ)
		}
	}
	return out
}

// planIssueTypes checks the issue types of an update and records in p the
// issues that follow the workflow once saved and the types that leave it
// for the default.
func (s *Server) planIssueTypes(ctx context.Context, cat *catalog, projectID, id int64, in workflowInput,
	p *workflowPlan) error {
	types, err := s.projectTypes(ctx, cat, projectID)
	if err != nil {
		return err
	}
	if !p.becomesDefault {
		for _, typ := range in.IssueTypes {
			if !slices.ContainsFunc(types, func(t typeRef) bool { return t.ID == typ }) {
				return failf(http.StatusBadRequest, i18n.IssueTypeNotInProject, typ)
			}
		}
	}
	explicit, err := s.typeWorkflows(ctx, s.db, projectID)
	if err != nil {
		return err
	}
	for _, typ := range typesOn(explicit, id) {
		delete(explicit, typ)
		if !p.becomesDefault && !slices.Contains(in.IssueTypes, typ) {
			p.leaving = append(p.leaving, typ)
		}
	}
	if !p.becomesDefault {
		for _, typ := range in.IssueTypes {
			explicit[typ] = id
		}
	}
	// For the default, the issues of any type not given another workflow:
	// types the project does not list included, as issues moved in from
	// other projects keep theirs.
	p.followCond, p.followArgs = "FALSE", []any{}
	if p.becomesDefault {
		p.followCond = "TRUE"
		var others []any
		for typ, wf := range explicit {
			if wf != id {
				others = append(others, typ)
			}
		}
		if len(others) > 0 {
			p.followCond, p.followArgs = `issue_type_id NOT IN (`+sqlq.Marks(len(others))+`)`, others
		}
	} else if len(in.IssueTypes) > 0 {
		for _, typ := range in.IssueTypes {
			p.followArgs = append(p.followArgs, typ)
		}
		p.followCond = `issue_type_id IN (` + sqlq.Marks(len(p.followArgs)) + `)`
	}
	return nil
}

// strandedConflict records in p the issues left in a status the workflow
// drops, or that the default workflow lacks for the types going back to it,
// and refuses the update (409) while the remaps leave some of them out.
func (s *Server) strandedConflict(ctx context.Context, cat *catalog, projectID int64, in workflowInput,
	p *workflowPlan) error {
	var err error
	if p.lost, err = s.stranded(ctx, cat, projectID, p.followCond, p.followArgs, p.keep); err != nil {
		return err
	}
	var conflict map[string]any
	p.lostDefault, p.leavingCond, conflict, err = s.toDefault(ctx, cat, projectID, p.leaving, in.RemapDefault)
	if err != nil {
		return err
	}
	missing := unmapped(p.lost, in.Remap, p.keep)
	if len(missing) == 0 && conflict == nil {
		return nil
	}
	if conflict == nil {
		conflict = map[string]any{}
	}
	if missing == nil {
		missing = []strandedStatus{}
	}
	conflict["stranded"] = missing
	return &httpError{status: http.StatusConflict, extra: conflict, msg: i18n.M(i18n.WorkflowStatusesRemoved)}
}

// saveWorkflow writes the workflow of an update: whether it is the default,
// its name and initial status, its statuses, transitions and issue types.
func saveWorkflow(ctx context.Context, tx *sql.Tx, id, projectID int64, isDefault bool, in workflowInput,
	p *workflowPlan) error {
	if in.Default && !isDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE workflows SET is_default = FALSE WHERE project_id = ?`, projectID); err != nil {
			return err
		}
	}
	if p.becomesDefault {
		// The default takes every type not explicitly mapped elsewhere.
		if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_issue_types WHERE workflow_id = ?`, id); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE workflows SET name = ?, initial_status_id = ?, is_default = ? WHERE id = ?`,
		sqlq.Truncate(p.name, varcharWidth), in.Initial, p.becomesDefault, id); err != nil {
		return err
	}
	if err := writeWorkflowStatuses(ctx, tx, id, in.Statuses); err != nil {
		return err
	}
	if err := writeWorkflowTransitions(ctx, tx, id, in.Transitions); err != nil {
		return err
	}
	if p.becomesDefault {
		return nil
	}
	return writeWorkflowIssueTypes(ctx, tx, id, projectID, in.IssueTypes)
}

// writeWorkflowStatuses replaces the statuses of workflow id, in order.
func writeWorkflowStatuses(ctx context.Context, tx *sql.Tx, id int64, statuses []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_statuses WHERE workflow_id = ?`, id); err != nil {
		return err
	}
	for pos, st := range statuses {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_statuses (workflow_id, status_id, position) VALUES (?, ?, ?)`,
			id, st, pos); err != nil {
			return err
		}
	}
	return nil
}

// writeWorkflowTransitions replaces the transitions of workflow id: those
// with an id are kept and changed, which must be the workflow's own; the
// others are added, and the rest deleted.
func writeWorkflowTransitions(ctx context.Context, tx *sql.Tx, id int64, transitions []transitionInput) error {
	var keepIDs []any
	for _, t := range transitions {
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
	for pos, t := range transitions {
		var from any
		if t.From != nil {
			from = *t.From
		}
		name := sqlq.Truncate(strings.TrimSpace(t.Name), varcharWidth)
		var err error
		if t.ID != 0 {
			_, err = tx.ExecContext(ctx, `UPDATE workflow_transitions SET name = ?, from_status_id = ?, to_status_id = ?,
				position = ? WHERE id = ?`, name, from, t.To, pos, t.ID)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO workflow_transitions (workflow_id, name, from_status_id, to_status_id,
				position) VALUES (?, ?, ?, ?, ?)`, id, name, from, t.To, pos)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// writeWorkflowIssueTypes gives workflow id exactly the issue types listed.
func writeWorkflowIssueTypes(ctx context.Context, tx *sql.Tx, id, projectID int64, types []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM workflow_issue_types WHERE workflow_id = ?`, id); err != nil {
		return err
	}
	for _, typ := range types {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workflow_issue_types (project_id, issue_type_id, workflow_id)
			VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE workflow_id = VALUES(workflow_id)`, projectID, typ, id); err != nil {
			return err
		}
	}
	return nil
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
	p := &workflowPlan{becomesDefault: isDefault || in.Default}
	if p.name, p.keep, err = checkWorkflowInput(cat, in); err != nil {
		return err
	}
	if err := s.planIssueTypes(ctx, cat, projectID, id, in, p); err != nil {
		return err
	}
	if err := s.strandedConflict(ctx, cat, projectID, in, p); err != nil {
		return err
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := saveWorkflow(ctx, tx, id, projectID, isDefault, in, p); err != nil {
		return err
	}
	// Move the stranded issues, each with its history entry.
	if err := moveStranded(ctx, tx, cat, projectID, sub.UserID, p.followCond, p.followArgs, p.lost, in.Remap,
		now); err != nil {
		return err
	}
	if err := moveStranded(ctx, tx, cat, projectID, sub.UserID, p.leavingCond, p.leaving, p.lostDefault,
		in.RemapDefault, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auditWorkflow(r, audit.WorkflowUpdate, id, projectID, p.name, map[string]any{"statuses": in.Statuses,
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
	projectID, err := s.browsableProject(r)
	if err != nil {
		return err
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
	for _, typ := range typesOn(explicit, id) {
		leaving = append(leaving, typ)
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

// forgetCatalogue drops the cached catalogue after a change to it.
func (s *Server) forgetCatalogue() {
	s.catMu.Lock()
	s.catalog = nil
	s.catMu.Unlock()
}
