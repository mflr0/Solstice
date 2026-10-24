package tests

// Instance settings: what anyone may read, what administrators change, and
// how a change applies at once (banner, default language, password length).

import (
	"strings"
	"testing"

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
