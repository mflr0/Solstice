package tests

// Versions, components and saved filters through the Jira-compatible REST API.

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

// restAs returns a function that calls the REST API with a user's token.
func restAs(t *testing.T, username string) func(method, path string, body any) (int, map[string]any) {
	token := newToken(t, username, nil)
	return func(method, path string, body any) (int, map[string]any) {
		t.Helper()
		status, _, raw := call(t, nil, testURL, method, path, body, bearer(token))
		var m map[string]any
		json.Unmarshal(raw, &m)
		if status >= 400 {
			t.Logf("%s %s: %d %s", method, path, status, raw)
		}
		return status, m
	}
}

func TestRESTVersions(t *testing.T) {
	admin, paul := restAs(t, "admin"), restAs(t, "paul.martin")

	status, v := admin("POST", "/rest/api/2/version", map[string]any{"project": "DEMO", "name": "9.1-rest",
		"description": "Via l'API", "startDate": "2026-11-01", "releaseDate": "2026-12-01", "released": false})
	if status != 201 || str(v, "name") != "9.1-rest" || str(v, "description") != "Via l'API" ||
		str(v, "startDate") != "2026-11-01" || str(v, "releaseDate") != "2026-12-01" || str(v, "id") == "" ||
		str(v, "self") != testURL+"/rest/api/2/version/"+str(v, "id") {
		t.Fatalf("create: %d %v", status, v)
	}
	id := str(v, "id")
	path := "/rest/api/2/version/" + id
	if got := pick(t, `SELECT p.pkey FROM versions v JOIN projects p ON p.id = v.project_id WHERE v.id = ?`, id); got != "DEMO" {
		t.Errorf("the version is in project %q", got)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'version.create' AND target_type = 'version' AND target_id = ?`, id); n != "1" {
		t.Errorf("audit entries of the creation: %s", n)
	}

	// Anyone who sees the project reads it; project administrators change it.
	if status, got := paul("GET", path, nil); status != 200 || str(got, "name") != "9.1-rest" {
		t.Errorf("paul.martin GET: %d %v", status, got)
	}
	if status, _ := paul("PUT", path, map[string]any{"name": "9.1-paul"}); status != 403 {
		t.Errorf("paul.martin PUT: %d", status)
	}
	if status, _ := paul("DELETE", path, nil); status != 403 {
		t.Errorf("paul.martin DELETE: %d", status)
	}
	if status, _ := paul("POST", "/rest/api/2/version", map[string]any{"project": "DEMO", "name": "9.1-paul"}); status != 403 {
		t.Errorf("paul.martin POST: %d", status)
	}

	// Partial edit, with what a client sends back after reading the version.
	status, v = admin("PUT", path, map[string]any{"self": "x", "id": id, "projectId": 1, "released": true,
		"releaseDate": "2026-11-30", "userReleaseDate": "30/Nov/26"})
	if status != 200 || v["released"] != true || str(v, "releaseDate") != "2026-11-30" || str(v, "name") != "9.1-rest" ||
		str(v, "startDate") != "2026-11-01" {
		t.Fatalf("edit: %d %v", status, v)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'version.update' AND target_type = 'version' AND target_id = ?`, id); n != "1" {
		t.Errorf("audit entries of the edit: %s", n)
	}

	// Refusals, in Jira's format.
	if status, e := admin("PUT", path, map[string]any{"couleur": "bleu"}); status != 400 || at(e, "errors", "couleur") == nil {
		t.Errorf("unknown field: %d %v", status, e)
	}
	if status, e := admin("POST", "/rest/api/2/version", map[string]any{"name": "sans projet"}); status != 400 ||
		at(e, "errors", "project") == nil {
		t.Errorf("no project: %d %v", status, e)
	}
	if status, e := admin("POST", "/rest/api/2/version", map[string]any{"project": "NOPE", "name": "x"}); status != 404 ||
		at(e, "errors", "project") == nil {
		t.Errorf("unknown project: %d %v", status, e)
	}
	if status, _ := admin("POST", "/rest/api/2/version", map[string]any{"project": "DEMO", "name": "9.1-REST"}); status != 409 {
		t.Errorf("duplicate name: %d", status)
	}
	if status, _ := admin("PUT", path, map[string]any{"startDate": "2027-01-01"}); status != 400 {
		t.Errorf("start after release: %d", status)
	}
	if status, _ := admin("PUT", path, map[string]any{"releaseDate": "demain"}); status != 400 {
		t.Errorf("bad date: %d", status)
	}
	if status, _ := admin("GET", "/rest/api/2/version/0", nil); status != 404 {
		t.Errorf("unknown version: %d", status)
	}

	// A version of a project one does not see does not exist.
	status, rh := admin("POST", "/rest/api/2/version", map[string]any{"projectId": pick(t, `SELECT id FROM projects WHERE pkey = 'RH'`),
		"name": "9.1-rh"})
	if status != 201 {
		t.Fatalf("create in RH: %d %v", status, rh)
	}
	for method, body := range map[string]any{"GET": nil, "PUT": map[string]any{"name": "x"}, "DELETE": nil} {
		if status, _ := paul(method, "/rest/api/2/version/"+str(rh, "id"), body); status != 404 {
			t.Errorf("paul.martin %s a version of RH: %d", method, status)
		}
	}
	if status, _ := paul("POST", "/rest/api/2/version", map[string]any{"project": "RH", "name": "x"}); status != 404 {
		t.Errorf("paul.martin created a version in RH: %d", status)
	}
	admin("DELETE", "/rest/api/2/version/"+str(rh, "id"), nil)

	// Deleting moves the fix and affected versions of issues where asked.
	status, to := admin("POST", "/rest/api/2/version", map[string]any{"project": "DEMO", "name": "9.2-rest"})
	if status != 201 {
		t.Fatalf("create target: %d %v", status, to)
	}
	key := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	iid := issueID(t, key)
	for _, kind := range []string{"fix", "affects"} {
		if _, err := testDB.Exec(`INSERT INTO issue_versions (issue_id, version_id, kind) VALUES (?, ?, ?)`, iid, id, kind); err != nil {
			t.Fatal(err)
		}
	}
	if status, _ := admin("DELETE", path+"?moveFixIssuesTo=abc", nil); status != 400 {
		t.Errorf("bad moveFixIssuesTo: %d", status)
	}
	if status, _ := admin("DELETE", path+"?moveFixIssuesTo="+id, nil); status != 400 {
		t.Errorf("moving to itself: %d", status)
	}
	if status, _ := admin("DELETE", path+"?moveFixIssuesTo="+str(to, "id"), nil); status != 204 {
		t.Fatalf("delete: %d", status)
	}
	if n := pick(t, `SELECT COUNT(*) FROM issue_versions WHERE issue_id = ? AND version_id = ? AND kind = 'fix'`, iid, str(to, "id")); n != "1" {
		t.Errorf("fix version not moved: %s", n)
	}
	if n := pick(t, `SELECT COUNT(*) FROM issue_versions WHERE issue_id = ? AND kind = 'affects' AND version_id IN (?, ?)`, iid, id, str(to, "id")); n != "0" {
		t.Errorf("affected version moved or kept: %s", n)
	}
	if n := pick(t, `SELECT COUNT(*) FROM change_groups g JOIN change_items it ON it.group_id = g.id
		WHERE g.issue_id = ? AND it.field = 'Fix Version'`, iid); n == "0" {
		t.Error("no history of the moved fix version")
	}
	if status, _ := admin("GET", path, nil); status != 404 {
		t.Errorf("deleted version: %d", status)
	}
	admin("DELETE", "/rest/api/2/version/"+str(to, "id"), nil)
}

