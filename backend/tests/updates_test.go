package tests

// The update endpoints: release checks against a fake GitHub, and requests
// to the update helper in a directory of the test's own.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"solstice/internal/api"
)

// githubRelease is GitHub's answer for the latest release, trimmed.
const githubRelease = `{
  "url": "https://api.github.com/repos/mflr0/Solstice/releases/1",
  "html_url": "https://github.com/mflr0/Solstice/releases/tag/v1.2.0",
  "tag_name": "v1.2.0",
  "name": "Solstice 1.2.0",
  "draft": false,
  "prerelease": false,
  "published_at": "2026-10-01T09:30:00Z",
  "body": "## Nouveautés\n\n- Mises à jour depuis l'interface\n- <script>alert(1)</script>",
  "assets": [
    {"name": "solstice-1.2.0-linux-amd64.tar.gz", "size": 12345,
     "browser_download_url": "https://github.com/mflr0/Solstice/releases/download/v1.2.0/solstice-1.2.0-linux-amd64.tar.gz"},
    {"name": "SHA256SUMS", "size": 400,
     "browser_download_url": "https://github.com/mflr0/Solstice/releases/download/v1.2.0/SHA256SUMS"},
    {"name": "solstice-1.2.0-linux-arm64.tar.gz", "size": 1,
     "browser_download_url": "https://evil.example/solstice-1.2.0-linux-arm64.tar.gz"},
    {"name": "../../etc/passwd", "size": 1,
     "browser_download_url": "https://github.com/mflr0/Solstice/releases/download/v1.2.0/x"}
  ]
}`

type updatesFixture struct {
	t    *testing.T
	srv  *api.Server
	url  string
	hits atomic.Int32
	tag  atomic.Value
	mu   sync.Mutex
	now  time.Time
	dir  string
}

func (f *updatesFixture) login(username, password string) *client {
	f.t.Helper()
	return loginAt(f.t, f.url, username, password)
}

// state does a request that must answer want, and decodes the update state.
func state(c *client, method, path string, body any, want int) updateStateOut {
	c.t.Helper()
	var out updateStateOut
	c.call(method, path, body, want, &out)
	return out
}

func (f *updatesFixture) clock() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *updatesFixture) advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.mu.Unlock()
}

// newUpdatesFixture starts a server of its own: its own throttling, helper
// directory, clock and GitHub.
func newUpdatesFixture(t *testing.T) *updatesFixture {
	needDB(t)
	f := &updatesFixture{t: t, now: time.Now().UTC(), dir: t.TempDir()}
	f.tag.Store("v1.2.0")
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		io.WriteString(w, strings.ReplaceAll(githubRelease, "v1.2.0", f.tag.Load().(string)))
	}))
	t.Cleanup(gh.Close)

	f.srv = api.New(testDB, t.TempDir(), time.UTC, quiet())
	f.srv.SetVersion("1.0.0")
	f.srv.TestHooks().SetUpdateEnv(api.UpdateEnv{GitHubAPI: gh.URL, HelperDir: f.dir, GOOS: "linux", GOARCH: "amd64",
		DockerEnv: filepath.Join(f.dir, "no-dockerenv"), Getenv: func(string) string { return "" }, Now: f.clock})
	hs := httptest.NewServer(f.srv.Handler(http.NotFoundHandler()))
	t.Cleanup(hs.Close)
	f.url = hs.URL

	clean := func() { testDB.Exec(`DELETE FROM settings WHERE name LIKE 'updates.%'`) }
	clean()
	t.Cleanup(clean)
	return f
}

func TestUpdatesAdminOnly(t *testing.T) {
	f := newUpdatesFixture(t)
	anon := anonymousAt(t, f.url)
	user := f.login("chloe.simon", "motdepasse")
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/admin/updates"},
		{"POST", "/api/admin/updates/check"},
		{"PUT", "/api/admin/updates/settings"},
		{"POST", "/api/admin/updates/apply"},
	} {
		body := map[string]any{"checksEnabled": false, "version": "1.2.0", "password": "motdepasse"}
		if c.method == "GET" {
			body = nil
		}
		if status, _ := anon.do(c.method, c.path, body); status != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: %d", c.method, c.path, status)
		}
		if status, _ := user.do(c.method, c.path, body); status != http.StatusForbidden {
			t.Errorf("non-admin %s %s: %d", c.method, c.path, status)
		}
	}
	if n := f.hits.Load(); n != 0 {
		t.Errorf("GitHub asked %d times for refused requests", n)
	}

	// Writes need the anti-forgery header, as everywhere else.
	admin := f.login("admin", "admin")
	req, _ := http.NewRequest("POST", f.url+"/api/admin/updates/check", nil)
	resp, err := admin.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("without %s: %d", csrfHeader, resp.StatusCode)
	}
}

