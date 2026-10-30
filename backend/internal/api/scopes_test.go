package api

import (
	"io"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"solstice/internal/auth"
)

// Every route requires a known scope; only the sign-in and setup routes are
// public, /api/admin needs admin, and a read-only scope never writes.
func TestRouteScopes(t *testing.T) {
	s := New(nil, t.TempDir(), time.UTC, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.Handler(http.NotFoundHandler())
	if len(s.routeScopes) < 200 {
		t.Fatalf("only %d routes noted", len(s.routeScopes))
	}
	public := []string{
		"POST /api/login", "POST /api/logout", "GET /api/auth/oidc/start", "GET /api/auth/oidc/callback",
		"POST /api/auth/forgot", "GET /api/auth/reset/{token}", "POST /api/auth/reset",
		"GET /api/settings/public", "GET /api/setup/status",
		"GET /rest/api/2/serverInfo", "GET /rest/api/latest/serverInfo", "POST /rest/auth/1/session",
		"GET /.well-known/oauth-protected-resource",
		"GET /.well-known/oauth-authorization-server", "POST /oauth/token", "POST /oauth/revoke", "POST /register",
	}
	readingWrites := []string{
		"POST /api/refs", "POST /rest/api/2/search", "POST /rest/api/latest/search", "DELETE /rest/auth/1/session",
	}
	var gotPublic []string
	for _, pattern := range slices.Sorted(maps.Keys(s.routeScopes)) {
		scope := s.routeScopes[pattern]
		method, path, _ := strings.Cut(pattern, " ")
		switch {
		case scope == scopePublic:
			gotPublic = append(gotPublic, pattern)
		case !scope.Known():
			t.Errorf("%s: unknown scope %q", pattern, scope)
		case strings.HasPrefix(path, "/api/admin/") != (scope == auth.ScopeAdmin):
			t.Errorf("%s: scope %s", pattern, scope)
		case method == http.MethodGet && scope != auth.ScopeRead && scope != auth.ScopeAdmin:
			t.Errorf("%s: a GET needs %s", pattern, scope)
		case method != http.MethodGet && scope == auth.ScopeRead && !slices.Contains(readingWrites, pattern):
			t.Errorf("%s: a write needs only read", pattern)
		}
	}
	slices.Sort(public)
	if !slices.Equal(gotPublic, public) {
		t.Errorf("public routes:\n%v\nwant\n%v", gotPublic, public)
	}
}
