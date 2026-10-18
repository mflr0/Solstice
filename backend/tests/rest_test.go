package tests

// The Jira-compatible REST API and personal access tokens.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Sign-in ----------------------------------------------------------------------

func TestTokenLifecycle(t *testing.T) {
	allowTokensForEveryone(t)
	paul := login(t, "paul.martin", "motdepasse")
	status, body := paul.do("POST", "/api/me/tokens", map[string]any{"name": "CI nocturne", "expires": "2099-12-31"})
	if status != 201 {
		t.Fatalf("create token: %d %s", status, body)
	}
	created := obj(t, body)
	token := str(created, "token")
	if !strings.HasPrefix(token, "sol_") || len(token) < 40 || str(created, "expires") == "" {
		t.Fatalf("token answer: %s", body)
	}
	// Only a hash is stored.
	if n := pick(t, `SELECT COUNT(*) FROM api_tokens WHERE token_hash = ? OR name = ?`, token, token); n != "0" {
		t.Fatal("the token is stored in clear")
	}

	for name, setup := range map[string]func(*http.Request){
		"bearer":           bearer(token),
		"basic + token":    basic("paul.martin", token),
		"basic + password": basic("paul.martin", "motdepasse"),
	} {
		status, _, body := call(t, nil, testURL, "GET", "/rest/api/2/myself", nil, setup)
		if status != 200 || str(obj(t, body), "name") != "paul.martin" {
			t.Errorf("%s: %d %s", name, status, body)
		}
	}
	// The interface's API takes tokens too.
	if status, _, body := call(t, nil, testURL, "GET", "/api/me", nil, bearer(token)); status != 200 ||
		str(obj(t, body), "username") != "paul.martin" {
		t.Errorf("bearer on /api/me: %d %s", status, body)
	}
	// A token makes no other token.
	if status, _, _ := call(t, nil, testURL, "POST", "/api/me/tokens", map[string]any{"name": "x"},
		func(r *http.Request) { bearer(token)(r); r.Header.Set(csrfHeader, "1") }); status != 403 {
		t.Errorf("token created a token: %d", status)
	}

	var list []tokenOut
	paul.get("/api/me/tokens", &list)
	i := slices.IndexFunc(list, func(t tokenOut) bool { return t.Name == "CI nocturne" })
	if i < 0 || list[i].LastUsed == nil || list[i].Token != "" || list[i].Expired {
		t.Fatalf("token list: %+v", list)
	}
	// Someone else cannot revoke it; its owner can.
	if status, _ := login(t, "hugo.robert", "motdepasse").do("DELETE", fmt.Sprintf("/api/me/tokens/%d", list[i].ID), nil); status != 404 {
		t.Errorf("another user revoked the token: %d", status)
	}
	if status, _ := paul.do("DELETE", fmt.Sprintf("/api/me/tokens/%d", list[i].ID), nil); status != 204 {
		t.Fatalf("revoke: %d", status)
	}
	status, h, body := call(t, nil, testURL, "GET", "/rest/api/2/myself", nil, bearer(token))
	if status != 401 || h.Get("X-Seraph-LoginReason") != "AUTHENTICATED_FAILED" || len(obj(t, body)["errorMessages"].([]any)) != 1 {
		t.Errorf("revoked token: %d %v %s", status, h, body)
	}

	// Expired tokens are refused; inactive accounts' tokens too.
	past := time.Now().Add(-time.Hour)
	if status, _, _ := call(t, nil, testURL, "GET", "/rest/api/2/myself", nil, bearer(newToken(t, "paul.martin", &past))); status != 401 {
		t.Errorf("expired token: %d", status)
	}
	if status, _, _ := call(t, nil, testURL, "GET", "/rest/api/2/myself", nil, bearer(newToken(t, "emma.morel", nil))); status != 401 {
		t.Errorf("deactivated user's token: %d", status)
	}
	if status, body := paul.do("POST", "/api/me/tokens", map[string]any{"name": "x", "expires": "2001-01-01"}); status != 400 {
		t.Errorf("token expiring in the past: %d %s", status, body)
	}

	// Administrators see and revoke everyone's tokens.
	other := newToken(t, "hugo.robert", nil)
	admin := login(t, "admin", "admin")
	var all []tokenOut
	if status := admin.get("/api/admin/tokens?user=hugo.robert", &all); status != 200 || len(all) == 0 || all[0].User == nil ||
		all[0].User.Username != "hugo.robert" {
		t.Fatalf("admin token list: %d %+v", status, all)
	}
	if status := paul.get("/api/admin/tokens", nil); status != 403 {
		t.Errorf("non-admin token list: %d", status)
	}
	if status, _ := admin.do("DELETE", fmt.Sprintf("/api/admin/tokens/%d", all[0].ID), nil); status != 204 {
		t.Errorf("admin revoke: %d", status)
	}
	if status, _, _ := call(t, nil, testURL, "GET", "/rest/api/2/myself", nil, bearer(other)); status != 401 {
		t.Errorf("token revoked by an admin still works: %d", status)
	}
}

