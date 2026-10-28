package i18n

import (
	"encoding/json"
	"testing"
)

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

// Pick owns the language rule: the first supported candidate wins, and
// nothing supported means English.
func TestPick(t *testing.T) {
	for _, c := range []struct {
		candidates []string
		want       string
	}{
		{nil, "en"},                  // no candidate: English
		{[]string{"fr"}, "fr"},       // the user's choice
		{[]string{"xx", "de"}, "de"}, // an unknown language falls through
		{[]string{"", "it"}, "it"},   // an empty choice too
		{[]string{"fr", "de"}, "fr"}, // the first supported one wins
		{[]string{"xx", "yy"}, "en"}, // none supported: English
		{[]string{"xx", "es"}, "en"}, // an unsupported default counts as none
	} {
		if got := Pick(c.candidates...); got != c.want {
			t.Errorf("Pick(%q) = %q, want %q", c.candidates, got, c.want)
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

func TestN(t *testing.T) {
	for n, want := range map[int]string{
		1: "1 comment of a missing issue, not imported",
		0: "0 comments of a missing issue, not imported",
		2: "2 comments of a missing issue, not imported",
	} {
		if got := N(n, ImportLeftOutCommentOne, ImportLeftOutCommentMany).String(); got != want {
			t.Errorf("N(%d) = %q, want %q", n, got, want)
		}
	}
	m := N(2, ImportWarnUnknownValuesOne, ImportWarnUnknownValuesMany, "issuestatus")
	if got, want := m.In("hi"), "issuestatus में Jira के 2 अज्ञात मान \"Unknown (id)\" नाम से फिर बनाए गए"; got != want {
		t.Errorf("reordered values: %q, want %q", got, want)
	}
	b, err := json.Marshal([]Message{m})
	if want := `["2 unknown values from Jira in issuestatus, recreated under the name \"Unknown (id)\""]`; err != nil || string(b) != want {
		t.Errorf("JSON = %s (%v), want %s", b, err, want)
	}
}

func TestMatch(t *testing.T) {
	keys := []Key{DefaultPriorityHigh, DefaultPriorityLow}
	for text, want := range map[string]Key{
		"High":      DefaultPriorityHigh,
		"Haute":     DefaultPriorityHigh,
		"Hoch":      DefaultPriorityHigh,
		"निम्न":     DefaultPriorityLow,
		"high":      "", // as written, not folded
		"Haute ":    "",
		"Very high": "", // not among keys
		"Urgent":    "",
	} {
		if got := Match(text, keys...); got != want {
			t.Errorf("Match(%q) = %q, want %q", text, got, want)
		}
	}
}
