package api

import (
	"context"
	"errors"
	"fmt"
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

// fakePasswords is a verify function that knows one account and its password.
func fakePasswords(user, password string) func(context.Context, string, string) (*auth.User, error) {
	return func(_ context.Context, name, typed string) (*auth.User, error) {
		if name == user && typed == password {
			return &auth.User{ID: 1, Username: user}, nil
		}
		return nil, auth.ErrNoSession
	}
}

// Someone guessing the administrator's password from one address locks that
// address, not the account: the owner still signs in, from another address.
func TestAttackerLockedOwnerStillSignsIn(t *testing.T) {
	srv := quietServer()
	srv.verify = fakePasswords("admin", "correct horse")
	srv.sleep = func(context.Context, time.Duration) {}
	attacker, owner := fromAddress("203.0.113.9"), fromAddress("198.51.100.20")

	for i := range 5 {
		if _, err := srv.checkPassword(attacker, "admin", "guess"+string(rune('a'+i))); !errors.Is(err, auth.ErrNoSession) {
			t.Fatalf("guess %d: %v, want a plain failure", i+1, err)
		}
	}
	var he *httpError
	// The attacker is locked, even with the right password...
	if _, err := srv.checkPassword(attacker, "admin", "correct horse"); !errors.As(err, &he) || he.status != http.StatusTooManyRequests {
		t.Fatalf("attacker's address: %v, want 429", err)
	}
	// ...and the owner is not.
	if u, err := srv.checkPassword(owner, "admin", "correct horse"); err != nil || u == nil {
		t.Fatalf("owner from another address: %v, want a sign-in", err)
	}
	// Neither is the owner, who types the name with a trailing space.
	if _, err := srv.checkPassword(owner, "Admin ", "correct horse"); err != nil {
		t.Fatalf("owner, other spelling: %v", err)
	}
}

// Many addresses guessing one account never lock it: the right password
// works at any time, while the wrong ones are held back more and more.
func TestAccountUnderDistributedAttackKeepsAcceptingTheRightPassword(t *testing.T) {
	srv := quietServer()
	srv.verify = fakePasswords("admin", "correct horse")
	var delays []time.Duration
	srv.sleep = func(_ context.Context, d time.Duration) { delays = append(delays, d) }

	for i := range 60 {
		r := fromAddress(fmt.Sprintf("203.0.113.%d", i+1)) // each address tries once or twice
		if _, err := srv.checkPassword(r, "admin", "guess"); !errors.Is(err, auth.ErrNoSession) {
			t.Fatalf("guess %d: %v, want a plain failure", i+1, err)
		}
	}
	if len(delays) == 0 || delays[len(delays)-1] <= delays[0] {
		t.Fatalf("wrong answers not held back more and more: %v", delays)
	}
	owner := fromAddress("198.51.100.20")
	if u, err := srv.checkPassword(owner, "admin", "correct horse"); err != nil || u == nil {
		t.Fatalf("right password under attack: %v", err)
	}
	if n := len(delays); n > 0 {
		before := n
		srv.checkPassword(owner, "admin", "correct horse")
		if len(delays) != before {
			t.Error("the right password was held back")
		}
	}
	// A wrong password typed by the owner is held back like any other.
	before := len(delays)
	if _, err := srv.checkPassword(owner, "admin", "typo"); !errors.Is(err, auth.ErrNoSession) || len(delays) != before+1 {
		t.Fatalf("owner's typo: %v, delays %d -> %d", err, before, len(delays))
	}
}

// Other accounts are not slowed down by an attack on one.
func TestAttackOnOneAccountDoesNotSlowAnother(t *testing.T) {
	srv := quietServer()
	srv.verify = fakePasswords("marie", "pw")
	var delayed int
	srv.sleep = func(context.Context, time.Duration) { delayed++ }
	for i := range 40 {
		srv.checkPassword(fromAddress(fmt.Sprintf("203.0.113.%d", i+1)), "admin", "guess")
	}
	if delayed == 0 {
		t.Fatal("the attacked account is not slowed down")
	}
	delayed = 0
	other := fromAddress("198.51.100.1")
	srv.checkPassword(other, "marie", "typo")
	_, err := srv.checkPassword(other, "marie", "pw")
	if delayed != 0 || err != nil {
		t.Fatalf("another account: %d delays, error %v", delayed, err)
	}
}
