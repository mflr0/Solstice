package tests

// The administration of accounts cannot change an administrator's own
// password or address, nor any credential through an access token: the
// profile asks for the current password and refuses tokens.

import (
	"fmt"
	"net/http"
	"testing"
)

func TestAdminUpdateKeepsTheProfileChecks(t *testing.T) {
	admin := login(t, "admin", "admin")
	// A second administrator, so a broken check cannot change the shared one.
	selfID, self := newAccount(t, admin, "une phrase assez longue")
	admin.call("PATCH", fmt.Sprintf("/api/admin/users/%d", selfID), map[string]any{"admin": true}, 204, nil)
	boss := login(t, self, "une phrase assez longue")
	otherID, other := newAccount(t, admin, "une phrase assez longue")
	selfPath := fmt.Sprintf("/api/admin/users/%d", selfID)
	otherPath := fmt.Sprintf("/api/admin/users/%d", otherID)

	for _, c := range []struct {
		name string
		body map[string]any
		want string
	}{
		{"own password", map[string]any{"password": "une autre phrase assez longue"}, "Change your own password from your profile."},
		{"own address", map[string]any{"email": "ailleurs." + self + "@exemple.fr"}, "Change your own email address from your profile."},
	} {
		if status, msg, _ := boss.errorIn("en", "PATCH", selfPath, c.body); status != 400 || msg != c.want {
			t.Errorf("%s: %d %q, want 400 %q", c.name, status, msg, c.want)
		}
	}

	token := newToken(t, self, nil)
	withToken := func(r *http.Request) { bearer(token)(r); r.Header.Set(csrfHeader, "1") }
	for _, c := range []struct {
		name string
		body map[string]any
	}{
		{"password with a token", map[string]any{"password": "une autre phrase assez longue"}},
		{"address with a token", map[string]any{"email": "ailleurs." + other + "@exemple.fr"}},
	} {
		if status, _, raw := call(t, nil, testURL, "PATCH", otherPath, c.body, withToken); status != 403 {
			t.Errorf("%s: %d %s, want 403", c.name, status, raw)
		}
	}

	// Nothing changed: the old passwords still sign in, the addresses stay.
	login(t, self, "une phrase assez longue")
	login(t, other, "une phrase assez longue")
	for id, username := range map[int64]string{selfID: self, otherID: other} {
		if got := pick(t, `SELECT email FROM users WHERE id = ?`, id); got != username+"@exemple.fr" {
			t.Errorf("%s: address %q", username, got)
		}
	}

	// What the profile checks does not cover still works.
	boss.call("PATCH", selfPath, map[string]any{"name": "Nouveau nom", "email": self + "@exemple.fr"}, 204, nil)
	if status, _, raw := call(t, nil, testURL, "PATCH", otherPath, map[string]any{"name": "Par jeton",
		"email": other + "@exemple.fr"}, withToken); status != 204 {
		t.Errorf("name with a token: %d %s, want 204", status, raw)
	}
	boss.call("PATCH", otherPath, map[string]any{"password": "une autre phrase assez longue",
		"email": "ailleurs." + other + "@exemple.fr"}, 204, nil)
	login(t, other, "une autre phrase assez longue")
}
