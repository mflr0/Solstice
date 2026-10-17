package tests

// The audit entries of the events added with the four new actions: issue
// deletion, CSV export, sign-in lockout and refused access tokens.

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/config"
)

// Issue deletions are audited, from the interface and from the REST API
// (which goes through the same handler).
func TestIssueDeletionIsAudited(t *testing.T) {
	admin := login(t, "admin", "admin")
	key := createIssue(t, admin, "DEMO", "Suppression auditée", "")
	before, _ := latestAudit(t, "issue.delete")
	admin.call("DELETE", "/api/issues/"+key, nil, 204, nil)
	n, _ := latestAudit(t, "issue.delete")
	if n != before+1 {
		t.Fatalf("interface deletion: %d entries (was %d)", n, before)
	}
	if label := pick(t, `SELECT target_label FROM audit_log WHERE action = 'issue.delete' ORDER BY id DESC LIMIT 1`); label != key {
		t.Errorf("target label %q, want %s", label, key)
	}
	if who := pick(t, `SELECT COALESCE(username, '') FROM audit_log WHERE action = 'issue.delete' ORDER BY id DESC LIMIT 1`); who != "admin" {
		t.Errorf("author %q, want admin", who)
	}

	// The REST API goes through the interface's handler: one entry, once.
	rest := createIssue(t, admin, "DEMO", "Suppression auditée par l'API", "")
	if status, _, _ := call(t, nil, testURL, "DELETE", "/rest/api/2/issue/"+rest, nil, basic("admin", "admin")); status != 204 {
		t.Fatalf("REST deletion: %d", status)
	}
	if n2, _ := latestAudit(t, "issue.delete"); n2 != n+1 {
		t.Errorf("REST deletion: %d entries (was %d)", n2, n)
	}
}

// A CSV export is audited with the search, never with the exported rows.
func TestCSVExportIsAudited(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	key := createIssue(t, paul, "DEMO", "Export audité", "")
	before, _ := latestAudit(t, "issue.export")
	query := "jql=" + url.QueryEscape("key = "+key)
	if status, _, _ := exportCSVFor(t, paul, query, ""); status != 200 {
		t.Fatalf("export: %d", status)
	}
	n, details := latestAudit(t, "issue.export")
	if n != before+1 || !strings.Contains(details, key) {
		t.Fatalf("export: %d entries (was %d), details %s", n, before, details)
	}
	if strings.Contains(details, "Export audité") {
		t.Errorf("the exported rows reached the audit log: %s", details)
	}
	if who := pick(t, `SELECT COALESCE(username, '') FROM audit_log WHERE action = 'issue.export' ORDER BY id DESC LIMIT 1`); who != "paul.martin" {
		t.Errorf("author %q, want paul.martin", who)
	}
	if !strings.Contains(details, `"rows":1`) {
		t.Errorf("the number of rows is missing: %s", details)
	}
	// A whole project, with no query, is audited by its key and its size.
	before, _ = latestAudit(t, "issue.export")
	status, header, _ := exportCSVFor(t, paul, "project=demo", "")
	if status != 200 {
		t.Fatalf("project export: %d", status)
	}
	n, details = latestAudit(t, "issue.export")
	if n != before+1 || !strings.Contains(details, `"project":"DEMO"`) ||
		!strings.Contains(details, `"rows":`+header.Get("X-Total-Count")) {
		t.Errorf("project export: %d entries (was %d), details %s, %s rows", n, before, details, header.Get("X-Total-Count"))
	}
	// A refused export (an unknown column) leaves no entry.
	bad, _ := latestAudit(t, "issue.export")
	if status, _, _ := exportCSVFor(t, paul, "columns=key,couleur", ""); status != 400 {
		t.Fatalf("unknown column: %d", status)
	}
	if n2, _ := latestAudit(t, "issue.export"); n2 != bad {
		t.Errorf("a refused export was recorded: %d entries (was %d)", n2, bad)
	}
}

