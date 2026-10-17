package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// An error that is not meant for the user never reaches the client: the
// answer is the generic message, in the request's language.
func TestWriteErrorHidesInternalErrors(t *testing.T) {
	secret := errors.New("dial tcp 10.0.0.5:3306: connection refused")
	r := httptest.NewRequest("GET", "/api/anything", nil)
	r.Header.Set("Accept-Language", "fr")

	w := httptest.NewRecorder()
	writeError(w, r, secret)
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "10.0.0.5") ||
		!strings.Contains(w.Body.String(), "Erreur interne") {
		t.Errorf("interface error: %d %s", w.Code, w.Body)
	}

	w = httptest.NewRecorder()
	writeRESTError(w, r, secret)
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "10.0.0.5") ||
		!strings.Contains(w.Body.String(), "Erreur interne") {
		t.Errorf("REST error: %d %s", w.Code, w.Body)
	}
}
