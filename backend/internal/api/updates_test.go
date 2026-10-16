package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
