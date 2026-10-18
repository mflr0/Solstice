package tests

// Moving an issue to another project: POST /api/issues/{key}/move.

import (
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type moveOptionsOut struct {
	Unmatched []idName
	Options   []idName
}

type moveCheckOut struct {
	Project         struct{ Key string }
	Ready           bool
	Missing         []string
	Types           []typeRef
	SubtaskTypes    []typeRef
	Statuses        []statusRef
	Components      *moveOptionsOut
	AffectsVersions *moveOptionsOut
	FixVersions     *moveOptionsOut
	SecurityLevel   *moveOptionsOut
	DroppedFields   []string
	Sprints         []string
	Assignee        *struct{ From, To *userRef }
	Subtasks        struct{ Count, Reset, Lost int }
	Restricted      *struct {
		Comments, Worklogs int
		Roles, Groups      []idName
	}
}

// moveInsert runs an INSERT on the test database and returns the new id.
func moveInsert(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	needDB(t)
	res, err := testDB.Exec(query, args...)
	if err != nil {
		t.Fatalf("fixture insert failed: %v\n%s", err, query)
	}
	id, _ := res.LastInsertId()
	return id
}

// moveEnv is two projects and an issue with a sub-task to move from one to
// the other. The target lacks the issue's type, its sub-task's type, its
// status, a component, a version and the security level, and has no context
// for one of its custom fields.
type moveEnv struct {
	t                     *testing.T
	admin                 *client
	src, dst              string
	srcID, dstID          int64
	typeA, typeB          string
	subSrc, subDst        string
	inReview, toDo        string
	apiSrc, legacySrc     int64
	apiDst, newDst        int64
	v1Src, v2Src          int64
	v1Dst, v3Dst          int64
	levelSrc, levelDst    int64
	scoped, global        int64
	scopedName            string
	issue, child          string
	issueID, childID      int64
	leaving, staying      string // names of the open sprints
	closedSprint          int64
	stayingSprint         int64
	leavingSprint         int64
	chloe                 int64
	allChoices, firstStep map[string]any
}

// refreshRules makes the server read the rules again after a direct change.
func (e *moveEnv) refreshRules(project string) {
	var perms struct{ Grants []grantOut }
	e.admin.call("GET", "/api/projects/"+project+"/permissions", nil, 200, &perms)
	e.admin.call("PUT", "/api/projects/"+project+"/permissions", map[string]any{"grants": perms.Grants}, 200, nil)
}

func newMoveEnv(t *testing.T) *moveEnv {
	t.Helper()
	needDB(t)
	admin := login(t, "admin", "admin")
	e := &moveEnv{t: t, admin: admin, chloe: userID(t, "chloe.simon")}
	e.typeA = pick(t, `SELECT id FROM issue_types WHERE NOT subtask ORDER BY position, id LIMIT 1`)
	e.typeB = pick(t, `SELECT id FROM issue_types WHERE NOT subtask ORDER BY position, id LIMIT 1 OFFSET 1`)
	e.inReview = pick(t, `SELECT id FROM statuses WHERE LOWER(name) = 'in review' ORDER BY position, id LIMIT 1`)

	// A second sub-task type, which only the target offers. Registered before
	// the projects, so that it goes after them.
	var sub catalogueItem
	admin.call("POST", "/api/admin/issue-types", map[string]any{"name": unique("Sous-tâche bis "), "subtask": true}, 201, &sub)
	e.subDst = sub.id()
	t.Cleanup(func() { admin.do("DELETE", "/api/admin/issue-types/"+e.subDst, nil) })
	e.subSrc = pick(t, `SELECT id FROM issue_types WHERE subtask AND id <> ? ORDER BY position, id LIMIT 1`, e.subDst)

	e.src = newProject(t, admin, map[string]any{"issueTypes": []string{e.typeA, e.subSrc},
		"access": map[string]any{"holder": "everyone"}})
	e.dst = newProject(t, admin, map[string]any{"issueTypes": []string{e.typeB, e.subDst}, "template": "simple",
		"defaultAssignee": "lead"})
	e.srcID = dbInt(t, `SELECT id FROM projects WHERE pkey = ?`, e.src)
	e.dstID = dbInt(t, `SELECT id FROM projects WHERE pkey = ?`, e.dst)
	e.toDo = pick(t, `SELECT id FROM statuses WHERE LOWER(name) = 'to do' ORDER BY position, id LIMIT 1`)

	// Components and versions: one matches by name (the case differs).
	component := func(project int64, name string) int64 {
		return moveInsert(t, `INSERT INTO components (project_id, name) VALUES (?, ?)`, project, name)
	}
	version := func(project int64, name string) int64 {
		return moveInsert(t, `INSERT INTO versions (project_id, name) VALUES (?, ?)`, project, name)
	}
	e.apiSrc, e.legacySrc = component(e.srcID, "API"), component(e.srcID, "Héritage")
	e.apiDst, e.newDst = component(e.dstID, "api"), component(e.dstID, "Nouveau")
	e.v1Src, e.v2Src = version(e.srcID, "1.0"), version(e.srcID, "2.0")
	e.v1Dst, e.v3Dst = version(e.dstID, "1.0"), version(e.dstID, "3.0")

	// Security levels, seen by the project leads (the administrator).
	e.levelSrc = moveInsert(t, `INSERT INTO security_levels (project_id, name) VALUES (?, ?)`, e.srcID, "Interne")
	e.levelDst = moveInsert(t, `INSERT INTO security_levels (project_id, name) VALUES (?, ?)`, e.dstID, "Autre")
	for _, l := range []int64{e.levelSrc, e.levelDst} {
		moveInsert(t, `INSERT INTO security_level_members (level_id, holder) VALUES (?, 'lead')`, l)
	}
	e.refreshRules(e.src)
	e.refreshRules(e.dst)

	// Custom fields: one that applies in the source only, one everywhere.
	e.scopedName = unique("Portée ")
	var f fieldDetail
	admin.call("POST", "/api/admin/fields", map[string]any{"name": e.scopedName, "type": "text",
		"context": map[string]any{"projects": []string{e.src}}}, 201, &f)
	e.scoped = f.ID
	admin.call("POST", "/api/admin/fields", map[string]any{"name": unique("Partout "), "type": "text"}, 201, &f)
	e.global = f.ID
	t.Cleanup(func() {
		for _, id := range []int64{e.scoped, e.global} {
			admin.do("DELETE", fmt.Sprintf("/api/admin/fields/%d", id), nil)
		}
	})

	var made struct{ Key string }
	admin.call("POST", "/api/issues", map[string]any{"project": e.src, "type": e.typeA, "summary": "À déplacer",
		"assignee": e.chloe, "components": []int64{e.apiSrc, e.legacySrc}, "affectsVersions": []int64{e.v1Src},
		"fixVersions": []int64{e.v2Src}, "customFields": map[string]any{
			strconv.FormatInt(e.scoped, 10): "valeur de portée", strconv.FormatInt(e.global, 10): "valeur partout"}},
		201, &made)
	e.issue = made.Key
	admin.call("POST", "/api/issues", map[string]any{"project": e.src, "type": e.subSrc, "summary": "Sous-tâche à déplacer",
		"parent": e.issue}, 201, &made)
	e.child = made.Key
	admin.call("PATCH", "/api/issues/"+e.issue, map[string]any{"status": e.inReview, "securityLevel": e.levelSrc}, 200, nil)
	e.issueID, e.childID = issueIDOf(t, e.issue), issueIDOf(t, e.child)

	// Sprints: the board of the first leaves out the target, the other has it.
	board1 := dbInt(t, `SELECT id FROM boards ORDER BY id LIMIT 1`)
	board2 := dbInt(t, `SELECT id FROM boards ORDER BY id LIMIT 1 OFFSET 1`)
	moveInsert(t, `INSERT INTO board_projects (board_id, project_id) VALUES (?, ?)`, board1, e.srcID)
	moveInsert(t, `INSERT INTO board_projects (board_id, project_id) VALUES (?, ?)`, board2, e.srcID)
	moveInsert(t, `INSERT INTO board_projects (board_id, project_id) VALUES (?, ?)`, board2, e.dstID)
	e.leaving, e.staying = unique("Sprint partant "), unique("Sprint restant ")
	e.leavingSprint = moveInsert(t, `INSERT INTO sprints (board_id, name, state, position) VALUES (?, ?, 'future', 0)`,
		board1, e.leaving)
	e.stayingSprint = moveInsert(t, `INSERT INTO sprints (board_id, name, state, position) VALUES (?, ?, 'future', 0)`,
		board2, e.staying)
	e.closedSprint = moveInsert(t, `INSERT INTO sprints (board_id, name, state, position) VALUES (?, ?, 'closed', 0)`,
		board1, unique("Sprint fini "))
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM sprints WHERE id IN (?, ?, ?)`, e.leavingSprint, e.stayingSprint, e.closedSprint)
	})
	for _, sp := range [][2]int64{{e.leavingSprint, e.issueID}, {e.closedSprint, e.issueID}, {e.stayingSprint, e.childID}} {
		moveInsert(t, `INSERT INTO sprint_issues (sprint_id, issue_id) VALUES (?, ?)`, sp[0], sp[1])
	}

	e.firstStep = map[string]any{"project": e.dst}
	e.allChoices = map[string]any{"project": e.dst, "type": e.typeB, "subtaskType": e.subDst, "status": e.toDo,
		"components": []int64{e.apiDst, e.newDst}, "fixVersions": []int64{e.v3Dst}, "securityLevel": e.levelDst}
	return e
}

// moveCheckOf is a dry run that must succeed.
func moveCheckOf(t *testing.T, who *client, key string, body map[string]any) moveCheckOut {
	t.Helper()
	in := map[string]any{"check": true}
	for k, v := range body {
		in[k] = v
	}
	var out moveCheckOut
	who.call("POST", "/api/issues/"+key+"/move", in, 200, &out)
	return out
}

func (e *moveEnv) check(who *client, key string, body map[string]any) moveCheckOut {
	e.t.Helper()
	return moveCheckOf(e.t, who, key, body)
}

func names(list []idName) []string {
	var out []string
	for _, v := range list {
		out = append(out, v.Name)
	}
	sort.Strings(out)
	return out
}

func statusNames(list []statusRef) []string {
	var out []string
	for _, v := range list {
		out = append(out, v.Name)
	}
	sort.Strings(out)
	return out
}

func TestMoveDryRunAsks(t *testing.T) {
	e := newMoveEnv(t)
	counter := dbInt(t, `SELECT issue_counter FROM projects WHERE id = ?`, e.dstID)

	// Nothing chosen yet: what the target lacks is asked for. The status
	// waits for the type, as it depends on its workflow.
	c := e.check(e.admin, e.issue, e.firstStep)
	if c.Ready || c.Project.Key != e.dst ||
		!slices.Equal(c.Missing, []string{"type", "components", "fixVersions", "securityLevel", "subtaskType"}) {
		t.Errorf("first step: %+v", c)
	}
	if len(c.Types) != 1 || c.Types[0].ID != e.typeB || len(c.SubtaskTypes) != 1 || c.SubtaskTypes[0].ID != e.subDst ||
		len(c.Statuses) != 0 {
		t.Errorf("types asked: %+v %+v, statuses %+v", c.Types, c.SubtaskTypes, c.Statuses)
	}
	if c.Components == nil || len(c.Components.Unmatched) != 1 || c.Components.Unmatched[0].Name != "Héritage" ||
		!slices.Equal(names(c.Components.Options), []string{"Nouveau", "api"}) {
		t.Errorf("components asked: %+v", c.Components)
	}
	if c.AffectsVersions != nil {
		t.Errorf("1.0 has a match, yet affected versions are asked: %+v", c.AffectsVersions)
	}
	if c.FixVersions == nil || len(c.FixVersions.Unmatched) != 1 || c.FixVersions.Unmatched[0].Name != "2.0" ||
		!slices.Equal(names(c.FixVersions.Options), []string{"1.0", "3.0"}) {
		t.Errorf("fix versions asked: %+v", c.FixVersions)
	}
	if c.SecurityLevel == nil || len(c.SecurityLevel.Unmatched) != 1 || c.SecurityLevel.Unmatched[0].Name != "Interne" ||
		len(c.SecurityLevel.Options) != 1 || c.SecurityLevel.Options[0].ID != e.levelDst {
		t.Errorf("security level asked: %+v", c.SecurityLevel)
	}
	if !slices.Equal(c.Sprints, []string{e.leaving}) || c.Subtasks.Count != 1 {
		t.Errorf("sprints %v, sub-tasks %+v", c.Sprints, c.Subtasks)
	}

	// With the type, the status (In Review is not in the target's workflow)
	// and the custom field that does not apply there show up.
	c = e.check(e.admin, e.issue, map[string]any{"project": e.dst, "type": e.typeB, "subtaskType": e.subDst})
	if c.Ready || slices.Contains(c.Missing, "type") || slices.Contains(c.Missing, "subtaskType") ||
		!slices.Contains(c.Missing, "status") {
		t.Errorf("second step: %+v", c)
	}
	if got := statusNames(c.Statuses); len(got) != 3 || slices.Contains(got, "In Review") {
		t.Errorf("target statuses: %v", got)
	}
	if !slices.Equal(c.DroppedFields, []string{e.scopedName}) {
		t.Errorf("dropped fields: %v, want only %q", c.DroppedFields, e.scopedName)
	}
	if c.Subtasks.Reset != 0 || c.Subtasks.Lost != 0 {
		t.Errorf("sub-tasks: %+v", c.Subtasks)
	}

	// Everything chosen: ready, and the assignee is said to change (chloe.simon
	// cannot be assigned in the target, whose default assignee is its lead).
	c = e.check(e.admin, e.issue, e.allChoices)
	if !c.Ready || len(c.Missing) != 0 || c.Assignee == nil || c.Assignee.From == nil ||
		c.Assignee.From.Username != "chloe.simon" || c.Assignee.To == nil || c.Assignee.To.Username != "admin" {
		t.Errorf("last step: %+v assignee %+v", c, c.Assignee)
	}

	// A dry run changes nothing.
	var d issueDetail
	e.admin.call("GET", "/api/issues/"+e.issue, nil, 200, &d)
	if d.Key != e.issue || d.Project.Key != e.src ||
		dbInt(t, `SELECT issue_counter FROM projects WHERE id = ?`, e.dstID) != counter ||
		dbInt(t, `SELECT COUNT(*) FROM moved_issue_keys WHERE issue_id IN (?, ?)`, e.issueID, e.childID) != 0 {
		t.Errorf("the dry run moved the issue: %s in %s", d.Key, d.Project.Key)
	}
}

func TestMoveNeedsTheChoices(t *testing.T) {
	e := newMoveEnv(t)
	counter := dbInt(t, `SELECT issue_counter FROM projects WHERE id = ?`, e.dstID)

	status, body := e.admin.do("POST", "/api/issues/"+e.issue+"/move", e.firstStep)
	missing, _ := obj(t, body)["missing"].([]any)
	if status != 400 || len(missing) != 5 {
		t.Errorf("move without choices: %d %s", status, body)
	}
	// Part of the choices is not enough.
	status, body = e.admin.do("POST", "/api/issues/"+e.issue+"/move", map[string]any{"project": e.dst, "type": e.typeB,
		"subtaskType": e.subDst})
	if missing, _ = obj(t, body)["missing"].([]any); status != 400 || !slices.Contains(missing, any("status")) {
		t.Errorf("move with the types only: %d %s", status, body)
	}

	// Choices must be the target's.
	step := func(extra map[string]any) map[string]any {
		in := map[string]any{"project": e.dst, "type": e.typeB}
		for k, v := range extra {
			in[k] = v
		}
		return in
	}
	for name, in := range map[string]map[string]any{
		"a type of the source":      {"project": e.dst, "type": e.typeA},
		"a sub-task type for it":    {"project": e.dst, "type": e.subDst},
		"an unknown status":         step(map[string]any{"status": "NOPE"}),
		"a component of the source": step(map[string]any{"components": []int64{e.apiSrc}}),
		"a version of the source":   step(map[string]any{"fixVersions": []int64{e.v2Src}}),
		"a sub-task type unknown":   step(map[string]any{"subtaskType": e.subSrc}),
		"a level of the source":     step(map[string]any{"securityLevel": e.levelSrc}),
		"a level that is no number": step(map[string]any{"securityLevel": "haut"}),
		"an unknown field":          step(map[string]any{"colour": "red"}),
	} {
		if status, body := e.admin.do("POST", "/api/issues/"+e.issue+"/move", in); status != 400 {
			t.Errorf("%s: %d %s, want 400", name, status, body)
		}
	}

	var d issueDetail
	e.admin.call("GET", "/api/issues/"+e.issue, nil, 200, &d)
	if d.Key != e.issue || dbInt(t, `SELECT issue_counter FROM projects WHERE id = ?`, e.dstID) != counter {
		t.Errorf("a refused move changed something: %s", d.Key)
	}
}

func TestMoveMovesTheIssueAndItsSubtasks(t *testing.T) {
	e := newMoveEnv(t)
	srcCounter := dbInt(t, `SELECT issue_counter FROM projects WHERE id = ?`, e.srcID)
	var moved struct{ Key string }
	e.admin.call("POST", "/api/issues/"+e.issue+"/move", e.allChoices, 200, &moved)
	newKey, newChild := e.dst+"-1", e.dst+"-2"
	if moved.Key != newKey {
		t.Fatalf("new key %q, want %s", moved.Key, newKey)
	}

	var d issueDetail
	e.admin.call("GET", "/api/issues/"+newKey, nil, 200, &d)
	if d.ID != e.issueID || d.Project.Key != e.dst || d.Type.ID != e.typeB || d.Status.ID != e.toDo ||
		d.Assignee == nil || d.Assignee.Username != "admin" || d.Summary != "À déplacer" {
		t.Errorf("moved issue: %+v", d)
	}
	if !slices.Equal(names(d.Components), []string{"Nouveau", "api"}) || len(d.AffectsVersions) != 1 ||
		d.AffectsVersions[0].ID != e.v1Dst || len(d.FixVersions) != 1 || d.FixVersions[0].ID != e.v3Dst {
		t.Errorf("components %v, affects %v, fix %v", d.Components, d.AffectsVersions, d.FixVersions)
	}
	if level := dbInt(t, `SELECT security_level_id FROM issues WHERE id = ?`, e.issueID); level != e.levelDst {
		t.Errorf("security level %d, want %d", level, e.levelDst)
	}
	var fieldNames []string
	for _, f := range d.CustomFields {
		fieldNames = append(fieldNames, f.Name)
	}
	if len(d.CustomFields) != 1 || strings.HasPrefix(fieldNames[0], "Portée") {
		t.Errorf("custom fields after the move: %v", fieldNames)
	}
	if dbInt(t, `SELECT COUNT(*) FROM custom_field_values WHERE issue_id = ? AND field_id = ?`, e.issueID, e.scoped) != 0 ||
		dbInt(t, `SELECT COUNT(*) FROM custom_field_values WHERE issue_id = ? AND field_id = ?`, e.issueID, e.global) != 1 {
		t.Error("the value of the field without context was kept, or the other one dropped")
	}

	// The sub-task came along with the same mapping, and is as confidential.
	var c issueDetail
	e.admin.call("GET", "/api/issues/"+newChild, nil, 200, &c)
	if c.ID != e.childID || c.Project.Key != e.dst || c.Type.ID != e.subDst || c.Status.ID != e.toDo ||
		dbInt(t, `SELECT parent_id FROM issues WHERE id = ?`, e.childID) != e.issueID ||
		dbInt(t, `SELECT security_level_id FROM issues WHERE id = ?`, e.childID) != e.levelDst {
		t.Errorf("moved sub-task: %+v", c)
	}

	// Keys came from the target's counter, one each; the source's is untouched.
	if n := dbInt(t, `SELECT issue_counter FROM projects WHERE id = ?`, e.dstID); n != 2 {
		t.Errorf("target counter %d, want 2", n)
	}
	if n := dbInt(t, `SELECT issue_counter FROM projects WHERE id = ?`, e.srcID); n != srcCounter {
		t.Errorf("source counter %d, was %d", n, srcCounter)
	}
	fresh := createIssue(t, e.admin, e.src, "Après le départ", "")
	if fresh == e.issue || fresh == e.child {
		t.Errorf("the source reused the key %s", fresh)
	}

	// Open sprints of a board without the target are left, the others and the
	// closed ones kept.
	inSprint := func(sprint, issue int64) bool {
		return dbInt(t, `SELECT COUNT(*) FROM sprint_issues WHERE sprint_id = ? AND issue_id = ?`, sprint, issue) == 1
	}
	if inSprint(e.leavingSprint, e.issueID) || !inSprint(e.closedSprint, e.issueID) || !inSprint(e.stayingSprint, e.childID) {
		t.Error("sprint membership after the move is wrong")
	}

	// The old keys keep working: the interface's API, the REST API and JQL.
	for old, now := range map[string]string{e.issue: newKey, e.child: newChild} {
		var o struct{ Key string }
		if status := e.admin.get("/api/issues/"+old, &o); status != 200 || o.Key != now {
			t.Errorf("GET /api/issues/%s: %d %q, want %s", old, status, o.Key, now)
		}
		status, _, body := call(t, nil, testURL, "GET", "/rest/api/2/issue/"+old, nil, basic("admin", "admin"))
		if status != 200 || str(obj(t, body), "key") != now {
			t.Errorf("GET /rest/api/2/issue/%s: %d %s", old, status, body)
		}
		var found struct {
			Total  int
			Issues []struct{ Key string }
		}
		if status := e.admin.get("/api/issues?jql="+url.QueryEscape("key = "+old), &found); status != 200 ||
			found.Total != 1 || found.Issues[0].Key != now {
			t.Errorf("JQL key = %s: %d %+v", old, status, found)
		}
	}
	var found struct {
		Total  int
		Issues []struct{ Key string }
	}
	if e.admin.get("/api/issues?jql="+url.QueryEscape("parent = "+e.issue), &found); found.Total != 1 ||
		found.Issues[0].Key != newChild {
		t.Errorf("JQL parent = %s: %+v", e.issue, found)
	}
	if e.admin.get("/api/issues?jql="+url.QueryEscape("key in ("+e.issue+", "+newChild+") ORDER BY key"), &found); found.Total != 2 {
		t.Errorf("JQL key in (old, new): %+v", found)
	}
	if e.admin.get("/api/issues?jql="+url.QueryEscape("key != "+e.issue+" AND project = "+e.dst), &found); found.Total != 1 {
		t.Errorf("JQL key != old: %+v", found)
	}

	// History: the lines of an edit, on the issue and on its sub-task.
	var history []historyEntry
	e.admin.call("GET", "/api/issues/"+newKey+"/history", nil, 200, &history)
	for _, field := range []string{"Key", "project", "issuetype", "status", "Component", "Fix Version", "assignee", "Sprint",
		e.scopedName} {
		if !historyHas(history, field) {
			t.Errorf("no history line for %s", field)
		}
	}
	for _, entry := range history {
		for _, it := range entry.Items {
			if it.Field == "Key" && (it.From == nil || *it.From != e.issue || it.To == nil || *it.To != newKey) {
				t.Errorf("Key line: %v → %v", it.From, it.To)
			}
		}
	}
	// The history API leaves the security level out; the line is recorded.
	if dbInt(t, `SELECT COUNT(*) FROM change_items ci JOIN change_groups cg ON cg.id = ci.group_id
		WHERE cg.issue_id = ? AND ci.field = 'security' AND ci.new_value = ?`, e.issueID, strconv.FormatInt(e.levelDst, 10)) != 1 {
		t.Error("no history line for the security level")
	}
	e.admin.call("GET", "/api/issues/"+newChild+"/history", nil, 200, &history)
	if !historyHas(history, "Key") || !historyHas(history, "issuetype") {
		t.Errorf("sub-task history: %+v", history)
	}

	// The audit log has the move.
	var page auditPage
	e.admin.call("GET", "/api/admin/audit?action=issue.move", nil, 200, &page)
	if i := slices.IndexFunc(page.Entries, func(a auditEntryOut) bool {
		return a.Target != nil && a.Target.Label != nil && *a.Target.Label == newKey
	}); i < 0 || page.Entries[i].Details["from"] != e.issue || page.Entries[i].Details["project"] != e.dst {
		t.Errorf("audit entries: %+v", page.Entries)
	}

	// The search bar finds the issue under its old key as well as the new one.
	for _, k := range []string{e.issue, newKey} {
		var found struct {
			Total int
			Mode  string
		}
		e.admin.call("GET", "/api/issues?q="+k, nil, 200, &found)
		if found.Total != 1 || found.Mode != "simple" {
			t.Errorf("search %s: %d issues in %s mode", k, found.Total, found.Mode)
		}
	}

	// Moved back, the issue gets a new key and answers to all its old ones.
	e.admin.call("POST", "/api/issues/"+newKey+"/move", map[string]any{"project": e.src, "type": e.typeA,
		"subtaskType": e.subSrc, "status": e.inReview}, 400, nil) // a component and a version are asked for
	var back struct{ Key string }
	e.admin.call("POST", "/api/issues/"+newKey+"/move", map[string]any{"project": e.src, "type": e.typeA,
		"subtaskType": e.subSrc, "status": e.inReview, "components": []int64{}, "fixVersions": []int64{},
		"securityLevel": e.levelSrc}, 200, &back)
	if !strings.HasPrefix(back.Key, e.src+"-") || back.Key == e.issue {
		t.Errorf("moved back as %s", back.Key)
	}
	for _, old := range []string{e.issue, newKey} {
		var o struct{ Key string }
		if status := e.admin.get("/api/issues/"+old, &o); status != 200 || o.Key != back.Key {
			t.Errorf("GET /api/issues/%s after the second move: %d %q, want %s", old, status, o.Key, back.Key)
		}
	}
}

func TestMovePermissions(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")  // developpeurs
	chloe := login(t, "chloe.simon", "motdepasse") // not a developer
	types := []string{pick(t, `SELECT id FROM issue_types WHERE NOT subtask ORDER BY position, id LIMIT 1`)}
	src := newProject(t, admin, map[string]any{"issueTypes": types})
	dst := newProject(t, admin, map[string]any{"issueTypes": types})
	issue := createIssue(t, paul, src, "À déplacer ou pas", "")
	move := map[string]any{"project": dst}
	path := "/api/issues/" + issue + "/move"

	grantsOf := func(project string) []grantOut {
		var p struct{ Grants []grantOut }
		admin.call("GET", "/api/projects/"+project+"/permissions", nil, 200, &p)
		return p.Grants
	}
	put := func(project string, grants []grantOut) {
		t.Helper()
		admin.call("PUT", "/api/projects/"+project+"/permissions", map[string]any{"grants": grants}, 200, nil)
	}
	without := func(grants []grantOut, drop func(grantOut) bool) []grantOut {
		var out []grantOut
		for _, g := range grants {
			if !drop(g) {
				out = append(out, g)
			}
		}
		return out
	}
	developers := func(permissions ...string) func(grantOut) bool {
		return func(g grantOut) bool { return g.Holder == "group" && slices.Contains(permissions, g.Permission) }
	}
	srcGrants, dstGrants := grantsOf(src), grantsOf(dst)

	// Allowed with the usual rights; the dry run needs them as much as the move.
	c := moveCheckOf(t, paul, issue, move)
	if !c.Ready {
		t.Errorf("paul.martin's dry run: %+v", c)
	}
	// An issue one does not see is not found.
	chloe.call("POST", path, move, 404, nil)
	chloe.call("POST", path, map[string]any{"project": dst, "check": true}, 404, nil)

	// In the source: browse and nothing to edit with.
	put(src, without(srcGrants, developers("edit")))
	paul.call("POST", path, map[string]any{"project": dst, "check": true}, 403, nil)
	paul.call("POST", path, move, 403, nil)
	put(src, srcGrants)

	// In the target: sees it, may not create in it.
	put(dst, without(dstGrants, developers("create")))
	paul.call("POST", path, map[string]any{"project": dst, "check": true}, 403, nil)
	paul.call("POST", path, move, 403, nil)
	// An archived target is read-only.
	put(dst, dstGrants)
	admin.call("POST", "/api/projects/"+dst+"/archive", nil, 200, nil)
	paul.call("POST", path, move, 403, nil)
	admin.call("POST", "/api/projects/"+dst+"/unarchive", nil, 200, nil)

	// Not even seeing the target: it does not exist for paul.martin.
	put(dst, without(dstGrants, developers("browse", "create", "edit")))
	paul.call("POST", path, move, 404, nil)
	put(dst, dstGrants)
	if dbInt(t, `SELECT COUNT(*) FROM moved_issue_keys WHERE issue_id = ?`, issueIDOf(t, issue)) != 0 {
		t.Fatal("a refused move left an old key")
	}

	// Allowed again: the move is done, and the old key answers to the user.
	var moved struct{ Key string }
	paul.call("POST", path, move, 200, &moved)
	var d struct{ Key string }
	if status := paul.get("/api/issues/"+issue, &d); status != 200 || d.Key != moved.Key ||
		!strings.HasPrefix(moved.Key, dst+"-") {
		t.Errorf("after the move: %d %q (answer %q)", status, d.Key, moved.Key)
	}
}

func TestMoveRefusesWhatCannotMove(t *testing.T) {
	e := newMoveEnv(t)
	post := func(key string, body map[string]any) int {
		status, _ := e.admin.do("POST", "/api/issues/"+key+"/move", body)
		return status
	}
	for name, c := range map[string]struct {
		key  string
		body map[string]any
		want int
	}{
		"the project it is in":      {e.issue, map[string]any{"project": e.src}, 400},
		"a sub-task on its own":     {e.child, map[string]any{"project": e.dst}, 400},
		"no project":                {e.issue, map[string]any{}, 400},
		"a project that is unknown": {e.issue, map[string]any{"project": "ZZZNOPE"}, 404},
		"an issue that is unknown":  {e.dst + "-999", map[string]any{"project": e.dst}, 404},
	} {
		if got := post(c.key, c.body); got != c.want {
			t.Errorf("%s: %d, want %d", name, got, c.want)
		}
	}
	// The messages are in the language asked for.
	status, msg, _ := e.admin.errorIn("fr", "POST", "/api/issues/"+e.child+"/move", map[string]any{"project": e.dst})
	if status != 400 || !strings.Contains(msg, "sous-tâche") {
		t.Errorf("sub-task refusal in French: %d %q", status, msg)
	}
	status, msg, _ = e.admin.errorIn("de", "POST", "/api/issues/"+e.issue+"/move", e.firstStep)
	if status != 400 || msg == "" || strings.Contains(msg, "Choose before moving") {
		t.Errorf("missing choices in German: %d %q", status, msg)
	}
}

func TestMoveDryRunHidesTheLevelsFromWhoCannotSetThem(t *testing.T) {
	e := newMoveEnv(t)
	paul := login(t, "paul.martin", "motdepasse")
	// paul.martin sees the level of the issue, which gives him Edit here and
	// Create in the target, but not the right to set a level there.
	moveInsert(t, `INSERT INTO security_level_members (level_id, holder, user_id) VALUES (?, 'user', ?)`,
		e.levelSrc, userID(t, "paul.martin"))
	e.refreshRules(e.src)
	body := map[string]any{"project": e.dst, "type": e.typeB, "subtaskType": e.subDst, "status": e.toDo,
		"components": []int64{e.apiDst}, "fixVersions": []int64{e.v3Dst}}

	c := e.check(e.admin, e.issue, body)
	if c.SecurityLevel == nil || len(c.SecurityLevel.Options) != 1 || c.SecurityLevel.Options[0].ID != e.levelDst {
		t.Fatalf("the project lead is not offered the target's levels: %+v", c.SecurityLevel)
	}
	c = e.check(paul, e.issue, body)
	if c.Ready || !slices.Contains(c.Missing, "securityLevel") || c.SecurityLevel == nil ||
		len(c.SecurityLevel.Unmatched) != 1 || c.SecurityLevel.Unmatched[0].Name != "Interne" {
		t.Errorf("no match must still be reported: %+v", c)
	}
	if c.SecurityLevel != nil && len(c.SecurityLevel.Options) != 0 {
		t.Errorf("the target's levels are listed to a user without the Security right: %+v", c.SecurityLevel.Options)
	}
}

// restrictedMove is a move env where the issue and its sub-task carry comments
// and work logs restricted to a role of the source project, to a group, and to
// no one. The administrator holds the source role, and a role of the target
// other than the one the test will choose.
type restrictedMove struct {
	*moveEnv
	srcRole, dstRole, otherRole int64
	restricted                  []int64 // comments restricted to the source role
	worklog, groupComment       int64
}

func newRestrictedMove(t *testing.T) *restrictedMove {
	t.Helper()
	e := &restrictedMove{moveEnv: newMoveEnv(t)}
	role := func() int64 {
		var r struct{ ID int64 }
		e.admin.call("POST", "/api/admin/roles", map[string]any{"name": unique("Rôle ")}, 201, &r)
		return r.ID
	}
	e.srcRole, e.dstRole, e.otherRole = role(), role(), role()
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM comments WHERE visibility_role_id IN (?, ?, ?)`, e.srcRole, e.dstRole, e.otherRole)
		testDB.Exec(`DELETE FROM worklogs WHERE visibility_role_id IN (?, ?, ?)`, e.srcRole, e.dstRole, e.otherRole)
		for _, id := range []int64{e.srcRole, e.dstRole, e.otherRole} {
			e.admin.do("DELETE", fmt.Sprintf("/api/admin/roles/%d", id), nil)
		}
	})
	admin := userID(t, "admin")
	member := func(project, role int64) {
		moveInsert(t, `INSERT INTO project_role_members (project_id, role_id, user_id) VALUES (?, ?, ?)`, project, role, admin)
	}
	member(e.srcID, e.srcRole)
	member(e.dstID, e.otherRole)
	// A role that has no member in the target is not one of its roles.
	moveInsert(t, `INSERT INTO project_role_members (project_id, role_id, user_id) VALUES (?, ?, ?)`, e.dstID, e.dstRole,
		userID(t, "paul.martin"))

	comment := func(issue int64, body string, group any, role any) int64 {
		return moveInsert(t, `INSERT INTO comments (issue_id, author_id, body, created_at, updated_at, visibility_group,
			visibility_role_id) VALUES (?, ?, ?, NOW(3), NOW(3), ?, ?)`, issue, admin, body, group, role)
	}
	e.restricted = []int64{comment(e.issueID, "Réservé au rôle", nil, e.srcRole),
		comment(e.childID, "Réservé au rôle (sous-tâche)", nil, e.srcRole)}
	e.groupComment = comment(e.issueID, "Réservé au groupe", "developpeurs", nil)
	comment(e.issueID, "Pour tous", nil, nil)
	e.worklog = moveInsert(t, `INSERT INTO worklogs (issue_id, author_id, started_at, seconds, created_at, updated_at,
		visibility_role_id) VALUES (?, ?, NOW(3), 600, NOW(3), NOW(3), ?)`, e.issueID, admin, e.srcRole)
	return e
}