// An address that keeps failing is locked: one audit entry when the lock
// starts, none for the attempts it refuses while it lasts.
func TestSignInLockoutIsAudited(t *testing.T) {
	srv, base := newServer(t)
	proxies, err := config.ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	srv.SetTrustedProxies(proxies)
	try := func(password string) int {
		t.Helper()
		setup := func(r *http.Request) {
			r.Header.Set(csrfHeader, "1")
			r.Header.Set("X-Forwarded-For", "203.0.113.7")
		}
		status, _, _ := call(t, nil, base, "POST", "/api/login",
			map[string]string{"username": "hugo.robert", "password": password}, setup)
		return status
	}
	before, _ := latestAudit(t, "login.lockout")
	for i := range 5 {
		if status := try("faux"); status != http.StatusUnauthorized {
			t.Fatalf("guess %d: %d, want 401", i+1, status)
		}
	}
	n, details := latestAudit(t, "login.lockout")
	if n != before+1 || !strings.Contains(details, "seconds") {
		t.Fatalf("lockout: %d entries (was %d), details %s", n, before, details)
	}
	if who := pick(t, `SELECT COALESCE(username, '') FROM audit_log WHERE action = 'login.lockout' ORDER BY id DESC LIMIT 1`); who != "hugo.robert" {
		t.Errorf("lockout name %q, want hugo.robert", who)
	}
	if addr := pick(t, `SELECT COALESCE(ip, '') FROM audit_log WHERE action = 'login.lockout' ORDER BY id DESC LIMIT 1`); addr != "203.0.113.7" {
		t.Errorf("lockout address %q, want 203.0.113.7", addr)
	}
	// While locked, even the right password is refused, and nothing more is recorded.
	if status := try("motdepasse"); status != http.StatusTooManyRequests {
		t.Fatalf("locked address with the right password: %d, want 429", status)
	}
	if n2, _ := latestAudit(t, "login.lockout"); n2 != n {
		t.Errorf("a refused attempt while locked was recorded: %d entries (was %d)", n2, n)
	}
}

// A token that exists but can no longer be used gets one audit entry, naming
// its id and name and never the token; an unknown string is only logged.
func TestRefusedTokensAreAudited(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	token := newToken(t, "paul.martin", &past)
	before, _ := latestAudit(t, "token.reject")
	if status, _, _ := call(t, nil, testURL, "GET", "/api/me", nil, bearer(token)); status != http.StatusUnauthorized {
		t.Fatalf("expired token: %d, want 401", status)
	}
	n, _ := latestAudit(t, "token.reject")
	if n != before+1 {
		t.Fatalf("refused token: %d entries (was %d)", n, before)
	}
	const row = `SELECT COALESCE(target_label, '') FROM audit_log WHERE action = 'token.reject' ORDER BY id DESC LIMIT 1`
	if label := pick(t, row); label != "test paul.martin" {
		t.Errorf("target label %q, want the token name", label)
	}
	if id := pick(t, `SELECT COALESCE(target_id, '') FROM audit_log WHERE action = 'token.reject' ORDER BY id DESC LIMIT 1`); id == "" {
		t.Error("target id empty")
	}
	if who := pick(t, `SELECT COALESCE(username, '') FROM audit_log WHERE action = 'token.reject' ORDER BY id DESC LIMIT 1`); who != "paul.martin" {
		t.Errorf("owner %q, want paul.martin", who)
	}
	var recorded string
	if err := testDB.QueryRow(`SELECT CONCAT(COALESCE(target_id, ''), COALESCE(target_label, ''), COALESCE(details, ''),
		COALESCE(username, '')) FROM audit_log WHERE action = 'token.reject' ORDER BY id DESC LIMIT 1`).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(recorded, token) {
		t.Fatal("the token reached the audit log")
	}

	// The same token within the hour adds nothing...
	if status, _, _ := call(t, nil, testURL, "GET", "/api/me", nil, bearer(token)); status != http.StatusUnauthorized {
		t.Fatalf("expired token again: %d", status)
	}
	if n2, _ := latestAudit(t, "token.reject"); n2 != n {
		t.Errorf("a repeated presentation was recorded: %d entries (was %d)", n2, n)
	}
	// ...nor does a string that matches no token at all.
	if status, _, _ := call(t, nil, testURL, "GET", "/api/me", nil, bearer("sol_"+strings.Repeat("x", 40))); status != http.StatusUnauthorized {
		t.Fatalf("unknown token: %d", status)
	}
	if n3, _ := latestAudit(t, "token.reject"); n3 != n {
		t.Errorf("an unknown token was recorded: %d entries (was %d)", n3, n)
	}
}

