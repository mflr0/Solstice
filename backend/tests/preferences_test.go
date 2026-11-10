package tests

// Interface preferences stored on the account.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"solstice/internal/auth"
	"solstice/internal/jirafixture"
)

func TestPreferences(t *testing.T) {
	paul := login(t, "paul.martin", jirafixture.Password)
	marie := login(t, "marie.durand", jirafixture.Password)
	t.Cleanup(func() {
		paul.do("DELETE", "/api/me/preferences/shortcuts", nil)
		paul.do("DELETE", "/api/me/preferences/shortcutsEnabled", nil)
		paul.do("DELETE", "/api/me/preferences/tourSeen", nil)
	})
	prefs := func(raw []byte) map[string]any {
		t.Helper()
		p, ok := obj(t, raw)["preferences"].(map[string]any)
		if !ok {
			t.Fatalf("no preferences object in %s", raw)
		}
		return p
	}
	me := func(c *client) map[string]any {
		t.Helper()
		return prefs(c.call("GET", "/api/me", nil, 200, nil))
	}
	same := func(got, want any) bool {
		a, _ := json.Marshal(got)
		b, _ := json.Marshal(want)
		return string(a) == string(b)
	}

	if p := me(paul); len(p) != 0 {
		t.Fatalf("before any choice: %v", p)
	}
	bindings := map[string]any{"search": []any{"Alt-k", "/"}, "watch": []any{}}
	if p := prefs(paul.call("PUT", "/api/me/preferences/shortcuts", bindings, 200, nil)); !same(p["shortcuts"], bindings) {
		t.Errorf("PUT shortcuts answers %v", p)
	}
	paul.call("PUT", "/api/me/preferences/shortcutsEnabled", false, 200, nil)
	if p := me(paul); !same(p, map[string]any{"shortcuts": bindings, "shortcutsEnabled": false}) {
		t.Errorf("read back %v", p)
	}
	_, body := anonymous(t).do("POST", "/api/login", map[string]string{"username": "paul.martin", "password": jirafixture.Password})
	if p := prefs(body); !same(p["shortcutsEnabled"], false) {
		t.Errorf("login answers %v", p)
	}
	if p := me(marie); len(p) != 0 {
		t.Errorf("another user sees %v", p)
	}

	for _, c := range []struct {
		path string
		body any
	}{
		{"/api/me/preferences/theme", "dark"},
		{"/api/me/preferences/shortcutsEnabled", "no"},
		{"/api/me/preferences/shortcuts", map[string]any{"fly": []string{"f"}}},
		{"/api/me/preferences/shortcuts", map[string]any{"search": []string{"a", "b", "c", "d", "e"}}},
	} {
		if status, raw := paul.do("PUT", c.path, c.body); status != 400 {
			t.Errorf("PUT %s %v: %d %s, want 400", c.path, c.body, status, raw)
		}
	}
	if status, _ := paul.do("DELETE", "/api/me/preferences/theme", nil); status != 400 {
		t.Errorf("DELETE an unknown preference: %d, want 400", status)
	}
	if p := me(paul); !same(p["shortcuts"], bindings) {
		t.Errorf("after refused changes: %v", p)
	}

	if p := prefs(paul.call("DELETE", "/api/me/preferences/shortcuts", nil, 200, nil)); !same(p, map[string]any{"shortcutsEnabled": false}) {
		t.Errorf("DELETE shortcuts answers %v", p)
	}
	paul.call("DELETE", "/api/me/preferences/shortcuts", nil, 200, nil)

	if p := prefs(paul.call("PUT", "/api/me/preferences/tourSeen", true, 200, nil)); !same(p["tourSeen"], true) {
		t.Errorf("PUT tourSeen answers %v", p)
	}
	if p := me(paul); !same(p["tourSeen"], true) {
		t.Errorf("tourSeen read back %v", p)
	}
	for _, body := range []string{`"yes"`, `null`} {
		if status, raw := paul.do("PUT", "/api/me/preferences/tourSeen", json.RawMessage(body)); status != 400 {
			t.Errorf("PUT tourSeen %s: %d %s, want 400", body, status, raw)
		}
	}
	if p := prefs(paul.call("DELETE", "/api/me/preferences/tourSeen", nil, 200, nil)); !same(p, map[string]any{"shortcutsEnabled": false}) {
		t.Errorf("DELETE tourSeen answers %v", p)
	}

	// A token without the profile scope reads them but cannot change them.
	token, _, err := auth.NewToken(context.Background(), testDB, userID(t, "paul.martin"), "test preferences",
		[]auth.Scope{auth.ScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	withToken := func(r *http.Request) { bearer(token)(r); r.Header.Set(csrfHeader, "1") }
	if status, _, raw := call(t, nil, testURL, "GET", "/api/me", nil, withToken); status != 200 || !same(prefs(raw)["shortcutsEnabled"], false) {
		t.Errorf("GET /api/me with a read token: %d %s", status, raw)
	}
	for _, method := range []string{"PUT", "DELETE"} {
		if status, _, raw := call(t, nil, testURL, method, "/api/me/preferences/shortcutsEnabled", true, withToken); status != 403 {
			t.Errorf("%s with a read token: %d %s, want 403", method, status, raw)
		}
	}
	if p := me(paul); !same(p["shortcutsEnabled"], false) {
		t.Errorf("after the token's attempts: %v", p)
	}
}
