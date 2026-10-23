package tests

// Bulk changes from the search results: POST /api/issues/bulk.

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

type bulkResultOut struct {
	Key        string
	OK         bool
	Error      string
	NewKey     string
	WithParent bool
}

func bulk(t *testing.T, c *client, body map[string]any) map[string]bulkResultOut {
	t.Helper()
	var out struct{ Results []bulkResultOut }
	c.call("POST", "/api/issues/bulk", body, 200, &out)
	byKey := map[string]bulkResultOut{}
	for _, r := range out.Results {
		byKey[r.Key] = r
	}
	return byKey
}

func lastHistory(t *testing.T, c *client, key string) historyEntry {
	t.Helper()
	var history []historyEntry
	c.get("/api/issues/"+key+"/history", &history)
	if len(history) == 0 {
		t.Fatalf("%s has no history", key)
	}
	return history[len(history)-1]
}

// One issue refused does not stop the others, and each one changed has the
// history a single edit would have written.
func TestBulkEdit(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	open, archived := newProject(t, admin, nil), newProject(t, admin, nil)
	a := createIssue(t, paul, open, "Groupé A", "")
	b := createIssue(t, paul, open, "Groupé B", "")
	single := createIssue(t, paul, open, "Modifié seul", "")
	frozen := createIssue(t, paul, archived, "Lecture seule", "")
	admin.call("POST", "/api/projects/"+archived+"/archive", nil, 200, nil)
	priority := pick(t, `SELECT id FROM priorities WHERE NOT is_default ORDER BY position LIMIT 1`)
	for _, key := range []string{a, b, single} {
		paul.call("PATCH", "/api/issues/"+key, map[string]any{"labels": []string{"garder", "ôter"}}, 200, nil)
	}

	got := bulk(t, paul, map[string]any{"op": "edit", "keys": []string{a, frozen, b, "NOPE-1"},
		"priority": priority, "labels": map[string]any{"mode": "add", "values": []string{"ajout"}}})
	if !got[a].OK || !got[b].OK || got[frozen].OK || got[frozen].Error == "" || got["NOPE-1"].OK {
		t.Fatalf("results: %+v", got)
	}
	var d issueDetail
	for _, key := range []string{a, b} {
		paul.get("/api/issues/"+key, &d)
		if d.Priority == nil || d.Priority.ID != priority || !slices.Equal(d.Labels, []string{"ajout", "garder", "ôter"}) {
			t.Errorf("%s after bulk: priority %+v, labels %v", key, d.Priority, d.Labels)
		}
	}
	paul.get("/api/issues/"+frozen, &d)
	if d.Priority != nil && d.Priority.ID == priority {
		t.Error("the refused issue changed")
	}

	paul.call("PATCH", "/api/issues/"+single, map[string]any{"priority": priority,
		"labels": []string{"ajout", "garder", "ôter"}}, 200, nil)
	if got, want := lastHistory(t, paul, a), lastHistory(t, paul, single); !slices.Equal(historyLines(got), historyLines(want)) {
		t.Errorf("bulk history %v, single edit's %v", historyLines(got), historyLines(want))
	}

	bulk(t, paul, map[string]any{"op": "edit", "keys": []string{a}, "labels": map[string]any{"mode": "remove",
		"values": []string{"ôter"}}})
	paul.get("/api/issues/"+a, &d)
	if !slices.Equal(d.Labels, []string{"ajout", "garder"}) {
		t.Errorf("labels after remove: %v", d.Labels)
	}

	for name, body := range map[string]map[string]any{
		"no keys":       {"op": "edit", "keys": []string{}, "priority": priority},
		"nothing":       {"op": "edit", "keys": []string{a}},
		"unknown op":    {"op": "rename", "keys": []string{a}},
		"bad mode":      {"op": "edit", "keys": []string{a}, "labels": map[string]any{"mode": "merge"}},
		"two projects":  {"op": "edit", "keys": []string{a, frozen}, "components": map[string]any{"mode": "add", "values": []int64{}}},
		"too many keys": {"op": "delete", "keys": make([]string, 201)},
	} {
		if status, _ := paul.do("POST", "/api/issues/bulk", body); status != 400 {
			t.Errorf("%s: %d, want 400", name, status)
		}
	}
}

