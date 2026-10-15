package i18n

import "testing"

func TestNegotiate(t *testing.T) {
	for header, want := range map[string]string{
		"":                               "",
		"en":                             "en",
		"DE":                             "de",
		"de-CH":                          "de",
		"it_IT":                          "it",
		"hi-IN,hi;q=0.9,en;q=0.8":        "hi",
		"es, en;q=0.5":                   "en", // the first supported one
		"en;q=0.5, de;q=0.8":             "de", // highest q wins, whatever the order
		"fr-CA;q=0.7, it;q=0.7":          "fr", // a tie goes to the first
		"en;q=0, de;q=0.1":               "de", // q=0 means "not this one"
		"en;q=abc, it":                   "it", // a malformed q is ignored with its language
		"es, pt-BR":                      "",   // nothing supported
		"*":                              "",
		" en-GB ; q=0.9 , de ; q=0.95 ":  "de",
		"zh-Hant-TW, de-AT-1996;q=0.5":   "de",
		"en-US,en;q=0.9,fr;q=0.8,de;q=1": "en",
	} {
		if got := Negotiate(header); got != want {
			t.Errorf("Negotiate(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestSupported(t *testing.T) {
	for _, l := range []string{"fr", "en", "de", "it", "hi"} {
		if !Supported(l) {
			t.Errorf("%s not supported", l)
		}
	}
	for _, l := range []string{"", "EN", "en-GB", "es", "xx"} {
		if Supported(l) {
			t.Errorf("%q supported", l)
		}
	}
}

func TestSprintf(t *testing.T) {
	for _, c := range []struct {
		locale, template string
		args             []any
		want             string
	}{
		{"fr", "Statut inconnu : %s.", []any{"Fait"}, "Statut inconnu : Fait."},
		{"en", "Statut inconnu : %s.", []any{"Fait"}, "Unknown status: Fait."},
		{"de", "Trop de tentatives : réessayez dans %d s.", []any{3}, "Zu viele Versuche. Versuchen Sie es in 3 s erneut."},
		{"xx", "Statut inconnu : %s.", []any{"Fait"}, "Statut inconnu : Fait."},   // unknown language: French
		{"en", "Pas encore traduit : %s.", []any{"x"}, "Pas encore traduit : x."}, // missing translation: French
		{"en", "Sans valeur, 100 % tel quel.", nil, "Sans valeur, 100 % tel quel."},
		// Messages given as values are rendered in the same language.
		{"de", "JQL : %s", []any{M("Champ inconnu : « %s ».", "couleur")}, "JQL: Unbekanntes Feld: „couleur“."},
		{"en", "%s : %s", []any{M("Filtre rapide « %s »", "Mes bugs"), M("JQL : %s", M("Valeur attendue."))},
			"Quick filter “Mes bugs”: JQL: Value expected."},
	} {
		if got := Sprintf(c.locale, c.template, c.args...); got != c.want {
			t.Errorf("Sprintf(%s, %q) = %q, want %q", c.locale, c.template, got, c.want)
		}
	}
	if got := M("Statut inconnu : %s.", "Fait").String(); got != "Statut inconnu : Fait." {
		t.Errorf("String() = %q, want the French text", got)
	}
}

// Translations may put the values in another order.
func TestReorderedValues(t *testing.T) {
	en["« %s » avant « %s »."] = "%[2]s after %[1]s."
	defer delete(en, "« %s » avant « %s ».")
	if got := Sprintf("en", "« %s » avant « %s ».", "a", "b"); got != "b after a." {
		t.Errorf("got %q", got)
	}
}
