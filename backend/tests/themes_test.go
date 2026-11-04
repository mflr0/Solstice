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
	var fixture *themes.Manifest
	for i, m := range idx.Themes {
		if m.ID == "fixture" {
			fixture = &idx.Themes[i]
		}
	}
	if fixture == nil || fixture.Name["en"] == "" || len(fixture.Preview) != len(fixture.Schemes) {
		t.Fatalf("no fixture in %+v", idx.Themes)
	}

	status, h, body := call(t, c, c.base, "GET", "/api/themes/fixture/assets/mark.svg", nil, nil)
	if status != 200 || h.Get("Content-Type") != "image/svg+xml" || !strings.Contains(string(body), "<svg") {
		t.Errorf("mark.svg: %d %q", status, h.Get("Content-Type"))
	}
	if got := h.Get("Content-Security-Policy"); got != "default-src 'none'; style-src 'unsafe-inline'" {
		t.Errorf("mark.svg: Content-Security-Policy %q", got)
	}
	for _, path := range []string{
		"/api/themes/nobody/theme.css",
		"/api/themes/fixture/theme.json",
		"/api/themes/fixture/%2e%2e/fixture/theme.css",
		"/api/themes/fixture/assets/%2e%2e%2ftheme.json",
	} {
		if status, _, _ := call(t, c, c.base, "GET", path, nil, nil); status != 404 {
			t.Errorf("%s: %d, want 404", path, status)
		}
	}
}
