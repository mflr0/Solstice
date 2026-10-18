package tests

// Changing a project's key: the issues take the new key, the old keys keep
// finding them, and the key is checked like a new project's.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/db"
)

func TestProjectRename(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse") // not an administrator

	old := newProject(t, admin, nil)
	issue := createIssue(t, admin, old, "Avant le changement", "")
	num := strings.TrimPrefix(issue, old+"-")
	path := func(key string) string { return "/api/projects/" + key + "/rename" }

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

// renamedProject makes a project with a version, a component, an epic and a
// story, renames it, and returns the old key, the new one and the issue keys
// as the old key wrote them.
func renamedProject(t *testing.T, admin *client) (old, fresh, epic, story string) {
	t.Helper()
	old = newProject(t, admin, nil)
	admin.call("POST", "/api/projects/"+old+"/versions", map[string]any{"name": "1.0"}, 201, nil)
	admin.call("POST", "/api/projects/"+old+"/components", map[string]any{"name": "Noyau"}, 201, nil)
	var p struct {
		IssueTypes []typeRef `json:"issueTypes"`
	}
	admin.call("GET", "/api/projects/"+old, nil, 200, &p)
	var epicType string
	for _, it := range p.IssueTypes {
		if strings.EqualFold(it.Name, "epic") {
			epicType = it.ID
		}
	}
	if epicType == "" {
		t.Fatalf("%s has no epic type", old)
	}
	var created struct{ Key string }
	admin.call("POST", "/api/issues", map[string]any{"project": old, "type": epicType, "summary": "Grand chantier"}, 201, &created)
	epic = created.Key
	story = createIssue(t, admin, old, "Petit chantier", "")
	fresh = newProjectKey()
	admin.call("POST", "/api/projects/"+old+"/rename", map[string]any{"newKey": fresh, "confirm": old}, 200, nil)
	t.Cleanup(func() { admin.do("DELETE", "/api/projects/"+fresh, map[string]string{"confirm": fresh}) })
	return old, fresh, epic, story
}

// After a rename the old key still names the project where a key is typed:
// the simple search, the project filter of lists and pickers, a new issue.
func TestProjectOldKeyWorks(t *testing.T) {
	admin := login(t, "admin", "admin")
	old, fresh, epic, story := renamedProject(t, admin)
	storyNum := strings.TrimPrefix(story, old+"-")

	// The simple search: a filter and an issue key typed as a word.
	for _, q := range []string{"project:" + old, "projet:" + strings.ToLower(old), "project:" + fresh,
		story, strings.ToLower(story), fresh + "-" + storyNum} {
		var res struct{ Total int }
		if status := admin.get("/api/issues?q="+url.QueryEscape(q), &res); status != 200 || res.Total < 1 {
			t.Errorf("simple search %q: %d, %d issues", q, status, res.Total)
		}
	}
	var res struct{ Total int }
	admin.call("GET", "/api/issues?q="+url.QueryEscape("project:"+old+" "+story), nil, 200, &res)
	if res.Total != 1 {
		t.Errorf("simple search of one issue by its old key: %d issues, want 1", res.Total)
	}
	admin.call("GET", "/api/issues?q="+url.QueryEscape("-project:"+old+" "+story), nil, 200, &res)
	if res.Total != 0 {
		t.Errorf("not project:OLD finds %d issues of the project", res.Total)
	}

	// The project filter of the list and the facets.
	for _, key := range []string{old, fresh} {
		admin.call("GET", "/api/issues?project="+key, nil, 200, &res)
		if res.Total != 2 {
			t.Errorf("/api/issues?project=%s: %d issues, want 2", key, res.Total)
		}
		var facets struct{ Total int }
		admin.call("GET", "/api/issues/facets?fields=status&project="+key, nil, 200, &facets)
		if facets.Total != 2 {
			t.Errorf("facets of project=%s: %d issues, want 2", key, facets.Total)
		}
	}

	// The pickers.
	for _, key := range []string{old, fresh} {
		var epics []struct{ Key string }
		admin.call("GET", "/api/epics?project="+key, nil, 200, &epics)
		if len(epics) != 1 || epics[0].Key != fresh+"-"+strings.TrimPrefix(epic, old+"-") {
			t.Errorf("epics of project=%s: %+v", key, epics)
		}
		for _, kind := range []string{"versions", "components"} {
			var parts []struct{ Name string }
			admin.call("GET", "/api/"+kind+"?project="+key, nil, 200, &parts)
			if len(parts) != 1 {
				t.Errorf("%s of project=%s: %+v", kind, key, parts)
			}
		}
	}

	// A new issue created in the old key lands in the project.
	var created struct{ Key string }
	admin.call("POST", "/api/issues", map[string]any{"project": strings.ToLower(old), "type": standardType(t, admin, fresh),
		"summary": "Créé sous l'ancienne clé"}, 201, &created)
	if !strings.HasPrefix(created.Key, fresh+"-") {
		t.Errorf("an issue created in %s has the key %s, want %s-n", old, created.Key, fresh)
	}
	admin.call("POST", "/api/issues", map[string]any{"project": "NOPE", "type": standardType(t, admin, fresh),
		"summary": "Nulle part"}, 404, nil)
}

// The digest checks the issues a change names; a key written before a rename
// is still an issue the reader may see.
func TestProjectOldKeyInDigest(t *testing.T) {
	env := newNotifyEnv(t)
	admin := env.login("admin", "admin")
	paul := env.login("paul.martin", "motdepasse")
	old := newProject(t, admin, nil)
	a, b := createIssue(t, admin, old, "Source du lien", ""), createIssue(t, admin, old, "Cible du lien", "")
	paul.call("PUT", "/api/me/notifications", map[string]any{"delivery": "digest", "digestHour": 7}, 200, nil)
	paul.call("PUT", "/api/issues/"+a+"/watch", nil, 200, nil)
	linkType := pick(t, `SELECT id FROM link_types ORDER BY position LIMIT 1`)
	admin.call("POST", "/api/issues/"+a+"/links", map[string]any{"type": json.Number(linkType), "direction": "outward",
		"target": b}, 200, nil)
	fresh := newProjectKey()
	admin.call("POST", "/api/projects/"+old+"/rename", map[string]any{"newKey": fresh, "confirm": old}, 200, nil)
	t.Cleanup(func() { admin.do("DELETE", "/api/projects/"+fresh, map[string]string{"confirm": fresh}) })

	got := to(env.deliverAt(25*time.Hour), "paul.martin@exemple.fr")
	if len(got) != 1 || !strings.Contains(got[0].Text, b) {
		t.Fatalf("the digest lost the link to %s: %+v", b, got)
	}
}

// A project may take back a key it had; a key another project has or had
// stays refused.
func TestProjectRenameBack(t *testing.T) {
	admin := login(t, "admin", "admin")
	first := newProject(t, admin, nil)
	issue := createIssue(t, admin, first, "Aller-retour", "")
	num := strings.TrimPrefix(issue, first+"-")
	second := newProjectKey()
	path := func(key string) string { return "/api/projects/" + key + "/rename" }
	admin.call("POST", path(first), map[string]any{"newKey": second, "confirm": first}, 200, nil)
	t.Cleanup(func() {
		admin.do("DELETE", "/api/projects/"+first, map[string]string{"confirm": first})
		admin.do("DELETE", "/api/projects/"+second, map[string]string{"confirm": second})
	})

	// Another project cannot take either key; the one it has is refused too.
	other := newProject(t, admin, nil)
	admin.call("POST", path(other), map[string]any{"newKey": first, "confirm": other}, 409, nil)
	admin.call("POST", path(second), map[string]any{"newKey": second, "confirm": second}, 409, nil)

	// The check as one types: for this project its former key is free, for a
	// new project or another one it is not, and neither is the key it has.
	for _, c := range []struct {
		query     string
		available bool
	}{
		{"key=" + first, false}, {"key=" + first + "&project=" + second, true},
		{"key=" + first + "&project=" + first, true}, {"key=" + first + "&project=" + other, false},
		{"key=" + second + "&project=" + second, false}, {"key=" + first + "&project=", false},
	} {
		var check struct{ Available bool }
		admin.call("GET", "/api/admin/project-key?"+c.query, nil, 200, &check)
		if check.Available != c.available {
			t.Errorf("project-key?%s: available %t, want %t", c.query, check.Available, c.available)
		}
	}
	admin.call("GET", "/api/admin/project-key?key=ABCD&project=NOPE", nil, 404, nil)

	// Back to the first key: it is the project's again, and the second one,
	// now a former key, still finds the issue.
	var back struct{ Key string }
	admin.call("POST", path(second), map[string]any{"newKey": first, "confirm": second}, 200, &back)
	if back.Key != first {
		t.Fatalf("renamed back to %q, want %s", back.Key, first)
	}
	if got := pick(t, `SELECT COUNT(*) FROM project_keys WHERE pkey = ?`, first); got != "0" {
		t.Errorf("the key %s is still listed as former (%s rows)", first, got)
	}
	if got := pick(t, `SELECT COUNT(*) FROM project_keys WHERE pkey = ?`, second); got != "1" {
		t.Errorf("the key %s is listed %s times as former, want 1", second, got)
	}
	for _, key := range []string{first + "-" + num, second + "-" + num} {
		var d struct{ Key string }
		if status := admin.get("/api/issues/"+key, &d); status != 200 || d.Key != first+"-"+num {
			t.Errorf("GET /api/issues/%s: %d %q, want %s-%s", key, status, d.Key, first, num)
		}
	}
	for _, q := range []string{"project = " + first, "project = " + second, "key = " + second + "-" + num} {
		if status, total, _ := runSearch(t, admin, q); status != 200 || total != 1 {
			t.Errorf("%s: %d, %d issues, want 1", q, status, total)
		}
	}

	// Still refused: the former key of this project, for another one, and the
	// key another project has, for this one.
	admin.call("POST", path(other), map[string]any{"newKey": second, "confirm": other}, 409, nil)
	admin.call("POST", path(first), map[string]any{"newKey": other, "confirm": first}, 409, nil)
}

// The version functions of JQL take a project key, and a former key names
// the project there too.
func TestProjectOldKeyInVersionFunctions(t *testing.T) {
	admin := login(t, "admin", "admin")
	old := newProject(t, admin, nil)
	base := "/api/projects/" + old + "/versions"
	var shipped, coming versionAdmin
	admin.call("POST", base, map[string]any{"name": "1.0"}, 201, &shipped)
	admin.call("POST", base, map[string]any{"name": "2.0"}, 201, &coming)
	typ := standardType(t, admin, old)
	issue := func(summary string, version int64) string {
		var c struct{ Key string }
		admin.call("POST", "/api/issues", map[string]any{"project": old, "type": typ, "summary": summary,
			"fixVersions": []int64{version}}, 201, &c)
		return c.Key
	}
	done, open := issue("Livré", shipped.ID), issue("À venir", coming.ID)

	// 1.0 is released once its only issue is resolved.
	var d issueDetail
	admin.get("/api/issues/"+done, &d)
	for _, s := range d.Statuses {
		if s.Category == "done" {
			admin.call("PATCH", "/api/issues/"+done, map[string]any{"status": s.ID, "resolution": firstResolution(t)}, 200, nil)
			break
		}
	}
	admin.call("POST", base+"/"+strconv.FormatInt(shipped.ID, 10)+"/release", map[string]any{}, 200, nil)

	fresh := newProjectKey()
	admin.call("POST", "/api/projects/"+old+"/rename", map[string]any{"newKey": fresh, "confirm": old}, 200, nil)
	t.Cleanup(func() { admin.do("DELETE", "/api/projects/"+fresh, map[string]string{"confirm": fresh}) })

	for _, c := range []struct{ fn, want string }{
		{"releasedVersions", done}, {"latestReleasedVersion", done},
		{"unreleasedVersions", open}, {"earliestUnreleasedVersion", open},
	} {
		for _, key := range []string{old, fresh} {
			q := "fixVersion in " + c.fn + "(" + key + ") AND project = " + fresh
			status, total, mode := runSearch(t, admin, q)
			if status != 200 || mode != "jql" || total != 1 {
				t.Errorf("%s: %d, %d issues (%s), want 1", q, status, total, mode)
				continue
			}
			var res struct{ Issues []struct{ Key string } }
			admin.get("/api/issues?q="+url.QueryEscape(q), &res)
			if len(res.Issues) != 1 || res.Issues[0].Key != fresh+"-"+strings.TrimPrefix(c.want, old+"-") {
				t.Errorf("%s: %+v, want the issue %s", q, res.Issues, c.want)
			}
		}
	}
}

// Projects renamed at the same moment to one key: one wins, the others are
// refused as taken, none fails with a server error, and the key is never
// both current and former.
func TestProjectRenameConcurrent(t *testing.T) {
	admin := login(t, "admin", "admin")
	path := func(key string) string { return "/api/projects/" + key + "/rename" }
	for round := 0; round < 5; round++ {
		const racers = 4
		keys := make([]string, racers)
		for i := range keys {
			keys[i] = newProject(t, admin, nil)
		}
		target := newProjectKey()
		statuses := make([]int, racers)
		bodies := make([][]byte, racers)
		var wg sync.WaitGroup
		for i := range keys {
			wg.Add(1)
			go func() {
				defer wg.Done()
				statuses[i], bodies[i] = admin.do("POST", path(keys[i]), map[string]any{"newKey": target, "confirm": keys[i]})
			}()
		}
		wg.Wait()
		won := 0
		for i, status := range statuses {
			switch status {
			case 200:
				won++
				t.Cleanup(func() { admin.do("DELETE", "/api/projects/"+target, map[string]string{"confirm": target}) })
			case 409:
				t.Cleanup(func() { admin.do("DELETE", "/api/projects/"+keys[i], map[string]string{"confirm": keys[i]}) })
			default:
				t.Errorf("round %d: renaming %s to %s: %d, want 200 or 409: %q", round, keys[i], target, status, bodies[i])
			}
		}
		if won != 1 {
			t.Errorf("round %d: %d renames to %s succeeded, want 1", round, won, target)
		}
		if got := pick(t, `SELECT COUNT(*) FROM project_keys WHERE pkey = ?`, target); got != "0" {
			t.Errorf("round %d: %s is also a former key", round, target)
		}
	}
}

// Two renames to one key in a row: the second is refused as taken, and the
// key is not also a former key. A project created with a key another one has
// just left is refused too.
func TestProjectRenameSameKeyInARow(t *testing.T) {
	admin := login(t, "admin", "admin")
	path := func(key string) string { return "/api/projects/" + key + "/rename" }
	first, second := newProject(t, admin, nil), newProject(t, admin, nil)
	target := newProjectKey()
	admin.call("POST", path(first), map[string]any{"newKey": target, "confirm": first}, 200, nil)
	t.Cleanup(func() { admin.do("DELETE", "/api/projects/"+target, map[string]string{"confirm": target}) })
	admin.call("POST", path(second), map[string]any{"newKey": target, "confirm": second}, 409, nil)
	if got := pick(t, `SELECT COUNT(*) FROM project_keys WHERE pkey = ?`, target); got != "0" {
		t.Errorf("%s is also a former key", target)
	}
	if got := pick(t, `SELECT COUNT(*) FROM projects WHERE pkey = ?`, second); got != "1" {
		t.Errorf("a refused rename changed the project (%s with the key %s)", got, second)
	}
	admin.call("POST", "/api/projects", map[string]any{"key": first, "name": "Reprise " + first}, 409, nil)
}

// A project created and another renamed at the same moment to one key: one
// wins, the other is refused as taken, none fails with a server error, and
// the key is never both current and former.
func TestProjectCreateRenameConcurrent(t *testing.T) {
	admin := login(t, "admin", "admin")
	for round := 0; round < 5; round++ {
		old := newProject(t, admin, nil)
		target := newProjectKey()
		var statuses [2]int
		var bodies [2][]byte
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			statuses[0], bodies[0] = admin.do("POST", "/api/projects/"+old+"/rename",
				map[string]any{"newKey": target, "confirm": old})
		}()
		go func() {
			defer wg.Done()
			statuses[1], bodies[1] = admin.do("POST", "/api/projects", map[string]any{"key": target, "name": "Course " + target})
		}()
		wg.Wait()
		t.Cleanup(func() {
			admin.do("DELETE", "/api/projects/"+target, map[string]string{"confirm": target})
			admin.do("DELETE", "/api/projects/"+old, map[string]string{"confirm": old})
		})
		renamed, created := statuses[0], statuses[1]
		if !(renamed == 200 && created == 409) && !(renamed == 409 && created == 201) {
			t.Errorf("round %d: rename %d, creation %d, want one success and one 409: %q %q",
				round, renamed, created, bodies[0], bodies[1])
		}
		if got := pick(t, `SELECT COUNT(*) FROM project_keys WHERE pkey = ?`, target); got != "0" {
			t.Errorf("round %d: %s is also a former key", round, target)
		}
	}
}