// seen lists the ids of the comments of an issue the user can read.
func (e *restrictedMove) seen(key string) []int64 {
	e.t.Helper()
	var list []commentOut
	e.admin.call("GET", "/api/issues/"+key+"/comments", nil, 200, &list)
	var out []int64
	for _, c := range list {
		out = append(out, c.ID)
	}
	return out
}

func TestMoveAsksWhatBecomesOfRoleRestrictedItems(t *testing.T) {
	e := newRestrictedMove(t)
	path := "/api/issues/" + e.issue + "/move"
	c := e.check(e.admin, e.issue, e.allChoices)
	if c.Ready || !slices.Contains(c.Missing, "restricted") || c.Restricted == nil || c.Restricted.Comments != 2 ||
		c.Restricted.Worklogs != 1 {
		t.Fatalf("the dry run does not count the restricted items: %+v", c)
	}
	// Only the roles that have a member in the target are offered.
	if got := c.Restricted.Roles; len(got) != 2 {
		t.Errorf("roles offered: %+v", got)
	}
	if !slices.ContainsFunc(c.Restricted.Roles, func(r idName) bool { return r.ID == e.dstRole }) ||
		slices.ContainsFunc(c.Restricted.Roles, func(r idName) bool { return r.ID == e.srcRole }) {
		t.Errorf("roles offered: %+v", c.Restricted.Roles)
	}
	if len(c.Restricted.Groups) == 0 {
		t.Error("an administrator is offered no group")
	}

	// Without a choice the move is refused, and so are bad ones: nothing changes.
	refused := map[string]map[string]any{
		"no choice":                 nil,
		"a refusal":                 {"to": "refuse"},
		"a role of the source":      {"to": "role", "role": e.srcRole},
		"no role":                   {"to": "role"},
		"a group that is unknown":   {"to": "group", "group": -1},
		"something else":            {"to": "public"},
		"a role with a group's key": {"to": "role", "group": 1},
	}
	for name, choice := range refused {
		body := map[string]any{}
		for k, v := range e.allChoices {
			body[k] = v
		}
		if choice != nil {
			body["restricted"] = choice
		}
		if status, raw := e.admin.do("POST", path, body); status != 400 {
			t.Errorf("%s: %d %s, want 400", name, status, raw)
		}
	}
	if dbInt(t, `SELECT project_id FROM issues WHERE id = ?`, e.issueID) != e.srcID ||
		dbInt(t, `SELECT COUNT(*) FROM comments WHERE visibility_role_id = ?`, e.srcRole) != 2 {
		t.Error("a refused move changed something")
	}
	status, msg, _ := e.admin.errorIn("fr", "POST", path, map[string]any{"project": e.dst, "restricted": map[string]any{"to": "refuse"}})
	if status != 400 || !strings.Contains(msg, "rôle") {
		t.Errorf("refusal in French: %d %q", status, msg)
	}
}

