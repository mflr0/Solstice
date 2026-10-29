package api

import (
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"solstice/internal/auth"
	"solstice/internal/issue"
)

// Every read of the REST API has its tool, with the route's scope, apart
// from those other tools will cover.
func TestMCPToolsMatchRoutes(t *testing.T) {
	s := New(nil, t.TempDir(), time.UTC, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.Handler(http.NotFoundHandler())
	later := []string{"/filter/", "/version/", "/component/", "/versions", "/components"}
	tools := map[string]*mcpTool{}
	names := map[string]bool{}
	for i := range mcpTools {
		tool := &mcpTools[i]
		if names[tool.name] {
			t.Errorf("tool %s twice", tool.name)
		}
		names[tool.name] = true
		pattern := "GET " + issue.RESTAPIPath + tool.path
		scope, ok := s.routeScopes[pattern]
		switch {
		case !ok:
			t.Errorf("%s: no route %s", tool.name, pattern)
		case scope != tool.scope && !(scope == scopePublic && tool.scope == auth.ScopeRead):
			t.Errorf("%s: scope %s, route's %s", tool.name, tool.scope, scope)
		}
		tools[pattern] = tool
		for _, p := range tool.params {
			if p.pathValue != "" && !strings.Contains(tool.path, "{"+p.pathValue+"}") {
				t.Errorf("%s: %s fills no path value", tool.name, p.name)
			}
		}
	}
	for pattern := range s.routeScopes {
		if !strings.HasPrefix(pattern, "GET "+issue.RESTAPIPath+"/") || tools[pattern] != nil {
			continue
		}
		if !slices.ContainsFunc(later, func(l string) bool { return strings.Contains(pattern, l) }) {
			t.Errorf("%s has no tool", pattern)
		}
	}
}