func TestRESTBasicAuthThrottled(t *testing.T) {
	base := ownServer(t)
	token := newToken(t, "lucas.bernard", nil)
	for i := range 5 {
		if status, _, _ := call(t, nil, base, "GET", "/rest/api/2/myself", nil, basic("lucas.bernard", "faux")); status != 401 {
			t.Fatalf("attempt %d: %d, want 401", i+1, status)
		}
	}
	status, h, _ := call(t, nil, base, "GET", "/rest/api/2/myself", nil, basic("lucas.bernard", "motdepasse"))
	if status != http.StatusTooManyRequests || h.Get("X-Seraph-LoginReason") != "AUTHENTICATION_DENIED" {
		t.Fatalf("right password after 5 failures: %d %v", status, h)
	}
	// Someone guessing the password does not block the account's scripts.
	for _, setup := range []func(*http.Request){bearer(token), basic("lucas.bernard", token)} {
		if status, _, _ := call(t, nil, base, "GET", "/rest/api/2/myself", nil, setup); status != 200 {
			t.Errorf("token during a lock: %d", status)
		}
	}
	// A token is only good for its owner's name (here the address is
	// locked by now: 429).
	if status, _, _ := call(t, nil, base, "GET", "/rest/api/2/myself", nil, basic("hugo.robert", token)); status == 200 {
		t.Errorf("token under another name: %d", status)
	}
}

func TestRESTForgeryProtection(t *testing.T) {
	key := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	path := "/rest/api/2/issue/" + key + "/comment"
	comment := map[string]string{"body": "Vérifié."}
	paul := login(t, "paul.martin", "motdepasse")
	token := newToken(t, "paul.martin", nil)
	textPlain := func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }
	for _, c := range []struct {
		name  string
		user  *client
		setup func(*http.Request)
		want  int
	}{
		{"cookie without header", paul, nil, 403},
		{"cookie with X-Atlassian-Token", paul, header("X-Atlassian-Token", "no-check"), 201},
		{"cookie with X-Solstice", paul, header(csrfHeader, "1"), 201},
		{"bearer", nil, bearer(token), 201},
		{"basic, JSON", nil, basic("paul.martin", "motdepasse"), 201},
		{"basic, form-like body", nil, func(r *http.Request) { basic("paul.martin", "motdepasse")(r); textPlain(r) }, 403},
		{"basic, form-like body with header", nil, func(r *http.Request) {
			basic("paul.martin", "motdepasse")(r)
			textPlain(r)
			r.Header.Set("X-Atlassian-Token", "no-check")
		}, 201},
	} {
		status, _, body := call(t, c.user, testURL, "POST", path, comment, c.setup)
		if status != c.want {
			t.Errorf("%s: %d, want %d: %s", c.name, status, c.want, body)
		}
	}
	// Reading needs no header.
	if status, _, _ := call(t, paul, testURL, "GET", path, nil, nil); status != 200 {
		t.Errorf("GET with cookie: %d", status)
	}
	// Without any credentials: 401 in Jira's format.
	status, h, body := call(t, nil, testURL, "GET", "/rest/api/2/issue/"+key, nil, nil)
	if status != 401 || h.Get("X-AUSERNAME") != "anonymous" || at(obj(t, body), "errorMessages", 0) == nil {
		t.Errorf("anonymous: %d %s", status, body)
	}
}

// Who sees what -----------------------------------------------------------------

