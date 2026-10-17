package tests

// Remote links, Jira's session route and groups of the REST API.

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// Remote links -----------------------------------------------------------------

func TestRESTRemoteLinks(t *testing.T) {
	needDB(t)
	key := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	path := "/rest/api/2/issue/" + key + "/remotelink"
	paul := login(t, "paul.martin", "motdepasse")
	do := func(who *client, method, p string, body any) (int, []byte) {
		status, _, raw := call(t, who, testURL, method, p, body, header("X-Atlassian-Token", "no-check"))
		return status, raw
	}
	link := func(url, title string) map[string]any {
		return map[string]any{"object": map[string]any{"url": url, "title": title}}
	}

	status, raw := do(paul, "POST", path, map[string]any{
		"globalId":     "gitlab:mr:7",
		"application":  map[string]string{"type": "com.gitlab", "name": "GitLab"},
		"relationship": "implemented by",
		"object":       map[string]any{"url": "https://git.example.com/mr/7", "title": "Merge request 7", "summary": "ignored"},
	})
	if status != 201 {
		t.Fatalf("create: %d %s", status, raw)
	}
	id := num(obj(t, raw), "id")
	one := path + "/" + strconv.Itoa(id)
	if id == 0 || !strings.HasSuffix(str(obj(t, raw), "self"), one) {
		t.Fatalf("create answer: %s", raw)
	}

	status, raw = do(paul, "GET", one, nil)
	got := obj(t, raw)
	if status != 200 || num(got, "id") != id || str(got, "relationship") != "implemented by" ||
		str(got, "object", "url") != "https://git.example.com/mr/7" || str(got, "object", "title") != "Merge request 7" {
		t.Fatalf("get: %d %s", status, raw)
	}
	var list []map[string]any
	status, raw = do(paul, "GET", path, nil)
	if err := json.Unmarshal(raw, &list); err != nil || status != 200 {
		t.Fatalf("list: %d %s", status, raw)
	}
	found := false
	for _, l := range list {
		found = found || num(l, "id") == id
	}
	if !found {
		t.Errorf("list lacks the new link: %s", raw)
	}
	// The interface shows it too.
	var d struct {
		RemoteLinks []struct {
			ID int `json:"id"`
		} `json:"remoteLinks"`
	}
	paul.get("/api/issues/"+key, &d)
	found = false
	for _, l := range d.RemoteLinks {
		found = found || l.ID == id
	}
	if !found {
		t.Errorf("the interface lacks the remote link: %+v", d.RemoteLinks)
	}

	// Update replaces everything.
	if status, raw = do(paul, "PUT", one, link("mailto:team@example.com", "Team")); status != 204 {
		t.Fatalf("update: %d %s", status, raw)
	}
	_, raw = do(paul, "GET", one, nil)
	got = obj(t, raw)
	if str(got, "object", "url") != "mailto:team@example.com" || str(got, "object", "title") != "Team" || got["relationship"] != nil {
		t.Errorf("after update: %s", raw)
	}

	// Refused: other schemes, missing parts, unknown links, links of other issues.
	for name, body := range map[string]map[string]any{
		"javascript":       link("javascript:alert(1)", "x"),
		"relative address": link("/browse/DEMO-1", "x"),
		"no scheme":        link("example.com", "x"),
		"data":             link("data:text/html,x", "x"),
		"no address":       link("", "x"),
		"no title":         link("https://example.com", ""),
		"no object":        {"relationship": "x"},
		"title too long":   link("https://example.com", strings.Repeat("é", 256)),
	} {
		if status, raw := do(paul, "POST", path, body); status != 400 {
			t.Errorf("create, %s: %d %s", name, status, raw)
		}
		if status, raw := do(paul, "PUT", one, body); status != 400 {
			t.Errorf("update, %s: %d %s", name, status, raw)
		}
	}
	if status, raw := do(paul, "POST", path, link("javascript:x", "x")); status != 400 ||
		str(obj(t, raw), "errors", "object.url") == "" {
		t.Errorf("error format: %d %s", status, raw)
	}
	if status, _ := do(paul, "GET", path+"/999999999", nil); status != 404 {
		t.Errorf("unknown link: %d", status)
	}
	if status, _ := do(paul, "GET", path+"/abc", nil); status != 404 {
		t.Errorf("link id not a number: %d", status)
	}
	other := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' AND i.security_level_id IS NULL AND `+keyExpr+` <> ? ORDER BY i.num LIMIT 1`, key)
	if status, _ := do(paul, "GET", "/rest/api/2/issue/"+other+"/remotelink/"+strconv.Itoa(id), nil); status != 404 {
		t.Errorf("link read through another issue: %d", status)
	}
	// Links are not stored with a globalId: asking for one is refused, not answered wrongly.
	if status, _ := do(paul, "GET", path+"?globalId=gitlab:mr:7", nil); status != 400 {
		t.Errorf("GET by globalId: %d", status)
	}
	if status, _ := do(paul, "DELETE", path+"?globalId=gitlab:mr:7", nil); status != 400 {
		t.Errorf("DELETE by globalId: %d", status)
	}

	// Who may do what: sign-in, forgery protection, visibility, editing.
	if status, _, _ := call(t, nil, testURL, "GET", path, nil, nil); status != 401 {
		t.Errorf("anonymous list: %d", status)
	}
	if status, _, _ := call(t, paul, testURL, "DELETE", one, nil, nil); status != 403 {
		t.Errorf("DELETE with a cookie and no header: %d", status)
	}
	hidden := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	chloe := login(t, "chloe.simon", "motdepasse")
	if status, raw := do(chloe, "POST", "/rest/api/2/issue/"+hidden+"/remotelink", link("https://example.com", "RH")); status != 201 {
		t.Fatalf("chloe.simon on %s: %d %s", hidden, status, raw)
	}
	rhLink := pick(t, `SELECT MAX(r.id) FROM remote_links r JOIN issues i ON i.id = r.issue_id JOIN projects p ON p.id = i.project_id
		WHERE `+keyExpr+` = ?`, hidden)
	for _, c := range []struct{ method, p string }{
		{"GET", "/rest/api/2/issue/" + hidden + "/remotelink"},
		{"GET", "/rest/api/2/issue/" + hidden + "/remotelink/" + rhLink},
		{"PUT", "/rest/api/2/issue/" + hidden + "/remotelink/" + rhLink},
		{"DELETE", "/rest/api/2/issue/" + hidden + "/remotelink/" + rhLink},
	} {
		if status, _ := do(paul, c.method, c.p, link("https://example.com", "x")); status != 404 {
			t.Errorf("paul.martin %s %s: %d, want 404", c.method, c.p, status)
		}
	}
	viewer := viewerOf(t, key)
	if status, _ := do(viewer, "POST", path, link("https://example.com", "x")); status != 403 {
		t.Errorf("a user who cannot edit %s created a link: %d, want 403", key, status)
	}
	if status, _ := do(viewer, "PUT", one, link("https://example.com", "x")); status != 403 {
		t.Errorf("a user who cannot edit %s changed a link: %d, want 403", key, status)
	}
	if status, _ := do(viewer, "DELETE", one, nil); status != 403 {
		t.Errorf("a user who cannot edit %s deleted a link: %d, want 403", key, status)
	}
	if status, _ := do(viewer, "GET", one, nil); status != 200 {
		t.Errorf("a user who sees %s reads its links: %d, want 200", key, status)
	}

	// The history records each change, as it does for issue links.
	_, raw = do(paul, "GET", "/rest/api/2/issue/"+key+"?expand=changelog&fields=summary", nil)
	var history []string
	for _, h := range at(obj(t, raw), "changelog", "histories").([]any) {
		for _, it := range at(h, "items").([]any) {
			if str(it, "field") == "RemoteIssueLink" {
				history = append(history, str(it, "fromString")+"|"+str(it, "toString"))
			}
		}
	}
	for _, want := range []string{`|This issue links to "Merge request 7"`,
		`This issue links to "Merge request 7"|This issue links to "Team"`} {
		if !strings.Contains(strings.Join(history, "\n"), want) {
			t.Errorf("history lacks %q: %q", want, history)
		}
	}

	if status, raw := do(paul, "DELETE", one, nil); status != 204 {
		t.Fatalf("delete: %d %s", status, raw)
	}
	if status, _ := do(paul, "GET", one, nil); status != 404 {
		t.Errorf("deleted link: %d", status)
	}
	if status, _ := do(paul, "DELETE", one, nil); status != 404 {
		t.Errorf("delete twice: %d", status)
	}
	if n := pick(t, `SELECT COUNT(*) FROM change_items WHERE field = 'RemoteIssueLink' AND old_value = ? AND new_value IS NULL`,
		strconv.Itoa(id)); n != "1" {
		t.Errorf("deletion in the history: %s", n)
	}
}

// viewerOf signs in a new account, which sees the issue (through a grant to
// everyone) without editing it.
func viewerOf(t *testing.T, key string) *client {
	t.Helper()
	const password = "viewer passphrase 2026"
	name := "links-viewer-" + strings.ToLower(key)
	login(t, "admin", "admin").call("POST", "/api/admin/users", map[string]any{"username": name,
		"name": "Viewer", "email": name + "@example.com", "password": password}, 201, nil)
	c := login(t, name, password)
	_, _, raw := call(t, c, testURL, "GET", "/rest/api/2/mypermissions?issueKey="+key, nil, nil)
	if at(obj(t, raw), "permissions", "BROWSE_PROJECTS", "havePermission") != true ||
		at(obj(t, raw), "permissions", "EDIT_ISSUES", "havePermission") != false {
		t.Fatalf("a new account on %s: %s", key, raw)
	}
	return c
}
