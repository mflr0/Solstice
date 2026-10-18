package api

// The cap on refused requests: refusedCap lines per client address per
// refusedPeriod, then one notice, and nothing after it until the period
// passes. The clock is a field of refuser so the test moves it.

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"solstice/internal/auth"
)

const (
	refusedLine   = `msg="request refused"`
	refusedNotice = `msg="further refused requests from this address are not logged until the period ends"`

	refusedTotalNotice = `msg="further refused requests are not logged until the period ends"`
)

// newRefuser is a refuser logging into buf, its clock at the instant the
// test chooses.
func newRefuser(buf *bytes.Buffer, now time.Time) *refuser {
	clock := now
	return &refuser{
		log: slog.New(slog.NewTextHandler(buf, nil)),
		now: func() time.Time { return clock },
	}
}

// moveTime hands the test a setter for the refuser's clock.
func moveTime(rf *refuser, now time.Time) {
	rf.now = func() time.Time { return now }
}

func refusedRequest(addr string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "http://example.test/issue/17", nil)
	r.RemoteAddr = addr
	return r
}

func count(s, sub string) int { return strings.Count(s, sub) }

func TestRefusedCapPerAddress(t *testing.T) {
	var buf bytes.Buffer
	start := time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	rf := newRefuser(&buf, start)
	r := refusedRequest("192.0.2.7:41000")

	for range refusedCap {
		rf.logRefused(r, http.StatusForbidden, nil)
	}
	if got := count(buf.String(), refusedLine); got != refusedCap {
		t.Fatalf("after %d refused requests: %d lines, want %d",
			refusedCap, got, refusedCap)
	}

	// Over the cap: one notice, once, then silence.
	rf.logRefused(r, http.StatusForbidden, nil)
	rf.logRefused(r, http.StatusForbidden, nil)
	rf.logRefused(r, http.StatusForbidden, nil)
	if got := count(buf.String(), refusedNotice); got != 1 {
		t.Errorf("over the cap: %d notices, want 1", got)
	}
	if got := count(buf.String(), refusedLine); got != refusedCap {
		t.Errorf("over the cap: %d refused lines, want %d", got, refusedCap)
	}

	// The cap is per address: another one is still logged.
	rf.logRefused(refusedRequest("198.51.100.9:41000"), http.StatusForbidden, nil)
	if got := count(buf.String(), refusedLine); got != refusedCap+1 {
		t.Errorf("a second address: %d refused lines, want %d", got, refusedCap+1)
	}

	// Once the period has passed, the first address is logged again.
	moveTime(rf, start.Add(refusedPeriod))
	rf.logRefused(r, http.StatusForbidden, nil)
	if got := count(buf.String(), refusedLine); got != refusedCap+2 {
		t.Errorf("after the period: %d refused lines, want %d", got, refusedCap+2)
	}
}

func TestRefusedLineFields(t *testing.T) {
	var buf bytes.Buffer
	rf := newRefuser(&buf, time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC))
	r := httptest.NewRequest(http.MethodPost,
		"http://example.test/rest/api/2/issue/KEY-17/attachments/report-reset-9f3a.pdf?token=secret-jql%20value", nil)
	r.Pattern = "POST /rest/api/2/issue/{key}/attachments/{name}"
	r.RemoteAddr = "192.0.2.8:41000"
	r.Header.Set("Authorization", "Bearer sneaky-token")
	r.Header.Set("Cookie", "solstice_session=sneaky-cookie")

	rf.logRefused(r, http.StatusUnauthorized, nil)

	line := buf.String()
	for _, want := range []string{
		"level=WARN",
		refusedLine,
		"status=401",
		"method=POST",
		`route="POST /rest/api/2/issue/{key}/attachments/{name}"`, // the pattern
		"client=192.0.2.8",
		`user=""`, // no signed-in user, so an empty one
	} {
		if !strings.Contains(line, want) {
			t.Errorf("logged line lacks %q:\n%s", want, line)
		}
	}
	for _, no := range []string{
		"token=", "secret-jql", "?", // never the query string
		"path=", "KEY-17", "report-reset-9f3a", // nor the path
		"sneaky-token", "sneaky-cookie", // never a header or cookie
	} {
		if strings.Contains(line, no) {
			t.Errorf("logged line holds %q:\n%s", no, line)
		}
	}
}

