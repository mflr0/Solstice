package i18n

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// messageFuncs are the functions that take a message template, by package
// directory: function name → index of the template argument.
var messageFuncs = map[string]map[string]int{
	"../api":    {"fail": 1, "failf": 1, "M": 0},
	"../jql":    {"fail": 0, "errorf": 0, "errAt": 1},
	"../notify": {"M": 0},
	"../issue":  {"M": 0},
}

// forwards are the template arguments that are not literals: the helpers
// passing on their own parameter, and messages already built elsewhere.
var forwards = []string{"msg", "format", "je.Template", "err.Error()"}

// sourceTemplates reads the code for every message template, with where it
// is used.
func sourceTemplates(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for dir, funcs := range messageFuncs {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no Go files in %s: %v", dir, err)
		}
		fset := token.NewFileSet()
		for _, name := range files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var fn string
				switch f := call.Fun.(type) {
				case *ast.Ident:
					fn = f.Name
				case *ast.SelectorExpr:
					fn = f.Sel.Name
				}
				i, ok := funcs[fn]
				if !ok || len(call.Args) <= i {
					return true
				}
				at := fset.Position(call.Pos()).String()
				lit, ok := call.Args[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					if expr := types.ExprString(call.Args[i]); !slices.Contains(forwards, expr) {
						t.Errorf("%s: %s takes a message that is not a literal (%s): it cannot be translated", at, fn, expr)
					}
					return true
				}
				tmpl, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				if call.Ellipsis == token.NoPos {
					if want, got := len(verbs(tmpl)), len(call.Args)-i-1; want != got {
						t.Errorf("%s: %q expects %d values, given %d", at, tmpl, want, got)
					}
				}
				out[tmpl] = at
				return true
			})
		}
	}
	if len(out) < 100 {
		t.Fatalf("only %d templates found: is the scan still looking in the right places?", len(out))
	}
	return out
}

// verbs lists the placeholders of a template as "argument index:verb", in
// argument order, so that a translation reordering them still compares equal.
func verbs(s string) []string {
	var out []string
	arg := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		i++
		if i < len(s) && s[i] == '%' {
			continue
		}
		for i < len(s) && strings.IndexByte("+-# 0", s[i]) >= 0 {
			i++
		}
		if i < len(s) && s[i] == '[' {
			end := strings.IndexByte(s[i:], ']')
			if end < 0 {
				return append(out, "bad index")
			}
			n, err := strconv.Atoi(s[i+1 : i+end])
			if err != nil {
				return append(out, "bad index")
			}
			arg, i = n-1, i+end+1
		}
		for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
			i++
		}
		if i < len(s) {
			out = append(out, fmt.Sprintf("%d:%c", arg, s[i]))
			arg++
		}
	}
	slices.Sort(out)
	return out
}

func TestCataloguesComplete(t *testing.T) {
	src := sourceTemplates(t)
	if os.Getenv("I18N_LIST") != "" { // prints the templates, to write a new catalogue
		keys := make([]string, 0, len(src))
		for k := range src {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			fmt.Printf("%q\n", k)
		}
	}
	for _, locale := range Locales[1:] {
		cat := catalogues[locale]
		if cat == nil {
			t.Fatalf("no catalogue for %s", locale)
		}
		for tmpl, at := range src {
			tr, ok := cat[tmpl]
			switch {
			case !ok || strings.TrimSpace(tr) == "":
				t.Errorf("%s: no translation of %q (%s)", locale, tmpl, at)
			case !slices.Equal(verbs(tr), verbs(tmpl)):
				t.Errorf("%s: placeholders differ: %q has %v, %q has %v", locale, tmpl, verbs(tmpl), tr, verbs(tr))
			}
		}
		for tmpl := range cat {
			if _, ok := src[tmpl]; !ok {
				t.Errorf("%s: %q is not used any more", locale, tmpl)
			}
		}
	}
}

func TestVerbs(t *testing.T) {
	for s, want := range map[string][]string{
		"Unknown status: %s.":    {"0:s"},
		"%d characters at most.": {"0:d"},
		"%s: %s":                 {"0:s", "1:s"},
		"%[2]s, then %[1]s":      {"0:s", "1:s"},
		"100 %% sûr, %5.2f":      {"0:f"},
		"Aucun":                  nil,
	} {
		if got := verbs(s); !slices.Equal(got, want) {
			t.Errorf("verbs(%q) = %v, want %v", s, got, want)
		}
	}
}