func TestRESTPermissionParity(t *testing.T) {
	paul, chloe := login(t, "paul.martin", "motdepasse"), login(t, "chloe.simon", "motdepasse")
	rhKey := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	restricted := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN security_levels l ON l.id = i.security_level_id JOIN users u ON u.id = i.reporter_id
		WHERE l.name = 'Restreint' AND u.username NOT IN ('paul.martin', 'marie.durand') AND u.active
		ORDER BY i.num LIMIT 1`)
	for _, c := range []struct {
		who  *client
		name string
		key  string
		want int
	}{{paul, "paul", rhKey, 404}, {chloe, "chloe", rhKey, 200}, {paul, "paul", restricted, 404}} {
		if status, _, _ := call(t, c.who, testURL, "GET", "/rest/api/2/issue/"+c.key, nil, nil); status != c.want {
			t.Errorf("%s GET %s: %d, want %d", c.name, c.key, status, c.want)
		}
	}

	// Searches find exactly what the interface finds.
	for _, who := range []*client{paul, chloe, login(t, "admin", "admin")} {
		for _, q := range []string{``, `project = RH`, `key = ` + restricted, `project in (DEMO, WEB) AND statusCategory != Done`,
			`labels = backend ORDER BY key`} {
			var ui struct{ Total int }
			if status := who.get("/api/issues?jql="+url.QueryEscape(q), &ui); status != 200 && q != "" {
				t.Fatalf("UI search %q: %d", q, status)
			}
			if q == "" {
				who.get("/api/issues", &ui)
			}
			status, _, body := call(t, who, testURL, "GET", "/rest/api/2/search?maxResults=0&jql="+url.QueryEscape(q), nil, nil)
			if status != 200 || num(obj(t, body), "total") != ui.Total {
				t.Errorf("search %q: REST %d %s, UI %d", q, status, body, ui.Total)
			}
		}
	}

	// Restricted comments: the same ones as in the interface, through every way in.
	key := pick(t, `SELECT `+keyExpr+` FROM comments c JOIN issues i ON i.id = c.issue_id
		JOIN projects p ON p.id = i.project_id
		WHERE c.visibility_group = 'developpeurs' AND i.security_level_id IS NULL AND p.pkey <> 'RH'
		ORDER BY c.id LIMIT 1`)
	for _, who := range []*client{paul, chloe} {
		var ui []commentOut
		who.get("/api/issues/"+key+"/comments", &ui)
		_, _, list := call(t, who, testURL, "GET", "/rest/api/2/issue/"+key+"/comment", nil, nil)
		_, _, issue := call(t, who, testURL, "GET", "/rest/api/2/issue/"+key+"?fields=comment", nil, nil)
		_, _, found := call(t, who, testURL, "GET", "/rest/api/2/search?fields=comment&jql="+url.QueryEscape("key = "+key), nil, nil)
		if n := num(obj(t, list), "total"); n != len(ui) {
			t.Errorf("comment list: %d, UI %d", n, len(ui))
		}
		if n := num(obj(t, issue), "fields", "comment", "total"); n != len(ui) {
			t.Errorf("issue comments: %d, UI %d", n, len(ui))
		}
		if n := num(obj(t, found), "issues", 0, "fields", "comment", "total"); n != len(ui) {
			t.Errorf("search comments: %d, UI %d", n, len(ui))
		}
		if who == chloe && strings.Contains(string(list)+string(issue)+string(found), `"developpeurs"`) {
			t.Errorf("chloe.simon reads a comment restricted to developpeurs on %s", key)
		}
	}

	// A link to an issue one may not see is not shown, nor named in the history.
	visible := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'WEB' AND i.security_level_id IS NULL ORDER BY i.num DESC LIMIT 1 OFFSET 1`)
	hidden := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL ORDER BY i.num DESC LIMIT 1 OFFSET 1`)
	if status, _, body := call(t, chloe, testURL, "POST", "/rest/api/2/issueLink", map[string]any{
		"type":        map[string]string{"name": pick(t, `SELECT name FROM link_types ORDER BY position LIMIT 1`)},
		"inwardIssue": map[string]string{"key": visible}, "outwardIssue": map[string]string{"key": hidden},
	}, header("X-Atlassian-Token", "no-check")); status != 201 {
		t.Fatalf("link: %d %s", status, body)
	}
	_, _, raw := call(t, paul, testURL, "GET", "/rest/api/2/issue/"+visible+"?expand=changelog", nil, nil)
	if strings.Contains(string(raw), hidden) {
		t.Errorf("paul.martin's view of %s mentions %s", visible, hidden)
	}
	_, _, raw = call(t, chloe, testURL, "GET", "/rest/api/2/issue/"+visible+"?fields=issuelinks", nil, nil)
	if str(obj(t, raw), "fields", "issuelinks", 0, "outwardIssue", "key") != hidden {
		t.Errorf("chloe.simon's links of %s: %s", visible, raw)
	}
	// Nor can it be deleted by someone who does not see both ends.
	linkID := str(obj(t, raw), "fields", "issuelinks", 0, "id")
	if status, _, _ := call(t, paul, testURL, "DELETE", "/rest/api/2/issueLink/"+linkID, nil,
		header("X-Atlassian-Token", "no-check")); status != 404 {
		t.Errorf("paul.martin deleted a link to %s: %d", hidden, status)
	}

	// Projects and boards follow the same rules.
	_, _, raw = call(t, paul, testURL, "GET", "/rest/api/2/project", nil, nil)
	if strings.Contains(string(raw), `"key":"RH"`) || !strings.Contains(string(raw), `"key":"DEMO"`) {
		t.Errorf("paul.martin's projects: %s", raw)
	}
	if status, _, _ := call(t, paul, testURL, "GET", "/rest/api/2/project/RH", nil, nil); status != 404 {
		t.Errorf("paul.martin GET project RH: %d", status)
	}
	if status, _, _ := call(t, paul, testURL, "GET", "/rest/agile/1.0/board/4", nil, nil); status != 404 {
		t.Errorf("paul.martin GET the RH board: %d", status)
	}
}

// Shapes -----------------------------------------------------------------------

var jiraTime = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}[+-]\d{4}$`)

