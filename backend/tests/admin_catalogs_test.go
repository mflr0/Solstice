package tests

// The shared catalogues: priorities, link types, issue types and the order
// of statuses. Items in use only go when their issues (or links) move.

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

type catalogueItem struct {
	ID       json.RawMessage `json:"id"` // text, a number for link types
	Name     string          `json:"name"`
	Color    *string         `json:"color"`
	Default  *bool           `json:"default"`
	Subtask  *bool           `json:"subtask"`
	Position int             `json:"position"`
	Issues   *int            `json:"issues"`
	Links    *int            `json:"links"`
}

// id gives an item's id as text.
func (it catalogueItem) id() string {
	var s string
	if json.Unmarshal(it.ID, &s) == nil {
		return s
	}
	return string(it.ID)
}

func catalogue(t *testing.T, admin *client, kind string) []catalogueItem {
	t.Helper()
	var list []catalogueItem
	admin.call("GET", "/api/admin/"+kind, nil, 200, &list)
	return list
}

func TestPriorities(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	paul.call("GET", "/api/admin/priorities", nil, 403, nil)
	before := catalogue(t, admin, "priorities")
	var oldDefault string
	var order []string
	for _, p := range before {
		order = append(order, p.id())
		if p.Default != nil && *p.Default {
			oldDefault = p.id()
		}
	}

	var p catalogueItem
	name := unique("Bloquant ")
	admin.call("POST", "/api/admin/priorities", map[string]any{"name": name, "color": "#1A2B3C"}, 201, &p)
	if p.Color == nil || *p.Color != "#1a2b3c" || p.Default == nil || *p.Default {
		t.Fatalf("created priority: %+v", p)
	}
	path := "/api/admin/priorities/" + p.id()
	for _, bad := range []struct {
		in   map[string]any
		want int
	}{
		{map[string]any{"name": name}, 409},
		{map[string]any{"name": "x", "color": "rouge"}, 400},
		{map[string]any{"name": "x", "subtask": true}, 400},
		{map[string]any{"color": "#000000"}, 400},
	} {
		if status, body := admin.do("POST", "/api/admin/priorities", bad.in); status != bad.want {
			t.Errorf("create %v: %d %s, want %d", bad.in, status, body, bad.want)
		}
	}

	// The default priority of new issues.
	admin.call("PATCH", path, map[string]any{"default": true}, 200, nil)
	t.Cleanup(func() {
		if oldDefault != "" {
			admin.do("PATCH", "/api/admin/priorities/"+oldDefault, map[string]any{"default": true})
		}
	})
	defaults := 0
	for _, it := range catalogue(t, admin, "priorities") {
		if it.Default != nil && *it.Default {
			defaults++
			if it.id() != p.id() {
				t.Errorf("%s is still a default", it.Name)
			}
		}
	}
	if defaults != 1 {
		t.Errorf("%d default priorities", defaults)
	}
	key := scratchIssue(t, paul, "Priorité par défaut")
	var d issueDetail
	paul.get("/api/issues/"+key, &d)
	if d.Priority == nil || d.Priority.ID != p.id() {
		t.Fatalf("new issue's priority: %+v, want %s", d.Priority, p.id())
	}

	// Order, then put back.
	reversed := append([]string{p.id()}, order...)
	slices.Reverse(reversed)
	admin.call("POST", "/api/admin/priorities/order", map[string]any{"ids": order}, 400, nil) // one missing
	var list []catalogueItem
	admin.call("POST", "/api/admin/priorities/order", map[string]any{"ids": reversed}, 200, &list)
	if list[0].id() != reversed[0] || list[len(list)-1].id() != reversed[len(reversed)-1] {
		t.Errorf("order: %+v", list)
	}
	admin.call("POST", "/api/admin/priorities/order", map[string]any{"ids": append(order, p.id())}, 200, nil)

	// In use: it goes only with its issues moved elsewhere.
	target := oldDefault
	if target == "" {
		target = order[0]
	}
	status, body := admin.do("DELETE", path, nil)
	if status != 409 || num(obj(t, body), "usage", "issues") != 1 {
		t.Fatalf("deleting a priority in use: %d %s", status, body)
	}
	admin.call("DELETE", path, map[string]any{"moveTo": p.id()}, 400, nil)
	admin.call("DELETE", path, map[string]any{"moveTo": target}, 204, nil)
	paul.get("/api/issues/"+key, &d)
	if d.Priority == nil || d.Priority.ID != target {
		t.Errorf("priority after the move: %+v, want %s", d.Priority, target)
	}
	var history []historyEntry
	paul.get("/api/issues/"+key+"/history", &history)
	if !historyHas(history, "priority") {
		t.Errorf("no history of the move: %+v", history)
	}
}