func TestMoveKeepsRoleRestrictedItemsToARoleOfTheTarget(t *testing.T) {
	e := newRestrictedMove(t)
	body := map[string]any{"restricted": map[string]any{"to": "role", "role": e.dstRole}}
	for k, v := range e.allChoices {
		body[k] = v
	}
	if c := e.check(e.admin, e.issue, body); !c.Ready {
		t.Fatalf("the dry run with a choice: %+v", c)
	}
	var moved struct{ Key string }
	e.admin.call("POST", "/api/issues/"+e.issue+"/move", body, 200, &moved)

	// The role is the chosen one, on the issue's and the sub-task's, comments and work logs.
	if n := dbInt(t, `SELECT COUNT(*) FROM comments WHERE visibility_role_id = ? AND issue_id IN (?, ?)`, e.dstRole,
		e.issueID, e.childID); n != 2 {
		t.Errorf("%d comments on the chosen role, want 2", n)
	}
	if dbInt(t, `SELECT visibility_role_id FROM worklogs WHERE id = ?`, e.worklog) != e.dstRole {
		t.Error("the work log did not follow")
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM comments WHERE visibility_role_id = ?`, e.srcRole); n != 0 {
		t.Errorf("%d comments still on the role of the source", n)
	}
	// What was restricted to a group stays so.
	if g := pick(t, `SELECT visibility_group FROM comments WHERE id = ?`, e.groupComment); g != "developpeurs" {
		t.Errorf("group-restricted comment: %q", g)
	}

	// The administrator holds another role of the target: the items are hidden from them, until they hold the chosen one.
	hidden := func(want bool) {
		t.Helper()
		got := e.seen(moved.Key)
		for _, id := range e.restricted[:1] {
			if slices.Contains(got, id) == want {
				t.Errorf("comment %d readable = %v, want %v", id, !want, want)
			}
		}
	}
	hidden(true)
	moveInsert(t, `INSERT INTO project_role_members (project_id, role_id, user_id) VALUES (?, ?, ?)`, e.dstID, e.dstRole,
		userID(t, "admin"))
	hidden(false)
}

func TestMoveRestrictsRoleRestrictedItemsToAGroup(t *testing.T) {
	e := newRestrictedMove(t)
	var g struct{ ID int64 }
	name := strings.ToLower(unique("pivot"))
	e.admin.call("POST", "/api/admin/groups", map[string]any{"name": name}, 201, &g)
	t.Cleanup(func() {
		testDB.Exec(`UPDATE comments SET visibility_group = NULL WHERE visibility_group = ?`, name)
		testDB.Exec(`UPDATE worklogs SET visibility_group = NULL WHERE visibility_group = ?`, name)
		e.admin.do("DELETE", fmt.Sprintf("/api/admin/groups/%d", g.ID), nil)
	})
	body := map[string]any{"restricted": map[string]any{"to": "group", "group": g.ID}}
	for k, v := range e.allChoices {
		body[k] = v
	}
	var moved struct{ Key string }
	e.admin.call("POST", "/api/issues/"+e.issue+"/move", body, 200, &moved)

	if n := dbInt(t, `SELECT COUNT(*) FROM comments WHERE visibility_group = ? AND visibility_role_id IS NULL
		AND id IN (?, ?)`, name, e.restricted[0], e.restricted[1]); n != 2 {
		t.Errorf("%d comments on the chosen group, want 2", n)
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM worklogs WHERE id = ? AND visibility_group = ? AND visibility_role_id IS NULL`,
		e.worklog, name); n != 1 {
		t.Error("the work log did not follow")
	}
	if g := pick(t, `SELECT visibility_group FROM comments WHERE id = ?`, e.groupComment); g != "developpeurs" {
		t.Errorf("group-restricted comment: %q", g)
	}
	// Outside the group nobody reads them; a member does.
	if slices.Contains(e.seen(moved.Key), e.restricted[0]) {
		t.Error("a user outside the group reads the comment")
	}
	e.admin.call("POST", fmt.Sprintf("/api/admin/groups/%d/members", g.ID), map[string]any{"users": []int64{userID(t, "admin")}}, 200, nil)
	if !slices.Contains(e.seen(moved.Key), e.restricted[0]) {
		t.Error("a member of the group does not read the comment")
	}
}

