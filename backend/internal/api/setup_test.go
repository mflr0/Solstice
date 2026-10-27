package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"solstice/internal/config"
	"solstice/internal/i18n"
)

// wizard drives the setup handler directly, as one browser.
type wizard struct {
	t      *testing.T
	h      http.Handler
	cookie *http.Cookie
	ip     string
	xff    string // X-Forwarded-For, set when ip is a trusted proxy
}

func (c *wizard) call(method, path, lang string, body any) (int, map[string]any) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	req.RemoteAddr = c.ip + ":40000"
	if c.xff != "" {
		req.Header.Set("X-Forwarded-For", c.xff)
	}
	if method != "GET" {
		req.Header.Set(csrfHeader, "1")
	}
	if lang != "" {
		req.Header.Set("Accept-Language", lang)
	}
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, req)
	for _, ck := range w.Result().Cookies() {
		if ck.Name == setupCookie {
			c.cookie = ck
		}
	}
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func newWizard(t *testing.T, opt SetupOptions) (*Setup, *wizard) {
	t.Helper()
	if opt.DataDir == "" {
		opt.DataDir = t.TempDir()
	}
	if opt.ConfigPath == "" {
		opt.ConfigPath = filepath.Join(t.TempDir(), "solstice.conf")
	}
	opt.Version = "test"
	opt.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewSetup(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	static := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "interface") })
	isAsset := func(p string) bool { return p == "/assets/index-1234.js" }
	return s, &wizard{t: t, h: s.Handler(static, isAsset), ip: "192.0.2.10"}
}

// Setup mode serves the wizard, its files and nothing else; it opens with
// the code only, and the code resists guessing.
func TestSetupModeGating(t *testing.T) {
	s, c := newWizard(t, SetupOptions{})

	for _, p := range []string{"/", "/projects", "/browse/DEMO-1", "/login"} {
		req := httptest.NewRequest("GET", p, nil)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, req)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/setup" {
			t.Errorf("GET %s: %d → %q, want a redirect to /setup", p, w.Code, w.Header().Get("Location"))
		}
	}
	for _, p := range []string{"/setup", "/assets/index-1234.js"} {
		req := httptest.NewRequest("GET", p, nil)
		w := httptest.NewRecorder()
		c.h.ServeHTTP(w, req)
		if w.Code != 200 || w.Body.String() != "interface" {
			t.Errorf("GET %s: %d %q, want the static file", p, w.Code, w.Body.String())
		}
	}
	if w := httptest.NewRecorder(); true {
		c.h.ServeHTTP(w, httptest.NewRequest("GET", "/setup", nil))
		if w.Header().Get("Content-Security-Policy") == "" {
			t.Error("setup pages without security headers")
		}
	}

	// The API answers 503, in the request's language, and says why.
	for _, p := range []string{"/api/me", "/api/issues", "/rest/api/2/issue/DEMO-1"} {
		status, body := c.call("GET", p, "en", nil)
		if status != http.StatusServiceUnavailable || body["setup"] != true ||
			body["error"] != "Solstice is not set up yet: finish the setup at /setup." {
			t.Errorf("GET %s: %d %v", p, status, body)
		}
	}
	if status, _ := c.call("POST", "/api/issues", "", map[string]string{}); status != http.StatusServiceUnavailable {
		t.Errorf("POST /api/issues: %d", status)
	}

	// The code is in the data directory, for the owner only.
	raw, err := os.ReadFile(filepath.Join(s.opt.DataDir, "setup-code"))
	if err != nil {
		t.Fatal(err)
	}
	code := strings.TrimSpace(string(raw))
	if code != s.Code() || len(code) != 9 || code[4] != '-' {
		t.Fatalf("code file %q, code %q", code, s.Code())
	}
	if info, _ := os.Stat(filepath.Join(s.opt.DataDir, "setup-code")); info.Mode().Perm() != 0o600 {
		t.Errorf("code file mode %v", info.Mode().Perm())
	}
	if !strings.Contains(s.Banner(":8080"), "Setup code: "+code) {
		t.Errorf("the banner does not show the code:\n%s", s.Banner(":8080"))
	}

	// Locked: the status says little, the steps refuse.
	status, body := c.call("GET", "/api/setup/status", "", nil)
	if status != 200 || body["setup"] != true || body["unlocked"] != false || body["dataDir"] != nil {
		t.Errorf("status before the code: %d %v", status, body)
	}
	for _, p := range []string{"/api/setup/database/test", "/api/setup/database", "/api/setup/instance",
		"/api/setup/source/test", "/api/setup/import", "/api/setup/admin"} {
		if status, _ := c.call("POST", p, "", map[string]string{}); status != http.StatusUnauthorized {
			t.Errorf("POST %s without the code: %d", p, status)
		}
	}
	if status, _ := c.call("GET", "/api/setup/accounts", "", nil); status != http.StatusUnauthorized {
		t.Errorf("accounts without the code: %d", status)
	}

	// Writes need the anti-forgery header, as everywhere.
	req := httptest.NewRequest("POST", "/api/setup/unlock", strings.NewReader(`{"code":"`+code+`"}`))
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("unlock without X-Solstice: %d", w.Code)
	}

	// Wrong codes lock the address out, even for the right code.
	for i := range 5 {
		if status, body := c.call("POST", "/api/setup/unlock", "", map[string]string{"code": "AAAA-AAAA"}); status != http.StatusForbidden {
			t.Fatalf("wrong code #%d: %d %v", i+1, status, body)
		}
	}
	if status, _ := c.call("POST", "/api/setup/unlock", "", map[string]string{"code": code}); status != http.StatusTooManyRequests {
		t.Fatalf("right code after five wrong ones: %d, want 429", status)
	}
	if c.cookie != nil {
		t.Fatal("a cookie was given to a locked-out address")
	}

	// From elsewhere, the code opens the wizard, typed loosely.
	c.ip = "192.0.2.11"
	loose := strings.ToLower(strings.ReplaceAll(code, "-", " "))
	if status, body := c.call("POST", "/api/setup/unlock", "", map[string]string{"code": loose}); status != http.StatusNoContent {
		t.Fatalf("unlock: %d %v", status, body)
	}
	if c.cookie == nil || !c.cookie.HttpOnly || c.cookie.Path != "/api/setup" {
		t.Fatalf("setup cookie %+v", c.cookie)
	}
	status, body = c.call("GET", "/api/setup/status", "", nil)
	if status != 200 || body["unlocked"] != true || body["dataDir"] != s.opt.DataDir {
		t.Errorf("status after the code: %d %v", status, body)
	}
	// Without a database, the next steps wait for one.
	if status, body := c.call("POST", "/api/setup/admin", "", map[string]string{"username": "chef", "email": "chef@example.org",
		"password": "une phrase de passe longue"}); status != http.StatusConflict {
		t.Errorf("admin before the database: %d %v", status, body)
	}

	// A forged cookie opens nothing.
	other := &wizard{t: t, h: c.h, ip: "192.0.2.12", cookie: &http.Cookie{Name: setupCookie, Value: "forged"}}
	if status, _ := other.call("POST", "/api/setup/database/test", "", map[string]string{}); status != http.StatusUnauthorized {
		t.Errorf("forged cookie: %d", status)
	}
}

