package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"solstice/internal/audit"
	"solstice/internal/events"
	"solstice/internal/i18n"
	"solstice/internal/jql"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// moveInput is the request. The choices are the ones the dry run
// ("check": true) asks for: a list given replaces what would be mapped by
// name, and is the whole list the issue ends with.
type moveInput struct {
	Project         string          `json:"project"`
	Check           bool            `json:"check"`
	Type            *string         `json:"type"`
	SubtaskType     *string         `json:"subtaskType"` // for the sub-tasks whose type the target lacks
	Status          *string         `json:"status"`
	Components      *[]int64        `json:"components"`
	FixVersions     *[]int64        `json:"fixVersions"`
	AffectsVersions *[]int64        `json:"affectsVersions"`
	SecurityLevel   json.RawMessage `json:"securityLevel"` // an id, or null for none
	Restricted      *moveChoice     `json:"restricted"`
}

// moveChoice says what becomes of the comments and work logs that are
// restricted to a project role, since roles are held per project: "role" keeps
// them restricted to a role of the target project, "group" restricts them to a
// group instead, "refuse" gives the move up.
type moveChoice struct {
	To    string `json:"to"`
	Role  int64  `json:"role"`
	Group int64  `json:"group"`
}

type moveRestrict struct {
	Comments int      `json:"comments"`
	Worklogs int      `json:"worklogs"`
	Roles    []idName `json:"roles"`
	Groups   []idName `json:"groups"`
}

// moveParts is what an issue has that the target project has no same-named
// match for, and what the target offers instead.
type moveParts struct {
	Unmatched []idName `json:"unmatched"`
	Options   []idName `json:"options"`
}

type moveSubtasks struct {
	Count int `json:"count"`
	// Reset counts the sub-tasks whose status is not in the target workflow:
	// they start over; Lost the components and versions they leave behind.
	Reset int `json:"reset"`
	Lost  int `json:"lost"`
	// Reassigned counts the ones whose assignee cannot be assigned in the target.
	Reassigned int `json:"reassigned"`
}

type moveAssignee struct {
	From *userRef `json:"from"`
	To   *userRef `json:"to"`
}

type moveCheck struct {
	Project         projectBrief  `json:"project"`
	Ready           bool          `json:"ready"`
	Missing         []string      `json:"missing"`
	Types           []typeRef     `json:"types,omitempty"`
	SubtaskTypes    []typeRef     `json:"subtaskTypes,omitempty"`
	Statuses        []statusRef   `json:"statuses,omitempty"`
	Components      *moveParts    `json:"components,omitempty"`
	AffectsVersions *moveParts    `json:"affectsVersions,omitempty"`
	FixVersions     *moveParts    `json:"fixVersions,omitempty"`
	SecurityLevel   *moveParts    `json:"securityLevel,omitempty"`
	Restricted      *moveRestrict `json:"restricted,omitempty"`
	DroppedFields   []string      `json:"droppedFields"`
	Sprints         []string      `json:"sprints"`
	Assignee        *moveAssignee `json:"assignee,omitempty"`
	Subtasks        moveSubtasks  `json:"subtasks"`
}

// moveKinds are the multi-valued fields that belong to a project, in the
// order of their choices.
var moveKinds = []struct {
	key, field, kind, from, options string
}{
	{"components", "Component", "", `SELECT c.id, c.name FROM issue_components ic JOIN components c ON c.id = ic.component_id
		WHERE ic.issue_id = ? ORDER BY c.name, c.id`,
		`SELECT id, name FROM components WHERE project_id = ? AND NOT archived ORDER BY name, id`},
	{"affectsVersions", "Version", "affects", `SELECT v.id, v.name FROM issue_versions iv JOIN versions v ON v.id = iv.version_id
		WHERE iv.issue_id = ? AND iv.kind = ? ORDER BY v.position, v.id`,
		`SELECT id, name FROM versions WHERE project_id = ? AND NOT archived ORDER BY position, id`},
	{"fixVersions", "Fix Version", "fix", `SELECT v.id, v.name FROM issue_versions iv JOIN versions v ON v.id = iv.version_id
		WHERE iv.issue_id = ? AND iv.kind = ? ORDER BY v.position, v.id`,
		`SELECT id, name FROM versions WHERE project_id = ? AND NOT archived ORDER BY position, id`},
}

type moveSet struct{ from, to []idName }

type moveItem struct {
	row        *issueRow
	typeID     string
	status     string
	resolution sql.NullString
	sets       [3]moveSet
	level      any
	assignee   any
	dropped    []int64
	sprints    []sprintRef
	restricted []movedRestriction
	num        int64
}

type moveDest struct {
	id, lead        int64
	key, name       string
	defaultAssignee string
	types           []typeRef
}

type movePlan struct {
	dest    *moveDest
	cat     *catalog
	items   []*moveItem
	check   moveCheck
	levels  [2]string // the level's name before and after
	fieldNm map[int64]string
}