func TestRESTComponents(t *testing.T) {
	admin, paul := restAs(t, "admin"), restAs(t, "paul.martin")

	status, c := admin("POST", "/rest/api/2/component", map[string]any{"project": "DEMO", "name": "Export-rest",
		"description": "Via l'API", "leadUserName": "marie.durand", "assigneeType": "COMPONENT_LEAD"})
	if status != 201 || str(c, "name") != "Export-rest" || str(c, "description") != "Via l'API" ||
		str(c, "assigneeType") != "COMPONENT_LEAD" || str(c, "lead", "name") != "marie.durand" || str(c, "project") != "DEMO" ||
		str(c, "self") != testURL+"/rest/api/2/component/"+str(c, "id") {
		t.Fatalf("create: %d %v", status, c)
	}
	id := str(c, "id")
	path := "/rest/api/2/component/" + id
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'component.create' AND target_type = 'component' AND target_id = ?`, id); n != "1" {
		t.Errorf("audit entries of the creation: %s", n)
	}

	if status, got := paul("GET", path, nil); status != 200 || str(got, "name") != "Export-rest" {
		t.Errorf("paul.martin GET: %d %v", status, got)
	}
	if status, _ := paul("PUT", path, map[string]any{"name": "x"}); status != 403 {
		t.Errorf("paul.martin PUT: %d", status)
	}
	if status, _ := paul("DELETE", path, nil); status != 403 {
		t.Errorf("paul.martin DELETE: %d", status)
	}
	if status, _ := paul("POST", "/rest/api/2/component", map[string]any{"project": "DEMO", "name": "x"}); status != 403 {
		t.Errorf("paul.martin POST: %d", status)
	}

	status, c = admin("PUT", path, map[string]any{"leadUserName": nil, "assigneeType": "PROJECT_DEFAULT",
		"description": "Changé", "self": "x", "id": id, "isAssigneeTypeValid": true})
	if status != 200 || c["lead"] != nil || str(c, "assigneeType") != "PROJECT_DEFAULT" || str(c, "description") != "Changé" ||
		str(c, "name") != "Export-rest" {
		t.Fatalf("edit: %d %v", status, c)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'component.update' AND target_type = 'component' AND target_id = ?`, id); n != "1" {
		t.Errorf("audit entries of the edit: %s", n)
	}

	if status, e := admin("PUT", path, map[string]any{"couleur": "bleu"}); status != 400 || at(e, "errors", "couleur") == nil {
		t.Errorf("unknown field: %d %v", status, e)
	}
	if status, e := admin("PUT", path, map[string]any{"leadUserName": "personne.inconnue"}); status != 400 ||
		at(e, "errors", "leadUserName") == nil {
		t.Errorf("unknown lead: %d %v", status, e)
	}
	if status, _ := admin("PUT", path, map[string]any{"assigneeType": "NOBODY"}); status != 400 {
		t.Errorf("unknown assignee type: %d", status)
	}
	if status, _ := admin("POST", "/rest/api/2/component", map[string]any{"project": "DEMO", "name": "EXPORT-REST"}); status != 409 {
		t.Errorf("duplicate name: %d", status)
	}

	// A component of a project one does not see does not exist.
	status, rh := admin("POST", "/rest/api/2/component", map[string]any{"project": "RH", "name": "Paie-rest"})
	if status != 201 {
		t.Fatalf("create in RH: %d %v", status, rh)
	}
	for method, body := range map[string]any{"GET": nil, "PUT": map[string]any{"name": "x"}, "DELETE": nil} {
		if status, _ := paul(method, "/rest/api/2/component/"+str(rh, "id"), body); status != 404 {
			t.Errorf("paul.martin %s a component of RH: %d", method, status)
		}
	}
	admin("DELETE", "/rest/api/2/component/"+str(rh, "id"), nil)

	// Deleting moves the issues to moveIssuesTo.
	status, to := admin("POST", "/rest/api/2/component", map[string]any{"project": "DEMO", "name": "Import-rest"})
	if status != 201 {
		t.Fatalf("create target: %d %v", status, to)
	}
	key := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	iid := issueID(t, key)
	if _, err := testDB.Exec(`INSERT INTO issue_components (issue_id, component_id) VALUES (?, ?)`, iid, id); err != nil {
		t.Fatal(err)
	}
	if status, _ := admin("DELETE", path+"?moveIssuesTo=abc", nil); status != 400 {
		t.Errorf("bad moveIssuesTo: %d", status)
	}
	if status, _ := admin("DELETE", path+"?moveIssuesTo="+str(to, "id"), nil); status != 204 {
		t.Fatalf("delete: %d", status)
	}
	if n := pick(t, `SELECT COUNT(*) FROM issue_components WHERE issue_id = ? AND component_id = ?`, iid, str(to, "id")); n != "1" {
		t.Errorf("issue not moved: %s", n)
	}
	if status, _ := admin("GET", path, nil); status != http.StatusNotFound {
		t.Errorf("deleted component: %d", status)
	}
	admin("DELETE", "/rest/api/2/component/"+str(to, "id"), nil)
}