// logBuffer is a log destination that the server and the test can share.
type logBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// A string that matches no token is logged at Debug level, so that anyone
// can send one without filling the log; a known token that is refused is a
// warning. Neither line holds the token.
func TestRefusedTokenLogLevels(t *testing.T) {
	needDB(t)
	logs := &logBuffer{}
	srv := api.New(testDB, t.TempDir(), time.UTC, slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	ts := httptest.NewServer(srv.Handler(http.NotFoundHandler()))
	t.Cleanup(ts.Close)

	unknown := "sol_" + strings.Repeat("y", 40)
	if status, _, _ := call(t, nil, ts.URL, "GET", "/api/me", nil, bearer(unknown)); status != http.StatusUnauthorized {
		t.Fatalf("unknown token: %d, want 401", status)
	}
	past := time.Now().Add(-time.Hour)
	expired := newToken(t, "paul.martin", &past)
	if status, _, _ := call(t, nil, ts.URL, "GET", "/api/me", nil, bearer(expired)); status != http.StatusUnauthorized {
		t.Fatalf("expired token: %d, want 401", status)
	}
	var sawUnknown, sawKnown bool
	for _, line := range strings.Split(logs.String(), "\n") {
		if !strings.Contains(line, "API token rejected") {
			continue
		}
		if strings.Contains(line, unknown) || strings.Contains(line, expired) {
			t.Errorf("a token reached the log: %s", line)
		}
		switch {
		case strings.Contains(line, "reason=unknown"):
			sawUnknown = true
			if !strings.Contains(line, "level=DEBUG") {
				t.Errorf("unknown token not logged at Debug: %s", line)
			}
		case strings.Contains(line, "owner=paul.martin"):
			sawKnown = true
			if !strings.Contains(line, "level=WARN") {
				t.Errorf("refused token not logged at Warn: %s", line)
			}
		}
	}
	if !sawUnknown || !sawKnown {
		t.Errorf("log lines missing (unknown %v, known %v):\n%s", sawUnknown, sawKnown, logs.String())
	}
}

// The log line of a lockout names the address and, if the typed name matches
// an account, its id: never the typed text, which may be a password.
func TestLockoutLogLineKeepsTypedNameOut(t *testing.T) {
	needDB(t)
	logs := &logBuffer{}
	srv := api.New(testDB, t.TempDir(), time.UTC, slog.New(slog.NewTextHandler(logs, nil)))
	proxies, err := config.ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	srv.SetTrustedProxies(proxies)
	ts := httptest.NewServer(srv.Handler(http.NotFoundHandler()))
	t.Cleanup(ts.Close)
	lockOut := func(address, typed string) {
		t.Helper()
		setup := func(r *http.Request) {
			r.Header.Set(csrfHeader, "1")
			r.Header.Set("X-Forwarded-For", address)
		}
		for range 5 {
			call(t, nil, ts.URL, "POST", "/api/login", map[string]string{"username": typed, "password": "faux"}, setup)
		}
	}
	lockLine := func(address string) string {
		t.Helper()
		for _, line := range strings.Split(logs.String(), "\n") {
			if strings.Contains(line, "sign-in address locked out") && strings.Contains(line, "address="+address) {
				return line
			}
		}
		t.Fatalf("no lockout line for %s in:\n%s", address, logs.String())
		return ""
	}

	lockOut("203.0.113.21", "Hugo.Robert")
	id := pick(t, `SELECT id FROM users WHERE username = 'hugo.robert'`)
	known := lockLine("203.0.113.21")
	if !strings.Contains(known, " user="+id) || strings.Contains(strings.ToLower(known), "robert") || strings.Contains(known, "username") {
		t.Errorf("lockout of a known account (id %s): %s", id, known)
	}
	lockOut("203.0.113.22", "Tapé-Par-Erreur")
	unknown := lockLine("203.0.113.22")
	if strings.Contains(unknown, "user=") || strings.Contains(strings.ToLower(unknown), "tap") {
		t.Errorf("lockout of an unknown name: %s", unknown)
	}
}
