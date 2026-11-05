package tests

// The check of a theme package kept in a user's own browser: findings and
// stylesheet back, nothing stored.

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
)

type themeFinding struct {
	Rule, Message, File string
	Line                int
}

type themeCheck struct {
	Error    string
	Manifest struct {
		ID, Version string
	}
	Findings []themeFinding
	CSS      *string
}

func checkTheme(t *testing.T, c *client, data []byte) (int, themeCheck, string) {
	t.Helper()
	status, raw := sendTheme(t, c, "POST", "/api/themes/check", data)
	var answer themeCheck
	if err := json.Unmarshal(raw, &answer); err != nil {
		t.Fatalf("%d: %v\n%.300s", status, err, raw)
	}
	return status, answer, string(raw)
}

func (a themeCheck) has(rule string) bool {
	return slices.ContainsFunc(a.Findings, func(f themeFinding) bool { return f.Rule == rule && f.File != "" })
}

var inkToken = regexp.MustCompile(`--ink:[^;]*;`)

func TestThemeCheckAcceptsFindings(t *testing.T) {
	user := login(t, "paul.martin", jirafixture.Password)
	id := unique("chk")
	before := publicThemeIDs(t)
	status, answer, raw := checkTheme(t, user, themeZip(t, id, "1.2.3", func(css string) string {
		return inkToken.ReplaceAllString(css, "--ink: #fffffe;")
	}))
	if status != 200 || answer.Manifest.ID != id || answer.Manifest.Version != "1.2.3" {
		t.Fatalf("%d\n%.500s", status, raw)
	}
	if !answer.has("contrast") {
		t.Errorf("no contrast finding: %+v", answer.Findings)
	}
	if answer.CSS == nil {
		t.Fatal("no stylesheet")
	}
	if !strings.Contains(*answer.CSS, "data:font/woff2;base64,") {
		t.Error("the fonts are not inlined")
	}
	if strings.Contains(*answer.CSS, ".woff2") {
		t.Error("a font is still loaded by name")
	}
	if after := publicThemeIDs(t); !slices.Equal(after, before) {
		t.Errorf("GET /api/themes went from %v to %v", before, after)
	}
	if themeListed(t, id) {
		t.Error("a checked package is listed")
	}
	if status, _ := themeServes(t, id); status != 404 {
		t.Errorf("its stylesheet answers %d", status)
	}
}

func TestThemeCheckRefuses(t *testing.T) {
	user := login(t, "paul.martin", jirafixture.Password)
	id := unique("chk")
	before := publicThemeIDs(t)
	status, answer, raw := checkTheme(t, user, themeZip(t, id, "1.0.0", func(css string) string {
		return css + "\n@import 'https://example.com/a.css';\n"
	}))
	if status != 400 || answer.Error == "" || !answer.has("import") || answer.CSS != nil {
		t.Errorf("%d\n%.500s", status, raw)
	}
	if after := publicThemeIDs(t); !slices.Equal(after, before) {
		t.Errorf("GET /api/themes went from %v to %v", before, after)
	}
}

// The admin import refuses the finding the check lets through.
func TestThemeCheckSplitIsItsOwn(t *testing.T) {
	admin := login(t, "admin", "admin")
	data := themeZip(t, unique("chk"), "1.0.0", func(css string) string {
		return inkToken.ReplaceAllString(css, "--ink: #fffffe;")
	})
	if status, raw := sendTheme(t, admin, "POST", "/api/admin/themes", data); status != 400 || !strings.Contains(string(raw), `"rule":"contrast"`) {
		t.Errorf("%d\n%.500s", status, raw)
	}
}

func TestThemeCheckNeedsASession(t *testing.T) {
	status, raw := sendTheme(t, anonymous(t), "POST", "/api/themes/check", themeZip(t, unique("chk"), "1.0.0", nil))
	if status != 401 {
		t.Errorf("%d\n%.300s", status, raw)
	}
}
