package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseVersion(t *testing.T) {
	for _, s := range []string{"1.0.0", "v1.0.0", "0.0.1", "10.20.30", "1.0.0-rc.1", "1.0.0-alpha-2.x"} {
		if _, ok := parseVersion(s); !ok {
			t.Errorf("parseVersion(%q) refused", s)
		}
	}
	for _, s := range []string{"", "dev", "1.0", "1.0.0.0", "01.0.0", "1.0.0+build", "1.0.0-", "1.0.0-rc..1",
		"1.0.0-01", "1.0.0;rm -rf /", "1.0.0\n2.0.0", " 1.0.0", "vv1.0.0", "1.0.0-" + strings.Repeat("a", 64)} {
		if _, ok := parseVersion(s); ok {
			t.Errorf("parseVersion(%q) accepted", s)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	// Each is older than the next (semver.org's precedence example, then more).
	ordered := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2",
		"1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "1.10.0", "2.0.0"}
	for i := range ordered {
		for j := range ordered {
			a, _ := parseVersion(ordered[i])
			b, _ := parseVersion(ordered[j])
			want := 0
			if i < j {
				want = -1
			} else if i > j {
				want = 1
			}
			if got := a.compare(b); got != want {
				t.Errorf("compare(%s, %s) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
	for _, c := range []struct {
		current, latest string
		newer           bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.0", "v1.1.0", true},
		{"v1.0.0", "1.0.0", false},
		{"1.2.0", "1.1.9", false},
		{"1.1.0-rc.1", "1.1.0", true},
		{"1.1.0", "1.1.0-rc.2", false},
		{"dev", "9.9.9", false}, // a development build never offers an update
		{"", "1.0.0", false},
		{"1.0.0", "garbage", false},
	} {
		if got := newerVersion(c.current, c.latest); got != c.newer {
			t.Errorf("newerVersion(%q, %q) = %v", c.current, c.latest, got)
		}
	}
}

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

func TestParseRelease(t *testing.T) {
	rel, err := parseRelease(strings.NewReader(githubRelease))
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "1.2.0" || rel.URL != "https://github.com/mflr0/Solstice/releases/tag/v1.2.0" {
		t.Errorf("release = %+v", rel)
	}
	if rel.PublishedAt == nil || !rel.PublishedAt.Equal(time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("published at %v", rel.PublishedAt)
	}
	// Notes are kept as written: the interface shows them as text.
	if !strings.Contains(rel.Notes, "- Mises à jour depuis l'interface") || !strings.Contains(rel.Notes, "<script>") {
		t.Errorf("notes = %q", rel.Notes)
	}
	if len(rel.Assets) != 2 || rel.Assets[0].Name != "solstice-1.2.0-linux-amd64.tar.gz" || rel.Assets[1].Name != "SHA256SUMS" {
		t.Errorf("assets = %+v", rel.Assets)
	}

	for name, body := range map[string]string{
		"draft":       `{"tag_name": "v1.3.0", "draft": true}`,
		"no v":        `{"tag_name": "1.3.0"}`,
		"bad version": `{"tag_name": "v1.3"}`,
		"injection":   `{"tag_name": "v1.3.0;reboot"}`,
		"not json":    `<html>`,
	} {
		if _, err := parseRelease(strings.NewReader(body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// A page elsewhere than the repository's releases is not linked.
	rel, err = parseRelease(strings.NewReader(`{"tag_name": "v1.3.0", "html_url": "https://evil.example/"}`))
	if err != nil || rel.URL != "https://github.com/mflr0/Solstice/releases/tag/v1.3.0" {
		t.Errorf("url = %q, %v", rel.URL, err)
	}
}

func TestFetchLatest(t *testing.T) {
	var status atomic.Int32
	var body atomic.Value
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/mflr0/Solstice/releases/latest" || r.Header.Get("User-Agent") != "Solstice" ||
			r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("unexpected request %s %v", r.URL.Path, r.Header)
		}
		w.WriteHeader(int(status.Load()))
		io.WriteString(w, body.Load().(string))
	}))
	defer gh.Close()
	now := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	u := &updater{apiBase: gh.URL, now: func() time.Time { return now }}

	for _, c := range []struct {
		status        int
		body, version string
		failure       string
	}{
		{200, githubRelease, "1.2.0", ""},
		{404, `{"message": "Not Found"}`, "", ""}, // nothing published yet: not an error
		{403, `{"message": "API rate limit exceeded"}`, "", checkRefused},
		{200, `{"tag_name": "nope"}`, "", checkInvalid},
	} {
		status.Store(int32(c.status))
		body.Store(c.body)
		rel, _ := u.fetchLatest(t.Context())
		if rel.Version != c.version || rel.Error != c.failure || !rel.CheckedAt.Equal(now) {
			t.Errorf("HTTP %d: %+v", c.status, rel)
		}
	}

	gh.Close()
	if rel, err := u.fetchLatest(t.Context()); rel.Error != checkUnreachable || err == nil {
		t.Errorf("unreachable: %+v %v", rel, err)
	}
}

func TestUpdateInstall(t *testing.T) {
	dir := t.TempDir()
	docker := filepath.Join(t.TempDir(), "dockerenv")
	env := map[string]string{}
	u := &updater{dir: dir, dockerEnv: docker, goos: "linux", getenv: func(k string) string { return env[k] }}
	if got := u.install(); got != installManual {
		t.Errorf("nothing: %s", got)
	}
	env["SOLSTICE_INSTALL"] = "docker"
	if got := u.install(); got != installDocker {
		t.Errorf("SOLSTICE_INSTALL=docker: %s", got)
	}
	delete(env, "SOLSTICE_INSTALL")
	os.WriteFile(docker, nil, 0o644)
	if got := u.install(); got != installDocker {
		t.Errorf("/.dockerenv: %s", got)
	}
	// The helper's marker wins: a container never has it.
	os.WriteFile(filepath.Join(dir, "helper"), []byte("1\n"), 0o644)
	if got := u.install(); got != installSystemd {
		t.Errorf("helper: %s", got)
	}
	u.goos = "darwin"
	if got := u.install(); got != installDocker {
		t.Errorf("helper marker off Linux: %s", got)
	}
}

func TestUpdateRequestFile(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	u := &updater{version: "1.0.0", dir: dir, now: func() time.Time { return now }}
	if a := u.lastAttempt(); a != nil {
		t.Fatalf("no attempt yet: %+v", a)
	}
	if err := u.writeRequest(updateRequest{Version: "1.2.0", RequestedBy: "admin", At: now}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "request"))
	if err != nil {
		t.Fatal(err)
	}
	// One line of JSON: the helper reads the version and nothing else.
	if want := `{"version":"1.2.0","requestedBy":"admin","at":"2026-10-15T08:00:00Z"}` + "\n"; string(raw) != want {
		t.Errorf("request = %q, want %q", raw, want)
	}
	if fi, _ := os.Stat(filepath.Join(dir, "request")); fi.Mode().Perm() != 0o640 {
		t.Errorf("mode %v", fi.Mode())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("left behind: %v", entries)
	}
	a := u.lastAttempt()
	if a == nil || a.Status != attemptPending || a.From != "1.0.0" || a.To != "1.2.0" || !u.busy(a) {
		t.Errorf("pending: %+v", a)
	}

	// The helper took it up: its status wins over a request.
	writeStatus := func(s string) {
		os.WriteFile(filepath.Join(dir, "status"), []byte(s), 0o644)
	}
	writeStatus(`{"at":"2026-10-15T08:00:01Z","from":"1.0.0","to":"1.2.0","status":"running","log":"downloading"}`)
	if a := u.lastAttempt(); a == nil || a.Status != attemptRunning || a.Log != "downloading" || !u.busy(a) {
		t.Errorf("running: %+v", a)
	}
	os.Remove(filepath.Join(dir, "request"))
	writeStatus(`{"at":"2026-10-15T08:00:01Z","from":"1.0.0","to":"1.2.0","status":"rolled_back","log":"unhealthy"}`)
	if a := u.lastAttempt(); a == nil || a.Status != attemptRolledBack || u.busy(a) {
		t.Errorf("rolled back: %+v", a)
	}
	writeStatus(`{"status":"pwned","to":"1.2.0"}`)
	if a := u.lastAttempt(); a != nil {
		t.Errorf("unknown status accepted: %+v", a)
	}
	// A run that never finished (the machine rebooted) does not block forever.
	writeStatus(`{"at":"2026-10-15T07:00:00Z","from":"1.0.0","to":"1.2.0","status":"running","log":""}`)
	if a := u.lastAttempt(); a == nil || u.busy(a) {
		t.Errorf("stale run: %+v", a)
	}
}

func TestUpdateInstructions(t *testing.T) {
	u := &updater{goos: "linux", goarch: "arm64"}
	cmds, download := u.instructions(installManual, "1.2.0")
	if download != "https://github.com/mflr0/Solstice/releases/download/v1.2.0/solstice-1.2.0-linux-arm64.tar.gz" ||
		len(cmds) != 4 || cmds[3] != "sudo ./install.sh" || cmds[2] != "cd solstice-1.2.0-linux-arm64" {
		t.Errorf("linux: %q %q", cmds, download)
	}
	u.goos, u.goarch = "windows", "amd64"
	if cmds, download := u.instructions(installManual, "1.2.0"); cmds != nil || !strings.HasSuffix(download, "/solstice-1.2.0-windows-amd64.zip") {
		t.Errorf("windows: %q %q", cmds, download)
	}
	if cmds, download := u.instructions(installDocker, "1.2.0"); len(cmds) != 3 || download != "" ||
		cmds[2] != "SOLSTICE_VERSION=1.2.0 ./deploy/docker/install.sh" {
		t.Errorf("docker: %q %q", cmds, download)
	}
	if cmds, _ := u.instructions(installSystemd, "1.2.0"); cmds != nil {
		t.Errorf("systemd: %q", cmds)
	}
}

// Integration tests: the endpoints, on a server of their own (its own
// throttling, helper directory, clock and GitHub).

type updatesFixture struct {
	t     *testing.T
	srv   *Server
	url   string
	hits  atomic.Int32
	tag   atomic.Value
	mu    sync.Mutex
	now   time.Time
	dir   string
	login func(username, password string) *updClient
}

type updClient struct {
	f    *updatesFixture
	http *http.Client
}

func (c *updClient) do(method, path string, body any) (int, []byte) {
	c.f.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.f.url+path, r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeader, "1")
	resp, err := c.http.Do(req)
	if err != nil {
		c.f.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func (c *updClient) state(method, path string, body any, want int) updateStateOut {
	c.f.t.Helper()
	status, raw := c.do(method, path, body)
	if status != want {
		c.f.t.Fatalf("%s %s: %d %s", method, path, status, raw)
	}
	var out updateStateOut
	if err := json.Unmarshal(raw, &out); err != nil {
		c.f.t.Fatalf("%s %s: %v %s", method, path, err, raw)
	}
	return out
}

func (f *updatesFixture) advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	f.mu.Unlock()
}

func newUpdatesFixture(t *testing.T) *updatesFixture {
	if testDB == nil {
		t.Skip("SOLSTICE_TEST_DB not set: integration test skipped")
	}
	f := &updatesFixture{t: t, now: time.Now().UTC(), dir: t.TempDir()}
	f.tag.Store("v1.2.0")
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		io.WriteString(w, strings.ReplaceAll(githubRelease, "v1.2.0", f.tag.Load().(string)))
	}))
	t.Cleanup(gh.Close)

	f.srv = New(testDB, t.TempDir(), time.UTC, slog.New(slog.NewTextHandler(io.Discard, nil)))
	f.srv.SetVersion("1.0.0")
	f.srv.upd.apiBase = gh.URL
	f.srv.upd.dir = f.dir
	f.srv.upd.goos, f.srv.upd.goarch = "linux", "amd64"
	f.srv.upd.dockerEnv = filepath.Join(f.dir, "no-dockerenv")
	f.srv.upd.getenv = func(string) string { return "" }
	f.srv.upd.now = func() time.Time {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.now
	}
	hs := httptest.NewServer(f.srv.Handler(http.NotFoundHandler()))
	t.Cleanup(hs.Close)
	f.url = hs.URL

	clean := func() { testDB.Exec(`DELETE FROM settings WHERE name LIKE 'updates.%'`) }
	clean()
	t.Cleanup(clean)

	f.login = func(username, password string) *updClient {
		t.Helper()
		jar, _ := cookiejar.New(nil)
		c := &updClient{f: f, http: &http.Client{Jar: jar, Timeout: 20 * time.Second}}
		if status, body := c.do("POST", "/api/login", map[string]string{"username": username, "password": password}); status != 200 {
			t.Fatalf("login %s: %d %s", username, status, body)
		}
		return c
	}
	return f
}

func TestUpdatesAdminOnly(t *testing.T) {
	f := newUpdatesFixture(t)
	jar, _ := cookiejar.New(nil)
	anon := &updClient{f: f, http: &http.Client{Jar: jar}}
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

	st := admin.state("GET", "/api/admin/updates", nil, 200)
	if st.Current != "1.0.0" || st.Latest == nil || *st.Latest != "1.2.0" || !st.Newer || !st.ChecksEnabled ||
		st.Install != installManual || st.CanApply || st.Download == "" || len(st.Commands) != 4 || st.CheckedAt == nil {
		t.Fatalf("state = %+v", st)
	}
	admin.state("GET", "/api/admin/updates", nil, 200)
	if n := f.hits.Load(); n != 1 {
		t.Errorf("two loads asked GitHub %d times", n)
	}

	// Twelve hours later, the next load checks again.
	f.tag.Store("v1.3.0")
	f.advance(11 * time.Hour)
	if st := admin.state("GET", "/api/admin/updates", nil, 200); *st.Latest != "1.2.0" || f.hits.Load() != 1 {
		t.Errorf("11 h: latest %s, %d checks", *st.Latest, f.hits.Load())
	}
	f.advance(time.Hour)
	if st := admin.state("GET", "/api/admin/updates", nil, 200); *st.Latest != "1.3.0" || f.hits.Load() != 2 {
		t.Errorf("12 h: latest %s, %d checks", *st.Latest, f.hits.Load())
	}

	// On demand: now, but not twice in a row.
	admin.state("POST", "/api/admin/updates/check", nil, 200)
	admin.state("POST", "/api/admin/updates/check", nil, 200)
	if n := f.hits.Load(); n != 3 {
		t.Errorf("two checks in a row asked GitHub %d times in all", n)
	}
	f.advance(time.Minute)
	admin.state("POST", "/api/admin/updates/check", nil, 200)
	if n := f.hits.Load(); n != 4 {
		t.Errorf("a check a minute later: %d in all", n)
	}

	// Checks off: nothing goes out by itself, what is known stays shown.
	if st := admin.state("PUT", "/api/admin/updates/settings", map[string]bool{"checksEnabled": false}, 200); st.ChecksEnabled {
		t.Error("checks still on")
	}
	f.advance(48 * time.Hour)
	if st := admin.state("GET", "/api/admin/updates", nil, 200); f.hits.Load() != 4 || st.Latest == nil {
		t.Errorf("checks off: %d checks, latest %v", f.hits.Load(), st.Latest)
	}
	if status, body := admin.do("PUT", "/api/admin/updates/settings", map[string]any{}); status != 400 {
		t.Errorf("settings without a value: %d %s", status, body)
	}
	admin.state("PUT", "/api/admin/updates/settings", map[string]bool{"checksEnabled": true}, 200)
	if n := f.hits.Load(); n != 5 {
		t.Errorf("checks back on: %d", n)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'update.settings' AND created_at >= ?`, start); n != "2" {
		t.Errorf("settings changes audited: %s", n)
	}

	// GitHub out of reach: the error shows, the release stays known.
	f.tag.Store("garbage")
	f.advance(time.Minute)
	st = admin.state("POST", "/api/admin/updates/check", nil, 200)
	if st.CheckError == nil || *st.CheckError != checkInvalid || st.Latest == nil || *st.Latest != "1.3.0" {
		t.Errorf("failed check: %+v", st)
	}

	// A development build shows the latest release but offers nothing.
	f.srv.SetVersion("dev")
	if st := admin.state("GET", "/api/admin/updates", nil, 200); st.Newer || st.CanApply || len(st.Commands) != 0 || st.Latest == nil {
		t.Errorf("dev: %+v", st)
	}
}

func TestUpdatesApply(t *testing.T) {
	f := newUpdatesFixture(t)
	start := time.Now().UTC().Add(-time.Second)
	admin := f.login("admin", "admin")
	apply := map[string]string{"version": "1.2.0", "password": "admin"}

	// Not installed with the helper: instructions, no update.
	if st := admin.state("GET", "/api/admin/updates", nil, 200); st.CanApply || st.Install != installManual {
		t.Fatalf("manual: %+v", st)
	}
	if status, body := admin.do("POST", "/api/admin/updates/apply", apply); status != http.StatusConflict {
		t.Errorf("manual apply: %d %s", status, body)
	}

	os.WriteFile(filepath.Join(f.dir, "helper"), []byte("1\n"), 0o644)
	st := admin.state("GET", "/api/admin/updates", nil, 200)
	if !st.CanApply || st.Install != installSystemd || len(st.Commands) != 0 || st.LastAttempt != nil {
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
		!strings.Contains(string(body), "Mot de passe incorrect.") {
		t.Errorf("wrong password: %d %s", status, body)
	}
	if _, err := os.Stat(filepath.Join(f.dir, "request")); err == nil {
		t.Fatal("a request was written without the password")
	}

	st = admin.state("POST", "/api/admin/updates/apply", apply, http.StatusAccepted)
	if st.CanApply || st.LastAttempt == nil || st.LastAttempt.Status != attemptPending || st.LastAttempt.To != "1.2.0" {
		t.Errorf("after apply: %+v", st)
	}
	var req updateRequest
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
	status := `{"at":"` + f.srv.upd.clock().Format(time.RFC3339) + `","from":"1.0.0","to":"1.2.0","status":"ok","log":"healthy"}`
	os.WriteFile(filepath.Join(f.dir, "status"), []byte(status), 0o644)
	f.srv.SetVersion("1.2.0")
	for range 2 {
		st = admin.state("GET", "/api/admin/updates", nil, 200)
	}
	if st.Newer || st.CanApply || st.LastAttempt == nil || st.LastAttempt.Status != attemptOK {
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
	admin.state("GET", "/api/admin/updates", nil, 200)
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
