package tests

// Projects made in Solstice: creation, settings, categories, archiving,
// deletion, and their components and versions.

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// newProjectKey is a project key no run of the tests used before.
func newProjectKey() string { return fmt.Sprintf("T%X", time.Now().UnixNano()%0xFFFFFFF) }

// newProject creates a project led by admin, open to the developpeurs group,
// and deletes it when the test ends.
func newProject(t *testing.T, admin *client, extra map[string]any) string {
	t.Helper()
	key := newProjectKey()
	in := map[string]any{"key": key, "name": "Projet " + key,
		"access": map[string]any{"holder": "group", "id": groupID(t, "developpeurs")}}
	for k, v := range extra {
		in[k] = v
	}
	admin.call("POST", "/api/projects", in, 201, nil)
	t.Cleanup(func() { admin.do("DELETE", "/api/projects/"+key, map[string]string{"confirm": key}) })
	return key
}

func groupID(t *testing.T, name string) int64 {
	var id int64
	needDB(t)
	if err := testDB.QueryRow(`SELECT id FROM user_groups WHERE name = ?`, name).Scan(&id); err != nil {
		t.Fatalf("group %s: %v", name, err)
	}
	return id
}

func TestProjectLifecycle(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")  // developpeurs
	chloe := login(t, "chloe.simon", "motdepasse") // not a developer

	// The key as one types it.
	var check struct {
		Key       string
		Available bool
		Reason    string
	}
	admin.call("GET", "/api/admin/project-key?key=demo", nil, 200, &check)
	if check.Key != "DEMO" || check.Available || check.Reason == "" {
		t.Errorf("taken key: %+v", check)
	}
	admin.call("GET", "/api/admin/project-key?key=SITE", nil, 200, &check)
	if check.Available {
		t.Error("a former key (SITE, now WEB) is offered again")
	}
	admin.call("GET", "/api/admin/project-key?key=1AB", nil, 200, &check)
	if check.Available {
		t.Error("an ill-formed key is offered")
	}
	paul.call("GET", "/api/admin/project-key?key=ABC", nil, 403, nil)

	// Bad creations name the field at fault.
	for _, c := range []struct {
		in   map[string]any
		want int
	}{
		{map[string]any{"key": "1AB", "name": "x"}, 400},
		{map[string]any{"key": "DEMO", "name": "x"}, 409},
		{map[string]any{"key": newProjectKey(), "name": ""}, 400},
		{map[string]any{"key": newProjectKey(), "name": pick(t, `SELECT name FROM projects WHERE pkey = 'DEMO'`)}, 409},
		{map[string]any{"key": newProjectKey(), "name": "x", "template": "kanban"}, 400},
		{map[string]any{"key": newProjectKey(), "name": "x", "access": map[string]any{"holder": "planet"}}, 400},
		{map[string]any{"key": newProjectKey(), "name": "x", "lead": userID(t, "emma.morel")}, 400}, // deactivated
		{map[string]any{"key": newProjectKey(), "name": "x", "defaultAssignee": "someone"}, 400},
	} {
		if status, body := admin.do("POST", "/api/projects", c.in); status != c.want {
			t.Errorf("create %v: %d %s, want %d", c.in, status, body, c.want)
		}
	}
	if status, _ := paul.do("POST", "/api/projects", map[string]any{"key": newProjectKey(), "name": "x"}); status != 403 {
		t.Errorf("a developer created a project: %d", status)
	}

	// A category, then a simple project in it, open to the developers.
	var cat struct {
		ID       int64
		Name     string
		Projects int
	}
	catName := unique("Catégorie ")
	admin.call("POST", "/api/admin/project-categories", map[string]any{"name": catName, "description": "Tests"}, 201, &cat)
	t.Cleanup(func() { admin.do("DELETE", fmt.Sprintf("/api/admin/project-categories/%d", cat.ID), nil) })
	admin.call("POST", "/api/admin/project-categories", map[string]any{"name": strings.ToUpper(catName)}, 409, nil)
	paul.call("POST", "/api/admin/project-categories", map[string]any{"name": "x"}, 403, nil)

	key := newProject(t, admin, map[string]any{"template": "simple", "category": cat.ID, "description": "Pour les tests"})
	var project struct {
		Project struct {
			Key        string
			Archived   bool
			CategoryID *int64 `json:"categoryId"`
		}
		IssueTypes []typeRef `json:"issueTypes"`
		Can        map[string]bool
	}
	admin.call("GET", "/api/projects/"+key, nil, 200, &project)
	if project.Project.CategoryID == nil || *project.Project.CategoryID != cat.ID || !project.Can["configure"] ||
		len(project.IssueTypes) == 0 {
		t.Fatalf("new project: %+v", project)
	}
	var wf struct {
		Workflows []struct {
			Statuses    []struct{ ID string }
			Transitions []struct{ ID int64 }
		}
	}
	admin.call("GET", "/api/projects/"+key+"/workflows", nil, 200, &wf)
	if len(wf.Workflows) != 1 || len(wf.Workflows[0].Statuses) != 3 || len(wf.Workflows[0].Transitions) != 3 {
		t.Errorf("simple template's workflow: %+v", wf)
	}
	var cats []struct {
		ID       int64
		Projects int
	}
	paul.call("GET", "/api/project-categories", nil, 200, &cats)
	if i := slices.IndexFunc(cats, func(c struct {
		ID       int64
		Projects int
	}) bool {
		return c.ID == cat.ID
	}); i < 0 || cats[i].Projects != 1 {
		t.Errorf("categories: %+v", cats)
	}

	// Access: the group works in it, the others do not see it.
	issue := createIssue(t, paul, key, "Premier ticket", "")
	if status := chloe.get("/api/projects/"+key, nil); status != 404 {
		t.Errorf("chloe.simon GET %s: %d", key, status)
	}
	if status := chloe.get("/api/issues/"+issue, nil); status != 404 {
		t.Errorf("chloe.simon GET %s: %d", issue, status)
	}
	paul.call("PATCH", "/api/projects/"+key, map[string]any{"name": "x"}, 403, nil)

	// Settings.
	var settings struct {
		Name            string
		Description     *string
		Lead            *userRef
		Category        *idName
		DefaultAssignee string
		Issues          int
	}
	admin.call("PATCH", "/api/projects/"+key, map[string]any{"name": "Projet renommé " + key, "description": nil,
		"lead": userID(t, "marie.durand"), "category": nil, "defaultAssignee": "lead"}, 200, &settings)
	if settings.Name != "Projet renommé "+key || settings.Description != nil || settings.Lead == nil ||
		settings.Lead.Username != "marie.durand" || settings.Category != nil || settings.DefaultAssignee != "lead" ||
		settings.Issues != 1 {
		t.Errorf("after PATCH: %+v", settings)
	}
	admin.call("GET", "/api/projects/"+key+"/settings", nil, 200, &settings)
	if settings.Lead.Username != "marie.durand" {
		t.Errorf("settings: %+v", settings)
	}
	// The project lead is the default assignee now.
	var d issueDetail
	paul.get("/api/issues/"+createIssue(t, paul, key, "Assigné au responsable", ""), &d)
	if d.Assignee == nil || d.Assignee.Username != "marie.durand" {
		t.Errorf("default assignee: %+v", d.Assignee)
	}
	// A type still in use cannot leave the project.
	status, body := admin.do("PATCH", "/api/projects/"+key, map[string]any{"issueTypes": []string{otherStandardType(t, project.IssueTypes, d.Type.ID)}})
	if status != 409 || !strings.Contains(string(body), `"inUse"`) {
		t.Errorf("removing a used issue type: %d %s", status, body)
	}

	// The global administrators' list shows every project.
	var all []projectOut
	admin.call("GET", "/api/admin/projects", nil, 200, &all)
	if !slices.ContainsFunc(all, func(p projectOut) bool { return p.Key == key }) ||
		!slices.ContainsFunc(all, func(p projectOut) bool { return p.Key == "RH" }) {
		t.Errorf("admin project list lacks %s or RH", key)
	}
	paul.call("GET", "/api/admin/projects", nil, 403, nil)

	// Archived: hidden from lists, readable, read-only.
	admin.call("POST", "/api/projects/"+key+"/archive", nil, 200, nil)
	var list []projectOut
	paul.call("GET", "/api/projects", nil, 200, &list)
	if slices.ContainsFunc(list, func(p projectOut) bool { return p.Key == key }) {
		t.Error("an archived project is listed")
	}
	paul.call("GET", "/api/projects?archived=1", nil, 200, &list)
	if i := slices.IndexFunc(list, func(p projectOut) bool { return p.Key == key }); i < 0 || !list[i].Archived {
		t.Error("?archived=1 does not list the archived project")
	}
	if status := paul.get("/api/issues/"+issue, nil); status != 200 {
		t.Errorf("issue of an archived project: %d", status)
	}
	if status, _ := paul.do("PATCH", "/api/issues/"+issue, map[string]any{"summary": "modifié"}); status != 403 {
		t.Errorf("edit in an archived project: %d", status)
	}
	if status, _ := paul.do("POST", "/api/issues/"+issue+"/comments", map[string]any{"body": "x"}); status != 403 {
		t.Errorf("comment in an archived project: %d", status)
	}
	paul.call("POST", "/api/projects/"+key+"/unarchive", nil, 403, nil)
	admin.call("POST", "/api/projects/"+key+"/unarchive", nil, 200, nil)
	paul.call("PATCH", "/api/issues/"+issue, map[string]any{"summary": "modifié"}, 200, nil)

	// Deletion: issues need the key typed.
	status, body = admin.do("DELETE", "/api/projects/"+key, nil)
	if status != 409 || num(obj(t, body), "issues") != 2 {
		t.Errorf("delete without confirmation: %d %s", status, body)
	}
	paul.call("DELETE", "/api/projects/"+key, map[string]string{"confirm": key}, 403, nil)
	admin.call("DELETE", "/api/projects/"+key, map[string]string{"confirm": key}, 204, nil)
	if status := admin.get("/api/projects/"+key, nil); status != 404 {
		t.Errorf("deleted project: %d", status)
	}
	if n := pick(t, `SELECT COUNT(*) FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.pkey = ?`, key); n != "0" {
		t.Errorf("%s issues left behind", n)
	}

	// Categories go; their projects lose them.
	admin.call("PATCH", fmt.Sprintf("/api/admin/project-categories/%d", cat.ID), map[string]any{"name": catName + " bis"}, 200, &cat)
	if cat.Name != catName+" bis" {
		t.Errorf("renamed category: %+v", cat)
	}
	admin.call("DELETE", fmt.Sprintf("/api/admin/project-categories/%d", cat.ID), nil, 204, nil)
	admin.call("DELETE", fmt.Sprintf("/api/admin/project-categories/%d", cat.ID), nil, 404, nil)
}

