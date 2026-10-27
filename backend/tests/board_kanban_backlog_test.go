package tests

// A Kanban board with a backlog: its first column's issues leave the board
// for the backlog page, and the setting needs a second column to show.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"
)

func TestKanbanBacklog(t *testing.T) {
	admin := login(t, "admin", "admin")
	key := newProject(t, admin, nil)
	var created struct{ ID int64 }
	admin.call("POST", "/api/boards", map[string]any{"kind": "kanban", "project": key}, 201, &created)
	path := fmt.Sprintf("/api/boards/%d", created.ID)
	t.Cleanup(func() { admin.do("DELETE", path, nil) })

	waiting := createIssue(t, admin, key, "En attente", "")
	moving := createIssue(t, admin, key, "En route", "")
	var d struct {
		Status      statusRef
		Transitions []struct{ To statusRef }
	}
	admin.call("GET", "/api/issues/"+moving, nil, 200, &d)
	if len(d.Transitions) == 0 {
		t.Fatalf("%s has no transition", moving)
	}
	start, reached := d.Status.ID, d.Transitions[0].To.ID

	type board struct {
		Board struct {
			KanbanBacklog bool `json:"kanbanBacklog"`
		} `json:"board"`
		Columns []struct{ ID int64 } `json:"columns"`
	}
	backlogOn := func() bool {
		var b board
		admin.call("GET", path, nil, 200, &b)
		return b.Board.KanbanBacklog
	}
	keys := func(p string) []string {
		var out struct{ Cards, Backlog []struct{ Key string } }
		admin.call("GET", p, nil, 200, &out)
		ks := []string{}
		for _, c := range append(out.Cards, out.Backlog...) {
			ks = append(ks, c.Key)
		}
		return ks
	}

	if status, _ := admin.do("GET", path+"/backlog", nil); status != 404 {
		t.Errorf("backlog of a Kanban board without one: %d, want 404", status)
	}

	// One column, or a first column without a status, leaves no board.
	one := []map[string]any{{"name": "Seule", "statuses": []string{start, reached}}}
	admin.call("PUT", path, map[string]any{"columns": one}, 200, nil)
	if status, msg, _ := admin.errorIn("en", "PUT", path, map[string]any{"kanbanBacklog": true}); status != 400 {
		t.Errorf("backlog with one column: %d %s, want 400", status, msg)
	}
	empty := []map[string]any{{"name": "Vide", "statuses": []string{}}, {"name": "Tout", "statuses": []string{start, reached}}}
	if status, msg, _ := admin.errorIn("en", "PUT", path, map[string]any{"columns": empty, "kanbanBacklog": true}); status != 400 {
		t.Errorf("backlog with an empty first column: %d %s, want 400", status, msg)
	}
	if backlogOn() {
		t.Fatal("a refused backlog is on")
	}

	two := []map[string]any{{"name": "Backlog", "statuses": []string{start}}, {"name": "Atteint", "statuses": []string{reached}}}
	admin.call("PUT", path, map[string]any{"columns": two, "kanbanBacklog": true}, 200, nil)
	if !backlogOn() {
		t.Fatal("backlog not on")
	}
	if got := keys(path + "/cards"); len(got) != 0 {
		t.Errorf("board cards with everything in the backlog: %v", got)
	}
	if got := keys(path + "/backlog"); !slices.Contains(got, waiting) || !slices.Contains(got, moving) {
		t.Errorf("backlog: %v, want %s and %s", got, waiting, moving)
	}

	var b board
	admin.call("GET", path, nil, 200, &b)
	admin.call("POST", path+"/move", map[string]any{"issue": moving, "column": b.Columns[1].ID}, 204, nil)
	if got := keys(path + "/cards"); !slices.Equal(got, []string{moving}) {
		t.Errorf("board cards after the move: %v, want %s", got, moving)
	}
	if got := keys(path + "/backlog"); !slices.Equal(got, []string{waiting}) {
		t.Errorf("backlog after the move: %v, want %s", got, waiting)
	}
	status, _, body := call(t, admin, testURL, "GET", fmt.Sprintf("/rest/agile/1.0/board/%d/backlog?fields=summary", created.ID), nil, nil)
	if status != http.StatusOK {
		t.Fatalf("REST backlog: %d %s", status, body)
	}
	var rest struct{ Issues []struct{ Key string } }
	if err := json.Unmarshal(body, &rest); err != nil {
		t.Fatal(err)
	}
	if len(rest.Issues) != 1 || rest.Issues[0].Key != waiting {
		t.Errorf("REST backlog: %+v, want %s", rest.Issues, waiting)
	}

	// Down to one column, the board has no backlog left.
	admin.call("PUT", path, map[string]any{"columns": one}, 200, nil)
	if backlogOn() {
		t.Error("backlog still on with one column")
	}

	if status, msg, _ := admin.errorIn("en", "PUT", "/api/boards/"+boardID(t, "DEMO — Scrum"),
		map[string]any{"kanbanBacklog": true}); status != 400 {
		t.Errorf("Kanban backlog on a Scrum board: %d %s, want 400", status, msg)
	}
}
