package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Errors are written in the language of Accept-Language. This one needs no
// database: unknown addresses and the CSRF check answer before any query.
func TestErrorLanguage(t *testing.T) {
	h := New(nil, t.TempDir(), time.UTC, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler(http.NotFoundHandler())
	for _, c := range []struct{ method, lang, want string }{
		{"GET", "", "Unknown address."},
		{"GET", "en-GB,en;q=0.9", "Unknown address."},
		{"GET", "de-CH, fr;q=0.5", "Unbekannte Adresse."},
		{"GET", "es", "Unknown address."},
		{"GET", "fr-FR,fr;q=0.9", "Adresse inconnue."},
		{"GET", "hi", "अज्ञात पता।"},
		{"POST", "it", "Richiesta rifiutata (intestazione X-Solstice mancante)."},
		{"POST", "", "Request refused (missing X-Solstice header)."},
	} {
		req := httptest.NewRequest(c.method, "/api/nope", nil)
		if c.lang != "" {
			req.Header.Set("Accept-Language", c.lang)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		var out struct{ Error string }
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Error != c.want {
			t.Errorf("%s with %q: %d %s, want %q", c.method, c.lang, w.Code, w.Body, c.want)
		}
	}
}
