package tests

// The system page, the guards of the import from the administration, and
// retrying failed e-mails.

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
	"solstice/internal/mail/mailtest"
)

func TestSystemInfo(t *testing.T) {
	admin := login(t, "admin", "admin")
	login(t, "paul.martin", jirafixture.Password).call("GET", "/api/admin/system", nil, 403, nil)
	raw := admin.call("GET", "/api/admin/system", nil, 200, nil)
	info := obj(t, raw)
	if !strings.HasPrefix(str(info, "database", "server"), "MariaDB") ||
		str(info, "database", "name") != pick(t, `SELECT DATABASE()`) || str(info, "dataDir", "path") == "" ||
		str(info, "goVersion") == "" || num(info, "migrations", "count") == 0 || at(info, "config") == nil {
		t.Errorf("system: %s", raw)
	}
	for _, k := range []string{"issues", "projects", "usersActive", "comments"} {
		if num(info, "counts", k) == 0 {
			t.Errorf("counts.%s is 0: %v", k, at(info, "counts"))
		}
	}
	if n := pick(t, `SELECT COUNT(*) FROM issues`); strconv.Itoa(num(info, "counts", "issues")) != n {
		t.Errorf("counts.issues = %d, the database has %s", num(info, "counts", "issues"), n)
	}
	// No secret: the database's password is in its URL only.
	if dsn := os.Getenv("SOLSTICE_TEST_DB"); strings.Contains(dsn, "@") {
		if user := strings.TrimPrefix(dsn[:strings.Index(dsn, "@")], "mysql://"); strings.Contains(string(raw), user) {
			t.Error("the database credentials show on the system page")
		}
	}
}

func TestImportGuards(t *testing.T) {
	admin := login(t, "admin", "admin")
	login(t, "paul.martin", jirafixture.Password).call("GET", "/api/admin/import", nil, 403, nil)
	var state struct {
		Confirm string
		Job     any
	}
	admin.call("GET", "/api/admin/import", nil, 200, &state)
	if state.Confirm != pick(t, `SELECT DATABASE()`) {
		t.Errorf("confirmation word %q", state.Confirm)
	}
	// Each refusal comes before the source is even reached: nothing starts.
	for _, in := range []map[string]any{
		{"url": "postgres://jira:jira@localhost:15432/jira", "tz": "Europe/Paris", "confirm": "pas la base"},
		{"url": "pas une adresse", "tz": "Europe/Paris", "confirm": state.Confirm},
		{"url": "postgres://jira:jira@localhost:15432/jira", "tz": "Mars/Olympus", "confirm": state.Confirm},
	} {
		if status, body := admin.do("POST", "/api/admin/import", in); status != 400 {
			t.Fatalf("import %v: %d %s", in, status, body)
		}
	}
	for _, in := range []map[string]any{{"url": "pas une adresse", "tz": "Europe/Paris"},
		{"url": "postgres://jira:jira@localhost:15432/jira", "tz": "Mars/Olympus"}} {
		if status, body := admin.do("POST", "/api/admin/import/test", in); status != 400 {
			t.Errorf("source test %v: %d %s", in, status, body)
		}
	}
	admin.call("GET", "/api/admin/import", nil, 200, &state)
	if state.Job != nil {
		t.Errorf("an import started: %v", state.Job)
	}
}

func TestMailQueueRetry(t *testing.T) {
	admin := login(t, "admin", "admin")
	login(t, "paul.martin", jirafixture.Password).call("POST", "/api/admin/mail/queue/retry", nil, 403, nil)
	mailtest.Lock(t, testDB)
	if _, err := testDB.Exec(`INSERT INTO mail_outbox (kind, to_addr, subject, text_body, html_body, message_id, status,
		attempts, next_attempt_at, created_at) VALUES ('retry-test', 'x@exemple.fr', 's', 't', 'h', '<retry@exemple.fr>',
		'failed', 5, UTC_TIMESTAMP(), UTC_TIMESTAMP())`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testDB.Exec(`DELETE FROM mail_outbox WHERE kind = 'retry-test'`) })
	var out struct{ Retried int }
	admin.call("POST", "/api/admin/mail/queue/retry", nil, 200, &out)
	if out.Retried < 1 || pick(t, `SELECT status FROM mail_outbox WHERE kind = 'retry-test'`) != "pending" {
		t.Errorf("retry: %+v", out)
	}
}
