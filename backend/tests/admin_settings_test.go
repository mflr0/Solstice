package tests

// Instance settings: what anyone may read, what administrators change, and
// how a change applies at once (banner, default language, password length).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/jirafixture"
)

type generalSettings struct {
	InstanceName      string            `json:"instanceName"`
	DefaultLanguage   string            `json:"defaultLanguage"`
	Banner            map[string]any    `json:"banner"`
	SessionDays       int               `json:"sessionDays"`
	PasswordMinLength int               `json:"passwordMinLength"`
	Sources           map[string]string `json:"sources"`
	Languages         []string          `json:"languages"`
}

// unknownAddress is the answer to an unknown address, by language.
var unknownAddress = map[string]string{"fr": "Adresse inconnue.", "en": "Unknown address.", "de": "Unbekannte Adresse."}

func TestInstanceSettings(t *testing.T) {
	admin := login(t, "admin", "admin")
	// The import may have set some of them: put back what was there.
	var before generalSettings
	admin.call("GET", "/api/admin/settings", nil, 200, &before)
	restore := map[string]any{}
	for field, v := range map[string]any{"instanceName": before.InstanceName, "defaultLanguage": before.DefaultLanguage,
		"banner": before.Banner, "sessionDays": before.SessionDays, "passwordMinLength": before.PasswordMinLength} {
		restore[field] = nil
		if before.Sources[field] == "database" {
			restore[field] = v
		}
	}
	t.Cleanup(func() { admin.do("PUT", "/api/admin/settings", restore) })

	var public map[string]any
	anonymous(t).call("GET", "/api/settings/public", nil, 200, &public)
	if public["instanceName"] != before.InstanceName || public["defaultLanguage"] != before.DefaultLanguage ||
		public["passwordMinLength"] != float64(before.PasswordMinLength) {
		t.Errorf("public settings %v, admin's %+v", public, before)
	}
	if len(before.Languages) != 5 || before.SessionDays < 1 {
		t.Errorf("settings: %+v", before)
	}
	login(t, "paul.martin", jirafixture.Password).call("GET", "/api/admin/settings", nil, 403, nil)

	for _, bad := range []map[string]any{
		{"instanceName": ""}, {"instanceName": strings.Repeat("x", 101)}, {"baseURL": "ftp://tickets.exemple.fr"},
		{"baseURL": "https://tickets.exemple.fr/sous-dossier"}, {"timeZone": "Mars/Olympus"}, {"defaultLanguage": "xx"},
		{"banner": map[string]any{"enabled": true, "text": "x", "level": "rouge"}},
		{"banner": map[string]any{"enabled": true, "text": strings.Repeat("x", 501)}},
		{"sessionDays": 0}, {"sessionDays": 366}, {"passwordMinLength": 7}, {"couleur": "bleu"},
	} {
		if status, body := admin.do("PUT", "/api/admin/settings", bad); status != 400 {
			t.Errorf("PUT %v: %d %s", bad, status, body)
		}
	}

	// Changes apply at once.
	var g generalSettings
	admin.call("PUT", "/api/admin/settings", map[string]any{"instanceName": "Tickets",
		"banner":            map[string]any{"enabled": true, "text": "Maintenance ce soir", "level": "warning"},
		"passwordMinLength": 20, "defaultLanguage": "de"}, 200, &g)
	if g.InstanceName != "Tickets" || g.Sources["instanceName"] != "database" || g.Sources["banner"] != "database" {
		t.Errorf("after PUT: %+v", g)
	}
	anonymous(t).call("GET", "/api/settings/public", nil, 200, &public)
	if public["instanceName"] != "Tickets" || at(public, "banner", "text") != "Maintenance ce soir" ||
		at(public, "banner", "level") != "warning" || public["passwordMinLength"] != 20.0 {
		t.Errorf("public settings after PUT: %v", public)
	}
	// The default language speaks to those who chose none.
	if _, msg, _ := anonymous(t).errorIn("", "GET", "/api/nope", nil); msg != unknownAddress["de"] {
		t.Errorf("default language de: %q", msg)
	}
	if _, msg, _ := anonymous(t).errorIn("fr", "GET", "/api/nope", nil); msg != unknownAddress["fr"] {
		t.Errorf("Accept-Language fr wins over the default: %q", msg)
	}
	// New passwords follow the new minimum.
	status, body := admin.do("POST", "/api/admin/users", map[string]any{"username": unique("court."),
		"password": "dix-neuf-caractères"})
	if status != 400 || !strings.Contains(string(body), "20") {
		t.Errorf("a 19-character password: %d %s", status, body)
	}
	var audit struct{ Total int }
	admin.call("GET", "/api/admin/audit?action=settings.general", nil, 200, &audit)
	if audit.Total == 0 {
		t.Error("settings changes are not in the audit log")
	}

	// null goes back to the inherited value.
	admin.call("PUT", "/api/admin/settings", map[string]any{"instanceName": nil, "passwordMinLength": nil, "banner": nil}, 200, &g)
	if g.InstanceName != "Solstice" || g.Sources["instanceName"] != "default" || g.PasswordMinLength != 12 || g.Banner["enabled"] != false {
		t.Errorf("after null: %+v", g)
	}
	anonymous(t).call("GET", "/api/settings/public", nil, 200, &public)
	if public["banner"] != nil {
		t.Errorf("banner after null: %v", public["banner"])
	}
	admin.call("PUT", "/api/admin/settings", restore, 200, nil)
	if want, ok := unknownAddress[before.DefaultLanguage]; ok {
		if _, msg, _ := anonymous(t).errorIn("", "GET", "/api/nope", nil); msg != want {
			t.Errorf("default language back to %s: %q", before.DefaultLanguage, msg)
		}
	}
}