// Behind a trusted proxy, each client has its own budget and is logged
// by its own address, not the proxy's.
func TestRefusedBehindProxy(t *testing.T) {
	var buf bytes.Buffer
	rf := newRefuser(&buf, time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC))
	proxies := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	viaProxy := func(client string) *http.Request {
		r := refusedRequest("192.0.2.1:41000")
		r.Header.Set("X-Forwarded-For", client)
		return r
	}

	for range refusedCap + 1 {
		rf.logRefused(viaProxy("203.0.113.5"), http.StatusForbidden, proxies)
	}
	rf.logRefused(viaProxy("203.0.113.6"), http.StatusForbidden, proxies)

	line := buf.String()
	if got := count(line, refusedLine); got != refusedCap+1 {
		t.Errorf("%d refused lines, want %d: the second client has its own budget", got, refusedCap+1)
	}
	for _, want := range []string{"client=203.0.113.5", "client=203.0.113.6"} {
		if !strings.Contains(line, want) {
			t.Errorf("logged lines lack %q:\n%s", want, line)
		}
	}
	if strings.Contains(line, "client=192.0.2.1") {
		t.Errorf("a line names the proxy as the client:\n%s", line)
	}
}

// An IPv6 address counts as its /64: changing the low bits does not
// give a new budget, another /64 does.
func TestRefusedIPv6Budget(t *testing.T) {
	var buf bytes.Buffer
	rf := newRefuser(&buf, time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC))

	for i := range refusedCap {
		rf.logRefused(refusedRequest(fmt.Sprintf("[2001:db8:1:2::%x]:41000", i+1)), http.StatusForbidden, nil)
	}
	rf.logRefused(refusedRequest("[2001:db8:1:2:aaaa::7]:41000"), http.StatusForbidden, nil)
	if got := count(buf.String(), refusedLine); got != refusedCap {
		t.Errorf("one /64: %d refused lines, want %d", got, refusedCap)
	}
	if got := count(buf.String(), refusedNotice); got != 1 {
		t.Errorf("one /64 over the cap: %d notices, want 1", got)
	}

	rf.logRefused(refusedRequest("[2001:db8:1:3::1]:41000"), http.StatusForbidden, nil)
	if got := count(buf.String(), refusedLine); got != refusedCap+1 {
		t.Errorf("another /64: %d refused lines, want %d", got, refusedCap+1)
	}
}

// All addresses together get refusedTotal lines per period, then one
// notice; the map of counts never outgrows that.
func TestRefusedTotalCap(t *testing.T) {
	var buf bytes.Buffer
	start := time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	rf := newRefuser(&buf, start)
	from := func(i int) *http.Request {
		return refusedRequest(fmt.Sprintf("10.%d.%d.%d:41000", i>>16&255, i>>8&255, i&255))
	}

	for i := range 3 * refusedTotal {
		rf.logRefused(from(i), http.StatusForbidden, nil)
	}
	if got := count(buf.String(), refusedLine); got != refusedTotal {
		t.Errorf("%d refused lines, want %d", got, refusedTotal)
	}
	if got := count(buf.String(), refusedTotalNotice); got != 1 {
		t.Errorf("%d notices, want 1", got)
	}
	if got := len(rf.count); got > refusedTotal {
		t.Errorf("%d addresses kept, want at most %d", got, refusedTotal)
	}

	moveTime(rf, start.Add(refusedPeriod))
	rf.logRefused(from(0), http.StatusForbidden, nil)
	if got := count(buf.String(), refusedLine); got != refusedTotal+1 {
		t.Errorf("after the period: %d refused lines, want %d", got, refusedTotal+1)
	}
}

// A 403 is always logged; a 401 only when the request sent a session
// cookie or an Authorization header, and an ignored 401 spends no budget.
func TestRefusedUnauthorizedNeedsCredentials(t *testing.T) {
	var buf bytes.Buffer
	rf := newRefuser(&buf, time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC))
	withHeader := func(name, value string) *http.Request {
		r := refusedRequest("192.0.2.7:41000")
		r.Header.Set(name, value)
		return r
	}

	for range 2 * refusedCap { // anonymous visitors: nothing logged, no budget spent
		rf.logRefused(refusedRequest("192.0.2.7:41000"), http.StatusUnauthorized, nil)
	}
	if buf.Len() != 0 {
		t.Fatalf("anonymous 401s were logged:\n%s", buf.String())
	}
	if len(rf.count) != 0 {
		t.Errorf("anonymous 401s spent budget: %v", rf.count)
	}

	rf.logRefused(withHeader("Authorization", "Bearer x"), http.StatusUnauthorized, nil)
	rf.logRefused(withHeader("Cookie", auth.CookieName+"=x"), http.StatusUnauthorized, nil)
	rf.logRefused(withHeader("Cookie", "other=x"), http.StatusUnauthorized, nil)
	if got := count(buf.String(), refusedLine); got != 2 {
		t.Errorf("401s with credentials: %d lines, want 2", got)
	}

	rf.logRefused(refusedRequest("192.0.2.7:41000"), http.StatusForbidden, nil)
	if got := count(buf.String(), refusedLine); got != 3 {
		t.Errorf("a 403 without credentials: %d lines, want 3", got)
	}
}