// moverWithoutTheRole is a user who may move the issue but holds none of the
// roles its comments are restricted to. The issue loses its security level
// first, which only someone who may set levels could carry over.
func (e *restrictedMove) moverWithoutTheRole() (*client, map[string]any) {
	e.t.Helper()
	e.admin.call("PATCH", "/api/issues/"+e.issue, map[string]any{"securityLevel": nil}, 200, nil)
	body := map[string]any{}
	for k, v := range e.allChoices {
		if k != "securityLevel" {
			body[k] = v
		}
	}
	return login(e.t, "paul.martin", "motdepasse"), body
}

func TestMoveRefusesRoleRestrictedItemsTheMoverCannotRead(t *testing.T) {
	e := newRestrictedMove(t)
	paul, body := e.moverWithoutTheRole()
	path := "/api/issues/" + e.issue + "/move"
	developers := dbInt(t, `SELECT id FROM user_groups WHERE name = 'developpeurs'`)

	// The target has no role of the same name, and a choice does not help.
	for name, choice := range map[string]map[string]any{
		"no choice": nil,
		"a group":   {"to": "group", "group": developers},
		"a role":    {"to": "role", "role": e.dstRole},
	} {
		in := map[string]any{}
		for k, v := range body {
			in[k] = v
		}
		if choice != nil {
			in["restricted"] = choice
		}
		if status, raw := paul.do("POST", path, in); status != 400 {
			t.Errorf("%s: %d %s, want 400", name, status, raw)
		}
		in["check"] = true
		if status, raw := paul.do("POST", path, in); status != 400 {
			t.Errorf("%s, dry run: %d %s, want 400", name, status, raw)
		}
	}
	status, msg, _ := paul.errorIn("fr", "POST", path, body)
	if status != 400 || !strings.Contains(msg, "3 ") || !strings.Contains(msg, "administrateur") {
		t.Errorf("refusal in French: %d %q", status, msg)
	}
	if dbInt(t, `SELECT project_id FROM issues WHERE id = ?`, e.issueID) != e.srcID ||
		dbInt(t, `SELECT COUNT(*) FROM comments WHERE visibility_role_id = ?`, e.srcRole) != 2 {
		t.Error("a refused move changed something")
	}
}

