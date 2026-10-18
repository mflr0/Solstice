package api

// Tests of the audit details for the SMTP settings and the webhooks: what
// they record, and that no secret reaches them.

import (
	"encoding/json"
	"reflect"
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
