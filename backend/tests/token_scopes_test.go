package tests

// Tests of the scopes of personal access tokens. They need SOLSTICE_TEST_DB
// and are skipped without it.

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
)

func TestTokenScopes(t *testing.T) {
	allowTokensForEveryone(t)
	paul := login(t, "paul.martin", jirafixture.Password)
	admin := login(t, "admin", "admin")
	key := createIssue(t, paul, "DEMO", "Portées des jetons", "")

	create := func(c *client, scopes ...string) string {
		t.Helper()
		var out struct{ ID int64 }
		raw := c.call("POST", "/api/me/tokens", map[string]any{"name": "portées", "scopes": scopes}, 201, &out)
		t.Cleanup(func() { c.do("DELETE", fmt.Sprintf("/api/me/tokens/%d", out.ID), nil) })
		return str(obj(t, raw), "token")
	}
	with := func(token string) func(*http.Request) {
		return func(r *http.Request) { bearer(token)(r); r.Header.Set(csrfHeader, "1") }
	}
	refused := func(name string, status int, h http.Header, body []byte, scope string) {
		t.Helper()
		want := fmt.Sprintf(`Bearer error="insufficient_scope", scope=%q`, scope)
		if status != 403 || h.Get("WWW-Authenticate") != want {
			t.Errorf("%s: %d %q %s, want 403 %q", name, status, h.Get("WWW-Authenticate"), body, want)
		}
	}
	comment := map[string]string{"body": "Vu par un script."}

	// A read token reads, and cannot comment, in either API.
	read := create(paul, "read")
	if status, _, body := call(t, nil, testURL, "GET", "/rest/api/2/issue/"+key, nil, with(read)); status != 200 {
		t.Errorf("read token reads an issue: %d %s", status, body)
	}
	if status, _, body := call(t, nil, testURL, "POST", "/rest/api/2/search", map[string]any{"jql": "key = " + key}, with(read)); status != 200 {
		t.Errorf("read token searches: %d %s", status, body)
	}
	status, h, body := call(t, nil, testURL, "POST", "/rest/api/2/issue/"+key+"/comment", comment, with(read))
	refused("read token comments (REST)", status, h, body, "comments:write")
	status, h, body = call(t, nil, testURL, "POST", "/api/issues/"+key+"/comments", comment, with(read))
	refused("read token comments", status, h, body, "comments:write")

	// A comments:write token comments (and reads), but cannot edit the issue.
	commenter := create(paul, "comments:write")
	if status, _, body := call(t, nil, testURL, "POST", "/rest/api/2/issue/"+key+"/comment", comment, with(commenter)); status != 201 {
		t.Errorf("comments:write token comments: %d %s", status, body)
	}
	if status, _, body := call(t, nil, testURL, "GET", "/api/issues/"+key, nil, with(commenter)); status != 200 {
		t.Errorf("comments:write token reads: %d %s", status, body)
	}
	status, h, body = call(t, nil, testURL, "PUT", "/rest/api/2/issue/"+key,
		map[string]any{"fields": map[string]any{"summary": "Changé par un script"}}, with(commenter))
	refused("comments:write token edits (REST)", status, h, body, "issues:write")
	status, h, body = call(t, nil, testURL, "PATCH", "/api/issues/"+key, map[string]any{"summary": "Changé par un script"}, with(commenter))
	refused("comments:write token edits", status, h, body, "issues:write")

	// The answer and the list give the scopes, in their canonical order.
	var list []tokenOut
	paul.get("/api/me/tokens", &list)
	i := slices.IndexFunc(list, func(t tokenOut) bool { return t.Name == "portées" && slices.Contains(t.Scopes, "comments:write") })
	if i < 0 || !slices.Equal(list[i].Scopes, []string{"comments:write"}) {
		t.Errorf("token list: %+v", list)
	}
	raw := admin.call("POST", "/api/me/tokens", map[string]any{"name": "ordre", "scopes": []string{"admin", "read", "admin"}}, 201, nil)
	if got := obj(t, raw)["scopes"]; fmt.Sprint(got) != "[read admin]" {
		t.Errorf("created scopes: %v", got)
	}
	admin.do("DELETE", fmt.Sprintf("/api/me/tokens/%d", int64(obj(t, raw)["id"].(float64))), nil)

	// Bad scopes are refused; only an administrator gets admin.
	for name, c := range map[string]struct {
		scopes any
		status int
		msg    string
	}{
		"missing": {nil, 400, "at least one scope"},
		"empty":   {[]string{}, 400, "at least one scope"},
		"unknown": {[]string{"read", "write"}, 400, "Unknown scope write"},
		"admin":   {[]string{"read", "admin"}, 403, "Only an administrator"},
	} {
		in := map[string]any{"name": "refusé"}
		if c.scopes != nil {
			in["scopes"] = c.scopes
		}
		if status, msg, _ := paul.errorIn("en", "POST", "/api/me/tokens", in); status != c.status || !strings.Contains(msg, c.msg) {
			t.Errorf("%s: %d %q", name, status, msg)
		}
	}

	// An admin scope gives a non-administrator nothing more.
	full := newToken(t, "paul.martin", nil)
	if status, _, _ := call(t, nil, testURL, "GET", "/api/admin/users", nil, with(full)); status != 403 {
		t.Errorf("non-administrator's admin token: %d", status)
	}
	// An administrator's token needs admin for the administration.
	adminRead := create(admin, "read")
	status, h, body = call(t, nil, testURL, "GET", "/api/admin/users", nil, with(adminRead))
	refused("administrator's read token", status, h, body, "admin")
	adminToken := create(admin, "admin")
	if status, _, body := call(t, nil, testURL, "GET", "/api/admin/users", nil, with(adminToken)); status != 200 {
		t.Errorf("administrator's admin token: %d %s", status, body)
	}
	if status, _, body := call(t, nil, testURL, "GET", "/api/me", nil, with(adminToken)); status != 200 {
		t.Errorf("admin implies read: %d %s", status, body)
	}

	// A session cookie keeps every right.
	if status, body := paul.do("POST", "/api/issues/"+key+"/comments", comment); status != 201 {
		t.Errorf("cookie comments: %d %s", status, body)
	}
	if status, body := paul.do("PATCH", "/api/issues/"+key, map[string]any{"summary": "Changé à la main"}); status != 200 {
		t.Errorf("cookie edits: %d %s", status, body)
	}
	if status := admin.get("/api/admin/users", nil); status != 200 {
		t.Errorf("administrator's cookie: %d", status)
	}
}
