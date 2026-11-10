package tests

// The default catalogues a new instance gets at setup, so that its first
// project can be created.

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/i18n"
)

// A new administrator on an empty database gets the default catalogues, in
// the wizard's language, and can create a project with their issue types.
func TestSetupSeedsCatalogues(t *testing.T) {
	dsn := emptyDB(t)
	s, c := unlocked(t, api.SetupOptions{})
	if status, body := c.call("POST", "/api/setup/database", "fr", dbFields(t, dsn)); status != 200 {
		t.Fatalf("database: %d %v", status, body)
	}
	if status, body := c.call("POST", "/api/setup/instance", "fr", map[string]string{"baseURL": "https://tickets.example.org", "tz": "Europe/Paris"}); status != 200 {
		t.Fatalf("instance: %d %v", status, body)
	}
	if status, body := c.call("POST", "/api/setup/admin", "fr", map[string]string{"username": "chef",
		"email": "chef@example.org", "password": "une phrase de passe longue"}); status != 200 {
		t.Fatalf("admin: %d %v", status, body)
	}

	ts := httptest.NewServer(api.New(s.DB(), t.TempDir(), time.UTC, quiet()).Handler(http.NotFoundHandler()))
	t.Cleanup(ts.Close)
	admin := signIn(t, ts.URL, "chef", "une phrase de passe longue")
	var meta struct {
		IssueTypes  []struct{ ID, Name string } `json:"issueTypes"`
		Priorities  []struct{ ID string }       `json:"priorities"`
		Resolutions []struct{ ID string }       `json:"resolutions"`
		LinkTypes   []struct{ ID int64 }        `json:"linkTypes"`
	}
	admin.call("GET", "/api/meta", nil, 200, &meta)
	if len(meta.IssueTypes) != 5 || len(meta.Priorities) != 5 || len(meta.Resolutions) != 4 || len(meta.LinkTypes) != 4 {
		t.Fatalf("catalogues after setup: %d issue types, %d priorities, %d resolutions, %d link types",
			len(meta.IssueTypes), len(meta.Priorities), len(meta.Resolutions), len(meta.LinkTypes))
	}
	if want := i18n.M(i18n.DefaultTypeTask).In("fr"); meta.IssueTypes[0].Name != want {
		t.Errorf("first issue type %q, want %q", meta.IssueTypes[0].Name, want)
	}
	var types []string
	for _, it := range meta.IssueTypes {
		types = append(types, it.ID)
	}
	admin.call("POST", "/api/projects", map[string]any{"key": "PREM", "name": "Premier", "template": "software",
		"access": map[string]string{"holder": "everyone"}, "issueTypes": types}, http.StatusCreated, nil)
}
