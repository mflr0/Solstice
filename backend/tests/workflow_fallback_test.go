package tests

// What a project without a workflow does today. The product never makes one,
// but the schema allows it.

import (
	"slices"
	"strconv"
	"testing"
)

// projectWithoutWorkflow is a new project whose workflows are deleted.
func projectWithoutWorkflow(t *testing.T, admin *client) string {
	t.Helper()
	key := newProject(t, admin, nil)
	if _, err := testDB.Exec(`DELETE FROM workflows WHERE project_id = (SELECT id FROM projects WHERE pkey = ?)`, key); err != nil {
		t.Fatal(err)
	}
	if n := pick(t, `SELECT COUNT(*) FROM workflows w JOIN projects p ON p.id = w.project_id WHERE p.pkey = ?`, key); n != "0" {
		t.Fatalf("%s workflows left", n)
	}
	return key
}

func statusOfCategory(t *testing.T, category string, offset int) string {
	t.Helper()
	return pick(t, `SELECT id FROM statuses WHERE category = ? ORDER BY position, id LIMIT 1 OFFSET `+strconv.Itoa(offset), category)
}

func setStatus(t *testing.T, key, status string) {
	t.Helper()
	if _, err := testDB.Exec(`UPDATE issues i JOIN projects p ON p.id = i.project_id SET i.status_id = ? WHERE `+keyExpr+` = ?`,
		status, key); err != nil {
		t.Fatal(err)
	}
}

func transitionTargets(d issueDetail) []string {
	var ids []string
	for _, tr := range d.Transitions {
		ids = append(ids, tr.To.ID)
	}
	return ids
}

func TestTransitionsWithoutWorkflow(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	todo, otherTodo := statusOfCategory(t, "todo", 0), statusOfCategory(t, "todo", 1)
	doing, done := statusOfCategory(t, "doing", 0), statusOfCategory(t, "done", 0)
	key := projectWithoutWorkflow(t, admin)

	// A new issue starts in the first "to do" status, and has nowhere to go
	// while no other status is in use.
	first := createIssue(t, paul, key, "Premier", "")
	var d issueDetail
	paul.call("GET", "/api/issues/"+first, nil, 200, &d)
	if d.Status.ID != todo || len(d.Transitions) != 0 || len(d.Statuses) != 1 {
		t.Fatalf("first issue: status %s, transitions %v, statuses %v, want %s alone", d.Status.ID, transitionTargets(d), d.Statuses, todo)
	}

	// Every status in use in the project is a way out, by category.
	second, third := createIssue(t, paul, key, "Second", ""), createIssue(t, paul, key, "Troisième", "")
	setStatus(t, second, doing)
	setStatus(t, third, done)
	paul.call("GET", "/api/issues/"+first, nil, 200, &d)
	if got := transitionTargets(d); !slices.Equal(got, []string{doing, done}) {
		t.Errorf("transitions of %s: %v, want %v", first, got, []string{doing, done})
	}
	for _, tr := range d.Transitions {
		if tr.ID != 0 {
			t.Errorf("transition to %s has the workflow id %d", tr.To.ID, tr.ID)
		}
	}
	var statuses []string
	for _, st := range d.Statuses {
		statuses = append(statuses, st.ID)
	}
	if !slices.Equal(statuses, []string{todo, doing, done}) {
		t.Errorf("statuses of %s: %v, want %v", first, statuses, []string{todo, doing, done})
	}
	paul.call("GET", "/api/issues/"+second, nil, 200, &d)
	if got := transitionTargets(d); !slices.Equal(got, []string{todo, done}) {
		t.Errorf("transitions of %s: %v, want %v", second, got, []string{todo, done})
	}

	// The REST API lists the same statuses for each issue type, and numbers the
	// transitions after their target.
	var rest []any
	if status := paul.get("/rest/api/2/project/"+key+"/statuses", &rest); status != 200 || len(rest) == 0 {
		t.Fatalf("project statuses: %d %v", status, rest)
	}
	for i := range rest {
		var got []string
		for j := range at(rest[i], "statuses").([]any) {
			got = append(got, str(rest[i], "statuses", j, "id"))
		}
		if !slices.Equal(got, []string{todo, doing, done}) {
			t.Errorf("REST statuses of %s: %v, want %v", str(rest[i], "name"), got, []string{todo, doing, done})
		}
	}
	var tr struct {
		Transitions []struct{ ID, Name string }
	}
	paul.call("GET", "/rest/api/2/issue/"+first+"/transitions", nil, 200, &tr)
	doingID, _ := strconv.Atoi(doing)
	doneID, _ := strconv.Atoi(done)
	want := []string{strconv.Itoa(100000000 + doingID), strconv.Itoa(100000000 + doneID)}
	var ids []string
	for _, x := range tr.Transitions {
		ids = append(ids, x.ID)
	}
	if !slices.Equal(ids, want) {
		t.Errorf("REST transitions of %s: %v, want %v", first, ids, want)
	}

	// A status nobody uses is refused, a used one is accepted.
	if status, body := paul.do("PATCH", "/api/issues/"+first, map[string]any{"status": otherTodo}); status != 409 {
		t.Errorf("to a status not in use: %d %s, want 409", status, body)
	}
	paul.call("POST", "/rest/api/2/issue/"+first+"/transitions", map[string]any{"transition": map[string]string{"id": want[0]}}, 204, nil)
	paul.call("GET", "/api/issues/"+first, nil, 200, &d)
	if d.Status.ID != doing {
		t.Errorf("%s in %s after the REST transition, want %s", first, d.Status.ID, doing)
	}

	// With no issue left in the first status, the new ones go to the "to do"
	// status most used.
	setStatus(t, second, otherTodo)
	setStatus(t, third, otherTodo)
	paul.call("GET", "/api/issues/"+createIssue(t, paul, key, "Quatrième", ""), nil, 200, &d)
	if d.Status.ID != otherTodo {
		t.Errorf("new issue in %s, want %s, the most used", d.Status.ID, otherTodo)
	}
}

