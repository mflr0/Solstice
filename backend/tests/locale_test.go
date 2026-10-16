package tests

// Errors and the account's language.

import (
	"bytes"
	"net/url"
	"testing"
)

func TestJQLErrorLanguage(t *testing.T) {
	c := login(t, "admin", "admin")
	path := "/api/issues?jql=" + url.QueryEscape("project = DEMO AND couleur = bleu")
	for lang, want := range map[string]string{
		"":      "JQL : Champ inconnu : « couleur ».",
		"en":    "JQL: Unknown field: “couleur”.",
		"de-DE": "JQL: Unbekanntes Feld: „couleur“.",
	} {
		status, msg, out := c.errorIn(lang, "GET", path, nil)
		if status != 400 || msg != want || out["pos"] != float64(19) {
			t.Errorf("%q: %d %v, want %q at 19", lang, status, out, want)
		}
	}
}

// The account's language applies when the request names none.
func TestUserLocale(t *testing.T) {
	c := login(t, "lea.richard", "motdepasse")
	t.Cleanup(func() { c.do("PATCH", "/api/me", map[string]any{"locale": nil}) })
	locale := func() any {
		t.Helper()
		var me map[string]any
		if status := c.get("/api/me", &me); status != 200 {
			t.Fatalf("GET /api/me: %d", status)
		}
		v, ok := me["locale"]
		if !ok {
			t.Fatalf("no locale in %v", me)
		}
		return v
	}
	const missing = "/api/issues/NOPE-999999"
	if status, msg, _ := c.errorIn("", "GET", missing, nil); status != 404 || msg != "Not found, or you do not have access to it." {
		t.Errorf("before any choice: %d %q", status, msg)
	}

	status, body := c.do("PATCH", "/api/me", map[string]any{"locale": "de"})
	if status != 200 || !bytes.Contains(body, []byte(`"locale":"de"`)) {
		t.Fatalf("PATCH locale de: %d %s", status, body)
	}
	if l := locale(); l != "de" {
		t.Errorf("locale %v, want de", l)
	}
	if _, msg, _ := c.errorIn("", "GET", missing, nil); msg != "Nicht gefunden, oder Sie haben keinen Zugriff darauf." {
		t.Errorf("without Accept-Language: %q, want German", msg)
	}
	if _, msg, _ := c.errorIn("en", "GET", missing, nil); msg != "Not found, or you do not have access to it." {
		t.Errorf("with Accept-Language en: %q, want English", msg)
	}
	if _, body := anonymous(t).do("POST", "/api/login", map[string]string{"username": "lea.richard", "password": "motdepasse"}); !bytes.Contains(body, []byte(`"locale":"de"`)) {
		t.Errorf("login answer without the locale: %s", body)
	}

	for _, bad := range []any{"xx", "EN", "de-CH", 3} {
		status, msg, _ := c.errorIn("", "PATCH", "/api/me", map[string]any{"locale": bad})
		if status != 400 || msg != "Sprache wird nicht unterstützt (zur Auswahl: fr, en, de, it, hi)." {
			t.Errorf("PATCH locale %v: %d %q", bad, status, msg)
		}
	}
	if status, _ := c.do("PATCH", "/api/me", map[string]any{}); status != 200 || locale() != "de" {
		t.Errorf("an empty PATCH changed the locale (%d, %v)", status, locale())
	}
	if status, _ := c.do("PATCH", "/api/me", map[string]any{"locale": nil}); status != 200 || locale() != nil {
		t.Errorf("PATCH locale null: %d, locale %v", status, locale())
	}
	if _, msg, _ := c.errorIn("", "GET", missing, nil); msg != "Not found, or you do not have access to it." {
		t.Errorf("after clearing: %q, want French", msg)
	}
}