func TestRESTShapes(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	get := func(path string) any {
		t.Helper()
		status, _, body := call(t, paul, testURL, "GET", path, nil, nil)
		if status != 200 {
			t.Fatalf("GET %s: %d %s", path, status, body)
		}
		var v any
		json.Unmarshal(body, &v)
		return v
	}

	status, _, body := call(t, nil, testURL, "GET", "/rest/api/2/serverInfo", nil, nil)
	info := obj(t, body)
	if status != 200 || info["deploymentType"] != "Server" || info["serverTitle"] != "Solstice" ||
		len(at(info, "versionNumbers").([]any)) != 3 || str(info, "baseUrl") != testURL {
		t.Errorf("serverInfo: %d %s", status, body)
	}

	key := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' AND i.security_level_id IS NULL AND i.parent_id IS NULL AND i.description IS NOT NULL
		AND EXISTS (SELECT 1 FROM custom_field_values v WHERE v.issue_id = i.id AND v.field_id = 10100)
		ORDER BY i.num LIMIT 1`)
	id := pick(t, `SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id WHERE `+keyExpr+` = ?`, key)
	issue := get("/rest/api/2/issue/" + key + "?expand=changelog,renderedFields,names,transitions")
	if str(issue, "id") != id || str(issue, "key") != key || str(issue, "self") != testURL+"/rest/api/2/issue/"+id {
		t.Errorf("issue identity: %v %v %v", at(issue, "id"), at(issue, "key"), at(issue, "self"))
	}
	f := at(issue, "fields")
	for _, path := range [][]any{{"summary"}, {"issuetype", "name"}, {"issuetype", "id"}, {"project", "key"},
		{"status", "name"}, {"status", "statusCategory", "key"}, {"priority", "name"}, {"reporter", "name"},
		{"reporter", "displayName"}, {"reporter", "key"}, {"reporter", "avatarUrls", "48x48"}, {"description"},
		{"watches", "self"}} {
		if str(f, path...) == "" {
			t.Errorf("fields.%v is missing", path)
		}
	}
	picture := testURL + "/api/avatars/" + strconv.FormatInt(userID(t, str(f, "reporter", "name")), 10)
	for _, size := range []string{"48x48", "32x32", "24x24", "16x16"} {
		if got := str(f, "reporter", "avatarUrls", size); got != picture {
			t.Errorf("reporter avatarUrls[%s] = %q, want %q", size, got, picture)
		}
	}
	for _, k := range []string{"created", "updated"} {
		if !jiraTime.MatchString(str(f, k)) {
			t.Errorf("%s = %q, not Jira's date format", k, str(f, k))
		}
	}
	if !slices.Contains([]string{"new", "indeterminate", "done"}, str(f, "status", "statusCategory", "key")) {
		t.Errorf("statusCategory: %v", at(f, "status", "statusCategory"))
	}
	if _, ok := at(f, "customfield_10100").(float64); !ok {
		t.Errorf("Story Points: %v", at(f, "customfield_10100"))
	}
	if _, ok := at(f, "labels").([]any); !ok {
		t.Errorf("labels: %v", at(f, "labels"))
	}
	if at(issue, "changelog", "histories") == nil || at(issue, "names", "summary") == nil ||
		!strings.Contains(str(issue, "renderedFields", "description"), "<") || at(issue, "transitions") == nil {
		t.Errorf("expansions: changelog %v, names %v, rendered %v", at(issue, "changelog") != nil, at(issue, "names") != nil,
			at(issue, "renderedFields"))
	}

	// Selected fields only.
	only := get("/rest/api/2/issue/" + key + "?fields=summary,status")
	if fields := at(only, "fields").(map[string]any); len(fields) != 2 || fields["summary"] == nil {
		t.Errorf("fields=summary,status gave %v", fields)
	}
	minus := get("/rest/api/2/issue/" + key + "?fields=-comment,-description")
	if fields := at(minus, "fields").(map[string]any); fields["comment"] != nil || fields["description"] != nil || fields["summary"] == nil {
		t.Errorf("fields=-comment,-description gave %v", fields)
	}

	// Search's envelope.
	page := get("/rest/api/2/search?jql=" + url.QueryEscape("project = DEMO ORDER BY key ASC") + "&startAt=2&maxResults=3")
	if num(page, "startAt") != 2 || num(page, "maxResults") != 3 || num(page, "total") < 5 || len(at(page, "issues").([]any)) != 3 {
		t.Errorf("search envelope: %v", page)
	}
	if at(page, "issues", 0, "fields", "comment") != nil {
		t.Error("search shows comments by default (not navigable in Jira)")
	}
	status, _, body = call(t, paul, testURL, "POST", "/rest/api/2/search", map[string]any{
		"jql": "project = DEMO", "maxResults": 2000, "fields": []string{"summary"}}, header("X-Atlassian-Token", "no-check"))
	if p := obj(t, body); status != 200 || num(p, "maxResults") != 1000 || len(at(p, "issues", 0, "fields").(map[string]any)) != 1 {
		t.Errorf("POST search: %d maxResults=%v", status, at(p, "maxResults"))
	}
	status, _, body = call(t, paul, testURL, "GET", "/rest/api/2/search?jql="+url.QueryEscape(`project = "DEMO`), nil, nil)
	if e := obj(t, body); status != 400 || !strings.Contains(str(e, "errorMessages", 0), "JQL") || e["errors"] == nil {
		t.Errorf("JQL error: %d %s", status, body)
	}

	// Catalogues and metadata.
	for _, path := range []string{"/rest/api/2/priority", "/rest/api/2/status", "/rest/api/2/issuetype",
		"/rest/api/2/resolution", "/rest/api/2/field", "/rest/api/2/project", "/rest/api/2/project/DEMO/statuses",
		"/rest/api/2/project/DEMO/versions", "/rest/api/2/project/DEMO/components", "/rest/api/2/user/search?username=mar",
		"/rest/api/2/user/assignable/search?project=DEMO", "/rest/api/2/filter/favourite", "/rest/api/latest/priority"} {
		if list, ok := get(path).([]any); !ok || (len(list) == 0 && !strings.Contains(path, "favourite") &&
			!strings.Contains(path, "components")) {
			t.Errorf("GET %s: %v", path, list)
		}
	}
	fields := get("/rest/api/2/field").([]any)
	if !slices.ContainsFunc(fields, func(v any) bool { return str(v, "id") == "customfield_10100" && str(v, "schema", "type") == "number" }) {
		t.Error("/field lacks Story Points")
	}
	project := get("/rest/api/2/project/DEMO")
	if str(project, "key") != "DEMO" || len(at(project, "issueTypes").([]any)) == 0 || str(project, "lead", "name") == "" {
		t.Errorf("project: %v", project)
	}
	user := get("/rest/api/2/user?username=marie.durand")
	if str(user, "name") != "marie.durand" || str(user, "displayName") == "" || num(user, "groups", "size") != 0 {
		t.Errorf("user (groups are only shown to oneself and administrators): %v", user)
	}
	if me := get("/rest/api/2/myself"); num(me, "groups", "size") == 0 {
		t.Errorf("myself without groups: %v", me)
	}
	// Absurd paging is not an error.
	if c := get("/rest/api/2/issue/" + key + "/comment?startAt=9223372036854775807&maxResults=1"); len(at(c, "comments").([]any)) != 0 {
		t.Errorf("comments far past the end: %v", c)
	}
	perms := get("/rest/api/2/mypermissions?projectKey=DEMO")
	if at(perms, "permissions", "BROWSE_PROJECTS", "havePermission") != true || at(perms, "permissions", "ADMINISTER", "havePermission") != false {
		t.Errorf("mypermissions: %v", at(perms, "permissions", "BROWSE_PROJECTS"))
	}
	if types := get("/rest/api/2/issueLinkType"); len(at(types, "issueLinkTypes").([]any)) == 0 {
		t.Error("no link types")
	}

	// Errors in Jira's format, translated.
	status, _, body = call(t, paul, testURL, "GET", "/rest/api/2/nothing/here", nil, header("Accept-Language", "en"))
	if e := obj(t, body); status != 404 || str(e, "errorMessages", 0) != "Unknown address." {
		t.Errorf("unknown endpoint: %d %s", status, body)
	}
	status, _, body = call(t, paul, testURL, "GET", "/rest/api/2/issue/NOPE-1", nil, header("Accept-Language", "de"))
	if e := obj(t, body); status != 404 || !strings.Contains(str(e, "errorMessages", 0), "Nicht gefunden") {
		t.Errorf("unknown issue: %d %s", status, body)
	}
}