type onboardingSteps struct {
	Projects, OtherUsers, Mail, Boards, Issues, Dismissed bool
}

// On a new instance the setup steps start undone and each one is done as
// its record appears; hiding the checklist is remembered and audited.
func TestOnboarding(t *testing.T) {
	needDB(t)
	dsn := emptyDB(t)
	s, c := unlocked(t, api.SetupOptions{Done: func(api.SetupResult) {}})
	if status, body := c.call("POST", "/api/setup/database", "", dbFields(t, dsn)); status != 200 {
		t.Fatalf("database: %d %v", status, body)
	}
	if status, body := c.call("POST", "/api/setup/admin", "", map[string]string{"username": "chef",
		"email": "chef@exemple.fr", "password": "une phrase de passe longue"}); status != 200 {
		t.Fatalf("admin: %d %v", status, body)
	}
	ts := httptest.NewServer(api.New(s.DB(), t.TempDir(), time.UTC, quiet()).Handler(http.NotFoundHandler()))
	t.Cleanup(ts.Close)
	admin := loginAt(t, ts.URL, "chef", "une phrase de passe longue")

	var want onboardingSteps
	check := func(step string) {
		t.Helper()
		var got onboardingSteps
		admin.call("GET", "/api/admin/onboarding", nil, 200, &got)
		if got != want {
			t.Fatalf("after %s: %+v, want %+v", step, got, want)
		}
	}
	check("setup")

	admin.call("POST", "/api/projects", map[string]any{"key": "ONB", "name": "Onboarding"}, 201, nil)
	want.Projects = true
	check("a project")
	admin.call("POST", "/api/admin/users", map[string]any{"username": "second", "email": "second@exemple.fr",
		"password": "une autre phrase longue"}, 201, nil)
	want.OtherUsers = true
	check("a second user")
	admin.call("PUT", "/api/admin/mail", map[string]any{"enabled": true, "host": "127.0.0.1", "port": 2525,
		"security": "none", "from": "solstice@exemple.fr"}, 200, nil)
	want.Mail = true
	check("mail")
	admin.call("POST", "/api/boards", map[string]any{"kind": "kanban", "project": "ONB"}, 201, nil)
	want.Boards = true
	check("a board")
	var project struct {
		IssueTypes []struct{ ID string } `json:"issueTypes"`
	}
	admin.call("GET", "/api/projects/ONB/settings", nil, 200, &project)
	if len(project.IssueTypes) == 0 {
		t.Fatal("the new project offers no issue type")
	}
	admin.call("POST", "/api/issues", map[string]any{"project": "ONB", "type": project.IssueTypes[0].ID,
		"summary": "La première"}, 201, nil)
	want.Issues = true
	check("an issue")

	admin.call("PUT", "/api/admin/onboarding", map[string]any{}, 400, nil)
	admin.call("PUT", "/api/admin/onboarding", map[string]any{"dismissed": true}, 200, nil)
	want.Dismissed = true
	check("hiding the checklist")
	var audited int
	s.DB().QueryRow(`SELECT COUNT(*) FROM audit_log WHERE action = 'settings.general' AND target_type = 'settings'
		AND target_id = 'onboarding' AND username = 'chef'`).Scan(&audited)
	if audited != 1 {
		t.Errorf("%d audit entries for hiding the checklist, want 1", audited)
	}
}