// otherStandardType is a standard issue type of the list other than id.
func otherStandardType(t *testing.T, types []typeRef, id string) string {
	t.Helper()
	for _, it := range types {
		if !it.Subtask && it.ID != id {
			return it.ID
		}
	}
	t.Fatalf("no standard issue type other than %s", id)
	return ""
}

type componentAdmin struct {
	ID           int64
	Name         string
	Lead         *userRef
	AssigneeType string
	Archived     bool
	Issues       int
}

func TestComponents(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	key := newProject(t, admin, nil)
	base := "/api/projects/" + key + "/components"

	var api, web componentAdmin
	admin.call("POST", base, map[string]any{"name": "API", "lead": userID(t, "hugo.robert"),
		"assigneeType": "component_lead"}, 201, &api)
	admin.call("POST", base, map[string]any{"name": "Web", "description": "Le site"}, 201, &web)
	if api.Lead == nil || api.Lead.Username != "hugo.robert" || api.AssigneeType != "component_lead" ||
		web.AssigneeType != "project_default" {
		t.Fatalf("components: %+v %+v", api, web)
	}
	for _, bad := range []struct {
		in   map[string]any
		want int
	}{
		{map[string]any{"name": "api"}, 409},
		{map[string]any{"name": "Autre", "assigneeType": "random"}, 400},
		{map[string]any{"name": "Autre", "lead": userID(t, "emma.morel")}, 400},
		{map[string]any{"description": "sans nom"}, 400},
	} {
		if status, body := admin.do("POST", base, bad.in); status != bad.want {
			t.Errorf("create %v: %d %s, want %d", bad.in, status, body, bad.want)
		}
	}
	paul.call("POST", base, map[string]any{"name": "Pirate"}, 403, nil)

	// A new issue in a component led by someone goes to them.
	var created struct{ Key string }
	paul.call("POST", "/api/issues", map[string]any{"project": key, "type": standardType(t, paul, key),
		"summary": "Dans l'API", "components": []int64{api.ID}}, 201, &created)
	var d issueDetail
	paul.get("/api/issues/"+created.Key, &d)
	if d.Assignee == nil || d.Assignee.Username != "hugo.robert" || len(d.Components) != 1 {
		t.Errorf("issue in API: assignee %+v, components %+v", d.Assignee, d.Components)
	}

	var list []componentAdmin
	paul.call("GET", base, nil, 200, &list)
	if len(list) != 2 || list[0].Name != "API" || list[0].Issues != 1 {
		t.Errorf("components seen by a developer: %+v", list)
	}
	admin.call("PATCH", fmt.Sprintf("%s/%d", base, web.ID), map[string]any{"name": "Site web", "archived": true,
		"lead": nil}, 200, &web)
	if web.Name != "Site web" || !web.Archived {
		t.Errorf("after PATCH: %+v", web)
	}

	// Deleting API moves its issue to Site web, with history.
	admin.call("DELETE", fmt.Sprintf("%s/%d", base, api.ID), map[string]any{"moveTo": api.ID}, 400, nil)
	admin.call("DELETE", fmt.Sprintf("%s/%d", base, api.ID), map[string]any{"moveTo": web.ID}, 204, nil)
	paul.get("/api/issues/"+created.Key, &d)
	if len(d.Components) != 1 || d.Components[0].ID != web.ID {
		t.Errorf("components after the move: %+v", d.Components)
	}
	var history []historyEntry
	paul.get("/api/issues/"+created.Key+"/history", &history)
	if !historyHas(history, "Component") {
		t.Errorf("history lacks the component change: %+v", history)
	}
	admin.call("DELETE", fmt.Sprintf("%s/%d", base, web.ID), nil, 204, nil)
	paul.get("/api/issues/"+created.Key, &d)
	if len(d.Components) != 0 {
		t.Errorf("components after deletion: %+v", d.Components)
	}
}

