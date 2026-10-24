package tests

// What issue creation, edition and attachment deletion write: the security
// level, components, versions, labels and the epic link.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"slices"
	"strconv"
	"testing"

	"solstice/internal/jirafixture"
)

func levelID(t *testing.T, name string) int64 {
	t.Helper()
	id, _ := strconv.ParseInt(pick(t, `SELECT l.id FROM security_levels l JOIN projects p ON p.id = l.project_id
		WHERE p.pkey = 'DEMO' AND l.name = ?`, name), 10, 64)
	return id
}

func levelName(t *testing.T, c *client, key string) string {
	t.Helper()
	var d map[string]any
	c.call("GET", "/api/issues/"+key, nil, 200, &d)
	return str(d, "securityLevel", "name")
}

// securityChanges counts the history lines of the level, which the history
// endpoint never shows.
func securityChanges(t *testing.T, key string) string {
	t.Helper()
	return pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM change_items ci JOIN change_groups g ON g.id = ci.group_id
		WHERE ci.field = 'security' AND g.issue_id = ?`, issueID(t, key))
}

func demoSubtask(t *testing.T, c *client, summary, parent string) string {
	t.Helper()
	var created struct{ Key string }
	c.call("POST", "/api/issues", map[string]any{"project": "DEMO", "summary": summary, "parent": parent,
		"type": pick(t, `SELECT t.issue_type_id FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
			JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND it.subtask LIMIT 1`)}, 201, &created)
	return created.Key
}

func TestSecurityLevelOnCreateAndUpdate(t *testing.T) {
	paul := login(t, "paul.martin", jirafixture.Password)
	chloe := login(t, "chloe.simon", jirafixture.Password)
	team, restricted := levelID(t, "Équipe"), levelID(t, "Restreint")
	newIssue := func(c *client, level any) (int, []byte) {
		return c.do("POST", "/api/issues", map[string]any{"project": "DEMO", "type": standardType(t, c, "DEMO"),
			"summary": "Niveau de sécurité", "securityLevel": level})
	}

	var created struct{ Key string }
	status, body := newIssue(paul, team)
	if status != 201 {
		t.Fatalf("create with a level: %d %s", status, body)
	}
	created.Key = obj(t, body)["key"].(string)
	if got := levelName(t, paul, created.Key); got != "Équipe" {
		t.Errorf("level of the new issue: %q", got)
	}
	if status := chloe.get("/api/issues/"+created.Key, nil); status != 404 {
		t.Errorf("an outsider sees the secured issue: %d", status)
	}
	if status, body := newIssue(chloe, team); status != 403 {
		t.Errorf("a user without the permission set a level: %d %s", status, body)
	}
	if status, _ := newIssue(paul, 999999999); status != 400 {
		t.Errorf("unknown level: %d, want 400", status)
	}
	if status, _ := newIssue(paul, "Équipe"); status != 400 {
		t.Errorf("a level given by name: %d, want 400", status)
	}
	status, body = newIssue(paul, nil)
	if status != 201 {
		t.Fatalf("create with a null level: %d %s", status, body)
	}
	plain := obj(t, body)["key"].(string)
	if got := levelName(t, paul, plain); got != "" {
		t.Errorf("level of an issue created with null: %q", got)
	}

	// Update.
	paul.call("PATCH", "/api/issues/"+plain, map[string]any{"securityLevel": restricted}, 200, nil)
	if got := levelName(t, paul, plain); got != "Restreint" {
		t.Errorf("level after the update: %q", got)
	}
	paul.call("PATCH", "/api/issues/"+plain, map[string]any{"securityLevel": restricted}, 200, nil)
	paul.call("PATCH", "/api/issues/"+plain, map[string]any{"summary": "Niveau inchangé"}, 200, nil)
	if got := securityChanges(t, plain); got != "1" {
		t.Errorf("history lines of the level: %s, want 1 and none for an unchanged level", got)
	}
	paul.call("PATCH", "/api/issues/"+plain, map[string]any{"securityLevel": nil}, 200, nil)
	if got := levelName(t, paul, plain); got != "" {
		t.Errorf("level after clearing: %q", got)
	}
	if got := securityChanges(t, plain); got != "2" {
		t.Errorf("history lines after clearing: %s, want 2", got)
	}

	for name, in := range map[string]any{"unknown": 999999999, "text": "Restreint"} {
		if status, _ := paul.do("PATCH", "/api/issues/"+plain, map[string]any{"securityLevel": in}); status != 400 {
			t.Errorf("%s level on update: %d, want 400", name, status)
		}
	}
	web := createIssue(t, paul, "WEB", "Niveau d'un autre projet", "")
	if status, _ := paul.do("PATCH", "/api/issues/"+web, map[string]any{"securityLevel": restricted}); status != 400 {
		t.Errorf("a level of another project: %d, want 400", status)
	}

	// Without the permission: chloe reports an issue and may edit it, not secure it.
	mine := createIssue(t, chloe, "DEMO", "Ticket de Chloé", "")
	if status, _ := chloe.do("PATCH", "/api/issues/"+mine, map[string]any{"securityLevel": team}); status != 403 {
		t.Errorf("a user without the permission set a level on update: %d, want 403", status)
	}

	// Securing someone else's issue can hide it from the one who did it.
	paul.call("PATCH", "/api/issues/"+mine, map[string]any{"securityLevel": restricted}, 200, nil)
	if status := paul.get("/api/issues/"+mine, nil); status != 404 {
		t.Errorf("the issue is still visible to the one who restricted it: %d", status)
	}
	if status := chloe.get("/api/issues/"+mine, nil); status != 200 {
		t.Errorf("the reporter lost the issue: %d", status)
	}
}

func TestSubtaskSecurityLevel(t *testing.T) {
	paul := login(t, "paul.martin", jirafixture.Password)
	chloe := login(t, "chloe.simon", jirafixture.Password)
	team, restricted := levelID(t, "Équipe"), levelID(t, "Restreint")

	open := newTestIssue(t, paul, "Parent ouvert", nil)
	child := demoSubtask(t, paul, "Enfant ouvert", open)
	if got := levelName(t, paul, child); got != "" {
		t.Errorf("a sub-task of an open issue has the level %q", got)
	}
	if status := chloe.get("/api/issues/"+child, nil); status != 200 {
		t.Errorf("an outsider does not see an open sub-task: %d", status)
	}

	parent := newTestIssue(t, paul, "Parent protégé", map[string]any{"securityLevel": team})
	child = demoSubtask(t, paul, "Enfant protégé", parent)
	if got := levelName(t, paul, child); got != "Équipe" {
		t.Errorf("a sub-task of a secured issue has the level %q", got)
	}
	if status, body := paul.do("PATCH", "/api/issues/"+child, map[string]any{"securityLevel": restricted}); status != 400 {
		t.Errorf("a level set on a sub-task: %d %s, want 400", status, body)
	}
	if status, _ := paul.do("PATCH", "/api/issues/"+child, map[string]any{"securityLevel": nil}); status != 400 {
		t.Errorf("a level cleared on a sub-task: %d, want 400", status)
	}
	if got := levelName(t, paul, child); got != "Équipe" {
		t.Errorf("level of the sub-task after the refusals: %q", got)
	}

	// The parent's level follows to its sub-tasks.
	paul.call("PATCH", "/api/issues/"+parent, map[string]any{"securityLevel": restricted}, 200, nil)
	if got := levelName(t, paul, child); got != "Restreint" {
		t.Errorf("level of the sub-task after its parent changed: %q", got)
	}
	paul.call("PATCH", "/api/issues/"+parent, map[string]any{"securityLevel": nil}, 200, nil)
	if got := levelName(t, paul, child); got != "" {
		t.Errorf("level of the sub-task after its parent was cleared: %q", got)
	}
	if status := chloe.get("/api/issues/"+child, nil); status != 200 {
		t.Errorf("the sub-task stays hidden after its parent was opened: %d", status)
	}
}

func nameSet(list []idName) []string {
	var out []string
	for _, v := range list {
		out = append(out, v.Name)
	}
	slices.Sort(out)
	return out
}

func wantNames(t *testing.T, what string, list []idName, want ...string) {
	t.Helper()
	slices.Sort(want)
	if got := nameSet(list); !slices.Equal(got, want) {
		t.Errorf("%s: %v, want %v", what, got, want)
	}
}

func TestIssueSets(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password)
	key, other := newProject(t, admin, nil), newProject(t, admin, nil)

	add := func(project, kind, name string) int64 {
		var out struct{ ID int64 }
		admin.call("POST", "/api/projects/"+project+"/"+kind, map[string]any{"name": name}, 201, &out)
		return out.ID
	}
	compA, compB, compC := add(key, "components", "A"), add(key, "components", "B"), add(key, "components", "C")
	v1, v2, v3 := add(key, "versions", "1.0"), add(key, "versions", "1.1"), add(key, "versions", "2.0")
	foreignComp, foreignVersion := add(other, "components", "Autre"), add(other, "versions", "9.9")
	typ := standardType(t, paul, key)

	issue := func() string {
		t.Helper()
		var c struct{ Key string }
		paul.call("POST", "/api/issues", map[string]any{"project": key, "type": typ, "summary": "Ensembles",
			"components": []int64{compA, compB}, "fixVersions": []int64{v1, v2}, "affectsVersions": []int64{v1},
			"labels": []string{"b c", "b   c", " a ", "a", ""}}, 201, &c)
		return c.Key
	}
	detail := func(k string) issueDetail {
		t.Helper()
		var d issueDetail
		paul.call("GET", "/api/issues/"+k, nil, 200, &d)
		return d
	}
	patch := func(k string, in map[string]any) {
		t.Helper()
		paul.call("PATCH", "/api/issues/"+k, in, 200, nil)
	}

	k := issue()
	d := detail(k)
	wantNames(t, "components on creation", d.Components, "A", "B")
	wantNames(t, "fix versions on creation", d.FixVersions, "1.0", "1.1")
	wantNames(t, "affected versions on creation", d.AffectsVersions, "1.0")
	if !slices.Equal(d.Labels, []string{"a", "b_c"}) {
		t.Errorf("labels on creation: %q, want duplicates and blanks dropped, spaces joined by _", d.Labels)
	}
	if h := historyFields(t, paul, k); h["Component"]+h["Fix Version"]+h["Version"]+h["labels"] != 0 {
		t.Errorf("history of a new issue: %v", h)
	}

	// Add, remove, replace.
	patch(k, map[string]any{"components": []int64{compA, compB, compC}, "fixVersions": []int64{v1, v2, v3}})
	d = detail(k)
	wantNames(t, "components after the addition", d.Components, "A", "B", "C")
	wantNames(t, "fix versions after the addition", d.FixVersions, "1.0", "1.1", "2.0")
	wantNames(t, "affected versions left alone", d.AffectsVersions, "1.0")
	h := historyFields(t, paul, k)
	if h["Component"] != 1 || h["Fix Version"] != 1 || h["Version"] != 0 {
		t.Errorf("history after the addition: %v", h)
	}

	patch(k, map[string]any{"components": []int64{compC}, "affectsVersions": []int64{v2, v3}})
	d = detail(k)
	wantNames(t, "components after the removal", d.Components, "C")
	wantNames(t, "affected versions after the replacement", d.AffectsVersions, "1.1", "2.0")
	wantNames(t, "fix versions left alone", d.FixVersions, "1.0", "1.1", "2.0")
	h = historyFields(t, paul, k)
	if h["Component"] != 3 || h["Version"] != 3 {
		t.Errorf("history after the removal and the replacement: %v", h)
	}

	// Nothing changes, nothing is recorded: same set, repeated ids, other fields.
	patch(k, map[string]any{"components": []int64{compC, compC}, "affectsVersions": []int64{v3, v2}})
	patch(k, map[string]any{"summary": "Ensembles, renommé"})
	if got := historyFields(t, paul, k); got["Component"] != 3 || got["Version"] != 3 {
		t.Errorf("history after a change that changes nothing: %v", got)
	}
	d = detail(k)
	wantNames(t, "components after an unchanged update", d.Components, "C")

	patch(k, map[string]any{"components": []int64{}, "fixVersions": nil})
	d = detail(k)
	if len(d.Components) != 0 || len(d.FixVersions) != 0 {
		t.Errorf("sets after clearing: %+v %+v", d.Components, d.FixVersions)
	}
	wantNames(t, "affected versions left alone by the clearing", d.AffectsVersions, "1.1", "2.0")

	// Another project's values, unknown ids and bad shapes.
	for name, in := range map[string]map[string]any{
		"component of another project":    {"components": []int64{foreignComp}},
		"fix version of another project":  {"fixVersions": []int64{foreignVersion}},
		"affected version of another one": {"affectsVersions": []int64{v1, foreignVersion}},
		"unknown component":               {"components": []int64{999999999}},
		"not a list":                      {"components": "A"},
		"names instead of ids":            {"fixVersions": []string{"1.0"}},
	} {
		in["summary"] = "Ne doit pas passer"
		if status, body := paul.do("PATCH", "/api/issues/"+k, in); status != 400 {
			t.Errorf("update with a %s: %d %s, want 400", name, status, body)
		}
	}
	d = detail(k)
	if d.Summary != "Ensembles, renommé" || len(d.Components) != 0 || len(d.FixVersions) != 0 {
		t.Errorf("a refused update changed the issue: %q %+v %+v", d.Summary, d.Components, d.FixVersions)
	}
	wantNames(t, "affected versions after the refusals", d.AffectsVersions, "1.1", "2.0")

	// A refused creation leaves nothing behind.
	var list struct{ Total int }
	paul.call("GET", "/api/issues?q=projet:"+key, nil, 200, &list)
	total := list.Total
	for name, in := range map[string]map[string]any{
		"component": {"components": []int64{foreignComp}},
		"version":   {"affectsVersions": []int64{foreignVersion}},
	} {
		in["project"], in["type"], in["summary"] = key, typ, "Refusé"
		if status, body := paul.do("POST", "/api/issues", in); status != 400 {
			t.Errorf("create with a %s of another project: %d %s, want 400", name, status, body)
		}
	}
	paul.call("GET", "/api/issues?q=projet:"+key, nil, 200, &list)
	if list.Total != total {
		t.Errorf("issues in the project after the refusals: %d, want %d", list.Total, total)
	}
}

func TestIssueLabels(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password)
	key := newProject(t, admin, nil)
	k := createIssue(t, paul, key, "Étiquettes", "")
	labels := func() []string {
		var d issueDetail
		paul.call("GET", "/api/issues/"+k, nil, 200, &d)
		return d.Labels
	}
	set := func(in any) { paul.call("PATCH", "/api/issues/"+k, map[string]any{"labels": in}, 200, nil) }

	set([]string{"zèbre", "alpha", " beta  gamma ", "alpha", "beta gamma", "", "   "})
	if want := []string{"alpha", "beta_gamma", "zèbre"}; !slices.Equal(labels(), want) {
		t.Errorf("labels: %q, want %q", labels(), want)
	}
	if got := historyFields(t, paul, k)["labels"]; got != 1 {
		t.Errorf("history lines of the labels: %d, want 1", got)
	}
	set([]string{"beta_gamma", "zèbre", "alpha"})
	set([]string{" alpha", "beta\tgamma", "zèbre  "})
	if got := historyFields(t, paul, k)["labels"]; got != 1 {
		t.Errorf("history lines after the same labels written otherwise: %d, want 1", got)
	}
	paul.call("PATCH", "/api/issues/"+k, map[string]any{"summary": "Étiquettes, renommé"}, 200, nil)
	if want := []string{"alpha", "beta_gamma", "zèbre"}; !slices.Equal(labels(), want) {
		t.Errorf("labels after an update without labels: %q", labels())
	}
	set([]string{"zèbre"})
	if want := []string{"zèbre"}; !slices.Equal(labels(), want) {
		t.Errorf("labels after the replacement: %q", labels())
	}
	set(nil)
	if len(labels()) != 0 {
		t.Errorf("labels after null: %q", labels())
	}
	set([]string{"a"})
	set([]string{})
	if len(labels()) != 0 {
		t.Errorf("labels after an empty list: %q", labels())
	}
	if got := historyFields(t, paul, k)["labels"]; got != 5 {
		t.Errorf("history lines of the labels at the end: %d, want 5", got)
	}
	if status, _ := paul.do("PATCH", "/api/issues/"+k, map[string]any{"labels": "a b"}); status != 400 {
		t.Errorf("labels as text: %d, want 400", status)
	}
}

func TestIssueCreatedInEpic(t *testing.T) {
	marie := login(t, "marie.durand", jirafixture.Password)
	epic := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN issue_types t ON t.id = i.issue_type_id WHERE p.pkey = 'DEMO' AND t.name = 'Epic' ORDER BY i.id LIMIT 1`)
	story := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN issue_types t ON t.id = i.issue_type_id WHERE p.pkey = 'DEMO' AND t.name = 'Story' AND i.parent_id IS NULL
		ORDER BY i.id LIMIT 1`)
	storyType := pick(t, `SELECT id FROM issue_types WHERE name = 'Story'`)
	epicType := pick(t, `SELECT id FROM issue_types WHERE name = 'Epic'`)
	create := func(in map[string]any) (int, []byte) {
		if in["type"] == nil {
			in["type"] = storyType
		}
		in["project"], in["summary"] = "DEMO", "Créé dans un chantier"
		return marie.do("POST", "/api/issues", in)
	}
	total := func() int {
		var list struct{ Total int }
		marie.call("GET", "/api/issues?q=projet:DEMO", nil, 200, &list)
		return list.Total
	}

	status, body := create(map[string]any{"epic": epic})
	if status != 201 {
		t.Fatalf("create in an epic: %d %s", status, body)
	}
	var d issueDetail
	marie.call("GET", "/api/issues/"+obj(t, body)["key"].(string), nil, 200, &d)
	if d.Epic == nil || d.Epic.Key != epic {
		t.Errorf("epic of the new issue: %+v, want %s", d.Epic, epic)
	}
	status, body = create(map[string]any{"epic": nil})
	if status != 201 {
		t.Fatalf("create with a null epic: %d %s", status, body)
	}
	marie.call("GET", "/api/issues/"+obj(t, body)["key"].(string), nil, 200, &d)
	if d.Epic != nil {
		t.Errorf("epic of an issue created with null: %+v", d.Epic)
	}

	before := total()
	subtaskType := pick(t, `SELECT t.issue_type_id FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND it.subtask LIMIT 1`)
	for name, in := range map[string]map[string]any{
		"an unknown epic": {"epic": "DEMO-999999"},
		"a story":         {"epic": story},
		"a number":        {"epic": 12},
		"a sub-task":      {"epic": epic, "parent": story, "type": subtaskType},
		"an epic":         {"epic": epic, "type": epicType},
	} {
		if status, body := create(in); status != 400 {
			t.Errorf("create with %s as epic: %d %s, want 400", name, status, body)
		}
	}
	if got := total(); got != before {
		t.Errorf("issues in DEMO after the refusals: %d, want %d", got, before)
	}
}

func uploadTo(t *testing.T, c *client, key, name, content string) int64 {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	fw.Write([]byte(content))
	mw.Close()
	req, _ := http.NewRequest("POST", c.base+"/api/issues/"+key+"/attachments", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(csrfHeader, "1")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		t.Fatalf("upload %s: %d %s", name, resp.StatusCode, raw)
	}
	var d issueDetail
	if err := json.Unmarshal(raw, &d); err != nil || len(d.Attachments) == 0 {
		t.Fatalf("upload %s: %v %s", name, err, raw)
	}
	return d.Attachments[len(d.Attachments)-1].ID
}

func TestAttachmentDeletion(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password)
	jeanne := login(t, "jeanne.dupont", jirafixture.Password)
	hugo := login(t, "hugo.robert", jirafixture.Password)
	key := newProject(t, admin, nil)

	var perms struct {
		Grants []grantOut `json:"grants"`
	}
	admin.call("GET", "/api/projects/"+key+"/permissions", nil, 200, &perms)
	grants := append(perms.Grants, grantOut{Permission: "moderate", Holder: "user", ID: userID(t, "hugo.robert")})
	admin.call("PUT", "/api/projects/"+key+"/permissions", map[string]any{"grants": grants}, 200, nil)

	issue := createIssue(t, paul, key, "Pièces jointes", "")
	attachments := func() []int64 {
		var d issueDetail
		paul.call("GET", "/api/issues/"+issue, nil, 200, &d)
		var ids []int64
		for _, a := range d.Attachments {
			ids = append(ids, a.ID)
		}
		return ids
	}
	path := func(id int64) string { return fmt.Sprintf("/api/attachments/%d", id) }

	first := uploadTo(t, paul, issue, "premier.txt", "un")
	second := uploadTo(t, paul, issue, "second.txt", "deux")
	third := uploadTo(t, paul, issue, "troisième.txt", "trois")

	if status, _ := jeanne.do("DELETE", path(first), nil); status != 403 {
		t.Errorf("someone else deleted an attachment: %d, want 403", status)
	}
	if status, _ := anonymous(t).do("DELETE", path(first), nil); status != 401 {
		t.Errorf("an anonymous visitor deleted an attachment: %d, want 401", status)
	}
	if status, _ := paul.do("DELETE", "/api/attachments/999999999", nil); status != 404 {
		t.Errorf("unknown attachment: %d, want 404", status)
	}
	if got := attachments(); len(got) != 3 {
		t.Fatalf("attachments after the refusals: %v", got)
	}

	if status, body := paul.do("DELETE", path(first), nil); status != 204 {
		t.Fatalf("author deleting: %d %s", status, body)
	}
	if status, body := hugo.do("DELETE", path(second), nil); status != 204 {
		t.Fatalf("moderator deleting: %d %s", status, body)
	}
	if got := attachments(); !slices.Equal(got, []int64{third}) {
		t.Errorf("attachments left: %v, want only %d", got, third)
	}
	for _, id := range []int64{first, second} {
		if status, _ := paul.do("DELETE", path(id), nil); status != 404 {
			t.Errorf("deleting attachment %d twice: %d, want 404", id, status)
		}
	}
	if got := historyFields(t, paul, issue)["Attachment"]; got != 5 {
		t.Errorf("history lines of the attachments: %d, want 3 added and 2 removed", got)
	}
}
