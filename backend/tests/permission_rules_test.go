package tests

import (
	"context"
	"reflect"
	"slices"
	"testing"
)

// TestPermissionsRoundTrip checks that putting back a project's grants
// changes none of them, and that the denies of the project, which the
// endpoint does not show, survive the put and apply.
func TestPermissionsRoundTrip(t *testing.T) {
	admin := login(t, "admin", "admin")
	key := newProject(t, admin, nil)
	projectID := pick(t, `SELECT id FROM projects WHERE pkey = ?`, key)

	var before, after struct {
		Grants []grantOut `json:"grants"`
	}
	admin.get("/api/projects/"+key+"/permissions", &before)
	if len(before.Grants) == 0 {
		t.Fatal("a new project has no grants")
	}

	// Paul may no longer be assigned issues here, through a project user deny
	// over the developpeurs' allow.
	if _, err := testDB.ExecContext(context.Background(), `INSERT INTO permission_rules
		(project_id, permission, effect, holder, user_id) VALUES (?, 'assignable', 'deny', 'user', ?)`,
		projectID, userID(t, "paul.martin")); err != nil {
		t.Fatal(err)
	}
	admin.call("PUT", "/api/projects/"+key+"/permissions", map[string]any{"grants": before.Grants}, 200, nil)
	admin.get("/api/projects/"+key+"/permissions", &after)
	if !reflect.DeepEqual(before.Grants, after.Grants) {
		t.Errorf("grants changed by putting them back:\nbefore %+v\nafter  %+v", before.Grants, after.Grants)
	}
	if n := pick(t, `SELECT COUNT(*) FROM permission_rules WHERE project_id = ? AND effect = 'deny'`, projectID); n != "1" {
		t.Errorf("%s denies after the put, want 1", n)
	}

	var assignable []struct{ Username string }
	admin.get("/api/projects/"+key+"/assignable", &assignable)
	names := []string{}
	for _, u := range assignable {
		names = append(names, u.Username)
	}
	if slices.Contains(names, "paul.martin") || !slices.Contains(names, "jeanne.dupont") {
		t.Errorf("assignable = %v, want jeanne.dupont and not paul.martin", names)
	}
}