func TestFirstWorkflowOfProject(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	todo, doing := statusOfCategory(t, "todo", 0), statusOfCategory(t, "doing", 0)
	key := projectWithoutWorkflow(t, admin)

	first := createIssue(t, paul, key, "Premier", "")
	setStatus(t, createIssue(t, paul, key, "Second", ""), doing)

	// The project's first workflow is its default, starting from the statuses
	// its issues use.
	var created struct{ ID int64 }
	admin.call("POST", "/api/projects/"+key+"/workflows", map[string]any{"name": "Premier"}, 201, &created)
	var wl workflowList
	admin.call("GET", "/api/projects/"+key+"/workflows", nil, 200, &wl)
	if len(wl.Workflows) != 1 || wl.Workflows[0].ID != created.ID || !wl.Workflows[0].Default || wl.Workflows[0].Initial.ID != todo {
		t.Fatalf("workflows: %+v, want the new one, default, starting in %s", wl.Workflows, todo)
	}
	var statuses []string
	for _, st := range wl.Workflows[0].Statuses {
		statuses = append(statuses, st.ID)
	}
	want := []string{todo, doing}
	slices.Sort(statuses)
	slices.Sort(want)
	if !slices.Equal(statuses, want) || len(wl.Workflows[0].Transitions) != 0 {
		t.Errorf("statuses %v and %d transitions, want %v and none", statuses, len(wl.Workflows[0].Transitions), want)
	}

	// Its transitions rule now: the new workflow has none.
	var d issueDetail
	paul.call("GET", "/api/issues/"+first, nil, 200, &d)
	if len(d.Transitions) != 0 {
		t.Errorf("transitions of %s with an empty workflow: %v", first, transitionTargets(d))
	}
}
