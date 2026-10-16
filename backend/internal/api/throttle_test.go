package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"solstice/internal/auth"
)

func quietServer() *Server {
	return New(nil, "", time.UTC, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func fromAddress(addr string) *http.Request {
	r := httptest.NewRequest("POST", "/api/login", nil)
	r.RemoteAddr = addr + ":40000"
	return r
}

// The database ignores trailing spaces and case when it compares user names,
// so every spelling of one account must be counted under the same keys.
func TestThrottleKeysCanonicalUsername(t *testing.T) {
	srv := quietServer()
	r := fromAddress("198.51.100.7")
	wantUser, wantIP := srv.throttleKeys(r, "admin")
	for _, typed := range []string{"admin ", "admin   ", " admin", "  admin  ", "Admin", "ADMIN ", "\tadmin"} {
		user, ip := srv.throttleKeys(r, typed)
		if user != wantUser || ip != wantIP {
			t.Errorf("%q: keys %q, %q; want %q, %q", typed, user, ip, wantUser, wantIP)
		}
	}
	if other, _ := srv.throttleKeys(r, "admin2"); other == wantUser {
		t.Error("two different accounts share a key")
	}
	if other, _ := srv.throttleKeys(fromAddress("198.51.100.8"), "admin"); other == wantUser {
		t.Error("two addresses share an account key")
	}
}

// Five wrong passwords typed with different spellings of the name lock the
// address and the account from this address: trailing spaces do not give
// fresh attempts.
func TestSpellingVariantsShareTheCounters(t *testing.T) {
	srv := quietServer()
	srv.verify = func(context.Context, string, string) (*auth.User, error) { return nil, auth.ErrNoSession }
	r := fromAddress("198.51.100.7")
	for i, typed := range []string{"admin", "admin ", "Admin", " admin", "admin  "} {
		if _, err := srv.checkPassword(r, typed, "wrong"); !errors.Is(err, auth.ErrNoSession) {
			t.Fatalf("attempt %d: %v, want a plain failure", i+1, err)
		}
	}
	var he *httpError
	if _, err := srv.checkPassword(r, "admin   ", "wrong"); !errors.As(err, &he) || he.status != http.StatusTooManyRequests {
		t.Fatalf("sixth spelling: %v, want 429", err)
	}
}
