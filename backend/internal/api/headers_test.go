package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The interface's policy allows only its own scripts and styles; inline
// styles stay allowed for skins, Svelte attributes and wiki colours.
func TestSecurityHeadersCSP(t *testing.T) {
	want := "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; object-src 'none'; " +
		"frame-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
	w := httptest.NewRecorder()
	securityHeaders(http.NotFoundHandler()).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if got := w.Header().Get("Content-Security-Policy"); got != want {
		t.Errorf("Content-Security-Policy = %q", got)
	}
}