// moveRemap is the SET clause that gives a role-restricted comment or work log
// its new restriction, and its argument.
type moveRemap struct {
	set string
	arg any
}

type movedRestriction struct {
	table string
	id    int64
	remap moveRemap
}

func (d *moveDest) hasType(id string) bool {
	return slices.ContainsFunc(d.types, func(t typeRef) bool { return t.ID == id })
}

// moveDestination loads the project an issue goes to: unknown or hidden to
// the user, it does not exist.
func (s *Server) moveDestination(ctx context.Context, sub *perm.Subject, cat *catalog, key string) (*moveDest, error) {
	if key == "" {
		return nil, failf(http.StatusBadRequest, i18n.FieldRequired, "project")
	}
	d := &moveDest{}
	var lead sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT id, pkey, name, lead_id, default_assignee FROM projects WHERE pkey = ?`,
		strings.ToUpper(key)).Scan(&d.id, &d.key, &d.name, &lead, &d.defaultAssignee)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	if !sub.CanBrowseProject(d.id) {
		return nil, errNotFound
	}
	if !sub.Can(perm.Create, d.id, nil) {
		return nil, errForbidden
	}
	d.lead = lead.Int64
	err = sqlq.Each(ctx, s.db, `SELECT issue_type_id FROM project_issue_types WHERE project_id = ? ORDER BY position`,
		func(rows *sql.Rows) error {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			d.types = append(d.types, cat.typeOf(id))
			return nil
		}, d.id)
	return d, err
}

func moveArgs(ids []int64) []any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, id)
	}
	return out
}

func (s *Server) moveNames(ctx context.Context, q string, args ...any) ([]idName, error) {
	out := []idName{}
	err := sqlq.Each(ctx, s.db, q, func(rows *sql.Rows) error {
		var v idName
		if err := rows.Scan(&v.ID, &v.Name); err != nil {
			return err
		}
		out = append(out, v)
		return nil
	}, args...)
	return out, err
}

// mapNames puts each of from on the option of the same name (case aside);
// the ones with none are left over.
func mapNames(from, options []idName) (mapped, unmatched []idName) {
	for _, f := range from {
		i := slices.IndexFunc(options, func(o idName) bool { return strings.EqualFold(o.Name, f.Name) })
		switch {
		case i < 0:
			unmatched = append(unmatched, f)
		case !slices.ContainsFunc(mapped, func(m idName) bool { return m.ID == options[i].ID }):
			mapped = append(mapped, options[i])
		}
	}
	return mapped, unmatched
}

// moveAssigneeFor keeps the assignee if the target project lets them be
// assigned; otherwise the project's rule for a new issue with these
// components applies.
func (s *Server) moveAssigneeFor(ctx context.Context, d *moveDest, row *issueRow, components []idName) (any, error) {
	if !row.AssigneeID.Valid {
		return nil, nil
	}
	ok, err := s.isAssignable(ctx, d.id, row.AssigneeID.Int64)
	if err != nil || ok {
		return sqlq.Value(row.AssigneeID), err
	}
	ids := make([]int64, 0, len(components))
	for _, c := range components {
		ids = append(ids, c.ID)
	}
	return s.newIssueAssignee(ctx, d.id, d.lead, d.defaultAssignee, ids)
}

// movedFields are the custom fields with values on an issue that do not
// apply in the target project for its type.
func (s *Server) movedFields(ctx context.Context, projectID int64, typeID string, issueID int64) ([]int64, error) {
	hidden := make([]any, 0, len(hiddenKinds))
	for kind := range hiddenKinds {
		hidden = append(hidden, kind)
	}
	marks := sqlq.Marks(len(hidden))
	args := append([]any{issueID}, hidden...)
	args = append(args, issueID)
	args = append(args, hidden...)
	var ids []int64
	err := sqlq.Each(ctx, s.db, `SELECT v.field_id FROM custom_field_values v JOIN custom_fields f ON f.id = v.field_id
		WHERE v.issue_id = ? AND SUBSTRING_INDEX(f.type_key, ':', -1) NOT IN (`+marks+`)
		UNION SELECT l.field_id FROM labels l JOIN custom_fields f ON f.id = l.field_id
		WHERE l.issue_id = ? AND SUBSTRING_INDEX(f.type_key, ':', -1) NOT IN (`+marks+`)`, func(rows *sql.Rows) error {
		var id int64
		err := rows.Scan(&id)
		ids = append(ids, id)
		return err
	}, args...)
	if err != nil {
		return nil, err
	}
	return s.fieldsOutside(ctx, projectID, typeID, ids)
}

// leavingSprints are the open sprints of an issue on a board whose filter
// names projects but not the target one.
func (s *Server) leavingSprints(ctx context.Context, dest *moveDest, issueID int64) ([]sprintRef, error) {
	var out []sprintRef
	err := sqlq.Each(ctx, s.db, `SELECT sp.id, sp.name, sp.state, f.query FROM sprint_issues si JOIN sprints sp ON sp.id = si.sprint_id
		LEFT JOIN boards b ON b.id = sp.board_id LEFT JOIN filters f ON f.id = b.filter_id
		WHERE si.issue_id = ? AND sp.state <> 'closed' ORDER BY sp.id`,
		func(rows *sql.Rows) error {
			var sp sprintRef
			var query sql.NullString
			if err := rows.Scan(&sp.ID, &sp.Name, &sp.State, &query); err != nil {
				return err
			}
			if !query.Valid || !filterReaches(query.String, dest) {
				out = append(out, sp)
			}
			return nil
		}, issueID)
	return out, err
}

// filterReaches tells whether a filter can show issues of the project: it can
// when it names no project at all.
func filterReaches(query string, dest *moveDest) bool {
	parsed, err := jql.Parse(query)
	if err != nil {
		return true
	}
	names := jql.Projects(parsed)
	return len(names) == 0 || slices.ContainsFunc(names, func(n string) bool {
		return strings.EqualFold(n, dest.key) || strings.EqualFold(n, dest.name) || n == strconv.FormatInt(dest.id, 10)
	})
}

func choice(key string, ids []int64, options []idName) ([]idName, error) {
	var out []idName
	for _, id := range ids {
		i := slices.IndexFunc(options, func(o idName) bool { return o.ID == id })
		if i < 0 {
			return nil, failf(http.StatusBadRequest, i18n.FieldValueNotInProject, key)
		}
		if !slices.Contains(out, options[i]) {
			out = append(out, options[i])
		}
	}
	return out, nil
}

// planMove works out what moving orig does, and what is still to choose.
func (s *Server) planMove(ctx context.Context, sub *perm.Subject, orig *issueRow, in *moveInput) (*movePlan, error) {
	if orig.ParentID.Valid {
		return nil, fail(http.StatusBadRequest, i18n.IssueMoveSubtask)
	}
	pi := orig.perm()
	if !sub.Can(perm.Edit, orig.ProjectID, &pi) {
		return nil, errForbidden
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return nil, err
	}
	dest, err := s.moveDestination(ctx, sub, cat, strings.TrimSpace(in.Project))
	if err != nil {
		return nil, err
	}
	if dest.id == orig.ProjectID {
		return nil, fail(http.StatusBadRequest, i18n.IssueMoveSameProject)
	}

	// The sub-tasks go with it: the user must be able to change each.
	var children []*issueRow
	if err := sqlq.Each(ctx, s.db, `SELECT `+issueColumns+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE i.parent_id = ? ORDER BY i.num`, func(rows *sql.Rows) error {
		child, err := scanIssue(rows)
		children = append(children, child)
		return err
	}, orig.ID); err != nil {
		return nil, err
	}
	for _, child := range children {
		cp := child.perm()
		if !sub.CanSee(cp) || !sub.Can(perm.Edit, child.ProjectID, &cp) {
			return nil, errForbidden
		}
	}

	plan := &movePlan{dest: dest, cat: cat}
	chk := &plan.check
	chk.Project = projectBrief{ID: dest.id, Key: dest.key, Name: dest.name}
	chk.Missing, chk.DroppedFields, chk.Sprints = []string{}, []string{}, []string{}
	chk.Subtasks.Count = len(children)
	missing := func(key string) { chk.Missing = append(chk.Missing, key) }
	head := &moveItem{row: orig, assignee: sqlq.Value(orig.AssigneeID)}
	plan.items = append(plan.items, head)

	// Type, then the status, which depends on the workflow of the type.
	switch {
	case in.Type != nil:
		if it, ok := cat.issueType[*in.Type]; !ok || it.Subtask || !dest.hasType(*in.Type) {
			return nil, fail(http.StatusBadRequest, i18n.IssueTypeUnavailable)
		}
		head.typeID = *in.Type
	case dest.hasType(orig.TypeID):
		head.typeID = orig.TypeID
	default:
		for _, t := range dest.types {
			if !t.Subtask {
				chk.Types = append(chk.Types, t)
			}
		}
		missing("type")
	}
	if head.typeID != "" {
		statuses, err := s.typeStatuses(ctx, cat, dest.id, head.typeID)
		if err != nil {
			return nil, err
		}
		has := func(id string) bool {
			return slices.ContainsFunc(statuses, func(st statusRef) bool { return st.ID == id })
		}
		switch {
		case in.Status != nil:
			if !has(*in.Status) {
				return nil, fail(http.StatusBadRequest, i18n.StatusUnknown)
			}
			// Changing the status takes the Transition right; a move only has to find one.
			if *in.Status != orig.StatusID && has(orig.StatusID) {
				return nil, fail(http.StatusBadRequest, i18n.IssueTransitionNotAllowed)
			}
			head.status = *in.Status
		case has(orig.StatusID):
			head.status = orig.StatusID
		default:
			chk.Statuses = statuses
			missing("status")
		}
	}
	head.resolution = orig.ResolutionID
	if head.status != "" && head.status != orig.StatusID {
		head.resolution = statusResolution(cat, orig.ResolutionID, head.status)
	}

	chosen := [3]*[]int64{in.Components, in.AffectsVersions, in.FixVersions}
	shown := [3]**moveParts{&chk.Components, &chk.AffectsVersions, &chk.FixVersions}
	options := [3][]idName{}
	for k, kind := range moveKinds {
		if options[k], err = s.moveNames(ctx, kind.options, dest.id); err != nil {
			return nil, err
		}
		from, err := s.movedParts(ctx, k, orig.ID)
		if err != nil {
			return nil, err
		}
		mapped, unmatched := mapNames(from, options[k])
		head.sets[k].from = from
		switch {
		case chosen[k] != nil:
			if head.sets[k].to, err = choice(kind.key, *chosen[k], options[k]); err != nil {
				return nil, err
			}
		case len(unmatched) == 0:
			head.sets[k].to = mapped
		default:
			missing(kind.key)
		}
		if len(unmatched) > 0 {
			*shown[k] = &moveParts{Unmatched: unmatched, Options: options[k]}
		}
	}

	if err := s.planLevel(ctx, sub, orig, dest, in, plan, missing); err != nil {
		return nil, err
	}

	// Sub-tasks: same mapping, and as confidential as their parent.
	subtaskTypes := []typeRef{}
	for _, t := range dest.types {
		if t.Subtask {
			subtaskTypes = append(subtaskTypes, t)
		}
	}
	statusesOf := map[string][]statusRef{}
	for _, child := range children {
		c := &moveItem{row: child, assignee: sqlq.Value(child.AssigneeID), level: head.level}
		plan.items = append(plan.items, c)
		switch {
		case dest.hasType(child.TypeID):
			c.typeID = child.TypeID
		case len(subtaskTypes) == 0:
			return nil, fail(http.StatusBadRequest, i18n.IssueMoveNoSubtaskType)
		case in.SubtaskType != nil:
			if it, ok := cat.issueType[*in.SubtaskType]; !ok || !it.Subtask || !dest.hasType(*in.SubtaskType) {
				return nil, fail(http.StatusBadRequest, i18n.IssueTypeUnavailable)
			}
			c.typeID = *in.SubtaskType
		case chk.SubtaskTypes == nil:
			chk.SubtaskTypes = subtaskTypes
			missing("subtaskType")
		}
		if c.typeID != "" {
			statuses, ok := statusesOf[c.typeID]
			if !ok {
				if statuses, err = s.typeStatuses(ctx, cat, dest.id, c.typeID); err != nil {
					return nil, err
				}
				statusesOf[c.typeID] = statuses
			}
			if slices.ContainsFunc(statuses, func(st statusRef) bool { return st.ID == child.StatusID }) {
				c.status = child.StatusID
			} else {
				if c.status, err = s.initialStatusFor(ctx, dest.id, c.typeID); err != nil {
					return nil, err
				}
				chk.Subtasks.Reset++
			}
			c.resolution = child.ResolutionID
			if c.status != child.StatusID {
				c.resolution = statusResolution(cat, child.ResolutionID, c.status)
			}
		}
		for k := range moveKinds {
			from, err := s.movedParts(ctx, k, child.ID)
			if err != nil {
				return nil, err
			}
			c.sets[k].from = from
			c.sets[k].to, _ = mapNames(from, options[k])
			chk.Subtasks.Lost += len(from) - len(c.sets[k].to)
		}
	}

	if err := s.planRestricted(ctx, sub, plan, in, missing); err != nil {
		return nil, err
	}

	dropped := map[int64]bool{}
	var droppedIDs []int64
	for _, it := range plan.items {
		if it.typeID != "" {
			if it.dropped, err = s.movedFields(ctx, dest.id, it.typeID, it.row.ID); err != nil {
				return nil, err
			}
			for _, id := range it.dropped {
				if !dropped[id] {
					dropped[id] = true
					droppedIDs = append(droppedIDs, id)
				}
			}
		}
		if it.sprints, err = s.leavingSprints(ctx, dest, it.row.ID); err != nil {
			return nil, err
		}
		for _, sp := range it.sprints {
			if !slices.Contains(chk.Sprints, sp.Name) {
				chk.Sprints = append(chk.Sprints, sp.Name)
			}
		}
		if it.assignee, err = s.moveAssigneeFor(ctx, dest, it.row, it.sets[0].to); err != nil {
			return nil, err
		}
		if it != head && sqlq.Value(it.row.AssigneeID) != it.assignee {
			chk.Subtasks.Reassigned++
		}
	}
	if len(droppedIDs) > 0 {
		plan.fieldNm = map[int64]string{}
		names, err := s.moveNames(ctx, `SELECT id, name FROM custom_fields WHERE id IN (`+sqlq.Marks(len(droppedIDs))+`)`,
			moveArgs(droppedIDs)...)
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			plan.fieldNm[n.ID] = n.Name
			chk.DroppedFields = append(chk.DroppedFields, n.Name)
		}
		sort.Strings(chk.DroppedFields)
	}
	if sqlq.Value(orig.AssigneeID) != head.assignee {
		ids := []int64{orig.AssigneeID.Int64}
		if to, ok := head.assignee.(int64); ok {
			ids = append(ids, to)
		}
		users, err := s.users(ctx, ids...)
		if err != nil {
			return nil, err
		}
		chk.Assignee = &moveAssignee{From: refOf(users, orig.AssigneeID), To: refOf(users, nullInt64(head.assignee))}
	}
	chk.Ready = len(chk.Missing) == 0
	return plan, nil
}

