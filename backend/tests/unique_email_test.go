package tests

import (
	"fmt"
	"strings"
	"testing"

	"solstice/internal/auth"
)

const emailPassword = "une phrase assez longue"

// TestEmailsAreUnique: an address names at most one account, in any case;
// any number of accounts have none.
func TestEmailsAreUnique(t *testing.T) {
	admin := login(t, "admin", "admin")
	owner := unique("courriel.")
	admin.call("POST", "/api/admin/users", map[string]any{"username": owner, "email": owner + "@example.org",
		"password": emailPassword}, 201, nil)
	taken := strings.ToUpper(owner[:1]) + owner[1:] + "@Example.org"

	for lang, want := range map[string]string{
		"en": "This email address is already used by another account.",
		"fr": "Cette adresse e-mail est déjà celle d'un autre compte.",
	} {
		status, body := admin.request(lang, "POST", "/api/admin/users", map[string]any{"username": unique("double."),
			"email": taken, "password": emailPassword})
		if status != 409 || !strings.Contains(string(body), want) {
			t.Errorf("create with a taken address in %s: %d %s, want 409 %q", lang, status, body, want)
		}
	}
	for range 2 {
		admin.call("POST", "/api/admin/users", map[string]any{"username": unique("sans.adresse."), "email": "",
			"password": emailPassword}, 201, nil)
	}

	id, username := newAccount(t, admin, emailPassword)
	admin.call("PATCH", fmt.Sprintf("/api/admin/users/%d", id), map[string]any{"email": taken}, 409, nil)

	me := login(t, username, emailPassword)
	me.call("PATCH", "/api/me", map[string]any{"email": taken, "currentPassword": emailPassword}, 409, nil)
	// Its own address in another case is still its own.
	me.call("PATCH", "/api/me", map[string]any{"email": strings.ToUpper(username) + "@EXEMPLE.FR",
		"currentPassword": emailPassword}, 200, nil)
}

// A directory entry whose address another account has signs in, without
// the address.
func TestDirectorySignInLeavesATakenAddress(t *testing.T) {
	srv, base := newServer(t)
	admin := login(t, "admin", "admin")
	owner := unique("annuaire.")
	admin.call("POST", "/api/admin/users", map[string]any{"username": owner, "email": owner + "@example.org",
		"password": emailPassword}, 201, nil)
	srv.SetDirectory(&fakeDirectory{username: "ldap.twin", entry: auth.DirectoryEntry{Name: "Twin",
		Email: strings.ToUpper(owner) + "@example.org"}})
	directoryUser(t, "ldap.twin")

	var me struct{ Username, Email string }
	loginAt(t, base, "ldap.twin", "pw").get("/api/me", &me)
	if me.Username != "ldap.twin" || me.Email != "" {
		t.Errorf("signed in as %+v, want ldap.twin without an address", me)
	}
}
