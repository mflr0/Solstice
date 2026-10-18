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
	en["TestOnlyEnglish"] = "Only in English: %s."
	en["TestNoValues"] = "No values, 100 % as is."
	defer delete(en, "TestOnlyEnglish")
	defer delete(en, "TestNoValues")
	for _, c := range []struct {
		locale string
		key    Key
		args   []any
		want   string
	}{
		{"fr", StatusNameUnknown, []any{"Fait"}, "Statut inconnu : Fait."},
		{"en", StatusNameUnknown, []any{"Fait"}, "Unknown status: Fait."},
		{"de", RequestTooManyAttempts, []any{3}, "Zu viele Versuche. Versuchen Sie es in 3 s erneut."},
		{"xx", StatusNameUnknown, []any{"Fait"}, "Unknown status: Fait."}, // unknown language: English
		{"fr", "TestOnlyEnglish", []any{"x"}, "Only in English: x."},      // missing translation: English
		{"en", "TestNoValues", nil, "No values, 100 % as is."},
		// Messages given as values are rendered in the same language.
		{"de", JQLError, []any{M(JQLFieldUnknown, "couleur")}, "JQL: Unbekanntes Feld: „couleur“."},
		{"en", BoardQueryInvalid, []any{M(BoardQuickFilterNamed, "Mes bugs"), M(JQLError, M(JQLValueExpected))},
			"Quick filter “Mes bugs”: JQL: Value expected."},
		{"fr", JQLError, []any{Raw("100 % raw")}, "JQL : 100 % raw"},
	} {
		if got := Sprintf(c.locale, c.key, c.args...); got != c.want {
			t.Errorf("Sprintf(%s, %s) = %q, want %q", c.locale, c.key, got, c.want)
		}
	}
	if got := M(StatusNameUnknown, "Fait").String(); got != "Unknown status: Fait." {
		t.Errorf("String() = %q, want the English text", got)
	}
	if got := Raw("Fait %s").In("fr"); got != "Fait %s" {
		t.Errorf("Raw(…).In(fr) = %q, want the text as is", got)
	}
}

// Translations may put the values in another order.
func TestReorderedValues(t *testing.T) {
	en["TestBefore"] = "%s before %s."
	fr["TestBefore"] = "%[2]s après %[1]s."
	defer delete(en, "TestBefore")
	defer delete(fr, "TestBefore")
	if got := Sprintf("fr", "TestBefore", "a", "b"); got != "b après a." {
		t.Errorf("got %q", got)
	}
}