// movedParts reads the components or versions (by kind index) of an issue.
func (s *Server) movedParts(ctx context.Context, k int, issueID int64) ([]idName, error) {
	args := []any{issueID}
	if moveKinds[k].kind != "" {
		args = append(args, moveKinds[k].kind)
	}
	return s.moveNames(ctx, moveKinds[k].from, args...)
}

// planLevel decides the security level of the moved issue: the choice given
// (which takes the right to set levels), else the target's level of the same
// name, else something to choose.
func (s *Server) planLevel(ctx context.Context, sub *perm.Subject, orig *issueRow, dest *moveDest, in *moveInput,
	plan *movePlan, missing func(string)) error {
	head := plan.items[0]
	levels, err := s.moveNames(ctx, `SELECT id, name FROM security_levels WHERE project_id = ? ORDER BY name, id`, dest.id)
	if err != nil {
		return err
	}
	var current idName
	if orig.LevelID.Valid {
		current.ID = orig.LevelID.Int64
		s.db.QueryRowContext(ctx, `SELECT name FROM security_levels WHERE id = ?`, current.ID).Scan(&current.Name)
		plan.levels[0] = current.Name
	}
	pi := orig.perm()
	mayChoose := sub.Can(perm.Security, dest.id, nil) && sub.Can(perm.Security, orig.ProjectID, &pi)
	switch {
	case len(in.SecurityLevel) > 0:
		if !mayChoose {
			return fail(http.StatusForbidden, i18n.IssueMoveSecurityNeeded)
		}
		if string(in.SecurityLevel) == "null" {
			return nil
		}
		var id int64
		if err := json.Unmarshal(in.SecurityLevel, &id); err != nil {
			return failf(http.StatusBadRequest, i18n.FieldValueInvalid, "securityLevel")
		}
		i := slices.IndexFunc(levels, func(l idName) bool { return l.ID == id })
		if i < 0 {
			return fail(http.StatusBadRequest, i18n.IssueSecurityLevelUnknown)
		}
		head.level, plan.levels[1] = id, levels[i].Name
	case !orig.LevelID.Valid:
	case !sub.Can(perm.Security, dest.id, nil):
		return fail(http.StatusForbidden, i18n.IssueMoveSecurityNeeded)
	default:
		if i := slices.IndexFunc(levels, func(l idName) bool { return strings.EqualFold(l.Name, current.Name) }); i >= 0 {
			head.level, plan.levels[1] = levels[i].ID, levels[i].Name
			return nil
		}
		if !mayChoose {
			return fail(http.StatusForbidden, i18n.IssueMoveSecurityNeeded)
		}
		plan.check.SecurityLevel = &moveParts{Unmatched: []idName{current}, Options: levels}
		missing("securityLevel")
	}
	return nil
}

