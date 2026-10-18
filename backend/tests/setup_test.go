package tests

// The setup wizard against real databases: the connection test, and a
// whole new instance on an empty database.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"solstice/internal/api"
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
		if ck.Name == "solstice_setup" {
			c.cookie = ck
		}
	}
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func newWizard(t *testing.T, opt api.SetupOptions) (*api.Setup, *wizard) {
	t.Helper()
	if opt.DataDir == "" {
		opt.DataDir = t.TempDir()
	}
	if opt.ConfigPath == "" {
		opt.ConfigPath = filepath.Join(t.TempDir(), "solstice.conf")
	}
	opt.Version = "test"
	opt.Log = quiet()
	s, err := api.NewSetup(context.Background(), opt)
	if err != nil {
		t.Fatal(err)
	}
	static := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "interface") })
	isAsset := func(p string) bool { return p == "/assets/index-1234.js" }
	return s, &wizard{t: t, h: s.Handler(static, isAsset), ip: "192.0.2.10"}
}

// A healthy instance says so.
func TestHealth(t *testing.T) {
	needDB(t)
	h := api.Health("1.2.3", func() (*sql.DB, bool) { return testDB, false })
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	var body map[string]any
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

func unlocked(t *testing.T, opt api.SetupOptions) (*api.Setup, *wizard) {
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
	_, c := unlocked(t, api.SetupOptions{})
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
		{with("port", 1), "Cannot reach"},
		{with("password", "wrong-password"), "rejected the user name or password"},
		{with("name", "solstice_no_such_db"), "or it does not exist"},
		{with("host", "bad host"), "Invalid host"},
		{with("name", "x;DROP"), "Invalid database name"},
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
	conn, err := db.Open(context.Background(), dsn, config.Defaults().DB.Pool())
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
	var done []api.SetupResult
	cfgPath := filepath.Join(t.TempDir(), "solstice.conf")
	dataDir := t.TempDir()
	s, c := unlocked(t, api.SetupOptions{ConfigPath: cfgPath, DataDir: dataDir,
		Done: func(r api.SetupResult) { done = append(done, r) }})

	status, body := c.call("POST", "/api/setup/database", "", dbFields(t, dsn))
	if status != 200 || body["existing"] != false {
		t.Fatalf("database: %d %v", status, body)
	}
	saved, _ := body["config"].(map[string]any)
	if saved["written"] != true {
		t.Fatalf("configuration not written: %v", saved)
	}
	values, err := config.Load([]string{config.KeyConfig + "=" + cfgPath})
	if err != nil {
		t.Fatal(err)
	}
	if values.DB.URL == "" || values.Data != dataDir {
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
	if values, _ = config.Load([]string{config.KeyConfig + "=" + cfgPath}); values.TZ.String() != "Europe/Paris" ||
		values.HTTP.BaseURL != "https://tickets.example.org" || values.DB.URL == "" {
		t.Errorf("configuration after the instance step: %+v", values)
	}

	// The administrator: a real password.
	admin := map[string]string{"username": "Chef", "name": "La cheffe", "email": "chef@example.org",
		"password": "une phrase de passe longue"}
	for _, tc := range []struct {
		field, value, want string
	}{
		{"password", "onze-signes", "at least 12 characters"},
		{"email", "pas-une-adresse", "Invalid email address"},
		{"email", "", "Invalid email address"},
		{"username", "Bad Name", "Invalid username"},
		{"password", "CHEF.DE.PROJET", "cannot be the username"},
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
	if _, err := os.Stat(filepath.Join(dataDir, "setup-code")); !os.IsNotExist(err) {
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
	_, c2 := unlocked(t, api.SetupOptions{DB: conn, Done: func(api.SetupResult) { stopped = true }})
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
