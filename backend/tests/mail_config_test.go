package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/config"
)

// SMTP settings of the configuration file win over the Mail page, which
// leaves them alone on save.
func TestMailSettingsFromConfig(t *testing.T) {
	env := newNotifyEnv(t)
	cfg, err := config.Load([]string{config.KeyConfig + "=" + filepath.Join(t.TempDir(), "absent.conf"),
		"SOLSTICE_MAIL_HOST=smtp.config.example", "SOLSTICE_MAIL_SECURITY=tls"})
	if err != nil {
		t.Fatal(err)
	}
	srv := api.New(testDB, t.TempDir(), time.UTC, quiet())
	srv.SetMailOverride(cfg.MailOverride())
	ts := httptest.NewServer(srv.Handler(http.NotFoundHandler()))
	t.Cleanup(func() { ts.Close(); srv.Close(context.Background()) })
	admin := loginAt(t, ts.URL, "admin", "admin")
	before, _ := latestAudit(t, "settings.mail")

	var got struct {
		mailSettingsOut
		Port       int      `json:"port"`
		Security   string   `json:"security"`
		FromConfig []string `json:"fromConfig"`
	}
	destination := []string{"host", "port", "security", "username", "password"}
	admin.call("GET", "/api/admin/mail", nil, 200, &got)
	if got.Host != "smtp.config.example" || got.Port != 465 || got.Security != "tls" || got.PasswordSet ||
		!slices.Equal(got.FromConfig, destination) || got.FromName != "Solstice" {
		t.Fatalf("GET with SOLSTICE_MAIL_HOST: %+v", got)
	}

	admin.call("PUT", "/api/admin/mail", map[string]any{"host": "smtp.autre.example", "port": 2525,
		"username": "autre", "password": "s3cret", "fromName": "Tickets"}, 200, &got)
	if got.Host != "smtp.config.example" || got.Username != "" || got.FromName != "Tickets" {
		t.Errorf("after a PUT changing the host: %+v", got)
	}
	stored := pick(t, `SELECT value FROM settings WHERE name = 'mail'`)
	if !strings.Contains(stored, `"host":"`+env.smtp.Host+`"`) || strings.Contains(stored, "smtp.config.example") ||
		strings.Contains(stored, "autre") {
		t.Errorf("stored settings: %s", stored)
	}
	n, details := latestAudit(t, "settings.mail")
	if n != before+1 || strings.Contains(details, "host") || strings.Contains(details, "password") ||
		!strings.Contains(details, "fromName") {
		t.Errorf("audit: %d entries (was %d), %s", n, before, details)
	}

	// The server without the override still reads the stored settings.
	env.login("admin", "admin").call("GET", "/api/admin/mail", nil, 200, &got)
	if got.Host != env.smtp.Host || got.FromConfig == nil || len(got.FromConfig) != 0 {
		t.Errorf("without the override: %+v", got)
	}
}
