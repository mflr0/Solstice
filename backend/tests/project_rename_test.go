package tests

// Changing a project's key: the issues take the new key, the old keys keep
// finding them, and the key is checked like a new project's.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectRename(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse") // not an administrator

	old := newProject(t, admin, nil)
	issue := createIssue(t, admin, old, "Avant le changement", "")
	num := strings.TrimPrefix(issue, old+"-")
	path := func(key string) string { return "/api/admin/projects/" + key + "/rename" }

	// Refused: not an administrator, an unknown project, a wrong confirmation,
	// and keys that are ill-formed or taken now or formerly.
	paul.call("POST", path(old), map[string]any{"newKey": "ABCD", "confirm": old}, 403, nil)
	admin.call("POST", path("NOPE"), map[string]any{"newKey": "ABCD", "confirm": "NOPE"}, 404, nil)
	for _, confirm := range []string{"", "DEMO", strings.ToLower(old) + "x"} {
		admin.call("POST", path(old), map[string]any{"newKey": "ABCD", "confirm": confirm}, 409, nil)
	}
	for _, c := range []struct {
		key  string
		want int
	}{
		{"", 400}, {"1AB", 400}, {"A", 400}, {"ABCDEFGHIJK", 400}, {"AB-C", 400},
		{"DEMO", 409}, {"SITE", 409}, {old, 409},
	} {
		if status, body := admin.do("POST", path(old), map[string]any{"newKey": c.key, "confirm": old}); status != c.want {
			t.Errorf("new key %q: %d, want %d\n%s", c.key, status, c.want, body)
		}
	}
	if got := pick(t, `SELECT COUNT(*) FROM projects WHERE pkey = ?`, old); got != "1" {
		t.Fatalf("a refused rename changed the project (%s with the key %s)", got, old)
	}
	if got := pick(t, `SELECT COUNT(*) FROM project_keys WHERE pkey = ?`, old); got != "0" {
		t.Errorf("a refused rename recorded the key as former")
	}

	// The key as one types it: a free key is offered.
	fresh := newProjectKey()
	var check struct{ Available bool }
	admin.call("GET", "/api/admin/project-key?key="+fresh, nil, 200, &check)
	if !check.Available {
		t.Fatalf("%s is not offered", fresh)
	}

	// The rename; the typed key may have stray spaces and the new key any case.
	var renamed struct{ Key string }
	admin.call("POST", path(old), map[string]any{"newKey": " " + strings.ToLower(fresh) + " ", "confirm": " " + old + " "},
		200, &renamed)
	if renamed.Key != fresh {
		t.Fatalf("renamed to %q, want %s", renamed.Key, fresh)
	}
	t.Cleanup(func() { admin.do("DELETE", "/api/projects/"+fresh, map[string]string{"confirm": fresh}) })
	admin.call("GET", "/api/admin/project-key?key="+old, nil, 200, &check)
	if check.Available {
		t.Error("the former key is offered again")
	}

	// The issue has the new key, and the old one still finds it: in the
	// interface (/browse/OLD-n reads /api/issues/OLD-n) and in the REST API.
	newIssue := fresh + "-" + num
	for _, key := range []string{newIssue, issue, strings.ToLower(issue)} {
		var d struct{ Key string }
		if status := admin.get("/api/issues/"+key, &d); status != 200 || d.Key != newIssue {
			t.Errorf("GET /api/issues/%s: %d %q, want %s", key, status, d.Key, newIssue)
		}
		status, _, body := call(t, nil, testURL, "GET", "/rest/api/2/issue/"+key, nil, basic("admin", "admin"))
		var r struct{ Key string }
		if err := json.Unmarshal(body, &r); status != 200 || err != nil || r.Key != newIssue {
			t.Errorf("GET /rest/api/2/issue/%s: %d %q, want %s", key, status, r.Key, newIssue)
		}
	}
	var p struct{ Project struct{ Key string } }
	if status := admin.get("/api/projects/"+old, &p); status != 200 || p.Project.Key != fresh {
		t.Errorf("GET /api/projects/%s: %d %q, want %s", old, status, p.Project.Key, fresh)
	}
	if status, _ := admin.do("GET", "/api/issues/"+old+"-9999", nil); status != 404 {
		t.Errorf("an old key and a number no issue has: %d, want 404", status)
	}

	// The searches that name the old key.
	for _, q := range []string{
		"project = " + old, "project = " + fresh, "project in (" + old + ", DEMO) AND key = " + newIssue,
		"key = " + issue, "key in (" + issue + ")", "key = " + newIssue, "key >= " + old + "-1 AND project = " + fresh,
		"key >= " + issue + " AND key <= " + issue,
	} {
		if status, total, mode := runSearch(t, admin, q); status != 200 || mode != "jql" || total != 1 {
			t.Errorf("%s: %d, %d issues (%s), want 1", q, status, total, mode)
		}
	}
	if _, total, _ := runSearch(t, admin, "project != "+old+" AND key = "+issue); total != 0 {
		t.Errorf("project != OLD finds %d issues of the project", total)
	}

	// The audit entry names both keys.
	if n, details := latestAudit(t, "project.rename"); n == 0 || !strings.Contains(details, old) || !strings.Contains(details, fresh) {
		t.Errorf("audit entry: %d %s", n, details)
	}

	// A project renamed again keeps every former key, and the confirmation is
	// the current key.
	again := newProjectKey()
	admin.call("POST", path(fresh), map[string]any{"newKey": again, "confirm": old}, 409, nil)
	admin.call("POST", path(old), map[string]any{"newKey": again, "confirm": fresh}, 200, nil)
	t.Cleanup(func() { admin.do("DELETE", "/api/projects/"+again, map[string]string{"confirm": again}) })
	for _, key := range []string{issue, newIssue, again + "-" + num} {
		var d struct{ Key string }
		if status := admin.get("/api/issues/"+key, &d); status != 200 || d.Key != again+"-"+num {
			t.Errorf("after a second rename, GET /api/issues/%s: %d %q", key, status, d.Key)
		}
	}
	if got := pick(t, `SELECT COUNT(*) FROM project_keys k JOIN projects p ON p.id = k.project_id WHERE p.pkey = ?`, again); got != "2" {
		t.Errorf("%s has %s former keys, want 2", again, got)
	}
}
