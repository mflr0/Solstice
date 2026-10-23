package tests

// Small fixes of the setup wizard and the import: the audit label of the
// first administrator, and which table the « not a Jira database » message
// names.

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"solstice/internal/api"
)

// A blank « existing » value creates the account, and the audit entry says so.
func TestSetupBlankExistingCreates(t *testing.T) {
	dsn := emptyDB(t)
	s, c := unlocked(t, api.SetupOptions{})
	if status, body := c.call("POST", "/api/setup/database", "", dbFields(t, dsn)); status != 200 {
		t.Fatalf("database: %d %v", status, body)
	}
	if status, body := c.call("POST", "/api/setup/instance", "", map[string]string{"baseURL": "https://tickets.example.org", "tz": "Europe/Paris"}); status != 200 {
		t.Fatalf("instance: %d %v", status, body)
	}
	status, body := c.call("POST", "/api/setup/admin", "", map[string]string{"existing": "  ", "username": "chef",
		"email": "chef@example.org", "password": "une phrase de passe longue"})
	if status != 200 || body["username"] != "chef" {
		t.Fatalf("admin: %d %v", status, body)
	}
	if a := lastAudit(t, s.DB()); a.action != "setup.complete" || a.details["administrator"] != "created" {
		t.Errorf("audit entry: %+v, want administrator created", a)
	}
}

// The message names the first table missing in a fixed order, every time.
func TestImportNotJiraNamesFirstTable(t *testing.T) {
	needDB(t)
	admin := login(t, "admin", "admin")
	form := map[string]string{"url": os.Getenv("SOLSTICE_TEST_DB"), "tz": "Europe/Paris"}
	for range 20 {
		status, msg, out := admin.errorIn("", "POST", "/api/admin/import/test", form)
		if status != http.StatusBadRequest || !strings.Contains(msg, "(no project table)") {
			t.Fatalf("a Solstice database as the source: %d %v", status, out)
		}
	}
}
