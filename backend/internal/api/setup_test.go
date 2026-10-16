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
	"strconv"
	"strings"
	"testing"

	"solstice/internal/auth"
	"solstice/internal/config"
	"solstice/internal/db"
	"solstice/internal/sqlq"
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

	if testDB == nil {
		t.Skip("SOLSTICE_TEST_DB not set: the healthy case is skipped")
	}
	h = Health("1.2.3", func() (*sql.DB, bool) { return testDB, false })
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	body = nil
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 200 || body["status"] != "ok" || body["version"] != "1.2.3" || body["setup"] != nil {
		t.Errorf("healthy: %d %v", w.Code, body)
	}
}

// dbFields splits a mysql:// URL into the wizard's form.
func dbFields(t *testing.T, raw string) map[string]any {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	pass, _ := u.User.Password()
	port, _ := strconv.Atoi(u.Port())
	return map[string]any{"host": u.Hostname(), "port": port, "name": strings.TrimPrefix(u.Path, "/"),
		"user": u.User.Username(), "password": pass}
}

func unlocked(t *testing.T, opt SetupOptions) (*Setup, *wizard) {
	t.Helper()
	s, c := newWizard(t, opt)
	if status, body := c.call("POST", "/api/setup/unlock", "", map[string]string{"code": s.Code()}); status != http.StatusNoContent {
		t.Fatalf("unlock: %d %v", status, body)
	}
	return s, c
}

// The database test explains what is wrong, and recognises a good server.
func TestSetupDatabaseTest(t *testing.T) {
	dsn := os.Getenv("SOLSTICE_TEST_DB")
	if dsn == "" {
		t.Skip("SOLSTICE_TEST_DB not set: integration test skipped")
	}
	_, c := unlocked(t, SetupOptions{})
	good := dbFields(t, dsn)
	with := func(k string, v any) map[string]any {
		f := map[string]any{}
		for kk, vv := range good {
			f[kk] = vv
		}
		f[k] = v
		return f
	}
	for _, tc := range []struct {
		form map[string]any
		want string
	}{
		{with("port", 1), "injoignable"},
		{with("password", "wrong-password"), "Utilisateur ou mot de passe refusé"},
		{with("name", "solstice_no_such_db"), "n'existe pas"},
		{with("host", "bad host"), "Hôte invalide"},
		{with("name", "x;DROP"), "Nom de base invalide"},
	} {
		status, body := c.call("POST", "/api/setup/database/test", "", tc.form)
		if msg, _ := body["error"].(string); status != http.StatusBadRequest && status != http.StatusForbidden || !strings.Contains(msg, tc.want) {
			t.Errorf("%v: %d %v, want %q", tc.form, status, body, tc.want)
		}
	}
	status, body := c.call("POST", "/api/setup/database/test", "", good)
	if status != 200 || !strings.HasPrefix(body["server"].(string), "MariaDB ") {
		t.Fatalf("good database: %d %v", status, body)
	}
	if n, _ := body["accounts"].(float64); n == 0 {
		t.Errorf("the test database holds an instance, yet %v accounts", body["accounts"])
	}
}