func TestHealth(t *testing.T) {
	h := Health("1.2.3", func() (*sql.DB, bool) { return nil, true })
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusServiceUnavailable || body["status"] != "unavailable" || body["version"] != "1.2.3" || body["setup"] != true {
		t.Errorf("no database: %d %v", w.Code, body)
	}
}

func TestSetupHelpers(t *testing.T) {
	f := dbForm{Host: "db.local", Name: "solstice", User: "sol stice", Password: "p@ss:w/rd$#\"'"}
	u, err := f.url()
	if err != nil {
		t.Fatal(err)
	}
	if err := config.CheckValue(u); err != nil {
		t.Errorf("%s cannot go in the configuration file: %v", u, err)
	}
	parsed, _ := url.Parse(u)
	if pass, _ := parsed.User.Password(); pass != f.Password || parsed.User.Username() != "sol stice" || parsed.Port() != "3306" {
		t.Errorf("%s does not round-trip", u)
	}
	for in, want := range map[string]string{
		"https://Tickets.example.org/": "https://tickets.example.org",
		"http://10.0.0.5:8080":         "http://10.0.0.5:8080",
	} {
		if got, err := normalizeBaseURL(in); err != nil || got != want {
			t.Errorf("normalizeBaseURL(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "ftp://x", "https://", "https://x/sub", "https://u:p@x", "https://x/?a=1"} {
		if _, err := normalizeBaseURL(bad); err == nil {
			t.Errorf("normalizeBaseURL(%q) accepted", bad)
		}
	}
}

func TestImportJobCrashedFailsARunningJob(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	job := &importJob{state: "running", started: time.Now()}
	func() {
		defer job.crashed(log)
		panic("boom")
	}()
	if v := job.view("en"); v.State != "failed" || v.Error == "" {
		t.Errorf("state = %q, error = %q, want a failed job with an error", v.State, v.Error)
	}
	if job.running() {
		t.Error("the job is still running")
	}
}

func TestImportJobCrashedKeepsAFinishedJob(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	job := &importJob{state: "done", started: time.Now()}
	func() {
		defer job.crashed(log)
		panic("late")
	}()
	if v := job.view("en"); v.State != "done" {
		t.Errorf("state = %q, want done", v.State)
	}
	func() { defer job.crashed(log) }() // no panic: nothing changes
}

func TestImportJobWarningsInTheReadersLanguage(t *testing.T) {
	job := &importJob{state: "done", started: time.Now(),
		warnings: []i18n.Message{i18n.N(2, i18n.ImportLeftOutCommentOne, i18n.ImportLeftOutCommentMany)}}
	for lang, want := range map[string]string{
		"en": "2 comments of a missing issue, not imported",
		"de": "2 Kommentare fehlender Vorgänge, nicht importiert",
	} {
		if got := job.view(lang).Warnings; len(got) != 1 || got[0] != want {
			t.Errorf("warnings in %s = %q, want %q", lang, got, want)
		}
	}
	if got := (&importJob{}).view("en").Warnings; got == nil || len(got) != 0 {
		t.Errorf("no warnings = %#v, want an empty list", got)
	}
}

func TestSetupAddress(t *testing.T) {
	for _, tc := range []struct{ addr, baseURL, want string }{
		{":8080", "", "http://localhost:8080/setup (or this server's address)"},
		{"0.0.0.0:8080", "", "http://localhost:8080/setup (or this server's address)"},
		{"[::]:8080", "", "http://localhost:8080/setup (or this server's address)"},
		{"127.0.0.1:18099", "", "http://127.0.0.1:18099/setup"},
		{"[::1]:18099", "", "http://[::1]:18099/setup"},
		{"tickets.example.org:8080", "", "http://tickets.example.org:8080/setup"},
		{":8080", "http://localhost:5173", "http://localhost:5173/setup"},
		{"127.0.0.1:8080", "https://tickets.example.org", "https://tickets.example.org/setup"},
	} {
		if got := setupAddress(tc.addr, tc.baseURL); got != tc.want {
			t.Errorf("setupAddress(%q, %q) = %q, want %q", tc.addr, tc.baseURL, got, tc.want)
		}
	}
}