func TestUpdatesCheckCache(t *testing.T) {
	f := newUpdatesFixture(t)
	start := time.Now().UTC().Add(-time.Second)
	admin := f.login("admin", "admin")

	st := state(admin, "GET", "/api/admin/updates", nil, 200)
	if st.Current != "1.0.0" || st.Latest == nil || *st.Latest != "1.2.0" || !st.Newer || !st.ChecksEnabled ||
		st.Install != "manual" || st.CanApply || st.Download == "" || len(st.Commands) != 4 || st.CheckedAt == nil {
		t.Fatalf("state = %+v", st)
	}
	state(admin, "GET", "/api/admin/updates", nil, 200)
	if n := f.hits.Load(); n != 1 {
		t.Errorf("two loads asked GitHub %d times", n)
	}

	// Twelve hours later, the next load checks again.
	f.tag.Store("v1.3.0")
	f.advance(11 * time.Hour)
	if st := state(admin, "GET", "/api/admin/updates", nil, 200); *st.Latest != "1.2.0" || f.hits.Load() != 1 {
		t.Errorf("11 h: latest %s, %d checks", *st.Latest, f.hits.Load())
	}
	f.advance(time.Hour)
	if st := state(admin, "GET", "/api/admin/updates", nil, 200); *st.Latest != "1.3.0" || f.hits.Load() != 2 {
		t.Errorf("12 h: latest %s, %d checks", *st.Latest, f.hits.Load())
	}

	// On demand: now, but not twice in a row.
	state(admin, "POST", "/api/admin/updates/check", nil, 200)
	state(admin, "POST", "/api/admin/updates/check", nil, 200)
	if n := f.hits.Load(); n != 3 {
		t.Errorf("two checks in a row asked GitHub %d times in all", n)
	}
	f.advance(time.Minute)
	state(admin, "POST", "/api/admin/updates/check", nil, 200)
	if n := f.hits.Load(); n != 4 {
		t.Errorf("a check a minute later: %d in all", n)
	}

	// Checks off: nothing goes out by itself, what is known stays shown.
	if st := state(admin, "PUT", "/api/admin/updates/settings", map[string]bool{"checksEnabled": false}, 200); st.ChecksEnabled {
		t.Error("checks still on")
	}
	f.advance(48 * time.Hour)
	if st := state(admin, "GET", "/api/admin/updates", nil, 200); f.hits.Load() != 4 || st.Latest == nil {
		t.Errorf("checks off: %d checks, latest %v", f.hits.Load(), st.Latest)
	}
	if status, body := admin.do("PUT", "/api/admin/updates/settings", map[string]any{}); status != 400 {
		t.Errorf("settings without a value: %d %s", status, body)
	}
	state(admin, "PUT", "/api/admin/updates/settings", map[string]bool{"checksEnabled": true}, 200)
	if n := f.hits.Load(); n != 5 {
		t.Errorf("checks back on: %d", n)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'update.settings' AND created_at >= ?`, start); n != "2" {
		t.Errorf("settings changes audited: %s", n)
	}

	// GitHub out of reach: the error shows, the release stays known.
	f.tag.Store("garbage")
	f.advance(time.Minute)
	st = state(admin, "POST", "/api/admin/updates/check", nil, 200)
	if st.CheckError == nil || *st.CheckError != "invalid" || st.Latest == nil || *st.Latest != "1.3.0" {
		t.Errorf("failed check: %+v", st)
	}

	// A development build shows the latest release but offers nothing.
	f.srv.SetVersion("dev")
	if st := state(admin, "GET", "/api/admin/updates", nil, 200); st.Newer || st.CanApply || len(st.Commands) != 0 || st.Latest == nil {
		t.Errorf("dev: %+v", st)
	}
}

func TestUpdatesApply(t *testing.T) {
	f := newUpdatesFixture(t)
	start := time.Now().UTC().Add(-time.Second)
	admin := f.login("admin", "admin")
	apply := map[string]string{"version": "1.2.0", "password": "admin"}

	// Not installed with the helper: instructions, no update.
	if st := state(admin, "GET", "/api/admin/updates", nil, 200); st.CanApply || st.Install != "manual" {
		t.Fatalf("manual: %+v", st)
	}
	if status, body := admin.do("POST", "/api/admin/updates/apply", apply); status != http.StatusConflict {
		t.Errorf("manual apply: %d %s", status, body)
	}

	os.WriteFile(filepath.Join(f.dir, "helper"), []byte("1\n"), 0o644)
	st := state(admin, "GET", "/api/admin/updates", nil, 200)
	if !st.CanApply || st.Install != "systemd" || len(st.Commands) != 0 || st.LastAttempt != nil {
		t.Fatalf("systemd: %+v", st)
	}

	// Only the latest release, and only with the password.
	if status, body := admin.do("POST", "/api/admin/updates/apply", map[string]string{"version": "1.1.0", "password": "admin"}); status != http.StatusConflict {
		t.Errorf("another version: %d %s", status, body)
	}
	if status, body := admin.do("POST", "/api/admin/updates/apply", map[string]string{"version": "1.2.0"}); status != http.StatusForbidden {
		t.Errorf("no password: %d %s", status, body)
	}
	if status, body := admin.do("POST", "/api/admin/updates/apply", map[string]string{"version": "1.2.0", "password": "Admin"}); status != http.StatusForbidden ||
		!strings.Contains(string(body), "Incorrect password.") {
		t.Errorf("wrong password: %d %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "request")); err == nil {
		t.Fatal("a request was written without the password")
	}

	st = state(admin, "POST", "/api/admin/updates/apply", apply, http.StatusAccepted)
	if st.CanApply || st.LastAttempt == nil || st.LastAttempt.Status != "pending" || st.LastAttempt.To != "1.2.0" {
		t.Errorf("after apply: %+v", st)
	}
	var req struct {
		Version     string    `json:"version"`
		RequestedBy string    `json:"requestedBy"`
		At          time.Time `json:"at"`
	}
	raw, err := os.ReadFile(filepath.Join(f.dir, "request"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &req); err != nil || req.Version != "1.2.0" || req.RequestedBy != "admin" ||
		time.Since(req.At) > time.Minute || bytes.Count(raw, []byte("\n")) != 1 {
		t.Errorf("request = %s (%v)", raw, err)
	}
	if status, body := admin.do("POST", "/api/admin/updates/apply", apply); status != http.StatusConflict {
		t.Errorf("second apply while pending: %d %s", status, body)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'update.request' AND target_id = '1.2.0' AND created_at >= ?`, start); n != "1" {
		t.Errorf("request audited %s times", n)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'update.denied' AND created_at >= ?`, start); n != "2" {
		t.Errorf("refusals audited %s times", n)
	}

	// The helper is done: its result goes to the audit log once.
	os.Remove(filepath.Join(f.dir, "request"))
	status := `{"at":"` + f.clock().Format(time.RFC3339) + `","from":"1.0.0","to":"1.2.0","status":"ok","log":"healthy"}`
	os.WriteFile(filepath.Join(f.dir, "status"), []byte(status), 0o644)
	f.srv.SetVersion("1.2.0")
	for range 2 {
		st = state(admin, "GET", "/api/admin/updates", nil, 200)
	}
	if st.Newer || st.CanApply || st.LastAttempt == nil || st.LastAttempt.Status != "ok" {
		t.Errorf("after the update: %+v", st)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'update.result' AND target_id = '1.2.0' AND created_at >= ?`, start); n != "1" {
		t.Errorf("result audited %s times", n)
	}
}

func TestUpdatesPasswordThrottled(t *testing.T) {
	f := newUpdatesFixture(t)
	os.WriteFile(filepath.Join(f.dir, "helper"), []byte("1\n"), 0o644)
	admin := f.login("admin", "admin")
	state(admin, "GET", "/api/admin/updates", nil, 200)
	for i := range 5 {
		if status, body := admin.do("POST", "/api/admin/updates/apply", map[string]string{"version": "1.2.0", "password": "guess"}); status != http.StatusForbidden {
			t.Fatalf("attempt %d: %d %s", i+1, status, body)
		}
	}
	// Locked now, even with the right password.
	if status, body := admin.do("POST", "/api/admin/updates/apply", map[string]string{"version": "1.2.0", "password": "admin"}); status != http.StatusTooManyRequests {
		t.Errorf("after 5 failures: %d %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "request")); err == nil {
		t.Error("a request was written while locked")
	}
}