// planRestricted decides what becomes of the comments and work logs restricted
// to a project role: the role is held per project, so they cannot keep it as
// it is. The ones the user reads take a role of the target, or a group, as
// chosen. The others take the role of the same name in the target, or the move
// is refused: a choice would let the user read what they could not.
func (s *Server) planRestricted(ctx context.Context, sub *perm.Subject, plan *movePlan, in *moveInput,
	missing func(string)) error {
	ids := make([]int64, 0, len(plan.items))
	byID := map[int64]*moveItem{}
	for _, it := range plan.items {
		ids = append(ids, it.row.ID)
		byID[it.row.ID] = it
	}
	marks := sqlq.Marks(len(ids))
	type restriction struct {
		movedRestriction
		item     *moveItem
		role     string
		readable bool
	}
	var found []*restriction
	var comments, worklogs int
	if err := sqlq.Each(ctx, s.db, `SELECT 'comments', x.id, x.issue_id, x.visibility_group, x.visibility_role_id, r.name
		FROM comments x JOIN project_roles r ON r.id = x.visibility_role_id WHERE x.issue_id IN (`+marks+`)
		UNION ALL SELECT 'worklogs', x.id, x.issue_id, x.visibility_group, x.visibility_role_id, r.name
		FROM worklogs x JOIN project_roles r ON r.id = x.visibility_role_id WHERE x.issue_id IN (`+marks+`)`,
		func(rows *sql.Rows) error {
			var f restriction
			var issueID, role int64
			var group sql.NullString
			if err := rows.Scan(&f.table, &f.id, &issueID, &group, &role, &f.role); err != nil {
				return err
			}
			f.item = byID[issueID]
			f.readable = sub.Admin || sub.SeesRestricted(f.item.row.ProjectID, group.String, role)
			if f.readable && f.table == "comments" {
				comments++
			} else if f.readable {
				worklogs++
			}
			found = append(found, &f)
			return nil
		}, append(moveArgs(ids), moveArgs(ids)...)...); err != nil {
		return err
	}
	if len(found) == 0 {
		return nil
	}
	roles, err := s.moveNames(ctx, `SELECT DISTINCT r.id, r.name FROM project_role_members m
		JOIN project_roles r ON r.id = m.role_id WHERE m.project_id = ? ORDER BY r.name, r.id`, plan.dest.id)
	if err != nil {
		return err
	}

	unmatched := 0
	for _, f := range found {
		if f.readable {
			continue
		}
		i := slices.IndexFunc(roles, func(o idName) bool { return strings.EqualFold(o.Name, f.role) })
		if i < 0 {
			unmatched++
			continue
		}
		f.remap = moveRemap{set: "visibility_role_id = ?", arg: roles[i].ID}
		f.item.restricted = append(f.item.restricted, f.movedRestriction)
	}
	if unmatched > 0 {
		return failf(http.StatusBadRequest, i18n.IssueMoveRestrictedHidden, unmatched)
	}
	if comments+worklogs == 0 {
		return nil
	}

	// Seeing only one's own issues in the target is not seeing who holds its roles.
	offered := []idName{}
	if sub.Can(perm.Browse, plan.dest.id, nil) {
		offered = roles
	}
	r := &moveRestrict{Comments: comments, Worklogs: worklogs, Roles: offered}
	// Groups are global: the mover may pick one they belong to (any, for an administrator).
	groups := `SELECT g.id, g.name FROM user_groups g JOIN group_members m ON m.group_id = g.id
		WHERE m.user_id = ? ORDER BY g.name, g.id`
	groupArgs := []any{sub.UserID}
	if sub.Admin {
		groups, groupArgs = `SELECT id, name FROM user_groups ORDER BY name, id`, nil
	}
	if r.Groups, err = s.moveNames(ctx, groups, groupArgs...); err != nil {
		return err
	}
	plan.check.Restricted = r

	c := in.Restricted
	if c == nil {
		missing("restricted")
		return nil
	}
	var remap moveRemap
	switch c.To {
	case "role":
		if !slices.ContainsFunc(r.Roles, func(o idName) bool { return o.ID == c.Role }) {
			return failf(http.StatusBadRequest, i18n.FieldValueNotInProject, "restricted")
		}
		remap = moveRemap{set: "visibility_role_id = ?", arg: c.Role}
	case "group":
		i := slices.IndexFunc(r.Groups, func(o idName) bool { return o.ID == c.Group })
		if i < 0 {
			return failf(http.StatusBadRequest, i18n.FieldValueInvalid, "restricted")
		}
		// An item already restricted to a group keeps it.
		remap = moveRemap{set: "visibility_group = COALESCE(visibility_group, ?), visibility_role_id = NULL",
			arg: r.Groups[i].Name}
	case "refuse":
		return fail(http.StatusBadRequest, i18n.IssueMoveRestrictedRefused)
	default:
		return failf(http.StatusBadRequest, i18n.FieldValueInvalid, "restricted")
	}
	for _, f := range found {
		if f.readable {
			f.remap = remap
			f.item.restricted = append(f.item.restricted, f.movedRestriction)
		}
	}
	return nil
}