// Round trips --------------------------------------------------------------------

func TestRESTIssueRoundTrip(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	token := newToken(t, "paul.martin", nil)
	do := func(method, path string, body any) (int, map[string]any) {
		t.Helper()
		status, _, raw := call(t, nil, testURL, method, path, body, bearer(token))
		var m map[string]any
		json.Unmarshal(raw, &m)
		if status >= 400 {
			t.Logf("%s %s: %d %s", method, path, status, raw)
		}
		return status, m
	}
	task := pick(t, `SELECT it.name FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND NOT it.subtask AND it.name NOT IN ('Epic', 'Épopée')
		ORDER BY t.position LIMIT 1`)
	option := pick(t, `SELECT value FROM custom_field_options WHERE field_id = 10101 AND parent_id IS NULL AND NOT disabled LIMIT 1`)

	status, created := do("POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
		"project": map[string]string{"key": "DEMO"}, "issuetype": map[string]string{"name": task},
		"summary": "Export PDF tronqué", "description": "h2. Constat\nLa *dernière* page manque.",
		"labels": []string{"export", "pdf"}, "priority": map[string]string{"name": pick(t, `SELECT name FROM priorities ORDER BY position LIMIT 1`)},
		"assignee": map[string]string{"name": "paul.martin"}, "duedate": "2026-12-01",
		"customfield_10100": 5, "customfield_10101": map[string]string{"value": option},
		"customfield_10108": "CLI-42", "customfield_10109": []string{"q4"},
	}})
	if status != 201 || str(created, "key") == "" || !strings.HasSuffix(str(created, "self"), "/rest/api/2/issue/"+str(created, "id")) {
		t.Fatalf("create: %d %v", status, created)
	}
	key := str(created, "key")
	_, issue := do("GET", "/rest/api/2/issue/"+key, nil)
	f := at(issue, "fields")
	if str(f, "summary") != "Export PDF tronqué" || str(f, "assignee", "name") != "paul.martin" || str(f, "duedate") != "2026-12-01" ||
		at(f, "customfield_10100") != 5.0 || str(f, "customfield_10101", "value") != option || str(f, "customfield_10108") != "CLI-42" ||
		len(at(f, "labels").([]any)) != 2 || str(f, "customfield_10109", 0) != "q4" || str(f, "reporter", "name") != "paul.martin" {
		t.Fatalf("created issue: %v", f)
	}

	// Edit: fields and update operations.
	if status, _ := do("PUT", "/rest/api/2/issue/"+key, map[string]any{
		"fields": map[string]any{"summary": "Export PDF : dernière page tronquée", "customfield_10100": 8},
		"update": map[string]any{"labels": []any{map[string]string{"add": "urgent"}, map[string]string{"remove": "pdf"}},
			"comment": []any{map[string]any{"add": map[string]string{"body": "Reproduit avec 3 pages."}}}},
	}); status != 204 {
		t.Fatalf("edit: %d", status)
	}
	_, issue = do("GET", "/rest/api/2/issue/"+key+"?fields=summary,labels,customfield_10100,comment", nil)
	f = at(issue, "fields")
	labels := at(f, "labels").([]any)
	if str(f, "summary") != "Export PDF : dernière page tronquée" || at(f, "customfield_10100") != 8.0 ||
		!slices.Contains(labels, any("urgent")) || slices.Contains(labels, any("pdf")) || num(f, "comment", "total") != 1 {
		t.Fatalf("edited issue: %v", f)
	}
	if status, e := do("PUT", "/rest/api/2/issue/"+key, map[string]any{"fields": map[string]any{"couleur": "bleu"}}); status != 400 ||
		at(e, "errors", "couleur") == nil {
		t.Errorf("unknown field: %d %v", status, e)
	}
	if status, e := do("POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
		"project": map[string]string{"key": "DEMO"}, "issuetype": map[string]string{"name": "Licorne"}, "summary": "x"}}); status != 400 ||
		at(e, "errors", "issuetype") == nil {
		t.Errorf("unknown issue type: %d %v", status, e)
	}

	// Transitions.
	_, tr := do("GET", "/rest/api/2/issue/"+key+"/transitions?expand=transitions.fields", nil)
	var done string
	for _, v := range at(tr, "transitions").([]any) {
		if str(v, "to", "statusCategory", "key") == "done" {
			done = str(v, "id")
		}
	}
	if done == "" {
		t.Fatalf("no transition to done: %v", tr)
	}
	resolution := pick(t, `SELECT name FROM resolutions ORDER BY position DESC LIMIT 1`)
	if status, _ := do("POST", "/rest/api/2/issue/"+key+"/transitions", map[string]any{
		"transition": map[string]string{"id": done}, "fields": map[string]any{"resolution": map[string]string{"name": resolution}},
	}); status != 204 {
		t.Fatalf("transition: %d", status)
	}
	_, issue = do("GET", "/rest/api/2/issue/"+key+"?fields=status,resolution,resolutiondate", nil)
	if str(issue, "fields", "status", "statusCategory", "key") != "done" || str(issue, "fields", "resolution", "name") != resolution ||
		!jiraTime.MatchString(str(issue, "fields", "resolutiondate")) {
		t.Fatalf("after transition: %v", at(issue, "fields"))
	}
	if status, _ := do("POST", "/rest/api/2/issue/"+key+"/transitions", map[string]any{"transition": map[string]string{"id": "999999"}}); status != 400 {
		t.Errorf("unknown transition: %d", status)
	}

	// Comments.
	status, c := do("POST", "/rest/api/2/issue/"+key+"/comment", map[string]any{"body": "Corrigé en 2.4.1.",
		"visibility": map[string]string{"type": "group", "value": "developpeurs"}})
	if status != 201 || str(c, "body") != "Corrigé en 2.4.1." || str(c, "visibility", "value") != "developpeurs" ||
		str(c, "author", "name") != "paul.martin" {
		t.Fatalf("comment: %d %v", status, c)
	}
	cid := str(c, "id")
	if status, c := do("PUT", "/rest/api/2/issue/"+key+"/comment/"+cid, map[string]any{"body": "Corrigé en 2.4.2."}); status != 200 ||
		str(c, "body") != "Corrigé en 2.4.2." {
		t.Errorf("edit comment: %d %v", status, c)
	}
	if status, _, _ := call(t, login(t, "chloe.simon", "motdepasse"), testURL, "GET", "/rest/api/2/issue/"+key+"/comment/"+cid, nil, nil); status != 404 {
		t.Errorf("chloe.simon reads a comment restricted to developpeurs: %d", status)
	}
	if status, _ := do("DELETE", "/rest/api/2/issue/"+key+"/comment/"+cid, nil); status != 204 {
		t.Errorf("delete comment: %d", status)
	}

	// Work logs.
	status, wl := do("POST", "/rest/api/2/issue/"+key+"/worklog", map[string]any{"timeSpent": "1h 30m", "comment": "Analyse",
		"started": "2026-10-14T09:00:00.000+0200"})
	if status != 201 || num(wl, "timeSpentSeconds") != 5400 || str(wl, "timeSpent") != "1h 30m" ||
		!strings.HasPrefix(str(wl, "started"), "2026-10-14T07:00:00.000") {
		t.Fatalf("worklog: %d %v", status, wl)
	}
	if _, list := do("GET", "/rest/api/2/issue/"+key+"/worklog", nil); num(list, "total") != 1 {
		t.Errorf("worklogs: %v", list)
	}

	// Watchers and votes.
	if _, w := do("GET", "/rest/api/2/issue/"+key+"/watchers", nil); at(w, "isWatching") != true || num(w, "watchCount") != 1 {
		t.Errorf("watchers: %v", w)
	}
	if status, _ := do("DELETE", "/rest/api/2/issue/"+key+"/watchers?username=paul.martin", nil); status != 204 {
		t.Errorf("unwatch: %d", status)
	}
	// Developers manage watchers in DEMO; other users only see the count.
	if status, _ := do("POST", "/rest/api/2/issue/"+key+"/watchers", "hugo.robert"); status != 204 {
		t.Errorf("paul.martin could not add a watcher: %d", status)
	}
	if _, w := do("GET", "/rest/api/2/issue/"+key+"/watchers", nil); num(w, "watchCount") != 1 || str(w, "watchers", 0, "name") != "hugo.robert" {
		t.Errorf("watchers seen by a developer: %v", w)
	}
	chloe := login(t, "chloe.simon", "motdepasse")
	if status, _, _ := call(t, chloe, testURL, "POST", "/rest/api/2/issue/"+key+"/watchers", "lucas.bernard",
		header("X-Atlassian-Token", "no-check")); status != 403 {
		t.Errorf("chloe.simon added a watcher without the permission: %d", status)
	}
	if _, _, raw := call(t, chloe, testURL, "GET", "/rest/api/2/issue/"+key+"/watchers", nil, nil); strings.Contains(string(raw), "hugo.robert") ||
		num(obj(t, raw), "watchCount") != 1 {
		t.Errorf("watchers seen by chloe.simon: %s", raw)
	}

	// Attachments: multipart with the header Jira demands.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "rapport.txt")
	fw.Write([]byte("page 3 absente"))
	mw.Close()
	status, _, raw := call(t, nil, testURL, "POST", "/rest/api/2/issue/"+key+"/attachments", nil, func(r *http.Request) {
		bearer(token)(r)
		r.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
		r.ContentLength = int64(buf.Len())
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.Header.Set("X-Atlassian-Token", "no-check")
	})
	var atts []any
	json.Unmarshal(raw, &atts)
	if status != 200 || len(atts) != 1 || str(atts[0], "filename") != "rapport.txt" {
		t.Fatalf("attach: %d %s", status, raw)
	}
	content := strings.TrimPrefix(str(atts[0], "content"), testURL)
	if status, _, data := call(t, nil, testURL, "GET", content, nil, bearer(token)); status != 200 || string(data) != "page 3 absente" {
		t.Errorf("download %s: %d %q", content, status, data)
	}

	// Links, assignment, deletion.
	other := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	if status, _ := do("POST", "/rest/api/2/issueLink", map[string]any{"type": map[string]string{"name": "Blocks"},
		"inwardIssue": map[string]string{"key": key}, "outwardIssue": map[string]string{"key": other}}); status != 201 {
		t.Fatalf("link: %d", status)
	}
	_, issue = do("GET", "/rest/api/2/issue/"+key+"?fields=issuelinks", nil)
	if str(issue, "fields", "issuelinks", 0, "outwardIssue", "key") != other || str(issue, "fields", "issuelinks", 0, "type", "name") != "Blocks" {
		t.Errorf("links: %v", at(issue, "fields", "issuelinks"))
	}
	if status, _ := do("PUT", "/rest/api/2/issue/"+key+"/assignee", map[string]any{"name": nil}); status != 204 {
		t.Errorf("unassign: %d", status)
	}
	_, issue = do("GET", "/rest/api/2/issue/"+key+"?fields=assignee&expand=changelog", nil)
	if at(issue, "fields", "assignee") != nil {
		t.Errorf("still assigned: %v", at(issue, "fields", "assignee"))
	}
	changed := map[string]bool{}
	for _, h := range at(issue, "changelog", "histories").([]any) {
		for _, it := range at(h, "items").([]any) {
			changed[str(it, "field")] = true
		}
	}
	for _, field := range []string{"summary", "status", "resolution", "assignee", "Story Points", "labels"} {
		if !changed[field] {
			t.Errorf("changelog lacks %s: %v", field, changed)
		}
	}
	// What was done through the REST API shows in the interface.
	var d issueDetail
	paul.get("/api/issues/"+key, &d)
	if d.Status.Category != "done" || len(d.Attachments) != 1 || len(d.Links) != 1 {
		t.Errorf("interface view: %+v", d)
	}
	// Only the project's administrators delete issues in DEMO.
	if status, _ := do("DELETE", "/rest/api/2/issue/"+key, nil); status != 403 {
		t.Fatalf("paul.martin deleted an issue: %d", status)
	}
	if status, _, _ := call(t, nil, testURL, "DELETE", "/rest/api/2/issue/"+key, nil, basic("admin", "admin")); status != 204 {
		t.Fatalf("delete: %d", status)
	}
	if status, _ := do("GET", "/rest/api/2/issue/"+key, nil); status != 404 {
		t.Errorf("deleted issue: %d", status)
	}
}

