package tests

// Theme packages: the list anyone may read, and their files.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/themes"
)

func TestThemesArePublic(t *testing.T) {
	c := anonymous(t)
	var idx themes.Index
	c.call("GET", "/api/themes", nil, 200, &idx)
	if idx.Contract != themes.Contract {
		t.Errorf("contract %d, want %d", idx.Contract, themes.Contract)
	}
	if len(idx.Themes) == 0 || idx.Themes[0].ID != "carnet" {
		t.Fatalf("Carnet is not first in %+v", idx.Themes)
	}
	var orbite *themes.Manifest
	for i, m := range idx.Themes {
		if m.ID == "orbite" {
			orbite = &idx.Themes[i]
		}
	}
	if orbite == nil || orbite.Name["en"] == "" || len(orbite.Preview) != len(orbite.Schemes) {
		t.Fatalf("no orbite in %+v", idx.Themes)
	}

	status, h, body := call(t, c, c.base, "GET", "/api/themes/orbite/assets/logo.svg", nil, nil)
	if status != 200 || h.Get("Content-Type") != "image/svg+xml" || !strings.Contains(string(body), "<svg") {
		t.Errorf("logo.svg: %d %q", status, h.Get("Content-Type"))
	}
	if got := h.Get("Content-Security-Policy"); got != "default-src 'none'; style-src 'unsafe-inline'" {
		t.Errorf("logo.svg: Content-Security-Policy %q", got)
	}
	for _, path := range []string{
		"/api/themes/nobody/theme.css",
		"/api/themes/orbite/theme.json",
		"/api/themes/orbite/%2e%2e/carnet/theme.css",
		"/api/themes/orbite/assets/%2e%2e%2ftheme.json",
	} {
		if status, _, _ := call(t, c, c.base, "GET", path, nil, nil); status != 404 {
			t.Errorf("%s: %d, want 404", path, status)
		}
	}
}

// storedManifest is the theme.json of a stored theme of the given manifest id.
func storedManifest(id, name string) string {
	return `{"id": "` + id + `", "contract": 1, "version": "1.2.3", "name": {"en": "` + name + `"},
		"description": {"en": "A stored theme"}, "author": "Tests", "schemes": ["light"],
		"preview": {"light": {"bar": "#111111", "paper": "#ffffff", "ink": "#000000", "accent": "#123456"}}}`
}

func TestStoredThemes(t *testing.T) {
	needDB(t)
	dataDir := t.TempDir()
	ts := httptest.NewServer(api.New(testDB, dataDir, time.UTC, quiet()).Handler(http.NotFoundHandler()))
	t.Cleanup(ts.Close)

	stored := []struct {
		id, manifestID, name string
		enabled              bool
	}{
		{"check-538-b", "check-538", "Check B", true},
		{"check-538-a", "check-538", "Check A", true},
		{"check-538-off", "check-538", "Check off", false},
		{"carnet", "carnet", "Check clash", true},
	}
	t.Cleanup(func() {
		for _, s := range stored {
			if _, err := testDB.Exec(`DELETE FROM themes WHERE id = ?`, s.id); err != nil {
				t.Error(err)
			}
		}
	})
	for _, s := range stored {
		manifest := storedManifest(s.manifestID, s.name)
		if _, err := testDB.Exec(`INSERT INTO themes (id, manifest_id, manifest, version, enabled, imported_by, imported_at)
			VALUES (?, ?, ?, '1.2.3', ?, 'admin', NOW(3))`, s.id, s.manifestID, manifest, s.enabled); err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string]string{
			"theme.json":    manifest,
			"theme.css":     "/* " + s.id + " */",
			"assets/a.svg":  "<svg/>",
			"notes/not.css": "not served",
		} {
			file := filepath.Join(dataDir, "themes", s.id, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte(data), 0o640); err != nil {
				t.Fatal(err)
			}
		}
	}

	c := anonymousAt(t, ts.URL)
	var idx themes.Index
	c.call("GET", "/api/themes", nil, 200, &idx)
	builtin := themes.Builtin().Index().Themes
	if len(idx.Themes) != len(builtin)+2 {
		t.Fatalf("%d themes, want the %d built-in and 2 stored", len(idx.Themes), len(builtin))
	}
	for i, m := range builtin {
		if idx.Themes[i].ID != m.ID || idx.Themes[i].Name["en"] != m.Name["en"] {
			t.Errorf("theme %d: %q %q, want the built-in %q", i, idx.Themes[i].ID, idx.Themes[i].Name["en"], m.ID)
		}
	}
	a, b := idx.Themes[len(builtin)], idx.Themes[len(builtin)+1]
	if a.ID != "check-538-a" || b.ID != "check-538-b" {
		t.Errorf("stored themes %q, %q: want them by name, under their served ids", a.ID, b.ID)
	}
	if a.Name["en"] != "Check A" || a.Version != "1.2.3" || a.Preview["light"].Accent != "#123456" {
		t.Errorf("stored manifest %+v", a)
	}

	status, h, body := call(t, c, c.base, "GET", "/api/themes/check-538-a/theme.css", nil, nil)
	if status != 200 || string(body) != "/* check-538-a */" || h.Get("ETag") == "" {
		t.Errorf("theme.css: %d %q, ETag %q", status, body, h.Get("ETag"))
	}
	status, _, _ = call(t, c, c.base, "GET", "/api/themes/check-538-a/theme.css", nil, func(r *http.Request) {
		r.Header.Set("If-None-Match", h.Get("ETag"))
	})
	if status != http.StatusNotModified {
		t.Errorf("If-None-Match: %d", status)
	}
	_, svg, body := call(t, c, c.base, "GET", "/api/themes/check-538-a/assets/a.svg", nil, nil)
	_, builtinSVG, _ := call(t, c, c.base, "GET", "/api/themes/orbite/assets/logo.svg", nil, nil)
	if string(body) != "<svg/>" {
		t.Errorf("a.svg: %q", body)
	}
	for _, name := range []string{"Content-Type", "X-Content-Type-Options", "Content-Security-Policy", "Cache-Control"} {
		if got, want := svg.Get(name), builtinSVG.Get(name); got != want || got == "" {
			t.Errorf("%s: %q, a built-in has %q", name, got, want)
		}
	}

	if _, _, body := call(t, c, c.base, "GET", "/api/themes/carnet/theme.css", nil, nil); strings.Contains(string(body), "/* carnet */") {
		t.Error("the stored carnet replaced the built-in")
	}
	for _, path := range []string{
		"/api/themes/check-538-off/theme.css",
		"/api/themes/check-538-a/theme.json",
		"/api/themes/check-538-a/notes/not.css",
		"/api/themes/check-538-a/%2e%2e/check-538-off/theme.css",
		"/api/themes/%2e%2e/theme.css",
	} {
		if status, _, _ := call(t, c, c.base, "GET", path, nil, nil); status != 404 {
			t.Errorf("%s: %d, want 404", path, status)
		}
	}
}