// moveIssue moves an issue, and its sub-tasks, to another project. The
// issue gets the next key of the target and keeps answering to its old one.
func (s *Server) moveIssue(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	orig, err := s.visibleIssue(r, sub)
	if err != nil {
		return err
	}
	var in moveInput
	if err := readJSON(r, &in); err != nil {
		return err
	}
	plan, err := s.planMove(ctx, sub, orig, &in)
	if err != nil {
		return err
	}
	if in.Check {
		return writeJSON(w, http.StatusOK, plan.check)
	}
	if !plan.check.Ready {
		return &httpError{status: http.StatusBadRequest,
			msg:   i18n.M(i18n.IssueMoveChoiceNeeded, strings.Join(plan.check.Missing, ", ")),
			extra: map[string]any{"missing": plan.check.Missing}}
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Someone else may have changed the issue or its sub-tasks since the plan
	// was made, and the plan would undo it.
	var project int64
	var updated time.Time
	if err := tx.QueryRowContext(ctx, `SELECT project_id, updated_at FROM issues WHERE id = ? FOR UPDATE`, orig.ID).
		Scan(&project, &updated); err != nil {
		return err
	}
	if project != orig.ProjectID || !updated.Equal(orig.Updated) {
		return fail(http.StatusConflict, i18n.IssueMoveChanged)
	}
	n, changed := 0, false
	if err := sqlq.Each(ctx, tx, `SELECT id, updated_at FROM issues WHERE parent_id = ? ORDER BY num FOR UPDATE`,
		func(rows *sql.Rows) error {
			var id int64
			var at time.Time
			err := rows.Scan(&id, &at)
			n++
			changed = changed || n >= len(plan.items) || plan.items[n].row.ID != id || !plan.items[n].row.Updated.Equal(at)
			return err
		}, orig.ID); err != nil {
		return err
	}
	if changed || n != len(plan.items)-1 {
		return fail(http.StatusConflict, i18n.IssueMoveChanged)
	}

	var next int64
	if err := tx.QueryRowContext(ctx, `SELECT issue_counter + 1 FROM projects WHERE id = ? FOR UPDATE`, plan.dest.id).
		Scan(&next); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET issue_counter = ? WHERE id = ?`,
		next+int64(len(plan.items))-1, plan.dest.id); err != nil {
		return err
	}
	changes := make([][]change, len(plan.items))
	for i, it := range plan.items {
		it.num = next + int64(i)
		if changes[i], err = s.applyMove(ctx, tx, plan, it, sub.UserID, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	newKey := plan.dest.key + "-" + strconv.FormatInt(plan.items[0].num, 10)
	details := map[string]any{"from": orig.Key(), "to": newKey, "fromProject": orig.ProjectKey, "project": plan.dest.key,
		"subtasks": len(plan.items) - 1}
	if plan.levels != [2]string{} {
		details["levelFrom"], details["levelTo"] = plan.levels[0], plan.levels[1]
	}
	if plan.check.Restricted != nil {
		details["restricted"] = in.Restricted
	}
	s.auditLog(r, audit.IssueMove, audit.Target{Type: "issue", ID: strconv.FormatInt(orig.ID, 10), Label: newKey}, details)

	// Announced like an edit; pages open on the old key refresh through the
	// second event, which only those who could see the issue there hear.
	for i, it := range plan.items {
		updated, err := s.findIssueByID(ctx, it.row.ID)
		if err != nil {
			return err
		}
		e := issueEvent(events.IssueUpdated, updated, sub.UserID, "issue")
		e.Changes = eventChanges(changes[i])
		s.emit(e)
		s.publish(it.row, "issue", sub.UserID)
	}
	return writeJSON(w, http.StatusOK, map[string]string{"key": newKey})
}

// applyMove writes one issue's move and returns its history lines.
func (s *Server) applyMove(ctx context.Context, tx *sql.Tx, plan *movePlan, it *moveItem, by int64, now time.Time) ([]change, error) {
	row, cat, dest := it.row, plan.cat, plan.dest
	newKey := dest.key + "-" + strconv.FormatInt(it.num, 10)
	changes := []change{
		{field: "Key", oldString: row.Key(), newString: newKey},
		{field: "project", oldValue: strconv.FormatInt(row.ProjectID, 10), oldString: row.ProjectName,
			newValue: strconv.FormatInt(dest.id, 10), newString: dest.name},
	}
	sets := []string{"project_id = ?", "num = ?", "issue_type_id = ?", "status_id = ?", "resolution_id = ?",
		"security_level_id = ?", "assignee_id = ?", "updated_at = ?"}
	args := []any{dest.id, it.num, it.typeID, it.status, sqlq.Value(it.resolution), it.level, it.assignee, now}
	if it.typeID != row.TypeID {
		changes = append(changes, change{"issuetype", row.TypeID, cat.typeOf(row.TypeID).Name, it.typeID,
			cat.typeOf(it.typeID).Name})
	}
	if it.status != row.StatusID {
		changes = append(changes, change{"status", row.StatusID, cat.statusOf(row.StatusID).Name, it.status,
			cat.statusOf(it.status).Name})
	}
	if it.resolution != row.ResolutionID {
		var oldName, newName any
		if v := cat.resolutionOf(row.ResolutionID); v != nil {
			oldName = v.Name
		}
		if v := cat.resolutionOf(it.resolution); v != nil {
			newName = v.Name
		}
		changes = append(changes, change{"resolution", sqlq.Value(row.ResolutionID), oldName, sqlq.Value(it.resolution), newName})
		sets = append(sets, "resolved_at = ?")
		if it.resolution.Valid {
			args = append(args, now)
		} else {
			args = append(args, nil)
		}
	}
	if sqlq.Value(row.LevelID) != it.level {
		changes = append(changes, change{"security", sqlq.Value(row.LevelID), plan.levels[0], it.level, plan.levels[1]})
	}
	if sqlq.Value(row.AssigneeID) != it.assignee {
		c := change{field: "assignee"}
		if row.AssigneeID.Valid {
			c.oldValue, c.oldString = s.userName(ctx, row.AssigneeID.Int64)
		}
		if who, ok := it.assignee.(int64); ok {
			c.newValue, c.newString = s.userName(ctx, who)
		}
		changes = append(changes, c)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET `+strings.Join(sets, ", ")+` WHERE id = ?`,
		append(args, row.ID)...); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO moved_issue_keys (old_key, issue_id) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE issue_id = ?`, row.Key(), row.ID, row.ID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO moved_issue_keys (old_key, issue_id)
		SELECT CONCAT(pkey, '-', ?), ? FROM project_keys WHERE project_id = ?`, row.Num, row.ID, row.ProjectID); err != nil {
		return nil, err
	}

	// A role is held per project: what was restricted to one follows the
	// choice made. The issue is locked, so nothing restricted appeared since the plan.
	for _, table := range []string{"comments", "worklogs"} {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE issue_id = ? AND visibility_role_id IS NOT NULL`,
			row.ID).Scan(&n); err != nil {
			return nil, err
		}
		planned := 0
		for _, r := range it.restricted {
			if r.table == table {
				planned++
			}
		}
		if n != planned {
			return nil, fail(http.StatusConflict, i18n.IssueMoveChanged)
		}
	}
	for _, r := range it.restricted {
		if _, err := tx.ExecContext(ctx, `UPDATE `+r.table+` SET `+r.remap.set+` WHERE id = ?`, r.remap.arg, r.id); err != nil {
			return nil, err
		}
	}

	for k, kind := range moveKinds {
		lines, err := replaceSet(ctx, tx, row.ID, kind.field, kind.kind, it.sets[k].from, it.sets[k].to)
		if err != nil {
			return nil, err
		}
		changes = append(changes, lines...)
	}

	if len(it.dropped) > 0 {
		marks, ids := sqlq.Marks(len(it.dropped)), moveArgs(it.dropped)
		values := map[int64][]string{}
		if err := sqlq.Each(ctx, tx, `SELECT v.field_id, o.value, v.string_value, v.number_value, v.text_value, v.date_value
			FROM custom_field_values v LEFT JOIN custom_field_options o ON o.field_id = v.field_id AND o.id = v.string_value
			WHERE v.issue_id = ? AND v.field_id IN (`+marks+`)`, func(rows *sql.Rows) error {
			var id int64
			var option, text, long sql.NullString
			var n sql.NullFloat64
			var d sql.NullTime
			if err := rows.Scan(&id, &option, &text, &n, &long, &d); err != nil {
				return err
			}
			switch {
			case option.Valid:
				values[id] = append(values[id], option.String)
			case text.Valid:
				values[id] = append(values[id], text.String)
			case n.Valid:
				values[id] = append(values[id], strconv.FormatFloat(n.Float64, 'f', -1, 64))
			case long.Valid:
				values[id] = append(values[id], long.String)
			case d.Valid:
				values[id] = append(values[id], d.Time.Format(time.DateOnly))
			}
			return nil
		}, append([]any{row.ID}, ids...)...); err != nil {
			return nil, err
		}
		if err := sqlq.Each(ctx, tx, `SELECT field_id, label FROM labels WHERE issue_id = ? AND field_id IN (`+marks+`)`,
			func(rows *sql.Rows) error {
				var id int64
				var label string
				err := rows.Scan(&id, &label)
				values[id] = append(values[id], label)
				return err
			}, append([]any{row.ID}, ids...)...); err != nil {
			return nil, err
		}
		for _, id := range it.dropped {
			changes = append(changes, change{field: plan.fieldNm[id], oldString: strings.Join(values[id], ", ")})
		}
		for _, table := range []string{"custom_field_values", "labels"} {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE issue_id = ? AND field_id IN (`+marks+`)`,
				append([]any{row.ID}, ids...)...); err != nil {
				return nil, err
			}
		}
	}

	// Open sprints of a board the target is not on are left; closed ones stay
	// in the history.
	for _, sp := range it.sprints {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sprint_issues WHERE sprint_id = ? AND issue_id = ?`, sp.ID, row.ID); err != nil {
			return nil, err
		}
		changes = append(changes, change{field: "Sprint", oldValue: strconv.FormatInt(sp.ID, 10), oldString: sp.Name})
	}
	return changes, recordChanges(ctx, tx, row.ID, by, now, changes)
}