// A url custom field only accepts http, https or mailto values: anything
// else (a javascript: URL, say) is refused, so a stored value can safely
// become a link.
func TestRESTURLCustomFieldScheme(t *testing.T) {
	token := newToken(t, "paul.martin", nil)
	task := pick(t, `SELECT it.name FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND NOT it.subtask AND it.name NOT IN ('Epic', 'Épopée')
		ORDER BY t.position LIMIT 1`)
	create := func(url string) (int, map[string]any) {
		t.Helper()
		status, _, raw := call(t, nil, testURL, "POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
			"project": map[string]string{"key": "DEMO"}, "issuetype": map[string]string{"name": task},
			"summary": "Lien maquette", "customfield_10107": url}}, bearer(token))
		var m map[string]any
		json.Unmarshal(raw, &m)
		return status, m
	}
	if status, e := create("javascript:alert(1)"); status != 400 || at(e, "errors", "customfield_10107") == nil {
		t.Errorf("javascript: URL accepted: %d %v", status, e)
	}
	if status, e := create("data:text/html,<b>"); status != 400 {
		t.Errorf("data: URL accepted: %d %v", status, e)
	}
	for _, ok := range []string{"https://maquettes.exemple.fr/x", "http://x.fr", "mailto:a@b.fr"} {
		if status, created := create(ok); status != 201 {
			t.Fatalf("valid URL %q refused: %d %v", ok, status, created)
		}
	}
}

