package tests

// The import job, started from the setup wizard and from the administration,
// against a synthetic Jira: its states and progress as polled, the failure
// path, the audit entries and the answers given while it runs.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/config"
	"solstice/internal/db"
	"solstice/internal/jiradb"
	"solstice/internal/jirafixture"
	"solstice/internal/sqlq"
)

// heldTable is a source table read only by heldPhase, after what the source
// test reads: locking it holds an import in that phase.
const (
	heldTable = "worklog"
	heldPhase = "comments and work logs"
)

// jobView is the JSON of a job, as /api/setup/import and /api/admin/import
// answer it.
type jobView struct {
	State string `json:"state"`
	Phase string `json:"phase"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
	Steps []struct {
		Name     string `json:"name"`
		Imported int    `json:"imported"`
	} `json:"steps"`
	Warnings []string `json:"warnings"`
	Error    string   `json:"error"`
}

// TestImportJob runs the import of the wizard, then the one of the
// administration over its result, from SOLSTICE_TEST_JIRA_PG (whose Jira
// tables it recreates) into SOLSTICE_TEST_IMPORT_DB (whose tables it drops).
// CI has no Jira distribution, so it skips there.
//
//	SOLSTICE_TEST_JIRA_PG=postgres://jira:jira@localhost:15432/jira \
//	SOLSTICE_TEST_IMPORT_DB=mysql://solstice:solstice@localhost:13306/solstice_import_t \
//	go test ./tests -run ImportJob -count=1
func TestImportJob(t *testing.T) {
	source, target := os.Getenv("SOLSTICE_TEST_JIRA_PG"), os.Getenv("SOLSTICE_TEST_IMPORT_DB")
	if source == "" || target == "" {
		t.Skip("SOLSTICE_TEST_JIRA_PG or SOLSTICE_TEST_IMPORT_DB not set: import job test skipped")
	}
	if !strings.Contains(target, "import") {
		t.Fatalf("SOLSTICE_TEST_IMPORT_DB=%s: the name must contain « import », it is wiped", target)
	}
	tz, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	if _, err := jirafixture.Run(t.Context(), jirafixture.Options{
		URL: source, JiraDist: jiraDist(t), JiraHome: home, Issues: 40, Seed: 42, TZ: tz,
	}); err != nil {
		t.Fatalf("generating the Jira fixture: %v", err)
	}
	wipe(t, target)
	form := map[string]any{"url": source, "tz": "Europe/Paris", "jiraHome": home}

	var conn *sql.DB
	t.Run("wizard", func(t *testing.T) {
		var done []api.SetupResult
		s, c := unlocked(t, api.SetupOptions{Done: func(r api.SetupResult) { done = append(done, r) }})
		if status, body := c.call("POST", "/api/setup/database", "", dbFields(t, target)); status != 200 {
			t.Fatalf("database: %d %v", status, body)
		}
		if status, _ := c.call("GET", "/api/setup/import", "", nil); status != http.StatusNotFound {
			t.Errorf("state before any import: %d", status)
		}
		poll := func() jobView {
			t.Helper()
			status, body := c.call("GET", "/api/setup/import", "", nil)
			if status != 200 {
				t.Fatalf("import state: %d %v", status, body)
			}
			return decodeJob(t, body)
		}

		// Another import holds the database: the job fails, nothing written.
		release := holdImportLock(t, s.DB())
		status, body := c.call("POST", "/api/setup/import", "", form)
		if status != http.StatusAccepted || body["state"] != "running" {
			t.Fatalf("import: %d %v", status, body)
		}
		end := until(t, poll, func(v jobView) bool { return v.State != "running" })
		if end.State != "failed" || !strings.Contains(end.Error, "another import") || len(end.Steps) != 0 {
			t.Errorf("import over a held database: %+v", end)
		}
		release()

		// Held in one phase: running, and the steps that need it wait.
		release = holdSourceTable(t, source)
		if status, body := c.call("POST", "/api/setup/import", "", form); status != http.StatusAccepted || body["state"] != "running" {
			t.Fatalf("import again: %d %v", status, body)
		}
		var seen []jobView
		held := until(t, func() jobView { v := poll(); seen = append(seen, v); return v },
			func(v jobView) bool { return v.Phase == heldPhase })
		if held.State != "running" || held.Total == 0 || held.Done >= held.Total {
			t.Errorf("held: %+v", held)
		}
		if status, _ := c.call("POST", "/api/setup/import", "", form); status != http.StatusConflict {
			t.Errorf("second import while one runs: %d", status)
		}
		if status, _ := c.call("GET", "/api/setup/accounts", "", nil); status != http.StatusConflict {
			t.Errorf("accounts while importing: %d", status)
		}
		if status, _ := c.call("POST", "/api/setup/admin", "", map[string]string{"existing": "admin"}); status != http.StatusConflict {
			t.Errorf("administrator while importing: %d", status)
		}
		release()
		end = until(t, func() jobView { v := poll(); seen = append(seen, v); return v },
			func(v jobView) bool { return v.State != "running" })
		checkDone(t, seen, end, held.Total)

		// The wizard's import leaves no audit entry; the administration's does.
		conn = s.DB()
		var audited int
		conn.QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action LIKE 'import.%'`).Scan(&audited)
		if audited != 0 {
			t.Errorf("%d import entries in the audit log after the wizard", audited)
		}
		if status, body := c.call("POST", "/api/setup/admin", "", map[string]string{"existing": "admin"}); status != 200 {
			t.Fatalf("administrator: %d %v", status, body)
		}
		if len(done) != 1 {
			t.Fatalf("Done called %d times", len(done))
		}
	})
	if conn == nil {
		t.Fatal("the wizard did not finish: no administration import")
	}

	t.Run("administration", func(t *testing.T) {
		ts := httptest.NewServer(api.New(conn, t.TempDir(), time.UTC, quiet()).Handler(http.NotFoundHandler()))
		t.Cleanup(ts.Close)
		admin := signIn(t, ts.URL, "admin", "admin")
		other := signIn(t, ts.URL, "paul.martin", jirafixture.Password)
		var state struct {
			Confirm string          `json:"confirm"`
			Job     json.RawMessage `json:"job"`
		}
		admin.call("GET", "/api/admin/import", nil, 200, &state)
		if string(state.Job) != "null" {
			t.Errorf("job before any import: %s", state.Job)
		}
		start := map[string]any{"confirm": state.Confirm}
		for k, v := range form {
			start[k] = v
		}
		poll := func() jobView {
			t.Helper()
			var out struct {
				Confirm string  `json:"confirm"`
				Job     jobView `json:"job"`
			}
			admin.call("GET", "/api/admin/import", nil, 200, &out)
			if out.Confirm != state.Confirm {
				t.Errorf("confirmation word %q, want %q", out.Confirm, state.Confirm)
			}
			return out.Job
		}
		label := "postgres://" + strings.SplitN(strings.TrimPrefix(source, "postgres://"), "@", 2)[1]

		release := holdImportLock(t, conn)
		var accepted jobView
		admin.call("POST", "/api/admin/import", start, http.StatusAccepted, &accepted)
		if accepted.State != "running" {
			t.Fatalf("import: %+v", accepted)
		}
		end := until(t, poll, func(v jobView) bool { return v.State != "running" })
		if end.State != "failed" || !strings.Contains(end.Error, "another import") || len(end.Steps) != 0 {
			t.Errorf("import over a held database: %+v", end)
		}
		release()
		failed := auditOf(t, conn, "import.failed")
		if failed.action != "import.failed" || failed.username != "admin" || failed.label != label ||
			!strings.Contains(failed.details["error"].(string), "another import") || failed.details["source"] != label {
			t.Errorf("audit entry of the failed import: %+v", failed)
		}

		release = holdSourceTable(t, source)
		admin.call("POST", "/api/admin/import", start, http.StatusAccepted, &accepted)
		var seen []jobView
		held := until(t, func() jobView { v := poll(); seen = append(seen, v); return v },
			func(v jobView) bool { return v.Phase == heldPhase })
		if held.State != "running" || held.Total == 0 || held.Done >= held.Total {
			t.Errorf("held: %+v", held)
		}
		if a := lastAudit(t, conn); a.action != "import.start" || a.username != "admin" || a.label != label ||
			a.details["source"] != label || a.details["tz"] != "Europe/Paris" || a.details["jiraHome"] != home {
			t.Errorf("audit entry of the start: %+v", a)
		}

		// Maintenance: the operator keeps their progress page and /api/me,
		// everything else under /api/ and /rest/ waits.
		var me struct{ Username string }
		admin.call("GET", "/api/me", nil, 200, &me)
		if me.Username != "admin" {
			t.Errorf("operator's /api/me: %+v", me)
		}
		for _, tc := range []struct {
			who          *client
			method, path string
		}{
			{admin, "GET", "/api/projects"},
			{admin, "POST", "/api/admin/import"},
			{admin, "GET", "/rest/api/2/myself"},
			{other, "GET", "/api/me"},
			{other, "GET", "/api/admin/import"},
			{signIn(t, ts.URL, "", ""), "GET", "/api/me"},
		} {
			status, _, out := tc.who.errorIn("", tc.method, tc.path, start)
			if status != http.StatusServiceUnavailable || out["maintenance"] != true {
				t.Errorf("%s %s while importing: %d %v", tc.method, tc.path, status, out)
			}
		}
		if status, _ := signIn(t, ts.URL, "", "").do("GET", "/api/settings/public", nil); status != 200 {
			t.Errorf("public settings while importing: %d", status)
		}
		if status, _ := signIn(t, ts.URL, "", "").do("GET", "/setup", nil); status == http.StatusServiceUnavailable {
			t.Errorf("a page outside the API while importing: %d", status)
		}

		release()
		end = until(t, func() jobView { v := poll(); seen = append(seen, v); return v },
			func(v jobView) bool { return v.State != "running" })
		checkDone(t, seen, end, held.Total)
		a := auditOf(t, conn, "import.done")
		if a.action != "import.done" || a.username != "admin" || a.label != label || a.details["source"] != label ||
			a.details["error"] != nil {
			t.Errorf("audit entry of the end: %+v", a)
		}
		if _, ok := a.details["seconds"].(float64); !ok {
			t.Errorf("no duration in the audit entry: %v", a.details)
		}
		// The entry names the operator's account as the import left it.
		var want string
		conn.QueryRow(`SELECT id FROM users WHERE username = 'admin'`).Scan(&want)
		if a.userID != want {
			t.Errorf("audit entry user id %s, admin is %s", a.userID, want)
		}
		admin.call("GET", "/api/projects", nil, 200, nil)
		other.call("GET", "/api/me", nil, 200, nil)
	})
}

