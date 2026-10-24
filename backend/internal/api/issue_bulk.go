package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"

	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// bulkLimit caps the issues of one bulk change.
const bulkLimit = 200

// bulkSet changes a multi-valued field: "add" and "remove" keep the other
// values, "replace" sets exactly these.
type bulkSet[T comparable] struct {
	Mode   string `json:"mode"`
	Values []T    `json:"values"`
}

func (b *bulkSet[T]) valid() bool {
	return b == nil || b.Mode == "add" || b.Mode == "remove" || b.Mode == "replace"
}

// apply returns the values an issue holding have ends up with.
func (b *bulkSet[T]) apply(have []T) []T {
	switch b.Mode {
	case "replace":
		return append([]T{}, b.Values...)
	case "remove":
		return slices.DeleteFunc(append([]T{}, have...), func(v T) bool { return slices.Contains(b.Values, v) })
	}
	out := append([]T{}, have...)
	for _, v := range b.Values {
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// bulkInput is one change applied to several issues. Op is "edit",
// "transition", "move" or "delete"; each reads its own fields.
type bulkInput struct {
	Op   string   `json:"op"`
	Keys []string `json:"keys"`

	Assignee    json.RawMessage  `json:"assignee"` // a user id, or null to unassign
	Priority    json.RawMessage  `json:"priority"` // an id, or null
	Labels      *bulkSet[string] `json:"labels"`
	Components  *bulkSet[int64]  `json:"components"`
	FixVersions *bulkSet[int64]  `json:"fixVersions"`
	Sprint      *int64           `json:"sprint"` // an open sprint, or 0 to leave it

	Status     string          `json:"status"`
	Resolution json.RawMessage `json:"resolution"`

	// Move is one set of choices for every issue: each takes those its own
	// move asks for. With Check, nothing moves.
	Move *moveInput `json:"move"`
}

// bulkResult is what became of one issue. WithParent marks a sub-task
// moved or deleted with its selected parent.
type bulkResult struct {
	Key        string `json:"key"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
	NewKey     string `json:"newKey,omitempty"`
	WithParent bool   `json:"withParent,omitempty"`
}

// bulkIssues applies one change to each of the issues given, one by one,
// through the code of the single-issue routes. An issue refused does not
// stop the others. Its events carry one bulk id, so each recipient gets one
// e-mail for them all.
func (s *Server) bulkIssues(w http.ResponseWriter, r *http.Request) error {
	r = r.WithContext(events.WithBulk(r.Context(), rand.Int64N(math.MaxInt64)+1))
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	var in bulkInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if err := s.checkBulk(ctx, sub, &in); err != nil {
		return err
	}

	// Each key once, in the order given; unknown or hidden ones fail alone.
	results := make([]bulkResult, 0, len(in.Keys))
	var ids []int64
	selected := map[int64]bool{}
	for _, key := range in.Keys {
		res := bulkResult{Key: strings.TrimSpace(key)}
		iss, err := s.findIssue(ctx, res.Key)
		if err == nil && !sub.CanSee(iss.perm()) {
			err = errNotFound
		}
		if err == nil && selected[iss.ID] {
			continue
		}
		var id int64
		if err != nil {
			res.Error = s.bulkError(r, err)
		} else {
			id, res.Key = iss.ID, iss.Key()
			selected[id] = true
		}
		results = append(results, res)
		ids = append(ids, id)
	}
	if in.Components != nil || in.FixVersions != nil {
		if err := s.bulkOneProject(ctx, ids); err != nil {
			return err
		}
	}

	// A sub-task whose parent is selected goes after it: moving or deleting
	// the parent takes it along.
	order := make([]int, 0, len(ids))
	var children []int
	parents := map[int]int64{}
	for i, id := range ids {
		if id == 0 {
			continue
		}
		if in.Op == "move" || in.Op == "delete" {
			if parent, err := s.parentOf(ctx, id); err != nil {
				return err
			} else if selected[parent] {
				children = append(children, i)
				parents[i] = parent
				continue
			}
		}
		order = append(order, i)
	}
	order = append(order, children...)

	if in.Op == "move" && in.Move.Check {
		return s.bulkMoveCheck(w, r, sub, &in, ids, order, parents, results)
	}
	done := map[int64]bool{}
	for _, i := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		if parent, ok := parents[i]; ok && done[parent] {
			results[i].OK, results[i].WithParent = true, true
			continue
		}
		iss, err := s.bulkIssue(ctx, sub, ids[i])
		if err == nil {
			results[i].NewKey, err = s.bulkOne(r, sub, &in, iss)
		}
		if err != nil {
			results[i].Error = s.bulkError(r, err)
			continue
		}
		results[i].OK = true
		done[ids[i]] = true
	}
	return writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// checkBulk refuses a request that no issue could take, before any change.
func (s *Server) checkBulk(ctx context.Context, sub *perm.Subject, in *bulkInput) error {
	if len(in.Keys) == 0 {
		return fail(http.StatusBadRequest, i18n.BulkNoIssues)
	}
	if len(in.Keys) > bulkLimit {
		return failf(http.StatusBadRequest, i18n.BulkTooMany, bulkLimit)
	}
	switch in.Op {
	case "edit":
		if len(in.Assignee) == 0 && len(in.Priority) == 0 && in.Labels == nil && in.Components == nil &&
			in.FixVersions == nil && in.Sprint == nil {
			return fail(http.StatusBadRequest, i18n.BulkNothingToChange)
		}
		for _, f := range []struct {
			key   string
			valid bool
		}{{"labels", in.Labels.valid()}, {"components", in.Components.valid()}, {"fixVersions", in.FixVersions.valid()}} {
			if !f.valid {
				return failf(http.StatusBadRequest, i18n.FieldValueInvalid, f.key)
			}
		}
		if in.Sprint != nil && *in.Sprint != 0 {
			if _, err := s.sprintBoard(ctx, sub, *in.Sprint); err != nil {
				return err
			}
		}
	case "transition":
		if in.Status == "" {
			return fail(http.StatusBadRequest, i18n.BulkNothingToChange)
		}
	case "move":
		if in.Move == nil {
			return fail(http.StatusBadRequest, i18n.BulkNothingToChange)
		}
	case "delete":
	default:
		return failf(http.StatusBadRequest, i18n.FieldValueInvalid, "op")
	}
	return nil
}

// bulkOneProject refuses components and versions, which belong to a
// project, on issues of several.
func (s *Server) bulkOneProject(ctx context.Context, ids []int64) error {
	var known []int64
	for _, id := range ids {
		if id != 0 {
			known = append(known, id)
		}
	}
	if len(known) == 0 {
		return nil
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT project_id) FROM issues WHERE id IN (`+
		sqlq.Marks(len(known))+`)`, moveArgs(known)...).Scan(&n); err != nil {
		return err
	}
	if n > 1 {
		return fail(http.StatusBadRequest, i18n.BulkOneProject)
	}
	return nil
}

func (s *Server) parentOf(ctx context.Context, id int64) (int64, error) {
	var parent sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT parent_id FROM issues WHERE id = ?`, id).Scan(&parent)
	return parent.Int64, err
}

// bulkIssue reads an issue again just before its change: an earlier one of
// the same request may have changed it.
func (s *Server) bulkIssue(ctx context.Context, sub *perm.Subject, id int64) (*issueRow, error) {
	iss, err := s.findIssueByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !sub.CanSee(iss.perm())) {
		return nil, errNotFound
	}
	return iss, err
}

// bulkError is what the user reads of one issue's failure.
func (s *Server) bulkError(r *http.Request, err error) string {
	var he *httpError
	if errors.As(boundary(s.log, "bulk change failed", r, err), &he) {
		return he.msg.In(locale(r))
	}
	return ""
}

// bulkOne applies the change to one issue, read just before; a move returns
// the new key.
func (s *Server) bulkOne(r *http.Request, sub *perm.Subject, in *bulkInput, iss *issueRow) (string, error) {
	ctx := r.Context()
	switch in.Op {
	case "edit":
		return "", s.bulkEdit(ctx, sub, in, iss)
	case "transition":
		f := fields{"status": jsonOf(in.Status)}
		if len(in.Resolution) > 0 {
			f["resolution"] = in.Resolution
		}
		return "", s.applyEdit(ctx, sub, nil, iss, f, nil)
	case "move":
		plan, err := s.bulkPlanMove(ctx, sub, iss, in.Move)
		if err != nil {
			return "", err
		}
		if !plan.check.Ready {
			return "", &httpError{status: http.StatusBadRequest,
				msg: i18n.M(i18n.IssueMoveChoiceNeeded, strings.Join(plan.check.Missing, ", "))}
		}
		return s.applyMovePlan(r, sub, iss, plan, in.Move)
	default:
		return "", s.removeIssue(r, sub, iss)
	}
}

func jsonOf(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// bulkEdit turns the bulk fields into the issue's own edit, then its sprint.
func (s *Server) bulkEdit(ctx context.Context, sub *perm.Subject, in *bulkInput, iss *issueRow) error {
	f := fields{}
	if len(in.Assignee) > 0 {
		f["assignee"] = in.Assignee
	}
	if len(in.Priority) > 0 {
		f["priority"] = in.Priority
	}
	if in.Labels != nil {
		var have []string
		if err := sqlq.Each(ctx, s.db, `SELECT label FROM labels WHERE issue_id = ? AND field_id IS NULL ORDER BY label`,
			func(rows *sql.Rows) error {
				var l string
				err := rows.Scan(&l)
				have = append(have, l)
				return err
			}, iss.ID); err != nil {
			return err
		}
		f["labels"] = jsonOf(in.Labels.apply(have))
	}
	for _, set := range []struct {
		key  string
		in   *bulkSet[int64]
		kind int
	}{{"components", in.Components, 0}, {"fixVersions", in.FixVersions, 2}} {
		if set.in == nil {
			continue
		}
		parts, err := s.movedParts(ctx, set.kind, iss.ID)
		if err != nil {
			return err
		}
		have := make([]int64, len(parts))
		for i, p := range parts {
			have[i] = p.ID
		}
		f[set.key] = jsonOf(set.in.apply(have))
	}
	if len(f) > 0 {
		if err := s.applyEdit(ctx, sub, nil, iss, f, nil); err != nil {
			return err
		}
	}
	if in.Sprint != nil {
		pi := iss.perm()
		if !sub.Can(perm.Edit, iss.ProjectID, &pi) {
			return errForbidden
		}
		return s.applySprint(ctx, sub, iss, *in.Sprint)
	}
	return nil
}

// bulkPlanMove plans one issue's move with the choices of the shared input
// that its move asks for, and only those: a type chosen for issues the target
// lacks the type of must not change the others'.
func (s *Server) bulkPlanMove(ctx context.Context, sub *perm.Subject, iss *issueRow, shared *moveInput) (*movePlan, error) {
	in := moveInput{Project: shared.Project}
	for {
		plan, err := s.planMove(ctx, sub, iss, &in)
		if err != nil || plan.check.Ready {
			return plan, err
		}
		taken := false
		for _, k := range plan.check.Missing {
			taken = takeMoveChoice(&in, shared, k) || taken
		}
		if !taken {
			return plan, nil
		}
	}
}

// takeMoveChoice copies the choice for key from shared into in, if shared
// has one and in does not yet.
func takeMoveChoice(in, shared *moveInput, key string) bool {
	switch key {
	case "type":
		if in.Type == nil && shared.Type != nil {
			in.Type = shared.Type
			return true
		}
	case "subtaskType":
		if in.SubtaskType == nil && shared.SubtaskType != nil {
			in.SubtaskType = shared.SubtaskType
			return true
		}
	case "status":
		if in.Status == nil && shared.Status != nil {
			in.Status = shared.Status
			return true
		}
	case "components":
		if in.Components == nil && shared.Components != nil {
			in.Components = shared.Components
			return true
		}
	case "affectsVersions":
		if in.AffectsVersions == nil && shared.AffectsVersions != nil {
			in.AffectsVersions = shared.AffectsVersions
			return true
		}
	case "fixVersions":
		if in.FixVersions == nil && shared.FixVersions != nil {
			in.FixVersions = shared.FixVersions
			return true
		}
	case "securityLevel":
		if len(in.SecurityLevel) == 0 && len(shared.SecurityLevel) > 0 {
			in.SecurityLevel = shared.SecurityLevel
			return true
		}
	case "restricted":
		if in.Restricted == nil && shared.Restricted != nil {
			in.Restricted = shared.Restricted
			return true
		}
	}
	return false
}

// bulkMoveCheck plans every move without making it, and answers what the
// moves still need, merged, with each issue that cannot move.
func (s *Server) bulkMoveCheck(w http.ResponseWriter, r *http.Request, sub *perm.Subject, in *bulkInput, ids []int64,
	order []int, parents map[int]int64, results []bulkResult) error {
	ctx := r.Context()
	var check *moveCheck
	planned := map[int64]bool{}
	for _, i := range order {
		if parent, ok := parents[i]; ok && planned[parent] {
			results[i].OK, results[i].WithParent = true, true
			continue
		}
		iss, err := s.bulkIssue(ctx, sub, ids[i])
		if err != nil {
			results[i].Error = s.bulkError(r, err)
			continue
		}
		plan, err := s.bulkPlanMove(ctx, sub, iss, in.Move)
		if err != nil {
			results[i].Error = s.bulkError(r, err)
			continue
		}
		results[i].OK = true
		planned[iss.ID] = true
		if check == nil {
			check = &plan.check
		} else {
			mergeMoveCheck(check, &plan.check)
		}
	}
	if check != nil {
		check.Assignee = nil
		check.Ready = len(check.Missing) == 0
	}
	return writeJSON(w, http.StatusOK, map[string]any{"check": check, "results": results})
}

// mergeMoveCheck adds to c what another issue's move needs. The target's
// options are the same for all; what is unmatched adds up.
func mergeMoveCheck(c, o *moveCheck) {
	for _, k := range o.Missing {
		if !slices.Contains(c.Missing, k) {
			c.Missing = append(c.Missing, k)
		}
	}
	if c.Types == nil {
		c.Types = o.Types
	}
	if c.SubtaskTypes == nil {
		c.SubtaskTypes = o.SubtaskTypes
	}
	for _, st := range o.Statuses {
		if !slices.ContainsFunc(c.Statuses, func(x statusRef) bool { return x.ID == st.ID }) {
			c.Statuses = append(c.Statuses, st)
		}
	}
	for _, p := range []struct{ to, from **moveParts }{
		{&c.Components, &o.Components}, {&c.AffectsVersions, &o.AffectsVersions},
		{&c.FixVersions, &o.FixVersions}, {&c.SecurityLevel, &o.SecurityLevel},
	} {
		switch {
		case *p.from == nil:
		case *p.to == nil:
			*p.to = *p.from
		default:
			for _, u := range (*p.from).Unmatched {
				if !slices.ContainsFunc((*p.to).Unmatched, func(x idName) bool { return x.ID == u.ID }) {
					(*p.to).Unmatched = append((*p.to).Unmatched, u)
				}
			}
		}
	}
	if o.Restricted != nil {
		if c.Restricted == nil {
			c.Restricted = o.Restricted
		} else {
			c.Restricted.Comments += o.Restricted.Comments
			c.Restricted.Worklogs += o.Restricted.Worklogs
		}
	}
	for _, f := range o.DroppedFields {
		if !slices.Contains(c.DroppedFields, f) {
			c.DroppedFields = append(c.DroppedFields, f)
		}
	}
	for _, sp := range o.Sprints {
		if !slices.Contains(c.Sprints, sp) {
			c.Sprints = append(c.Sprints, sp)
		}
	}
	c.Subtasks.Count += o.Subtasks.Count
	c.Subtasks.Reset += o.Subtasks.Reset
	c.Subtasks.Lost += o.Subtasks.Lost
	c.Subtasks.Reassigned += o.Subtasks.Reassigned
}
