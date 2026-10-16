package api

// The client address behind a reverse proxy: SOLSTICE_TRUSTED_PROXIES lists
// the proxies whose X-Forwarded-For the server believes, so that sign-in and
// setup throttling count each visitor and not the proxy they share.

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"solstice/internal/config"
)

// parseProxies reads a trusted proxy list as the configuration would.
func parseProxies(t *testing.T, list string) []netip.Prefix {
	t.Helper()
	proxies, err := config.ParseTrustedProxies(list)
	if err != nil {
		t.Fatal(err)
	}
	return proxies
}

func TestClientIP(t *testing.T) {
	for _, tc := range []struct {
		name    string
		trusted string
		remote  string
		xff     []string
		want    string
	}{
		{"no proxy configured", "", "198.51.100.7:1234", []string{"10.0.0.1"}, "198.51.100.7"},
		{"a spoofed header from an untrusted client", "192.0.2.1", "198.51.100.7:1234", []string{"10.0.0.1"},
			"198.51.100.7"},
		{"one trusted proxy", "192.0.2.1", "192.0.2.1:40000", []string{"198.51.100.7"}, "198.51.100.7"},
		{"a chain of two trusted proxies", "192.0.2.1,192.0.2.2", "192.0.2.2:40000",
			[]string{"198.51.100.7, 192.0.2.1"}, "198.51.100.7"},
		{"a CIDR range", "10.0.0.0/8", "10.1.2.3:40000", []string{"198.51.100.7"}, "198.51.100.7"},
		{"IPv6", "2001:db8::1", "[2001:db8::1]:40000", []string{"2001:db8:cafe::7"}, "2001:db8:cafe::7"},
		{"an IPv4-mapped IPv6 connection", "192.0.2.1", "[::ffff:192.0.2.1]:40000", []string{"::ffff:198.51.100.7"},
			"198.51.100.7"},
		{"the rightmost untrusted address wins", "192.0.2.1", "192.0.2.1:40000",
			[]string{"198.51.100.50, 198.51.100.60"}, "198.51.100.60"},
		{"a second header line added by the proxy", "192.0.2.1", "192.0.2.1:40000",
			[]string{"198.51.100.50", "198.51.100.60"}, "198.51.100.60"},
		{"only trusted addresses in the chain", "192.0.2.1,192.0.2.2", "192.0.2.1:40000", []string{"192.0.2.2"},
			"192.0.2.1"},
		{"no header", "192.0.2.1", "192.0.2.1:40000", nil, "192.0.2.1"},
		{"an empty header", "192.0.2.1", "192.0.2.1:40000", []string{" "}, "192.0.2.1"},
		{"a malformed header", "192.0.2.1", "192.0.2.1:40000", []string{"not-an-address"}, "192.0.2.1"},
		{"a spoofed leftmost address is never taken", "192.0.2.1", "192.0.2.1:40000",
			[]string{"10.9.9.9, 198.51.100.7"}, "198.51.100.7"},
		{"a spoofed address before a trusted proxy", "192.0.2.1,192.0.2.2", "192.0.2.2:40000",
			[]string{"10.9.9.9, 198.51.100.7, 192.0.2.1"}, "198.51.100.7"},
		{"a spoofed header from a neighbour of the proxy", "192.0.2.1", "192.0.2.2:40000",
			[]string{"198.51.100.7"}, "192.0.2.2"},
		{"a spoofed header from an address outside the range", "10.0.0.0/8", "11.0.0.1:40000",
			[]string{"198.51.100.7"}, "11.0.0.1"},
		{"a malformed entry next to the proxy hides what is left of it", "192.0.2.1", "192.0.2.1:40000",
			[]string{"198.51.100.7, garbage"}, "192.0.2.1"},
		{"a malformed entry left of the client is not read", "192.0.2.1", "192.0.2.1:40000",
			[]string{"garbage, 198.51.100.7"}, "198.51.100.7"},
		{"empty entries", "192.0.2.1", "192.0.2.1:40000", []string{",,"}, "192.0.2.1"},
		{"an empty entry at the end", "192.0.2.1", "192.0.2.1:40000", []string{"198.51.100.7,"}, "192.0.2.1"},
		{"a port after the client", "192.0.2.1", "192.0.2.1:40000", []string{"198.51.100.7:51000"}, "198.51.100.7"},
		{"an IPv6 client with a port", "192.0.2.1", "192.0.2.1:40000", []string{"[2001:db8:cafe::7]:51000"},
			"2001:db8:cafe::7"},
		{"a bracketed IPv6 client", "192.0.2.1", "192.0.2.1:40000", []string{"[2001:db8:cafe::7]"},
			"2001:db8:cafe::7"},
		{"an IPv6 zone is dropped", "192.0.2.1", "192.0.2.1:40000", []string{"fe80::7%eth0"}, "fe80::7"},
		{"a zone does not dodge a trusted range", "fe80::/10", "[fe80::1%eth0]:40000", []string{"198.51.100.7"},
			"198.51.100.7"},
		{"an IPv6 CIDR range", "2001:db8::/32", "[2001:db8:1::5]:40000", []string{"198.51.100.7"}, "198.51.100.7"},
		{"an IPv6 address outside the range", "2001:db8::/32", "[2001:db9::5]:40000", []string{"198.51.100.7"},
			"2001:db9::5"},
		{"a mapped address in the chain is trusted as its IPv4 form", "192.0.2.1,192.0.2.2", "192.0.2.2:40000",
			[]string{"198.51.100.7, ::ffff:192.0.2.1"}, "198.51.100.7"},
		{"a mapped peer is not a mapped answer", "", "[::ffff:198.51.100.7]:40000", nil, "198.51.100.7"},
		{"a header spread over lines is read as one chain", "192.0.2.1,192.0.2.2", "192.0.2.2:40000",
			[]string{"10.9.9.9", "198.51.100.7", "192.0.2.1"}, "198.51.100.7"},
		{"a connection without a port", "", "198.51.100.7", nil, "198.51.100.7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.remote
			for _, v := range tc.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if got := clientIP(r, parseProxies(t, tc.trusted)); got != tc.want {
				t.Errorf("clientIP(%q, %q) = %q, want %q", tc.remote, tc.xff, got, tc.want)
			}
		})
	}
}

