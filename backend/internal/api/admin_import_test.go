package api

import (
	"net/http/httptest"
	"testing"

	"solstice/internal/auth"
)

// The import operator's bearer token is recognised whatever the scheme's case,
// as credentials reads it.
func TestCredentialKeysScheme(t *testing.T) {
	want := auth.TokenHash("secret-token")
	for _, h := range []string{"Bearer secret-token", "bearer secret-token", "BEARER  secret-token "} {
		r := httptest.NewRequest("GET", "/api/admin/import", nil)
		r.Header.Set("Authorization", h)
		if got := credentialKeys(r); len(got) != 1 || got[0] != want {
			t.Errorf("%q: keys %v, want [%s]", h, got, want)
		}
	}
	r := httptest.NewRequest("GET", "/api/admin/import", nil)
	r.Header.Set("Authorization", "Basic YWRtaW46YWRtaW4=")
	if got := credentialKeys(r); len(got) != 0 {
		t.Errorf("basic credentials: keys %v", got)
	}
}