func TestRESTFilters(t *testing.T) {
	paul, hugo, admin := restAs(t, "paul.martin"), restAs(t, "hugo.robert"), restAs(t, "admin")

	status, f := paul("POST", "/rest/api/2/filter", map[string]any{"name": "Mes bugs (REST)", "description": "Via l'API",
		"jql": "project = DEMO ORDER BY key", "favourite": true})
	if status != 200 || str(f, "name") != "Mes bugs (REST)" || str(f, "jql") != "project = DEMO ORDER BY key" ||
		str(f, "description") != "Via l'API" || f["favourite"] != true || str(f, "owner", "name") != "paul.martin" ||
		str(f, "self") != testURL+"/rest/api/2/filter/"+str(f, "id") {
		t.Fatalf("create: %d %v", status, f)
	}
	id := str(f, "id")
	path := "/rest/api/2/filter/" + id
	// What was created shows in the interface.
	var ui []struct {
		ID        int64
		Name      string
		Query     string
		Favourite bool
	}
	login(t, "paul.martin", "motdepasse").get("/api/filters", &ui)
	found := false
	for _, v := range ui {
		found = found || (strconv.FormatInt(v.ID, 10) == id && v.Query == "project = DEMO ORDER BY key" && v.Favourite)
	}
	if !found {
		t.Errorf("the interface does not list the filter: %+v", ui)
	}

	// A private filter does not exist for others.
	for method, body := range map[string]any{"GET": nil, "PUT": map[string]any{"name": "x"}, "DELETE": nil} {
		if status, _ := hugo(method, path, body); status != 404 {
			t.Errorf("hugo.robert %s a private filter: %d", method, status)
		}
	}

	// Shared with everyone, it is read by others but changed by its owner (or an administrator) only.
	status, f = paul("PUT", path, map[string]any{"name": "Bugs partagés", "jql": "project = DEMO AND statusCategory != Done",
		"description": nil, "favourite": false, "self": "x", "id": id,
		"sharePermissions": []any{map[string]any{"type": "global"}}})
	if status != 200 || str(f, "name") != "Bugs partagés" || str(f, "jql") != "project = DEMO AND statusCategory != Done" ||
		f["description"] != nil && str(f, "description") != "" || f["favourite"] != false ||
		str(f, "sharePermissions", 0, "type") != "global" {
		t.Fatalf("edit: %d %v", status, f)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'filter.share' AND target_type = 'filter' AND target_id = ?`, id); n != "1" {
		t.Errorf("audit entries of the sharing: %s", n)
	}
	if status, got := hugo("GET", path, nil); status != 200 || str(got, "name") != "Bugs partagés" {
		t.Errorf("hugo.robert GET a shared filter: %d %v", status, got)
	}
	if status, _ := hugo("PUT", path, map[string]any{"name": "x"}); status != 403 {
		t.Errorf("hugo.robert PUT a shared filter: %d", status)
	}
	if status, _ := hugo("DELETE", path, nil); status != 403 {
		t.Errorf("hugo.robert DELETE a shared filter: %d", status)
	}

	// Sharing as the interface supports it, nothing more.
	if status, e := paul("PUT", path, map[string]any{"sharePermissions": []any{map[string]any{"type": "authenticated"}}}); status != 400 ||
		at(e, "errors", "sharePermissions") == nil {
		t.Errorf("unsupported share type: %d %v", status, e)
	}
	if status, e := paul("PUT", path, map[string]any{"sharePermissions": []any{map[string]any{"type": "group",
		"group": map[string]any{"name": "groupe.inconnu"}}}}); status != 400 || at(e, "errors", "sharePermissions") == nil {
		t.Errorf("unknown group: %d %v", status, e)
	}
	if status, _ := paul("PUT", path, map[string]any{"sharePermissions": []any{map[string]any{"type": "project",
		"project": map[string]any{"key": "RH"}}}}); status != 400 {
		t.Errorf("sharing with a project one does not see: %d", status)
	}
	if status, _ := paul("PUT", path, map[string]any{"sharePermissions": []any{map[string]any{"type": "user",
		"user": map[string]any{"name": "hugo.robert"}}, map[string]any{"type": "project", "project": map[string]any{"key": "DEMO"}}}}); status != 200 {
		t.Errorf("sharing with a person and a project: %d", status)
	}

	// Refusals, in Jira's format.
	if status, e := paul("PUT", path, map[string]any{"couleur": "bleu"}); status != 400 || at(e, "errors", "couleur") == nil {
		t.Errorf("unknown field: %d %v", status, e)
	}
	if status, _ := paul("POST", "/rest/api/2/filter", map[string]any{"jql": "project = DEMO"}); status != 400 {
		t.Errorf("filter without a name: %d", status)
	}
	if status, _ := paul("POST", "/rest/api/2/filter", map[string]any{"name": "Faux", "jql": "project = "}); status != 400 {
		t.Errorf("invalid JQL: %d", status)
	}
	if status, _ := paul("PUT", path, map[string]any{"favourite": "oui"}); status != 400 {
		t.Errorf("invalid favourite: %d", status)
	}
	if status, _ := paul("PUT", "/rest/api/2/filter/0", map[string]any{"name": "x"}); status != 404 {
		t.Errorf("unknown filter: %d", status)
	}

	// An administrator tidies up other people's filters.
	if status, _ := admin("DELETE", path, nil); status != 204 {
		t.Fatalf("admin DELETE: %d", status)
	}
	if status, _ := paul("GET", path, nil); status != 404 {
		t.Errorf("deleted filter: %d", status)
	}
	// The interface requires a query, so the REST route does too.
	if status, _ := paul("POST", "/rest/api/2/filter", map[string]any{"name": "Sans JQL (REST)"}); status != 400 {
		t.Errorf("create without jql: %d", status)
	}
	status, g := paul("POST", "/rest/api/2/filter", map[string]any{"name": "Pour supprimer (REST)", "jql": "project = DEMO"})
	if status != 200 {
		t.Fatalf("create: %d %v", status, g)
	}
	if status, _ := paul("DELETE", "/rest/api/2/filter/"+str(g, "id"), nil); status != 204 {
		t.Errorf("owner DELETE: %d", status)
	}
}