// emptyDB is a scratch database for the wizard (scripts/e2e-import.sh makes
// one): it is wiped first, so its name must say it is for setup tests.
func emptyDB(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("SOLSTICE_TEST_EMPTY_DB")
	if dsn == "" {
		t.Skip("SOLSTICE_TEST_EMPTY_DB not set: integration test skipped")
	}
	if !strings.Contains(dsn, "setup") {
		t.Fatalf("SOLSTICE_TEST_EMPTY_DB=%s: the name must contain « setup », it is wiped", dsn)
	}
	conn, err := db.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var tables []string
	if err := sqlq.Each(context.Background(), conn, `SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE()`, func(rows *sql.Rows) error {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		tables = append(tables, name)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// One statement per call with this driver, and on one connection for
	// the foreign key switch.
	one, err := conn.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer one.Close()
	if _, err := one.ExecContext(context.Background(), "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		t.Fatal(err)
	}
	for _, name := range tables {
		if _, err := one.ExecContext(context.Background(), "DROP TABLE `"+name+"`"); err != nil {
			t.Fatal(err)
		}
	}
	return dsn
}

// The whole new-instance path on an empty database: schema, configuration
// file, instance settings, administrator; then setup is over for good.
func TestSetupNewInstance(t *testing.T) {
	dsn := emptyDB(t)
	var done []SetupResult
	cfgPath := filepath.Join(t.TempDir(), "solstice.conf")
	s, c := unlocked(t, SetupOptions{ConfigPath: cfgPath, Done: func(r SetupResult) { done = append(done, r) }})

	status, body := c.call("POST", "/api/setup/database", "", dbFields(t, dsn))
	if status != 200 || body["existing"] != false {
		t.Fatalf("database: %d %v", status, body)
	}
	saved, _ := body["config"].(map[string]any)
	if saved["written"] != true {
		t.Fatalf("configuration not written: %v", saved)
	}
	values, err := config.Load(cfgPath, func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	if values.DB == "" || values.Data != s.opt.DataDir {
		t.Errorf("configuration file: %+v", values)
	}
	if info, _ := os.Stat(cfgPath); info.Mode().Perm() != 0o600 {
		t.Errorf("configuration file mode %v", info.Mode().Perm())
	}
	conn := s.DB()
	var migrations int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&migrations); err != nil || migrations == 0 {
		t.Fatalf("schema not created: %d %v", migrations, err)
	}

	// The instance: the address is checked, the time zone too.
	for _, bad := range []map[string]string{
		{"baseURL": "tickets.example.org", "tz": "Europe/Paris"},
		{"baseURL": "https://tickets.example.org/solstice", "tz": "Europe/Paris"},
		{"baseURL": "https://tickets.example.org", "tz": "Mars/Olympus"},
		{"baseURL": "https://tickets.example.org", "tz": "Local"},
	} {
		if status, body := c.call("POST", "/api/setup/instance", "", bad); status != http.StatusBadRequest {
			t.Errorf("instance %v: %d %v", bad, status, body)
		}
	}
	status, body = c.call("POST", "/api/setup/instance", "", map[string]string{"baseURL": "HTTPS://Tickets.Example.org/", "tz": "Europe/Paris"})
	if status != 200 || body["baseURL"] != "https://tickets.example.org" || body["tz"] != "Europe/Paris" {
		t.Fatalf("instance: %d %v", status, body)
	}
	if values, _ = config.Load(cfgPath, func(string) (string, bool) { return "", false }); values.TZ != "Europe/Paris" ||
		values.BaseURL != "https://tickets.example.org" || values.DB == "" {
		t.Errorf("configuration after the instance step: %+v", values)
	}

	// The administrator: a real password.
	admin := map[string]string{"username": "Chef", "name": "La cheffe", "email": "chef@example.org",
		"password": "une phrase de passe longue"}
	for _, tc := range []struct {
		field, value, want string
	}{
		{"password", "onze-signes", "au moins 12 caractères"},
		{"email", "pas-une-adresse", "Adresse e-mail invalide"},
		{"email", "", "Adresse e-mail invalide"},
		{"username", "Bad Name", "Identifiant invalide"},
		{"password", "CHEF.DE.PROJET", "ne peut pas être l'identifiant"},
	} {
		form := map[string]string{}
		for k, v := range admin {
			form[k] = v
		}
		form[tc.field] = tc.value
		if strings.HasPrefix(tc.value, "CHEF.") {
			form["username"] = "chef.de.projet"
		}
		status, body := c.call("POST", "/api/setup/admin", "", form)
		if msg, _ := body["error"].(string); status != http.StatusBadRequest || !strings.Contains(msg, tc.want) {
			t.Errorf("%s=%q: %d %v, want %q", tc.field, tc.value, status, body, tc.want)
		}
	}
	if len(done) != 0 {
		t.Fatal("setup ended on an invalid form")
	}
	status, body = c.call("POST", "/api/setup/admin", "", admin)
	if status != 200 || body["username"] != "chef" {
		t.Fatalf("admin: %d %v", status, body)
	}
	if len(done) != 1 || done[0].Admin != "chef" || done[0].DB != conn || done[0].TZ != "Europe/Paris" ||
		done[0].BaseURL != "https://tickets.example.org" {
		t.Fatalf("Done: %+v", done)
	}
	u, err := auth.Authenticate(context.Background(), conn, "chef", "une phrase de passe longue")
	if err != nil || !u.Admin || u.DisplayName != "La cheffe" || u.Email != "chef@example.org" {
		t.Fatalf("the administrator cannot sign in: %+v %v", u, err)
	}
	var hash string
	conn.QueryRow(`SELECT password_hash FROM users WHERE username = 'chef'`).Scan(&hash)
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash %q", hash)
	}
	var audited int
	conn.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = 'setup.complete' AND username = 'chef'`).Scan(&audited)
	if audited != 1 {
		t.Errorf("%d setup.complete entries in the audit log, want 1", audited)
	}
	if _, err := os.Stat(filepath.Join(s.opt.DataDir, "setup-code")); !os.IsNotExist(err) {
		t.Errorf("the setup code file is still there: %v", err)
	}

	// Setup cannot run twice: the code opens nothing, the steps refuse.
	if status, _ := c.call("POST", "/api/setup/admin", "", admin); status != http.StatusUnauthorized && status != http.StatusConflict {
		t.Errorf("second admin: %d", status)
	}
	if status, _ := c.call("POST", "/api/setup/unlock", "", map[string]string{"code": s.Code()}); status != http.StatusConflict {
		t.Errorf("unlock after setup: %d", status)
	}
	if status, body := c.call("GET", "/api/setup/status", "", nil); status != 200 || body["setup"] != false {
		t.Errorf("status after setup: %d %v", status, body)
	}
	if len(done) != 1 {
		t.Errorf("Done called %d times", len(done))
	}

	// A new start on this database (it has an account now) would not be in
	// setup mode; a wizard left running notices and stops too.
	var stopped bool
	_, c2 := unlocked(t, SetupOptions{DB: conn, DBFixed: true, Done: func(SetupResult) { stopped = true }})
	status, body = c2.call("POST", "/api/setup/admin", "", map[string]string{"username": "intrus",
		"email": "intrus@example.org", "password": "une autre phrase longue"})
	if status != http.StatusConflict || !stopped {
		t.Errorf("wizard over a database with accounts: %d %v, stopped=%v", status, body, stopped)
	}
	var intruders int
	conn.QueryRow(`SELECT COUNT(*) FROM users WHERE username = 'intrus'`).Scan(&intruders)
	if intruders != 0 {
		t.Error("an account was created over an existing instance")
	}
}

// Once set up, the application answers the wizard's addresses plainly.
func TestSetupOverInApplication(t *testing.T) {
	c := anonymous(t)
	if status := c.get("/api/setup/status", nil); status != 200 {
		t.Errorf("status: %d", status)
	}
	var out map[string]any
	c.get("/api/setup/status", &out)
	if out["setup"] != false {
		t.Errorf("status %v", out)
	}
	if status, _ := c.do("POST", "/api/setup/admin", map[string]string{"username": "x"}); status != http.StatusConflict {
		t.Errorf("admin on an installed instance: %d", status)
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