func historyHas(history []historyEntry, field string) bool {
	for _, h := range history {
		for _, it := range h.Items {
			if it.Field == field {
				return true
			}
		}
	}
	return false
}

type versionAdmin struct {
	ID          int64
	Name        string
	StartDate   *string
	ReleaseDate *string
	Released    bool
	Archived    bool
	Position    int
	Issues      struct{ Total, Unresolved int }
}

func TestVersions(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	key := newProject(t, admin, nil)
	base := "/api/projects/" + key + "/versions"

	var v1, v2, v3 versionAdmin
	admin.call("POST", base, map[string]any{"name": "1.0", "startDate": "2026-01-01", "releaseDate": "2026-03-01"}, 201, &v1)
	admin.call("POST", base, map[string]any{"name": "1.1"}, 201, &v2)
	admin.call("POST", base, map[string]any{"name": "2.0"}, 201, &v3)
	if v1.StartDate == nil || *v1.StartDate != "2026-01-01" || v2.Position <= v1.Position || v3.Position <= v2.Position {
		t.Fatalf("versions: %+v %+v %+v", v1, v2, v3)
	}
	for _, bad := range []map[string]any{{"name": "1.0"}, {"name": "x", "releaseDate": "demain"},
		{"name": "x", "startDate": "2026-05-01", "releaseDate": "2026-04-01"}} {
		if status, _ := admin.do("POST", base, bad); status != 400 && status != 409 {
			t.Errorf("create %v: %d", bad, status)
		}
	}
	paul.call("POST", base, map[string]any{"name": "pirate"}, 403, nil)
	admin.call("PATCH", fmt.Sprintf("%s/%d", base, v1.ID), map[string]any{"startDate": "2026-04-01"}, 400, nil)
	admin.call("PATCH", fmt.Sprintf("%s/%d", base, v1.ID), map[string]any{"startDate": nil}, 200, &v1)
	if v1.StartDate != nil {
		t.Errorf("start date not cleared: %+v", v1)
	}

	// Order.
	var list []versionAdmin
	admin.call("POST", base+"/order", map[string]any{"ids": []int64{v3.ID, v1.ID}}, 400, nil)
	admin.call("POST", base+"/order", map[string]any{"ids": []int64{v3.ID, v1.ID, v2.ID}}, 200, &list)
	if list[0].ID != v3.ID || list[2].ID != v2.ID {
		t.Errorf("order: %+v", list)
	}

	// Two issues fixed in 1.0, one of them resolved.
	typ := standardType(t, paul, key)
	issue := func(summary string) string {
		var c struct{ Key string }
		paul.call("POST", "/api/issues", map[string]any{"project": key, "type": typ, "summary": summary,
			"fixVersions": []int64{v1.ID}, "affectsVersions": []int64{v2.ID}}, 201, &c)
		return c.Key
	}
	open, closed := issue("Ouvert"), issue("Résolu")
	var d issueDetail
	paul.get("/api/issues/"+closed, &d)
	for _, s := range d.Statuses {
		if s.Category == "done" {
			paul.call("PATCH", "/api/issues/"+closed, map[string]any{"status": s.ID, "resolution": firstResolution(t)}, 200, nil)
			break
		}
	}
	paul.call("GET", base, nil, 200, &list)
	if i := slices.IndexFunc(list, func(v versionAdmin) bool { return v.ID == v1.ID }); i < 0 ||
		list[i].Issues.Total != 2 || list[i].Issues.Unresolved != 1 {
		t.Errorf("1.0's counts: %+v", list)
	}

	// A release date before the start date is refused, as when editing.
	var v5 versionAdmin
	admin.call("POST", base, map[string]any{"name": "3.0", "startDate": "2026-06-01"}, 201, &v5)
	admin.call("POST", fmt.Sprintf("%s/%d/release", base, v5.ID), map[string]any{"releaseDate": "2026-05-01"}, 400, nil)
	admin.call("POST", fmt.Sprintf("%s/%d/release", base, v5.ID), map[string]any{"releaseDate": "2026-06-01"}, 200, &v5)
	if !v5.Released || v5.ReleaseDate == nil || *v5.ReleaseDate != "2026-06-01" {
		t.Errorf("released on its start date: %+v", v5)
	}

	// Releasing 1.0 moves its unresolved issue to 1.1.
	admin.call("POST", fmt.Sprintf("%s/%d/release", base, v1.ID), map[string]any{"moveUnresolvedTo": v1.ID}, 400, nil)
	admin.call("POST", fmt.Sprintf("%s/%d/release", base, v1.ID), map[string]any{"releaseDate": "2026-10-01",
		"moveUnresolvedTo": v2.ID}, 200, &v1)
	if !v1.Released || v1.ReleaseDate == nil || *v1.ReleaseDate != "2026-10-01" {
		t.Errorf("released: %+v", v1)
	}
	paul.get("/api/issues/"+open, &d)
	if len(d.FixVersions) != 1 || d.FixVersions[0].ID != v2.ID {
		t.Errorf("%s's fix versions: %+v", open, d.FixVersions)
	}
	paul.get("/api/issues/"+closed, &d)
	if len(d.FixVersions) != 1 || d.FixVersions[0].ID != v1.ID {
		t.Errorf("%s (resolved) moved: %+v", closed, d.FixVersions)
	}
	var history []historyEntry
	paul.get("/api/issues/"+open+"/history", &history)
	if !historyHas(history, "Fix Version") {
		t.Errorf("no history of the move: %+v", history)
	}
	// Unresolved issues only go to an unreleased version.
	var v4 versionAdmin
	admin.call("POST", base, map[string]any{"name": "1.2"}, 201, &v4)
	admin.call("POST", fmt.Sprintf("%s/%d/release", base, v4.ID), map[string]any{"moveUnresolvedTo": v1.ID}, 400, nil)

	// Merging 1.1 into 2.0 moves fix and affects versions.
	admin.call("POST", fmt.Sprintf("%s/%d/merge", base, v2.ID), map[string]any{}, 400, nil)
	admin.call("POST", fmt.Sprintf("%s/%d/merge", base, v2.ID), map[string]any{"into": v3.ID}, 204, nil)
	paul.get("/api/issues/"+open, &d)
	if len(d.FixVersions) != 1 || d.FixVersions[0].ID != v3.ID || len(d.AffectsVersions) != 1 || d.AffectsVersions[0].ID != v3.ID {
		t.Errorf("after merge: fix %+v, affects %+v", d.FixVersions, d.AffectsVersions)
	}
	admin.call("PATCH", fmt.Sprintf("%s/%d", base, v2.ID), map[string]any{"name": "x"}, 404, nil)

	// Deleting 2.0 with a replacement for fix versions only.
	admin.call("DELETE", fmt.Sprintf("%s/%d", base, v3.ID), map[string]any{"fixTo": v4.ID}, 204, nil)
	paul.get("/api/issues/"+open, &d)
	if len(d.FixVersions) != 1 || d.FixVersions[0].ID != v4.ID || len(d.AffectsVersions) != 0 {
		t.Errorf("after deletion: fix %+v, affects %+v", d.FixVersions, d.AffectsVersions)
	}
	paul.call("DELETE", fmt.Sprintf("%s/%d", base, v4.ID), nil, 403, nil)
}

func firstResolution(t *testing.T) string {
	return pick(t, `SELECT id FROM resolutions ORDER BY position LIMIT 1`)
}
