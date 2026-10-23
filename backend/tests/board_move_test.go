package tests

// A card move that needs a permission the user lacks changes nothing, not
// even the part the user may do.

import (
	"fmt"
	"testing"
)

func TestBoardMoveChecksEveryPermissionFirst(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse") // developpeurs
	key := newProject(t, admin, nil)

	// The developers may change a status but not edit.
	var perms struct {
		Grants []grantOut `json:"grants"`
	}
	admin.call("GET", "/api/projects/"+key+"/permissions", nil, 200, &perms)
	kept := []grantOut{}
	for _, g := range perms.Grants {
		if g.Permission != "edit" || g.Holder != "group" {
			kept = append(kept, g)
		}
	}
	admin.call("PUT", "/api/projects/"+key+"/permissions", map[string]any{"grants": kept}, 200, nil)

	var created struct{ ID int64 }
	admin.call("POST", "/api/boards", map[string]any{"kind": "kanban", "project": key}, 201, &created)
	path := fmt.Sprintf("/api/boards/%d", created.ID)
	t.Cleanup(func() { admin.do("DELETE", path, nil) })

	anchor := createIssue(t, admin, key, "Ancre", "")
	moving := createIssue(t, admin, key, "Carte", "")
	var d struct {
		Status      statusRef
		Transitions []struct{ To statusRef }
	}
	paul.call("GET", "/api/issues/"+moving, nil, 200, &d)
	if len(d.Transitions) == 0 {
		t.Fatalf("%s has no transition", moving)
	}
	start, reached := d.Status.ID, d.Transitions[0].To.ID
	admin.call("PUT", path, map[string]any{"columns": []map[string]any{
		{"name": "Départ", "statuses": []string{start}},
		{"name": "Atteint", "statuses": []string{reached}},
	}}, 200, nil)
	b := readBoard(t, admin, path)
	reachedCol := b.Columns[1].ID

	rank := rankOf(t, moving)
	for name, body := range map[string]map[string]any{
		"column and rank":   {"issue": moving, "column": reachedCol, "after": anchor},
		"column and sprint": {"issue": moving, "column": reachedCol, "sprint": 0},
	} {
		if status, msg, _ := paul.errorIn("en", "POST", path+"/move", body); status != 403 {
			t.Errorf("%s without edit: %d %s, want 403", name, status, msg)
		}
		if got, n := statusOf(t, moving), statusChanges(t, moving); got != start || n != "0" {
			t.Errorf("%s refused: status %s, %s history lines, want %s and none", name, got, n, start)
		}
		if r := rankOf(t, moving); r != rank {
			t.Errorf("%s refused: ranked %q, want %q", name, r, rank)
		}
	}

	// The column alone needs only the transition.
	if status, msg, _ := paul.errorIn("en", "POST", path+"/move", map[string]any{"issue": moving, "column": reachedCol}); status != 204 {
		t.Fatalf("column alone: %d %s", status, msg)
	}
	if got := statusOf(t, moving); got != reached {
		t.Errorf("%s is in status %s, want %s", moving, got, reached)
	}
}