// Two visitors behind one trusted proxy lock out separately: their addresses
// come from X-Forwarded-For, not the proxy's, so five wrong passwords from one
// nobody else behind the proxy out. No database is needed: the lock is
// checked before the password.
func TestLoginThrottlingBehindTrustedProxy(t *testing.T) {
	srv := New(nil, t.TempDir(), time.UTC, slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv.SetTrustedProxies(parseProxies(t, "192.0.2.1"))
	at := func(client string) *http.Request {
		r := httptest.NewRequest("POST", "/api/login", nil)
		r.RemoteAddr = "192.0.2.1:40000"
		r.Header.Set("X-Forwarded-For", client)
		return r
	}
	guess, other := at("198.51.100.10"), at("198.51.100.11")
	userKey, ipKey := srv.throttleKeys(guess, "hugo.robert")
	if ipKey != "ip:198.51.100.10" {
		t.Fatalf("throttled address %s, want the forwarded one", ipKey)
	}
	for range 5 {
		srv.limiter.Failure(userKey, ipKey)
	}
	// The guessing address is locked out, right password or not...
	var he *httpError
	if _, err := srv.checkPassword(guess, "hugo.robert", "motdepasse"); !errors.As(err, &he) ||
		he.status != http.StatusTooManyRequests {
		t.Fatalf("guessing client behind the proxy: %v, want 429", err)
	}
	// ...and another visitor behind the same proxy is not.
	otherUser, otherIP := srv.throttleKeys(other, "marie.durand")
	if d := srv.limiter.Locked(otherUser, otherIP); d != 0 {
		t.Errorf("another visitor behind the proxy locked for %s with the guesser", d)
	}
}

// The setup code is throttled per client behind a trusted proxy too.
func TestSetupUnlockThrottlingBehindTrustedProxy(t *testing.T) {
	s, c := newWizard(t, SetupOptions{TrustedProxies: parseProxies(t, "192.0.2.1")})
	c.ip, c.xff = "192.0.2.1", "198.51.100.10"
	for i := range 5 {
		if status, _ := c.call("POST", "/api/setup/unlock", "", map[string]string{"code": "AAAA-AAAA"}); status != http.StatusForbidden {
			t.Fatalf("wrong code #%d: %d, want 403", i+1, status)
		}
	}
	if status, _ := c.call("POST", "/api/setup/unlock", "", map[string]string{"code": s.Code()}); status != http.StatusTooManyRequests {
		t.Fatalf("first client after five wrong codes: %d, want 429", status)
	}
	// Another client behind the same proxy is counted separately.
	c.xff = "198.51.100.11"
	if status, body := c.call("POST", "/api/setup/unlock", "", map[string]string{"code": s.Code()}); status != http.StatusNoContent {
		t.Fatalf("second client behind the same proxy: %d %v, want 204", status, body)
	}
}

// Without a trusted proxy, a client cannot change the address it is counted
// under by sending X-Forwarded-For: five wrong passwords lock it whatever the
// header says.
func TestLoginThrottlingIgnoresSpoofedHeader(t *testing.T) {
	srv := New(nil, t.TempDir(), time.UTC, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var first string
	for i, forged := range []string{"", "10.0.0.1", "10.0.0.2", "::1"} {
		r := httptest.NewRequest("POST", "/api/login", nil)
		r.RemoteAddr = "198.51.100.7:40000"
		if forged != "" {
			r.Header.Set("X-Forwarded-For", forged)
		}
		_, ipKey := srv.throttleKeys(r, "hugo.robert")
		if i == 0 {
			first = ipKey
		}
		if ipKey != first || ipKey != "ip:198.51.100.7" {
			t.Errorf("X-Forwarded-For %q: counted as %s, want ip:198.51.100.7", forged, ipKey)
		}
	}
}
