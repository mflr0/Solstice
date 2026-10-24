package api

// The client address behind a reverse proxy: SOLSTICE_HTTP_TRUSTEDPROXIES lists
// the proxies whose X-Forwarded-For the server believes, so that sign-in and
// setup throttling count each visitor and not the proxy they share.

import (
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"solstice/internal/config"
	"solstice/internal/jirafixture"
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
	if _, err := srv.checkPassword(guess, "hugo.robert", jirafixture.Password); !errors.As(err, &he) ||
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

func TestSecureRequest(t *testing.T) {
	for _, tc := range []struct {
		name    string
		tls     bool
		remote  string
		proto   string
		baseURL string
		want    bool
	}{
		{"direct TLS", true, "198.51.100.7:40000", "", "", true},
		{"header from a trusted proxy", false, "192.0.2.1:40000", "https", "", true},
		{"http header from a trusted proxy", false, "192.0.2.1:40000", "http", "", false},
		{"header from an untrusted peer", false, "198.51.100.7:40000", "https", "", false},
		{"https base URL", false, "198.51.100.7:40000", "", "https://example.test", true},
		{"http base URL", false, "198.51.100.7:40000", "", "http://example.test", false},
		{"plain request", false, "198.51.100.7:40000", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tc.remote
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if tc.proto != "" {
				r.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if got := secureRequest(r, tc.baseURL, parseProxies(t, "192.0.2.0/24"), slog.New(slog.DiscardHandler)); got != tc.want {
				t.Errorf("secureRequest = %v, want %v", got, tc.want)
			}
		})
	}
}

// A TLS proxy missing from the trusted list is told once, then once an hour,
// and never when the header is not believed for no such reason.
func TestUntrustedProtoWarning(t *testing.T) {
	var out strings.Builder
	log := slog.New(slog.NewTextHandler(&out, nil))
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	protoWarn.last, protoWarn.now = time.Time{}, func() time.Time { return now }
	t.Cleanup(func() { protoWarn.last, protoWarn.now = time.Time{}, nil })

	request := func(remote, proto string) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remote
		if proto != "" {
			r.Header.Set("X-Forwarded-Proto", proto)
		}
		secureRequest(r, "", parseProxies(t, "192.0.2.0/24"), log)
	}
	warnings := func() int { return strings.Count(out.String(), "level=WARN") }

	request("198.51.100.7:40000", "")
	request("198.51.100.7:40000", "http")
	request("192.0.2.1:40000", "https")
	if warnings() != 0 {
		t.Fatalf("no untrusted https claim, yet: %s", out.String())
	}
	request("198.51.100.7:40000", "https")
	if warnings() != 1 {
		t.Fatalf("first untrusted https claim: %d warnings, want 1: %s", warnings(), out.String())
	}
	for _, name := range []string{"SOLSTICE_HTTP_TRUSTEDPROXIES", "SOLSTICE_HTTP_BASEURL"} {
		if !strings.Contains(out.String(), name) {
			t.Errorf("the warning does not name %s: %s", name, out.String())
		}
	}
	now = now.Add(protoWarnEvery - time.Second)
	request("198.51.100.8:40000", "https")
	if warnings() != 1 {
		t.Errorf("within the hour: %d warnings, want 1", warnings())
	}
	now = now.Add(time.Second)
	request("198.51.100.8:40000", "https")
	if warnings() != 2 {
		t.Errorf("after the hour: %d warnings, want 2", warnings())
	}
}
