package tests

// The audit entries for the SMTP settings and the webhooks, and the password
// that must be entered again when the SMTP destination changes.

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// latestAudit returns how many entries of an action exist and the details of
// the newest one.
func latestAudit(t *testing.T, action string) (int, string) {
	t.Helper()
	n, _ := strconv.Atoi(pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = ?`, action))
	if n == 0 {
		return 0, ""
	}
	return n, pick(t, `SELECT COALESCE(details, '') FROM audit_log WHERE action = ? ORDER BY id DESC LIMIT 1`, action)
}

func TestMailSettingsAreAudited(t *testing.T) {
	env := newNotifyEnv(t)
	admin := env.login("admin", "admin")
	before, _ := latestAudit(t, "settings.mail")

	admin.call("PUT", "/api/admin/mail", map[string]any{"fromName": "Audit Tickets"}, 200, nil)
	n, details := latestAudit(t, "settings.mail")
	if n != before+1 || !strings.Contains(details, `"fromName"`) || !strings.Contains(details, "Audit Tickets") {
		t.Fatalf("after a sender change: %d entries (was %d), details %s", n, before, details)
	}
	// Nothing changed: nothing recorded.
	admin.call("PUT", "/api/admin/mail", map[string]any{"fromName": "Audit Tickets"}, 200, nil)
	if n2, _ := latestAudit(t, "settings.mail"); n2 != n {
		t.Errorf("a save without change was recorded")
	}

	admin.call("PUT", "/api/admin/mail", map[string]any{"username": "relai", "password": "s3cret-audit-smtp"}, 200, nil)
	n, details = latestAudit(t, "settings.mail")
	if n != before+2 || !strings.Contains(details, `"passwordChanged": true`) && !strings.Contains(details, `"passwordChanged":true`) ||
		!strings.Contains(details, `"username"`) {
		t.Fatalf("after a password change: %d entries, details %s", n, details)
	}
	if left := pick(t, `SELECT COUNT(*) FROM audit_log WHERE details LIKE ?`, "%s3cret-audit-smtp%"); left != "0" {
		t.Fatal("the SMTP password reached the audit log")
	}

	// A new destination with the stored password left out is refused...
	for _, change := range []map[string]any{{"host": "smtp.autre.example"}, {"port": 2525}, {"security": "tls"},
		{"username": "autre"}} {
		status, body := admin.do("PUT", "/api/admin/mail", change)
		if status != http.StatusBadRequest || !strings.Contains(string(body), "SMTP") {
			t.Errorf("%v without the password: %d %s, want 400", change, status, body)
		}
	}
	var cfg mailSettingsOut
	admin.call("GET", "/api/admin/mail", nil, 200, &cfg)
	if cfg.Host != env.smtp.Host || cfg.Username != "relai" || !cfg.PasswordSet {
		t.Errorf("a refused change was applied: %+v", cfg)
	}
	if n2, _ := latestAudit(t, "settings.mail"); n2 != n {
		t.Error("a refused change was recorded")
	}
	// ...but accepted when the password is entered again, or removed.
	admin.call("PUT", "/api/admin/mail", map[string]any{"host": env.smtp.Host, "port": env.smtp.Port, "username": "autre",
		"password": "s3cret-audit-smtp"}, 200, nil)
	admin.call("PUT", "/api/admin/mail", map[string]any{"username": "relai", "password": nil}, 200, &cfg)
	if cfg.PasswordSet {
		t.Error("password not removed")
	}
	if _, details := latestAudit(t, "settings.mail"); !strings.Contains(details, "passwordRemoved") {
		t.Errorf("removal not recorded: %s", details)
	}
	// With no password stored, there is nothing to protect.
	admin.call("PUT", "/api/admin/mail", map[string]any{"port": env.smtp.Port + 1}, 200, nil)
	admin.call("PUT", "/api/admin/mail", map[string]any{"port": env.smtp.Port}, 200, nil)
}

func TestWebhooksAreAudited(t *testing.T) {
	env := newNotifyEnv(t)
	admin := env.login("admin", "admin")
	rc := newHookReceiver(t)
	created, _ := latestAudit(t, "webhook.create")
	updated, _ := latestAudit(t, "webhook.update")
	deleted, _ := latestAudit(t, "webhook.delete")

	var wh struct{ ID int64 }
	admin.call("POST", "/api/admin/webhooks", map[string]any{"name": "Audité", "url": rc.srv.URL + "/services/jeton-secret-url",
		"events": []string{"jira:issue_created"}, "secret": "s3cret-audit-hook"}, 201, &wh)
	n, details := latestAudit(t, "webhook.create")
	if n != created+1 || !strings.Contains(details, "secretChanged") || !strings.Contains(details, "Audité") {
		t.Fatalf("creation: %d entries (was %d), details %s", n, created, details)
	}
	if label := pick(t, `SELECT target_label FROM audit_log WHERE action = 'webhook.create' ORDER BY id DESC LIMIT 1`); label != "Audité" {
		t.Errorf("target label %q", label)
	}
	if id := pick(t, `SELECT target_id FROM audit_log WHERE action = 'webhook.create' ORDER BY id DESC LIMIT 1`); id != fmt.Sprint(wh.ID) {
		t.Errorf("target id %q, want %d", id, wh.ID)
	}
	path := fmt.Sprintf("/api/admin/webhooks/%d", wh.ID)

	admin.call("PUT", path, map[string]any{"name": "Audité bis", "secret": "s3cret-audit-hook-2", "enabled": false}, 200, nil)
	n, details = latestAudit(t, "webhook.update")
	if n != updated+1 || !strings.Contains(details, "Audité bis") || !strings.Contains(details, "secretChanged") ||
		!strings.Contains(details, "enabled") {
		t.Fatalf("update: %d entries, details %s", n, details)
	}
	// Nothing changed: nothing recorded.
	admin.call("PUT", path, map[string]any{"name": "Audité bis"}, 200, nil)
	if n2, _ := latestAudit(t, "webhook.update"); n2 != n {
		t.Error("a save without change was recorded")
	}
	admin.call("DELETE", path, nil, 204, nil)
	if n, _ := latestAudit(t, "webhook.delete"); n != deleted+1 {
		t.Errorf("deletion not recorded: %d entries (was %d)", n, deleted)
	}
	for _, leak := range []string{"s3cret-audit-hook", "jeton-secret-url"} {
		if left := pick(t, `SELECT COUNT(*) FROM audit_log WHERE details LIKE ?`, "%"+leak+"%"); left != "0" {
			t.Errorf("%q reached the audit log", leak)
		}
	}
	// Only administrators change webhooks, and a refused change leaves no entry.
	paul := env.login("paul.martin", "motdepasse")
	paul.call("POST", "/api/admin/webhooks", map[string]any{"name": "x", "url": rc.srv.URL, "events": []string{"jira:issue_created"}}, 403, nil)
	if n, _ := latestAudit(t, "webhook.create"); n != created+1 {
		t.Error("a refused creation was recorded")
	}
}
