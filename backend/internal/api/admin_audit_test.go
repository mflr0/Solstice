package api

// Tests of the audit entries for the SMTP settings and the webhooks, and of
// the password that must be entered again when the SMTP destination changes.
// The integration tests need SOLSTICE_TEST_DB and are skipped without it.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"solstice/internal/mail"
	"solstice/internal/webhook"
)

func TestMailDestinationChanged(t *testing.T) {
	base := mail.Config{Host: "smtp.example.org", Port: 587, Security: mail.SecurityStartTLS, Username: "relay",
		From: "a@example.org"}
	same := base
	same.Host, same.From, same.FromName, same.Enabled = "SMTP.Example.org", "b@example.org", "Tickets", true
	if mailDestinationChanged(base, same) {
		t.Error("host case, sender, display name and enabled do not change the destination")
	}
	unset := mail.Config{Host: "smtp.example.org", Username: "relay"}
	if mailDestinationChanged(unset, base) {
		t.Error("an unset port and security mean STARTTLS on 587")
	}
	for name, change := range map[string]func(*mail.Config){
		"host":     func(c *mail.Config) { c.Host = "smtp.evil.example" },
		"port":     func(c *mail.Config) { c.Port = 25 },
		"security": func(c *mail.Config) { c.Security, c.Port = mail.SecurityNone, 587 },
		"username": func(c *mail.Config) { c.Username = "other" },
	} {
		next := base
		change(&next)
		if !mailDestinationChanged(base, next) {
			t.Errorf("a new %s should count as a new destination", name)
		}
	}
}

func TestMailChangesNeverHoldThePassword(t *testing.T) {
	before := mail.Config{Host: "smtp.example.org", Port: 587, Security: mail.SecurityStartTLS, Username: "relay",
		Password: "old-secret-pw", From: "a@example.org"}
	after := before
	after.Host, after.Password, after.Enabled = "smtp.other.example", "new-secret-pw", true

	got := mailChanges(before, after, true, true)
	want := map[string]any{
		"host":            map[string]any{"from": "smtp.example.org", "to": "smtp.other.example"},
		"enabled":         map[string]any{"from": false, "to": true},
		"passwordChanged": true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changes:\n got %v\nwant %v", got, want)
	}
	if raw, _ := json.Marshal(got); strings.Contains(string(raw), "secret-pw") {
		t.Errorf("a password is in %s", raw)
	}
	// Left out: nothing about the password. Emptied: removed, if there was one.
	if got := mailChanges(before, before, false, true); len(got) != 0 {
		t.Errorf("no change, got %v", got)
	}
	cleared := before
	cleared.Password = ""
	if got := mailChanges(before, cleared, true, true); !reflect.DeepEqual(got, map[string]any{"passwordRemoved": true}) {
		t.Errorf("removed password: %v", got)
	}
	if got := mailChanges(before, cleared, true, false); len(got) != 0 {
		t.Errorf("removing a password that was not set: %v", got)
	}
}

func TestWebhookChangesNeverHoldSecrets(t *testing.T) {
	before := &webhook.Webhook{Name: "CI", URL: "https://hooks.example.org/services/T000/B000/tokentoken", Events: []string{"jira:issue_created"},
		Enabled: true, HasSecret: true}
	after := &webhook.Webhook{Name: "CI bot", URL: "https://user:pw@hooks.example.org/services/T111/B111/othertoken?key=k",
		Events: []string{"jira:issue_created", "comment_created"}, Enabled: true, HasSecret: true}
	secret := "signing-secret"
	got := webhookChanges(before, after, &secret)
	want := map[string]any{
		"name":           map[string]any{"from": "CI", "to": "CI bot"},
		"events":         map[string]any{"from": []string{"jira:issue_created"}, "to": []string{"jira:issue_created", "comment_created"}},
		"urlPathChanged": true,
		"secretChanged":  true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changes:\n got %v\nwant %v", got, want)
	}
	raw, _ := json.Marshal(got)
	for _, leak := range []string{"signing-secret", "tokentoken", "othertoken", "user:pw", "key=k"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("%q is in %s", leak, raw)
		}
	}

	// Another host is shown, as a host; a secret left out is not mentioned;
	// one emptied is reported as removed.
	moved := *before
	moved.URL = "http://10.1.2.3:8080/hook"
	got = webhookChanges(before, &moved, nil)
	if !reflect.DeepEqual(got, map[string]any{"url": map[string]any{"from": "https://hooks.example.org/…", "to": "http://10.1.2.3:8080/…"}}) {
		t.Errorf("moved: %v", got)
	}
	empty := ""
	if got := webhookChanges(before, before, &empty); !reflect.DeepEqual(got, map[string]any{"secretRemoved": true}) {
		t.Errorf("removed secret: %v", got)
	}
	if got := webhookChanges(before, before, nil); len(got) != 0 {
		t.Errorf("nothing changed: %v", got)
	}
	// A new webhook is compared with an empty one.
	created := webhookChanges(&webhook.Webhook{}, after, &secret)
	for _, k := range []string{"name", "url", "events", "enabled", "secretChanged"} {
		if _, ok := created[k]; !ok {
			t.Errorf("creation lacks %s: %v", k, created)
		}
	}
}

func TestRedactedHookURL(t *testing.T) {
	for raw, want := range map[string]string{
		"https://hooks.example.org":                      "https://hooks.example.org",
		"https://hooks.example.org/":                     "https://hooks.example.org",
		"https://hooks.example.org/services/T0/B0/xyz":   "https://hooks.example.org/…",
		"https://u:p@hooks.example.org:8443/a?token=abc": "https://hooks.example.org:8443/…",
		"not a url": "",
	} {
		if got := redactedHookURL(raw); got != want {
			t.Errorf("redactedHookURL(%q) = %q, want %q", raw, got, want)
		}
	}
}

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
