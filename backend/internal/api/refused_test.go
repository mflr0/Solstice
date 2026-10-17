package api

// The cap on refused requests: refusedCap lines per client address per
// refusedPeriod, then one notice, and nothing after it until the period
// passes. The clock is a field of refuser so the test moves it.

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	refusedLine   = `msg="request refused"`
	refusedNotice = `msg="further refused requests from this address are not logged for a minute"`
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
		rf.logRefused(r, http.StatusForbidden)
	}
	if got := count(buf.String(), refusedLine); got != refusedCap {
		t.Fatalf("after %d refused requests: %d lines, want %d",
			refusedCap, got, refusedCap)
	}

	// Over the cap: one notice, once, then silence.
	rf.logRefused(r, http.StatusForbidden)
	rf.logRefused(r, http.StatusForbidden)
	rf.logRefused(r, http.StatusForbidden)
	if got := count(buf.String(), refusedNotice); got != 1 {
		t.Errorf("over the cap: %d notices, want 1", got)
	}
	if got := count(buf.String(), refusedLine); got != refusedCap {
		t.Errorf("over the cap: %d refused lines, want %d", got, refusedCap)
	}

	// The cap is per address: another one is still logged.
	rf.logRefused(refusedRequest("198.51.100.9:41000"), http.StatusUnauthorized)
	if got := count(buf.String(), refusedLine); got != refusedCap+1 {
		t.Errorf("a second address: %d refused lines, want %d", got, refusedCap+1)
	}

	// Once the period has passed, the first address is logged again.
	moveTime(rf, start.Add(refusedPeriod))
	rf.logRefused(r, http.StatusForbidden)
	if got := count(buf.String(), refusedLine); got != refusedCap+2 {
		t.Errorf("after the period: %d refused lines, want %d", got, refusedCap+2)
	}
}

func TestRefusedLineFields(t *testing.T) {
	var buf bytes.Buffer
	rf := newRefuser(&buf, time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC))
	r := httptest.NewRequest(http.MethodPost,
		"http://example.test/rest/api/2/issue?token=secret-jql%20value", nil)
	r.RemoteAddr = "192.0.2.8:41000"
	r.Header.Set("Authorization", "Bearer sneaky-token")
	r.Header.Set("Cookie", "solstice_session=sneaky-cookie")

	rf.logRefused(r, http.StatusUnauthorized)

	line := buf.String()
	for _, want := range []string{
		"level=WARN",
		refusedLine,
		"status=401",
		"method=POST",
		"path=/rest/api/2/issue", // the path, without the query string
		"client=192.0.2.8",
		`user=""`, // no signed-in user, so an empty one
	} {
		if !strings.Contains(line, want) {
			t.Errorf("logged line lacks %q:\n%s", want, line)
		}
	}
	for _, no := range []string{
		"token=", "secret-jql", "?", // never the query string
		"sneaky-token", "sneaky-cookie", // never a header or cookie
	} {
		if strings.Contains(line, no) {
			t.Errorf("logged line holds %q:\n%s", no, line)
		}
	}
}