func TestMoveKeepsUnreadableRoleRestrictedItemsOnTheSameNamedRole(t *testing.T) {
	e := newRestrictedMove(t)
	paul, body := e.moverWithoutTheRole()
	developers := dbInt(t, `SELECT id FROM user_groups WHERE name = 'developpeurs'`)
	// The target has the role of the source, held by someone else; and the mover
	// reads one comment, restricted to a role of theirs.
	moveInsert(t, `INSERT INTO project_role_members (project_id, role_id, user_id) VALUES (?, ?, ?)`, e.dstID, e.srcRole,
		userID(t, "admin"))
	var mine struct{ ID int64 }
	e.admin.call("POST", "/api/admin/roles", map[string]any{"name": unique("Rôle de paul ")}, 201, &mine)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM comments WHERE visibility_role_id = ?`, mine.ID)
		e.admin.do("DELETE", fmt.Sprintf("/api/admin/roles/%d", mine.ID), nil)
	})
	moveInsert(t, `INSERT INTO project_role_members (project_id, role_id, user_id) VALUES (?, ?, ?)`, e.srcID, mine.ID,
		userID(t, "paul.martin"))
	readable := moveInsert(t, `INSERT INTO comments (issue_id, author_id, body, created_at, updated_at, visibility_role_id)
		VALUES (?, ?, 'Pour le rôle de paul', NOW(3), NOW(3), ?)`, e.issueID, userID(t, "admin"), mine.ID)

	// Only the comment the mover reads is counted, and a group of theirs is offered for it.
	c := moveCheckOf(t, paul, e.issue, body)
	if c.Ready || !slices.Contains(c.Missing, "restricted") || c.Restricted == nil || c.Restricted.Comments != 1 ||
		c.Restricted.Worklogs != 0 {
		t.Fatalf("the dry run: %+v", c)
	}
	if !slices.ContainsFunc(c.Restricted.Groups, func(g idName) bool { return g.ID == developers }) {
		t.Errorf("groups offered: %+v", c.Restricted.Groups)
	}
	body["restricted"] = map[string]any{"to": "group", "group": developers}
	paul.call("POST", "/api/issues/"+e.issue+"/move", body, 200, nil)

	if g := pick(t, `SELECT visibility_group FROM comments WHERE id = ?`, readable); g != "developpeurs" {
		t.Errorf("the comment the mover reads: group %q", g)
	}
	// What the mover could not read keeps its role, which the target has.
	if n := dbInt(t, `SELECT COUNT(*) FROM comments WHERE visibility_role_id = ? AND issue_id IN (?, ?)`, e.srcRole,
		e.issueID, e.childID); n != 2 {
		t.Errorf("%d comments on the role of the same name, want 2", n)
	}
	if dbInt(t, `SELECT visibility_role_id FROM worklogs WHERE id = ?`, e.worklog) != e.srcRole {
		t.Error("the work log did not keep its role")
	}
}

func TestMoveDoesNotLimitAGlobalAdministratorToTheirRoles(t *testing.T) {
	e := newRestrictedMove(t)
	testDB.Exec(`DELETE FROM project_role_members WHERE project_id = ? AND role_id = ?`, e.srcID, e.srcRole)
	developers := dbInt(t, `SELECT id FROM user_groups WHERE name = 'developpeurs'`)
	body := map[string]any{"restricted": map[string]any{"to": "group", "group": developers}}
	for k, v := range e.allChoices {
		body[k] = v
	}
	if c := e.check(e.admin, e.issue, body); !c.Ready {
		t.Fatalf("the dry run: %+v", c)
	}
	e.admin.call("POST", "/api/issues/"+e.issue+"/move", body, 200, nil)
	if n := dbInt(t, `SELECT COUNT(*) FROM comments WHERE visibility_group = 'developpeurs' AND visibility_role_id IS NULL
		AND id IN (?, ?)`, e.restricted[0], e.restricted[1]); n != 2 {
		t.Errorf("%d comments on the chosen group, want 2", n)
	}
}

func TestMoveDryRunSendsEmptyListsNotNull(t *testing.T) {
	e := newRestrictedMove(t)
	bare := newProject(t, e.admin, map[string]any{"issueTypes": []string{e.typeA, e.subSrc}})
	status, raw := e.admin.do("POST", "/api/issues/"+e.issue+"/move", map[string]any{"project": bare, "check": true})
	if status != 200 {
		t.Fatalf("dry run: %d %s", status, raw)
	}
	out := obj(t, raw)
	for _, path := range [][2]string{{"components", "options"}, {"fixVersions", "options"}, {"securityLevel", "options"},
		{"restricted", "roles"}} {
		part, _ := out[path[0]].(map[string]any)
		if list, ok := part[path[1]].([]any); !ok || len(list) != 0 {
			t.Errorf("%s.%s: %v, want an empty list", path[0], path[1], part[path[1]])
		}
	}
}