// signIn is loginAt on a database other than SOLSTICE_TEST_DB; an empty
// username stays anonymous.
func signIn(t *testing.T, base, username, password string) *client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &client{t: t, base: base, http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
	if username != "" {
		c.call("POST", "/api/login", map[string]string{"username": username, "password": password}, 200, nil)
	}
	return c
}

// checkDone checks a finished job and the progress polled on the way.
func checkDone(t *testing.T, seen []jobView, end jobView, total int) {
	t.Helper()
	if end.State != "done" || end.Error != "" || end.Phase != "" || end.Done != total || end.Total != total ||
		len(end.Steps) == 0 {
		t.Errorf("finished: %+v", end)
	}
	for i := 1; i < len(seen); i++ {
		prev, v := seen[i-1], seen[i]
		// Total is 0 until the first step starts.
		if v.Total != 0 && v.Total != total || v.Done < prev.Done || len(v.Steps) < len(prev.Steps) {
			t.Errorf("progress went back: %+v then %+v", prev, v)
		}
		if v.Phase != prev.Phase && v.Phase != "" && prev.Phase != "" && v.Done <= prev.Done {
			t.Errorf("next phase without the count: %+v then %+v", prev, v)
		}
	}
}

func decodeJob(t *testing.T, body map[string]any) jobView {
	t.Helper()
	raw, _ := json.Marshal(body)
	var v jobView
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// until polls until stop holds, for a minute at most.
func until(t *testing.T, poll func() jobView, stop func(jobView) bool) jobView {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		v := poll()
		if stop(v) {
			return v
		}
		if time.Now().After(deadline) {
			t.Fatalf("still %+v after a minute", v)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// holdImportLock takes the lock an import takes on its database; the
// returned function gives it back.
func holdImportLock(t *testing.T, conn *sql.DB) func() {
	t.Helper()
	c, err := conn.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got int
	if err := c.QueryRowContext(context.Background(),
		"SELECT GET_LOCK(CONCAT('solstice-import:', DATABASE()), 0)").Scan(&got); err != nil || got != 1 {
		t.Fatalf("import lock: %d %v", got, err)
	}
	return func() {
		c.ExecContext(context.Background(), "DO RELEASE_LOCK(CONCAT('solstice-import:', DATABASE()))")
		c.Close()
	}
}

// holdSourceTable locks heldTable in the Jira source: an import waits in
// heldPhase until the returned function releases it.
func holdSourceTable(t *testing.T, source string) func() {
	t.Helper()
	src, _, err := jiradb.Open(source, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := src.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("LOCK TABLE " + heldTable + " IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	release := func() { tx.Rollback(); src.Close() }
	t.Cleanup(release)
	return release
}

type auditEntry struct {
	action, username, label, userID string
	details                         map[string]any
}

// auditOf waits for the job's last audit entry, written once its state is.
func auditOf(t *testing.T, conn *sql.DB, action string) auditEntry {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if e := lastAudit(t, conn); e.action == action || time.Now().After(deadline) {
			return e
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func lastAudit(t *testing.T, conn *sql.DB) auditEntry {
	t.Helper()
	var e auditEntry
	var uid, details sql.NullString
	if err := conn.QueryRow(`SELECT action, COALESCE(username, ''), COALESCE(target_label, ''), user_id, details
		FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&e.action, &e.username, &e.label, &uid, &details); err != nil {
		t.Fatal(err)
	}
	e.userID = uid.String
	_ = json.Unmarshal([]byte(details.String), &e.details)
	return e
}

func jiraDist(t *testing.T) string {
	t.Helper()
	if d := os.Getenv("JIRA_DIST"); d != "" {
		return d
	}
	found, _ := filepath.Glob("../../atlassian-jira-software-*-standalone")
	if len(found) == 0 {
		t.Fatal("no Jira distribution next to the repository: set JIRA_DIST")
	}
	return found[0]
}

// wipe drops every table of a database.
func wipe(t *testing.T, dsn string) {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, dsn, config.Defaults().DB.Pool())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var tables []string
	if err := sqlq.Each(ctx, conn, `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()`,
		func(rows *sql.Rows) error {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			tables = append(tables, name)
			return nil
		}); err != nil {
		t.Fatal(err)
	}
	one, err := conn.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer one.Close()
	if _, err := one.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		t.Fatal(err)
	}
	for _, name := range tables {
		if _, err := one.ExecContext(ctx, "DROP TABLE `"+name+"`"); err != nil {
			t.Fatal(err)
		}
	}
}