// A former key that does not follow the key rules (one the import kept as it
// was) cannot be taken back: the new key is always checked for its form.
func TestProjectRenameBackChecksForm(t *testing.T) {
	admin := login(t, "admin", "admin")
	key := newProject(t, admin, nil)
	t.Cleanup(func() { admin.do("DELETE", "/api/projects/"+key, map[string]string{"confirm": key}) })
	const legacy = "OLD_KEY"
	if _, err := testDB.Exec(`INSERT INTO project_keys (pkey, project_id) SELECT ?, id FROM projects WHERE pkey = ?`,
		legacy, key); err != nil {
		t.Fatal(err)
	}
	admin.call("POST", "/api/projects/"+key+"/rename", map[string]any{"newKey": legacy, "confirm": key}, 400, nil)
	if got := pick(t, `SELECT COUNT(*) FROM projects WHERE pkey = ?`, key); got != "1" {
		t.Errorf("a refused rename changed the project (%s with the key %s)", got, key)
	}
	if got := pick(t, `SELECT COUNT(*) FROM project_keys WHERE pkey = ?`, legacy); got != "1" {
		t.Errorf("a refused rename removed the former key (%s rows)", got)
	}
}

// With a pool of one connection, the lock on the keys is let go as soon as the
// change is committed: the audit entry and the read that follow need that
// connection. A rename and a creation both answer, instead of waiting for it.
func TestProjectKeyLockReleasedBeforeAudit(t *testing.T) {
	needDB(t)
	one, err := db.Open(context.Background(), os.Getenv("SOLSTICE_TEST_DB"),
		db.Pool{MaxOpenConns: 1, MaxIdleConns: 1, ConnMaxLifetime: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { one.Close() })
	ts := httptest.NewServer(api.New(one, t.TempDir(), time.UTC, quiet()).Handler(http.NotFoundHandler()))
	t.Cleanup(ts.Close)
	admin := loginAt(t, ts.URL, "admin", "admin")
	admin.http.Timeout = 10 * time.Second
	old, fresh := newProjectKey(), newProjectKey()
	admin.call("POST", "/api/projects", map[string]any{"key": old, "name": "Pool " + old}, 201, nil)
	t.Cleanup(func() {
		admin.do("DELETE", "/api/projects/"+fresh, map[string]string{"confirm": fresh})
		admin.do("DELETE", "/api/projects/"+old, map[string]string{"confirm": old})
	})
	admin.call("POST", "/api/projects/"+old+"/rename", map[string]any{"newKey": fresh, "confirm": old}, 200, nil)
}
