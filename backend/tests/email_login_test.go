package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/config"
)

// emailAccount creates an account whose address is <username>@example.org
// and returns its id, user name and address as an owner would type it.
func emailAccount(t *testing.T, admin *client) (int64, string, string) {
	t.Helper()
	username := unique("par.adresse.")
	var created struct{ ID int64 }
	admin.call("POST", "/api/admin/users", map[string]any{"username": username, "email": username + "@example.org",
		"password": emailPassword}, 201, &created)
	return created.ID, username, strings.ToUpper(username[:1]) + username[1:] + "@Example.org"
}

// An account signs in with its address, in any case, on the form and with
// HTTP basic; a wrong password and an inactive account's address do not.
// Its own server keeps those failures from throttling the other tests.
func TestLoginWithEmail(t *testing.T) {
	base := ownServer(t)
	admin := login(t, "admin", "admin")
	id, username, address := emailAccount(t, admin)

	var me struct{ Username string }
	loginAt(t, base, address, emailPassword).get("/api/me", &me)
	if me.Username != username {
		t.Errorf("signed in with the address as %q, want %q", me.Username, username)
	}

	status, msg, _ := anonymousAt(t, base).errorIn("en", "POST", "/api/login",
		map[string]string{"username": address, "password": "pas le bon"})
	if status != 401 || msg != "Incorrect username or password." {
		t.Errorf("wrong password with the address: %d %q, want 401 AccountSignInFailed", status, msg)
	}

	if status, _, body := call(t, nil, base, "GET", "/api/me", nil, basic(address, emailPassword)); status != 200 ||
		!strings.Contains(string(body), `"`+username+`"`) {
		t.Errorf("basic auth with the address: %d %s", status, body)
	}

	admin.call("PATCH", fmt.Sprintf("/api/admin/users/%d", id), map[string]any{"active": false}, 204, nil)
	if status, body := anonymousAt(t, base).do("POST", "/api/login",
		map[string]string{"username": address, "password": emailPassword}); status != 401 {
		t.Errorf("inactive account's address signed in: %d %s", status, body)
	}
}

// Wrong passwords typed with the address and with the user name count for
// one account: ten from many client addresses make the next ones count
// double, so three more lock the account out from that client address.
// Counted apart, neither spelling would reach ten.
func TestLoginEmailAndUsernameShareTheLockout(t *testing.T) {
	needDB(t)
	_, username, address := emailAccount(t, login(t, "admin", "admin"))
	srv := api.New(testDB, t.TempDir(), time.UTC, quiet())
	proxies, err := config.ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	srv.SetTrustedProxies(proxies)
	srv.TestHooks().SkipDelays()
	ts := httptest.NewServer(srv.Handler(http.NotFoundHandler()))
	defer ts.Close()
	try := func(from, name, password string) int {
		status, _, _ := call(t, nil, ts.URL, "POST", "/api/login", map[string]string{"username": name, "password": password},
			func(r *http.Request) { r.Header.Set(csrfHeader, "1"); r.Header.Set("X-Forwarded-For", from) })
		return status
	}
	for i := range 10 {
		name := []string{address, username}[i%2]
		if status := try(fmt.Sprintf("203.0.113.%d", 40+i), name, "faux"); status != http.StatusUnauthorized {
			t.Fatalf("guess %d: %d, want 401", i+1, status)
		}
	}
	for i := range 3 {
		if status := try("198.51.100.40", address, "faux"); status != http.StatusUnauthorized {
			t.Fatalf("guess %d from one client: %d, want 401", i+1, status)
		}
	}
	if status := try("198.51.100.40", username, emailPassword); status != http.StatusTooManyRequests {
		t.Fatalf("right password after mixed guesses: %d, want 429", status)
	}
	if status := try("198.51.100.41", address, emailPassword); status != http.StatusOK {
		t.Fatalf("owner with the address from another client: %d, want 200", status)
	}
}
