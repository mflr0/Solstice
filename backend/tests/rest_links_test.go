package tests

// Remote links, Jira's session route and groups of the REST API.

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
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

// Session ----------------------------------------------------------------------

func TestRESTSession(t *testing.T) {
	needDB(t)
	const path = "/rest/auth/1/session"
	base := ownServer(t) // failed sign-ins count against the address: not on the shared server
	signIn := func(base, user, password string, setup func(*http.Request)) (int, http.Header, []byte) {
		return call(t, nil, base, "POST", path, map[string]string{"username": user, "password": password}, setup)
	}

	// Nobody is signed in: 401 in Jira's format.
	if status, h, raw := call(t, nil, base, "GET", path, nil, nil); status != 401 || h.Get("X-AUSERNAME") != "anonymous" ||
		at(obj(t, raw), "errorMessages", 0) == nil {
		t.Errorf("anonymous GET: %d %s", status, raw)
	}
	if status, _, _ := call(t, nil, base, "DELETE", path, nil, nil); status != 401 {
		t.Errorf("anonymous DELETE: %d", status)
	}

	// A wrong password is refused and audited like the form's.
	before := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'login.failure' AND username = 'paul.martin'`)
	status, h, raw := signIn(base, "paul.martin", "faux", nil)
	if status != 401 || h.Get("X-Seraph-LoginReason") != "AUTHENTICATED_FAILED" || len(h.Values("Set-Cookie")) != 0 {
		t.Errorf("wrong password: %d %v %s", status, h, raw)
	}
	if after := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'login.failure' AND username = 'paul.martin'`); after == before {
		t.Error("the failed sign-in is not in the audit log")
	}

	// A browser could send a form-like body cross-site: that needs the header.
	if status, _, _ := signIn(base, "paul.martin", "motdepasse", header("Content-Type", "text/plain")); status != 403 {
		t.Errorf("form-like sign-in without header: %d", status)
	}

	// Signing in sets the cookie, as /api/login does, and answers Jira's session.
	status, h, raw = signIn(base, "paul.martin", "motdepasse", nil)
	out := obj(t, raw)
	value := str(out, "session", "value")
	if status != 200 || str(out, "session", "name") != "solstice_session" || value == "" || at(out, "loginInfo", "loginCount") == nil ||
		at(out, "loginInfo", "failedLoginCount") == nil {
		t.Fatalf("sign-in: %d %s", status, raw)
	}
	cookie := (&http.Response{Header: h}).Cookies()
	if len(cookie) != 1 || cookie[0].Value != value || !cookie[0].HttpOnly || cookie[0].Path != "/" {
		t.Errorf("cookie: %v", h.Values("Set-Cookie"))
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'login.success' AND username = 'paul.martin'
		AND details LIKE '%"session"%'`); n == "0" {
		t.Error("the sign-in is not in the audit log")
	}
	withCookie := header("Cookie", "solstice_session="+value)
	status, _, raw = call(t, nil, base, "GET", path, nil, withCookie)
	got := obj(t, raw)
	if status != 200 || str(got, "name") != "paul.martin" || !strings.HasSuffix(str(got, "self"), "/rest/api/2/user?username=paul.martin") ||
		num(got, "loginInfo", "loginCount") < 1 || num(got, "loginInfo", "failedLoginCount") != 0 ||
		!jiraTime.MatchString(str(got, "loginInfo", "lastFailedLoginTime")) {
		t.Errorf("GET: %d %s", status, raw)
	}
	// The session is the interface's too.
	if status, _, _ := call(t, nil, base, "GET", "/api/me", nil, withCookie); status != 200 {
		t.Errorf("/api/me with the session: %d", status)
	}
	if status, _, _ := call(t, nil, base, "GET", "/rest/api/2/myself", nil, withCookie); status != 200 {
		t.Errorf("/myself with the session: %d", status)
	}

	// Signing out ends the session and clears the cookie.
	status, h, _ = call(t, nil, base, "DELETE", path, nil, withCookie)
	cleared := (&http.Response{Header: h}).Cookies()
	if status != 204 || len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Errorf("sign-out: %d %v", status, h.Values("Set-Cookie"))
	}
	if status, _, _ := call(t, nil, base, "GET", path, nil, withCookie); status != 401 {
		t.Errorf("GET after the sign-out: %d", status)
	}

	// The same throttle as the form's.
	base = ownServer(t)
	for i := range 5 {
		if status, _, _ := signIn(base, "lucas.bernard", "faux", nil); status != 401 {
			t.Fatalf("attempt %d: %d, want 401", i+1, status)
		}
	}
	if status, h, _ := signIn(base, "lucas.bernard", "motdepasse", nil); status != http.StatusTooManyRequests ||
		h.Get("X-Seraph-LoginReason") != "AUTHENTICATION_DENIED" {
		t.Errorf("right password after 5 failures: %d %v", status, h)
	}
	if status, _ := anonymousAt(t, base).do("POST", "/api/login",
		map[string]string{"username": "lucas.bernard", "password": "motdepasse"}); status != http.StatusTooManyRequests {
		t.Errorf("the form is not locked as well: %d", status)
	}
}

// Groups -----------------------------------------------------------------------

func TestRESTGroups(t *testing.T) {
	needDB(t)
	admin, paul := login(t, "admin", "admin"), login(t, "paul.martin", "motdepasse")
	get := func(who *client, path string) (int, map[string]any) {
		status, _, raw := call(t, who, testURL, "GET", path, nil, nil)
		if status != 200 {
			return status, nil
		}
		return status, obj(t, raw)
	}
	group := pick(t, `SELECT g.name FROM user_groups g
		ORDER BY (SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id) DESC, g.name LIMIT 1`)
	groupCount := pick(t, `SELECT COUNT(*) FROM user_groups`)
	names := func(out map[string]any) []string {
		var l []string
		for _, g := range at(out, "groups").([]any) {
			l = append(l, str(g, "name"))
		}
		return l
	}

	// The picker: administrators find every group, others only their own.
	status, out := get(admin, "/rest/api/2/groups/picker?maxResults=1000")
	if status != 200 || strconv.Itoa(num(out, "total")) != groupCount || len(names(out)) != num(out, "total") ||
		!slices.IsSorted(names(out)) || !strings.Contains(str(out, "header"), groupCount) {
		t.Fatalf("admin picker: %d %v", status, out)
	}
	part := group[:min(3, len(group))]
	status, out = get(admin, "/rest/api/2/groups/picker?query="+url.QueryEscape(strings.ToUpper(part)))
	if status != 200 || !slices.Contains(names(out), group) {
		t.Fatalf("picker query %q: %d %v", part, status, out)
	}
	for _, g := range at(out, "groups").([]any) {
		if !strings.Contains(strings.ToLower(str(g, "name")), part) || !strings.Contains(str(g, "html"), "<b>") ||
			at(g, "labels") == nil {
			t.Errorf("picker group: %v", g)
		}
	}
	if status, out = get(admin, "/rest/api/2/groups/picker?query="+url.QueryEscape(group)+"&exclude="+url.QueryEscape(group)); status != 200 ||
		slices.Contains(names(out), group) {
		t.Errorf("picker exclude: %d %v", status, out)
	}
	if status, out = get(admin, "/rest/api/2/groups/picker?maxResults=1"); status != 200 || len(names(out)) != 1 ||
		strconv.Itoa(num(out, "total")) != groupCount {
		t.Errorf("picker maxResults: %d %v", status, out)
	}
	if status, out = get(admin, "/rest/api/2/groups/picker?query=%25%5F"); status != 200 || num(out, "total") != 0 {
		t.Errorf("picker query with wildcards: %d %v", status, out)
	}
	own := map[string]bool{}
	for _, g := range strings.Split(pick(t, `SELECT GROUP_CONCAT(g.name SEPARATOR ',') FROM group_members m
		JOIN user_groups g ON g.id = m.group_id JOIN users u ON u.id = m.user_id WHERE u.username = 'paul.martin'`), ",") {
		own[g] = true
	}
	status, out = get(paul, "/rest/api/2/groups/picker?maxResults=1000")
	if status != 200 || len(names(out)) != len(own) {
		t.Errorf("paul.martin's picker: %d %v, groups %v", status, out, own)
	}
	for _, g := range names(out) {
		if !own[g] {
			t.Errorf("paul.martin's picker shows %s, which he is not in", g)
		}
	}
	if status, _ := get(nil, "/rest/api/2/groups/picker"); status != 401 {
		t.Errorf("anonymous picker: %d", status)
	}

	// Members: for administrators only, as in the interface.
	if status := paul.get("/api/admin/groups", nil); status != 403 {
		t.Fatalf("the interface lets paul.martin list groups: %d", status)
	}
	member := "/rest/api/2/group/member?groupname=" + url.QueryEscape(group)
	if status, _ := get(paul, member); status != 403 {
		t.Errorf("paul.martin lists members: %d", status)
	}
	active := pick(t, `SELECT COUNT(*) FROM group_members m JOIN user_groups g ON g.id = m.group_id JOIN users u ON u.id = m.user_id
		WHERE g.name = ? AND u.active`, group)
	everyone := pick(t, `SELECT COUNT(*) FROM group_members m JOIN user_groups g ON g.id = m.group_id WHERE g.name = ?`, group)
	status, out = get(admin, member)
	values, _ := out["values"].([]any)
	if status != 200 || strconv.Itoa(num(out, "total")) != active || strconv.Itoa(len(values)) != active || out["isLast"] != true ||
		num(out, "startAt") != 0 || num(out, "maxResults") != 50 || str(out, "self") == "" || out["nextPage"] != nil {
		t.Fatalf("members: %d %v (%s active)", status, out, active)
	}
	for _, u := range values {
		if str(u, "name") == "" || str(u, "displayName") == "" || at(u, "active") != true || str(u, "self") == "" {
			t.Errorf("member: %v", u)
		}
	}
	if status, out = get(admin, member+"&includeInactiveUsers=true"); status != 200 || strconv.Itoa(num(out, "total")) != everyone {
		t.Errorf("members with the inactive: %d %v, want %s", status, out, everyone)
	}
	if status, out = get(admin, "/rest/api/2/group/member?groupname="+url.QueryEscape(strings.ToUpper(group))); status != 200 ||
		strconv.Itoa(num(out, "total")) != active {
		t.Errorf("group name in capitals: %d %v", status, out)
	}
	if n, _ := strconv.Atoi(active); n >= 2 {
		status, out = get(admin, member+"&maxResults=1&startAt=1")
		if status != 200 || len(at(out, "values").([]any)) != 1 || num(out, "startAt") != 1 || num(out, "maxResults") != 1 ||
			out["isLast"] != (n == 2) || (n > 2) != strings.Contains(str(out, "nextPage"), "startAt=2") {
			t.Errorf("second page: %d %v", status, out)
		}
		_, first := get(admin, member+"&maxResults=1")
		if str(first, "values", 0, "name") == str(out, "values", 0, "name") {
			t.Errorf("both pages start with %s", str(out, "values", 0, "name"))
		}
	}
	if status, _ := get(admin, "/rest/api/2/group/member?groupname=no-such-group"); status != 404 {
		t.Errorf("unknown group: %d", status)
	}
	if status, _ := get(admin, "/rest/api/2/group/member"); status != 400 {
		t.Errorf("no group name: %d", status)
	}
	if status, _ := get(nil, member); status != 401 {
		t.Errorf("anonymous members: %d", status)
	}
}
