package tests

// Every /api/admin/ route turns away anonymous callers and ordinary users
// before it reads the request.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
)

var pathParam = regexp.MustCompile(`\{[a-z]+\}`)

// routePattern reads a pattern given to route(): a string, possibly joined
// with the catalogue kind of the loop in adminRoutes (as %s).
func routePattern(e ast.Expr) (string, bool) {
	switch e := e.(type) {
	case *ast.BasicLit:
		s, err := strconv.Unquote(e.Value)
		return s, err == nil
	case *ast.Ident:
		return "%s", e.Name == "kind"
	case *ast.BinaryExpr:
		l, lok := routePattern(e.X)
		r, rok := routePattern(e.Y)
		return l + r, e.Op == token.ADD && lok && rok
	}
	return "", false
}

// adminRoutes lists the registered administration routes ("GET /api/…") from
// the source of the registration: the server keeps no list of them.
func adminRoutes(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../internal/api/*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var kinds, patterns []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ValueSpec:
				if n.Names[0].Name == "catalogueKinds" {
					for _, kv := range n.Values[0].(*ast.CompositeLit).Elts {
						kind, _ := routePattern(kv.(*ast.KeyValueExpr).Key)
						kinds = append(kinds, kind)
					}
				}
			case *ast.CallExpr:
				if fn, ok := n.Fun.(*ast.Ident); ok && fn.Name == "route" && len(n.Args) == 3 {
					p, ok := routePattern(n.Args[0])
					if !ok {
						t.Fatalf("%s: cannot read the route pattern", fset.Position(n.Pos()))
					}
					patterns = append(patterns, p)
				}
			}
			return true
		})
	}
	var routes []string
	for _, p := range patterns {
		if !strings.Contains(p, " /api/admin/") {
			continue
		}
		if !strings.Contains(p, "%s") {
			routes = append(routes, p)
			continue
		}
		for _, kind := range kinds {
			routes = append(routes, fmt.Sprintf(p, kind))
		}
	}
	if len(routes) == 0 {
		t.Fatal("no administration route found")
	}
	return routes
}

// brokenRequest sends a body that cannot be parsed, so that a route reading
// it before checking who asks would answer 400.
func brokenRequest(t *testing.T, c *client, method, path string) int {
	t.Helper()
	req, err := http.NewRequest(method, c.base+path, strings.NewReader("{"))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeader, "1")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestAdminRoutesRequireAdmin(t *testing.T) {
	needDB(t)
	callers := []struct {
		who    string
		client *client
		want   int
	}{
		{"anonymous", anonymous(t), 401},
		{"user", login(t, "paul.martin", jirafixture.Password), 403},
	}
	for _, route := range adminRoutes(t) {
		method, pattern, _ := strings.Cut(route, " ")
		path := pathParam.ReplaceAllString(pattern, "1")
		t.Run(route, func(t *testing.T) {
			for _, c := range callers {
				if status := brokenRequest(t, c.client, method, path); status != c.want {
					t.Errorf("%s: %d, want %d", c.who, status, c.want)
				}
			}
		})
	}
}
