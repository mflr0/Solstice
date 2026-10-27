package i18n

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// messageFuncs are the functions that take a message key, by package
// directory: function name → index of the key argument. i18n.M counts in
// every package.
var messageFuncs = map[string]map[string]int{
	"../api": {"fail": 1, "failf": 1},
	"../jql": {"fail": 0, "errorf": 0, "errAt": 1},
}

// forwards are the key arguments that are not constants: the helpers passing
// on their own parameter, and messages already built elsewhere.
var forwards = []string{"key", "je.Key", "e.Key", "one", "many"}

// declaredKeys reads keys.go for the Key constants, each valued its own name.
func declaredKeys(t *testing.T) map[Key]bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "keys.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	out := map[Key]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if v, err := strconv.Unquote(lit.Value); !ok || err != nil || v != name.Name {
					t.Errorf("keys.go: %s is not valued its own name", name.Name)
				}
				out[Key(name.Name)] = true
			}
		}
	}
	return out
}

// usedKeys reads the code outside this package for every key it uses, with
// where. It also checks that the message functions get a key and as many
// values as the English template has placeholders.
func usedKeys(t *testing.T, keys map[Key]bool) map[Key]string {
	t.Helper()
	out := map[Key]string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") ||
			filepath.Dir(path) == "../i18n" {
			return err
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		funcs := messageFuncs[filepath.Dir(path)]
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if x, ok := n.X.(*ast.Ident); ok && x.Name == "i18n" && keys[Key(n.Sel.Name)] {
					out[Key(n.Sel.Name)] = fset.Position(n.Pos()).String()
				}
			case *ast.CallExpr:
				var fn string
				i, ok := 0, false
				switch f := n.Fun.(type) {
				case *ast.Ident:
					fn = f.Name
					i, ok = funcs[fn]
				case *ast.SelectorExpr:
					if x, isIdent := f.X.(*ast.Ident); isIdent && x.Name == "i18n" && f.Sel.Name == "M" {
						fn, i, ok = "i18n.M", 0, true
					}
					if x, isIdent := f.X.(*ast.Ident); isIdent && x.Name == "i18n" && f.Sel.Name == "N" {
						checkCount(t, fset, keys, n)
						return true
					}
				}
				if !ok || len(n.Args) <= i {
					return true
				}
				checkKey(t, fset.Position(n.Pos()).String(), fn, keys, n.Args[i], n.Ellipsis == token.NoPos, len(n.Args)-i-1)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 100 {
		t.Fatalf("only %d keys used: is the scan still looking in the right places?", len(out))
	}
	return out
}

// checkKey checks that a message function gets a key and, when values is
// known, as many values as the English template has placeholders.
func checkKey(t *testing.T, at, fn string, keys map[Key]bool, arg ast.Expr, counted bool, values int) {
	t.Helper()
	sel, ok := arg.(*ast.SelectorExpr)
	if !ok || !keys[Key(sel.Sel.Name)] {
		if expr := types.ExprString(arg); !slices.Contains(forwards, expr) {
			t.Errorf("%s: %s takes a message that is not a key (%s)", at, fn, expr)
		}
		return
	}
	if counted {
		key := Key(sel.Sel.Name)
		if want := len(verbs(en[key])); want != values {
			t.Errorf("%s: %s expects %d values, given %d", at, key, want, values)
		}
	}
}

// checkCount checks a call of i18n.N: both keys, each given the count and
// the other values.
func checkCount(t *testing.T, fset *token.FileSet, keys map[Key]bool, n *ast.CallExpr) {
	t.Helper()
	at := fset.Position(n.Pos()).String()
	if len(n.Args) < 3 {
		t.Errorf("%s: i18n.N without its two keys", at)
		return
	}
	for _, arg := range n.Args[1:3] {
		checkKey(t, at, "i18n.N", keys, arg, n.Ellipsis == token.NoPos, len(n.Args)-2)
	}
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
	keys := declaredKeys(t)
	used := usedKeys(t, keys)
	for key := range keys {
		if _, ok := used[key]; !ok {
			t.Errorf("%s is not used any more", key)
		}
	}
	for _, locale := range Locales {
		cat := catalogues[locale]
		if cat == nil {
			t.Fatalf("no catalogue for %s", locale)
		}
		for key := range keys {
			tr, ok := cat[key]
			switch {
			case !ok || strings.TrimSpace(tr) == "":
				t.Errorf("%s: no message for %s", locale, key)
			case !slices.Equal(verbs(tr), verbs(en[key])):
				t.Errorf("%s: placeholders differ for %s: %q has %v, English %q has %v",
					locale, key, tr, verbs(tr), en[key], verbs(en[key]))
			}
		}
		for key := range cat {
			if !keys[key] {
				t.Errorf("%s: %s is not a key of keys.go", locale, key)
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