func TestLinkTypes(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	key := newProject(t, admin, nil)
	a, b := createIssue(t, paul, key, "Suit", ""), createIssue(t, paul, key, "Est suivi", "")

	var lt catalogueItem
	admin.call("POST", "/api/admin/link-types", map[string]any{"name": unique("Suite "), "inward": "est suivi par",
		"outward": "suit"}, 201, &lt)
	var n int64
	if json.Unmarshal(lt.ID, &n) != nil {
		t.Fatalf("a link type's id is a number: %s", lt.ID)
	}
	admin.call("POST", "/api/admin/link-types", map[string]any{"name": unique("Incomplet "), "inward": "x"}, 400, nil)
	paul.call("POST", "/api/issues/"+a+"/links", map[string]any{"type": n, "direction": "outward", "target": b}, 200, nil)

	path := fmt.Sprintf("/api/admin/link-types/%d", n)
	status, body := admin.do("DELETE", path, nil)
	if status != 409 || num(obj(t, body), "usage", "links") != 1 {
		t.Fatalf("deleting a link type in use: %d %s", status, body)
	}
	other := catalogue(t, admin, "link-types")[0]
	admin.call("DELETE", path, map[string]any{"moveTo": other.ID}, 204, nil)
	var d issueDetail
	paul.get("/api/issues/"+a, &d)
	if len(d.Links) != 1 {
		t.Errorf("links after the type went: %+v", d.Links)
	}
}

func TestIssueTypes(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	var standard, subtask string
	for _, it := range catalogue(t, admin, "issue-types") {
		switch {
		case *it.Subtask && subtask == "":
			subtask = it.id()
		case !*it.Subtask && standard == "":
			standard = it.id()
		}
	}
	var it catalogueItem
	admin.call("POST", "/api/admin/issue-types", map[string]any{"name": unique("Incident "), "description": "Pour les tests"}, 201, &it)
	path := "/api/admin/issue-types/" + it.id()
	// Unused: the sub-task flag still changes.
	admin.call("PATCH", path, map[string]any{"subtask": true}, 200, nil)
	admin.call("PATCH", path, map[string]any{"subtask": false}, 200, nil)

	key := newProject(t, admin, map[string]any{"issueTypes": []string{it.id(), standard, subtask}})
	var c struct{ Key string }
	paul.call("POST", "/api/issues", map[string]any{"project": key, "type": it.id(), "summary": "D'un type à supprimer"}, 201, &c)
	admin.call("PATCH", path, map[string]any{"subtask": true}, 409, nil)
	status, body := admin.do("DELETE", path, nil)
	if status != 409 || num(obj(t, body), "usage", "issues") != 1 {
		t.Fatalf("deleting a type in use: %d %s", status, body)
	}
	admin.call("DELETE", path, map[string]any{"moveTo": subtask}, 400, nil)
	admin.call("DELETE", path, map[string]any{"moveTo": standard}, 204, nil)
	var d issueDetail
	paul.get("/api/issues/"+c.Key, &d)
	if d.Type.ID != standard {
		t.Errorf("type after the move: %+v, want %s", d.Type, standard)
	}
	var history []historyEntry
	paul.get("/api/issues/"+c.Key+"/history", &history)
	if !historyHas(history, "issuetype") {
		t.Errorf("no history of the move: %+v", history)
	}
}

func TestStatusOrder(t *testing.T) {
	admin := login(t, "admin", "admin")
	login(t, "paul.martin", "motdepasse").call("GET", "/api/admin/statuses", nil, 403, nil)
	var list []struct {
		ID    string
		Usage map[string]int
	}
	admin.call("GET", "/api/admin/statuses", nil, 200, &list)
	if len(list) < 2 || list[0].Usage == nil {
		t.Fatalf("statuses: %+v", list)
	}
	var order []string
	for _, s := range list {
		order = append(order, s.ID)
	}
	t.Cleanup(func() { admin.do("POST", "/api/admin/statuses/order", map[string]any{"ids": order}) })
	reversed := slices.Clone(order)
	slices.Reverse(reversed)
	admin.call("POST", "/api/admin/statuses/order", map[string]any{"ids": reversed[1:]}, 400, nil)
	admin.call("POST", "/api/admin/statuses/order", map[string]any{"ids": reversed}, 200, &list)
	if list[0].ID != order[len(order)-1] {
		t.Errorf("first status after reordering: %s, want %s", list[0].ID, order[len(order)-1])
	}
}
