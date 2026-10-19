package notify

import (
	"bytes"
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