func historyLines(h historyEntry) []string {
	var out []string
	for _, it := range h.Items {
		line := it.Field + ":"
		if it.From != nil {
			line += *it.From
		}
		line += ">"
		if it.To != nil {
			line += *it.To
		}
		out = append(out, line)
	}
	return out
}

// A transition one issue's workflow lacks is refused for it alone.
func TestBulkTransition(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	project := newProject(t, admin, nil)

	// A straight line: first, then second, then third.
	var wl workflowList
	admin.call("GET", "/api/projects/"+project+"/workflows", nil, 200, &wl)
	wf := wl.Workflows[0]
	first := wf.Initial.ID
	var others []string
	for _, st := range wf.Statuses {
		if st.ID != first {
			others = append(others, st.ID)
		}
	}
	if len(others) < 2 {
		t.Fatalf("the new project's workflow has %d statuses", len(wf.Statuses))
	}
	second, third := others[0], others[1]
	admin.call("PUT", fmt.Sprintf("/api/workflows/%d", wf.ID), map[string]any{"name": wf.Name, "initial": first,
		"statuses": []string{first, second, third}, "transitions": []map[string]any{
			{"name": "Un", "from": first, "to": second}, {"name": "Deux", "from": second, "to": third}}}, 200, nil)

	moved := createIssue(t, paul, project, "Avancé", "")
	stays := createIssue(t, paul, project, "Resté", "")
	paul.call("PATCH", "/api/issues/"+moved, map[string]any{"status": second}, 200, nil)

	got := bulk(t, paul, map[string]any{"op": "transition", "keys": []string{stays, moved}, "status": third})
	if !got[moved].OK || got[stays].OK || got[stays].Error == "" {
		t.Fatalf("results: %+v", got)
	}
	if statusOf(t, moved) != third || statusOf(t, stays) != first {
		t.Errorf("statuses after: %s %s, want %s %s", statusOf(t, moved), statusOf(t, stays), third, first)
	}
}

// Deleting a parent with its sub-task selected deletes both, the sub-task
// reported as gone with its parent.
func TestBulkDelete(t *testing.T) {
	admin := login(t, "admin", "admin")
	project := newProject(t, admin, nil)
	parent := createIssue(t, admin, project, "Parent", "")
	other := createIssue(t, admin, project, "Autre", "")
	subtaskType := pick(t, `SELECT t.issue_type_id FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = ? AND it.subtask LIMIT 1`, project)
	var child struct{ Key string }
	admin.call("POST", "/api/issues", map[string]any{"project": project, "type": subtaskType,
		"summary": "Enfant", "parent": parent}, 201, &child)

	got := bulk(t, admin, map[string]any{"op": "delete", "keys": []string{child.Key, parent, other}})
	if !got[parent].OK || !got[other].OK || !got[child.Key].OK || !got[child.Key].WithParent {
		t.Fatalf("results: %+v", got)
	}
	for _, key := range []string{parent, other, child.Key} {
		if status := admin.get("/api/issues/"+key, nil); status != 404 {
			t.Errorf("%s after bulk delete: %d", key, status)
		}
	}
}

// Moving several issues takes them all to the target, each with a new key.
func TestBulkMove(t *testing.T) {
	admin := login(t, "admin", "admin")
	src, dst := newProject(t, admin, nil), newProject(t, admin, nil)
	a := createIssue(t, admin, src, "Déplacé A", "")
	b := createIssue(t, admin, src, "Déplacé B", "")

	var check struct {
		Check   moveCheckOut
		Results []bulkResultOut
	}
	admin.call("POST", "/api/issues/bulk", map[string]any{"op": "move", "keys": []string{a, b},
		"move": map[string]any{"project": dst, "check": true}}, 200, &check)
	if !check.Check.Ready || len(check.Results) != 2 || !check.Results[0].OK || !check.Results[1].OK {
		t.Fatalf("check: %+v", check)
	}

	got := bulk(t, admin, map[string]any{"op": "move", "keys": []string{a, b}, "move": map[string]any{"project": dst}})
	for _, key := range []string{a, b} {
		if !got[key].OK || !strings.HasPrefix(got[key].NewKey, dst+"-") {
			t.Errorf("%s: %+v", key, got[key])
		}
	}
}
