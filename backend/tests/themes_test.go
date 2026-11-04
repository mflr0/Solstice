package tests

// Theme packages: the list anyone may read, and their files.

import (
	"strings"
	"testing"

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