func TestRESTSubtaskAndEpic(t *testing.T) {
	token := newToken(t, "marie.durand", nil)
	do := func(method, path string, body any) (int, map[string]any) {
		t.Helper()
		status, _, raw := call(t, nil, testURL, method, path, body, bearer(token))
		var m map[string]any
		json.Unmarshal(raw, &m)
		return status, m
	}
	parent := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' AND i.parent_id IS NULL AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	subtask := pick(t, `SELECT it.id FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND it.subtask LIMIT 1`)
	status, created := do("POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
		"project": map[string]string{"key": "DEMO"}, "issuetype": map[string]string{"id": subtask},
		"summary": "Sous-tâche par l'API", "parent": map[string]string{"key": parent}}})
	if status != 201 {
		t.Fatalf("create sub-task: %d %v", status, created)
	}
	_, issue := do("GET", "/rest/api/2/issue/"+parent+"?fields=subtasks", nil)
	found := false
	for _, st := range at(issue, "fields", "subtasks").([]any) {
		found = found || str(st, "key") == str(created, "key")
	}
	if !found {
		t.Errorf("parent's sub-tasks lack %s", str(created, "key"))
	}
	if status, _ := do("DELETE", "/rest/api/2/issue/"+parent, nil); status != 400 {
		t.Errorf("deleting an issue with sub-tasks without deleteSubtasks: %d", status)
	}

	// Epic link and sprint through their custom fields.
	epic := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN issue_types it ON it.id = i.issue_type_id WHERE p.pkey = 'DEMO' AND it.name IN ('Epic', 'Épopée')
		AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	story := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN issue_types it ON it.id = i.issue_type_id WHERE p.pkey = 'DEMO' AND NOT it.subtask
		AND it.name NOT IN ('Epic', 'Épopée') AND i.security_level_id IS NULL ORDER BY i.num DESC LIMIT 1`)
	if status, e := do("PUT", "/rest/api/2/issue/"+story, map[string]any{"fields": map[string]any{
		"customfield_10110": epic, "customfield_10111": 6}}); status != 204 {
		t.Fatalf("epic link and sprint: %d %v", status, e)
	}
	_, issue = do("GET", "/rest/api/2/issue/"+story+"?fields=customfield_10110,customfield_10111", nil)
	if str(issue, "fields", "customfield_10110") != epic || !strings.Contains(str(issue, "fields", "customfield_10111", 0), "id=6,") {
		t.Errorf("epic link / sprint: %v", at(issue, "fields"))
	}
	_, list := do("GET", "/rest/agile/1.0/epic/"+epic+"/issue?fields=summary", nil)
	found = false
	for _, v := range at(list, "issues").([]any) {
		found = found || str(v, "key") == story
	}
	if !found {
		t.Errorf("epic %s issues lack %s", epic, story)
	}
	if status, e := do("GET", "/rest/agile/1.0/epic/"+epic, nil); status != 200 || str(e, "key") != epic {
		t.Errorf("epic: %d %v", status, e)
	}
}

func TestAgileREST(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	get := func(path string) map[string]any {
		t.Helper()
		status, _, body := call(t, marie, testURL, "GET", path, nil, nil)
		if status != 200 {
			t.Fatalf("GET %s: %d %s", path, status, body)
		}
		return obj(t, body)
	}
	boards := get("/rest/agile/1.0/board?type=scrum")
	values := at(boards, "values").([]any)
	if len(values) == 0 || num(boards, "total") != len(values) || at(boards, "isLast") != true {
		t.Fatalf("boards: %v", boards)
	}
	if get("/rest/agile/1.0/board?projectKeyOrId=DEMO")["total"] != 1.0 {
		t.Error("boards of DEMO")
	}
	board := get("/rest/agile/1.0/board/1")
	if str(board, "type") != "scrum" || str(board, "location", "projectKey") != "DEMO" {
		t.Errorf("board 1: %v", board)
	}
	config := get("/rest/agile/1.0/board/1/configuration")
	if len(at(config, "columnConfig", "columns").([]any)) == 0 || str(config, "filter", "id") == "" {
		t.Errorf("configuration: %v", config)
	}
	// Other tests start and complete sprints: work with what board 1 has now,
	// and a future sprint of our own.
	status, body := marie.do("POST", "/api/boards/1/sprints", map[string]any{"name": "Sprint REST"})
	if status != 201 {
		t.Fatalf("create sprint: %d %s", status, body)
	}
	future := num(obj(t, body), "id")
	sprints := get("/rest/agile/1.0/board/1/sprint?state=active,future")
	var states []string
	for _, v := range at(sprints, "values").([]any) {
		states = append(states, str(v, "state"))
	}
	if !slices.Contains(states, "future") || slices.Contains(states, "closed") || !slices.IsSortedFunc(states,
		func(a, b string) int { return strings.Compare(a, b) }) {
		t.Errorf("sprint states: %v", states)
	}
	if status, _, _ := call(t, marie, testURL, "GET", "/rest/agile/1.0/board/2/sprint", nil, nil); status != 400 {
		t.Errorf("sprints of a Kanban board: %d", status)
	}
	closed := pick(t, `SELECT id FROM sprints WHERE board_id = 1 AND state = 'closed' ORDER BY id LIMIT 1`)
	if s := get("/rest/agile/1.0/sprint/" + closed); str(s, "state") != "closed" || num(s, "originBoardId") != 1 ||
		str(s, "completeDate") == "" {
		t.Errorf("sprint %s: %v", closed, s)
	}
	inSprint := get("/rest/agile/1.0/sprint/" + closed + "/issue?fields=summary,sprint,closedSprints")
	if num(inSprint, "total") == 0 {
		t.Errorf("sprint %s has no issues", closed)
	}
	inClosed := false
	for _, sp := range at(inSprint, "issues", 0, "fields", "closedSprints").([]any) {
		inClosed = inClosed || strconv.Itoa(num(sp, "id")) == closed
	}
	if !inClosed {
		t.Errorf("first issue of sprint %s: %v", closed, at(inSprint, "issues", 0, "fields"))
	}
	if n := num(get("/rest/agile/1.0/board/1/issue?maxResults=0"), "total"); n == 0 {
		t.Error("board 1 has no issues")
	}

	// Move a backlog issue to the future sprint, then rank it first of the board.
	backlog := get("/rest/agile/1.0/board/1/backlog?fields=summary")
	if num(backlog, "total") < 2 {
		t.Fatalf("backlog: %v", backlog)
	}
	moved, first := str(backlog, "issues", 1, "key"), str(backlog, "issues", 0, "key")
	if status, _, body := call(t, marie, testURL, "POST", fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", future),
		map[string]any{"issues": []string{moved}}, header("X-Atlassian-Token", "no-check")); status != 204 {
		t.Fatalf("move to sprint: %d %s", status, body)
	}
	for _, v := range at(get("/rest/agile/1.0/board/1/backlog?fields=summary"), "issues").([]any) {
		if str(v, "key") == moved {
			t.Errorf("%s is still in the backlog", moved)
		}
	}
	if status, _, body := call(t, marie, testURL, "PUT", "/rest/agile/1.0/issue/rank",
		map[string]any{"issues": []string{moved}, "rankBeforeIssue": first}, header("X-Atlassian-Token", "no-check")); status != 204 {
		t.Fatalf("rank: %d %s", status, body)
	}
	rankOf := func(key string) string {
		return pick(t, `SELECT i.rank_key FROM issues i JOIN projects p ON p.id = i.project_id WHERE `+keyExpr+` = ?`, key)
	}
	if !(rankOf(moved) < rankOf(first)) {
		t.Errorf("rank(%s) = %s, not before rank(%s) = %s", moved, rankOf(moved), first, rankOf(first))
	}
	// One only ranks issues one sees (and may edit).
	rh := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	if status, _, _ := call(t, login(t, "paul.martin", "motdepasse"), testURL, "PUT", "/rest/agile/1.0/issue/rank",
		map[string]any{"issues": []string{rh}, "rankAfterIssue": first}, header("X-Atlassian-Token", "no-check")); status != 404 {
		t.Errorf("paul.martin ranked an RH issue: %d", status)
	}
}
