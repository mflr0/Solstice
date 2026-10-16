package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/config"
)

// Someone guessing a password locks their own address out, not the
// account: its owner signs in from another one.
func TestLoginOwnerSignsInWhileAnotherAddressGuesses(t *testing.T) {
	needDB(t)
	srv := api.New(testDB, t.TempDir(), time.UTC, quiet())
	proxies, err := config.ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	srv.SetTrustedProxies(proxies)
	srv.TestHooks().SkipDelays()
	ts := httptest.NewServer(srv.Handler(http.NotFoundHandler()))
	defer ts.Close()
	try := func(from, password string) int {
		req, _ := http.NewRequest("POST", ts.URL+"/api/login",
			strings.NewReader(`{"username":"hugo.robert","password":"`+password+`"}`))
		req.Header.Set(csrfHeader, "1")
		req.Header.Set("X-Forwarded-For", from)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for i := range 5 {
		if status := try("203.0.113.9", "faux"); status != http.StatusUnauthorized {
			t.Fatalf("guess %d: %d, want 401", i+1, status)
		}
	}
	if status := try("203.0.113.9", "motdepasse"); status != http.StatusTooManyRequests {
		t.Fatalf("guessing address with the right password: %d, want 429", status)
	}
	if status := try("198.51.100.20", "motdepasse"); status != http.StatusOK {
		t.Fatalf("owner from another address: %d, want 200", status)
	}
}
