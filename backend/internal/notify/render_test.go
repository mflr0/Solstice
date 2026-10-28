package notify

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// The HTML template runs over every field and branch of a page without an
// error: page.html drops the error it cannot return.
func TestPageHTML(t *testing.T) {
	full := &page{Lang: "fr", Title: "Titre", Heading: "En-tête", Intro: []string{"Bonjour"},
		Action: action{Label: "Choisir", URL: "https://solstice.example/reset"}, Outro: []string{"Merci"},
		Sections: []section{{Key: "DEMO-1", Summary: "Résumé", URL: "https://solstice.example/browse/DEMO-1",
			Meta: "Projet", Link: "Ouvrir", Entries: []entry{{Headline: "Paul a changé", When: "10:00",
				Changes: []changeRow{{Field: "Statut", From: "À faire", To: "Fait"}, {Field: "Étiquettes", To: "x"}},
				Text:    "Un commentaire", Note: "Note"}}}},
		Tables: []table{
			{Title: "Échéances", Columns: []string{"Clé", "Résumé"}, More: "Voir plus", MoreURL: "https://solstice.example/issues",
				Rows: []tableRow{{Key: "DEMO-2", URL: "https://solstice.example/browse/DEMO-2", Cells: []string{"DEMO-2", "Deux"}}}},
			{Title: "Vide", Empty: "Rien", More: "Plus"},
		},
		Why: "Pourquoi", Prefs: "Préférences", PrefsURL: "https://solstice.example/prefs"}
	for _, p := range []*page{full, {Prefs: "Préférences"}, {}} {
		var b bytes.Buffer
		if err := htmlPage.Execute(&b, p); err != nil {
			t.Fatalf("%+v: %v", p, err)
		}
	}
}

// Comments come out of e-mails as rendered wiki markup, styled inline, with
// links to the site made absolute, or left out with no address to give them.
func TestEntryWikiHTML(t *testing.T) {
	src := "*gras* <b>x</b>\n{code}a < b{code}\n[la fiche|/browse/WEB-1] [doc|https://exemple.org]"
	s := &Service{BaseURL: func() string { return "https://solstice.example/" }}
	p := &page{Sections: []section{{Key: "WEB-1", Entries: []entry{{Text: src, HTML: s.wikiHTML(context.Background(), src)}}}}}
	out := p.html()
	for _, want := range []string{`<p style="margin:0 0 6px;"><b>gras</b> &lt;b&gt;x&lt;/b&gt;</p>`, `<pre style="`, "a &lt; b</pre>",
		`<a style="color:#c2321b;" href="https://solstice.example/browse/WEB-1">la fiche</a>`,
		`<a style="color:#c2321b;" href="https://exemple.org">doc</a>`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
	if strings.Contains(out, "*gras*") || strings.Contains(out, "{code}") || strings.Contains(out, "white-space:pre-wrap;word-wrap:break-word;\"><p") {
		t.Errorf("wiki markup shown as written:\n%s", out)
	}
	if !strings.Contains(p.text(), "  > *gras* <b>x</b>") {
		t.Errorf("the plain text is not the wiki text as written:\n%s", p.text())
	}

	s.BaseURL = func() string { return "" }
	if h := string(s.wikiHTML(context.Background(), "[la fiche|/browse/WEB-1]")); h != `<p style="margin:0 0 6px;">la fiche</p>` {
		t.Errorf("without a base URL: %s", h)
	}
}
