package themes

import (
	"fmt"
	"io/fs"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// carnetPackage is a copy of the Carnet package, which breaks no rule, for a
// test to break one.
func carnetPackage(t *testing.T) fstest.MapFS {
	t.Helper()
	root, err := fs.Sub(builtin, "builtin/carnet")
	if err != nil {
		t.Fatal(err)
	}
	pkg := fstest.MapFS{}
	err = fs.WalkDir(root, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(root, name)
		pkg[name] = &fstest.MapFile{Data: data}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

// swap replaces the first old in a file of the package.
func swap(t *testing.T, pkg fstest.MapFS, file, old, new string) {
	t.Helper()
	text := string(pkg[file].Data)
	if !strings.Contains(text, old) {
		t.Fatalf("%s has no %q", file, old)
	}
	pkg[file] = &fstest.MapFile{Data: []byte(strings.Replace(text, old, new, 1))}
}

// setToken sets the first declaration of a token in theme.css.
func setToken(t *testing.T, pkg fstest.MapFS, name, value string) {
	t.Helper()
	re := regexp.MustCompile(`--` + name + `:[^;]*;`)
	text := pkg[cssFile].Data
	loc := re.FindIndex(text)
	if loc == nil {
		t.Fatalf("no --%s", name)
	}
	out := string(text[:loc[0]]) + "--" + name + ": " + value + ";" + string(text[loc[1]:])
	pkg[cssFile] = &fstest.MapFile{Data: []byte(out)}
}

func appendCSS(pkg fstest.MapFS, css string) {
	pkg[cssFile] = &fstest.MapFile{Data: append(slices.Clone(pkg[cssFile].Data), "\n"+css+"\n"...)}
}

func rulesOf(findings []Finding) []string {
	rules := map[string]bool{}
	for _, f := range findings {
		rules[f.Rule] = true
	}
	return slices.Sorted(maps.Keys(rules))
}

func TestValidateBuiltin(t *testing.T) {
	list := Builtin().list
	if len(list) != 8 {
		t.Errorf("%d built-in themes, want 8", len(list))
	}
	for _, p := range list {
		dir, err := fs.Sub(builtin, "builtin/"+p.Manifest.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range Validate(dir) {
			t.Errorf("%s: %s", p.Manifest.ID, f)
		}
	}
}

func TestValidateAllowsWhatLoadsNothing(t *testing.T) {
	for _, css := range []string{
		`.x::before { content: "\00d7"; }`,
		`/* @\69mport */`,
		`.x { background: image-set(url("data:image/png;base64,AA") 1x, "data:image/png;base64,AA" type("image/png") 2x); }`,
		`.x { background: my-image("https://example.com/a.png"); }`,
		`@font-face { font-family: "A"; src: local("https://example.com/a"); }`,
	} {
		pkg := carnetPackage(t)
		appendCSS(pkg, css)
		for _, f := range Validate(pkg) {
			t.Errorf("%s: %s", css, f)
		}
	}
}

func TestValidateRules(t *testing.T) {
	cases := []struct {
		name    string
		breakIt func(t *testing.T, pkg fstest.MapFS)
		rules   []string
		message string // part of one finding's message, optional
	}{
		{"no manifest", func(t *testing.T, pkg fstest.MapFS) { delete(pkg, manifestFile) }, []string{"manifest"}, ""},
		{"bad manifest", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, manifestFile, `"version": "1.0.0"`, `"version": "one"`)
		}, []string{"manifest"}, "not a semantic version"},
		{"no theme.css", func(t *testing.T, pkg fstest.MapFS) { delete(pkg, cssFile) }, []string{"css"}, ""},

		{"too many files", func(t *testing.T, pkg fstest.MapFS) {
			for i := range MaxPackageFiles {
				pkg[fmt.Sprintf("assets/extra-%d.png", i)] = &fstest.MapFile{}
			}
		}, []string{"size"}, "files"},
		{"too large", func(t *testing.T, pkg fstest.MapFS) {
			pkg["assets/large.png"] = &fstest.MapFile{Data: make([]byte, MaxPackageBytes)}
		}, []string{"size"}, "bytes"},

		{"token missing", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, cssFile, "--desk: var(--paper);", "")
		}, []string{"tokens"}, "--desk is not declared"},
		{"token only in a nested rule", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, cssFile, "--desk: var(--paper);", ".nested { --desk: var(--paper); }")
		}, []string{"tokens"}, "--desk is not declared"},
		{"token only in a comment", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, cssFile, "--desk: var(--paper);", "/* --desk: var(--paper); */")
		}, []string{"tokens"}, "--desk is not declared"},
		{"no first block", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, cssFile, "[data-skin='carnet'] {", ".other {")
		}, []string{"tokens"}, "no [data-skin='carnet'] block"},
		{"unclosed block", func(t *testing.T, pkg fstest.MapFS) {
			text := string(pkg[cssFile].Data)
			pkg[cssFile] = &fstest.MapFile{Data: []byte(text[:strings.Index(text, "--desk:")])}
		}, []string{"tokens"}, "unclosed block"},

		{"text contrast", func(t *testing.T, pkg fstest.MapFS) { setToken(t, pkg, "muted", "#eeeeee") },
			[]string{"contrast"}, "light: --muted on --paper:"},
		{"non-text contrast", func(t *testing.T, pkg fstest.MapFS) { setToken(t, pkg, "focus", "#f0f0f0") },
			[]string{"contrast"}, "--focus on --paper"},
		{"gradient stop", func(t *testing.T, pkg fstest.MapFS) {
			setToken(t, pkg, "primary", "linear-gradient(#d9381e, #ffffff)")
		}, []string{"contrast"}, "--primary-ink on --primary"},
		{"rgb colour", func(t *testing.T, pkg fstest.MapFS) { setToken(t, pkg, "muted", "rgb(238, 238, 238)") },
			[]string{"contrast"}, "--muted on --paper"},
		{"through var()", func(t *testing.T, pkg fstest.MapFS) {
			setToken(t, pkg, "muted", "var(--desk)")
		}, []string{"contrast"}, "--muted on --paper"},
		{"var() to nothing", func(t *testing.T, pkg fstest.MapFS) { setToken(t, pkg, "muted", "var(--nothing)") },
			[]string{"contrast"}, "--nothing is not defined"},
		{"var() to itself", func(t *testing.T, pkg fstest.MapFS) { setToken(t, pkg, "muted", "var(--muted)") },
			[]string{"contrast"}, "refers to itself"},
		{"not a colour", func(t *testing.T, pkg fstest.MapFS) { setToken(t, pkg, "muted", "inherit") },
			[]string{"contrast"}, "--muted = inherit is not a colour"},
		{"a stamp as a gradient", func(t *testing.T, pkg fstest.MapFS) {
			setToken(t, pkg, "stamp-todo", "linear-gradient(#000000, #111111)")
		}, []string{"contrast"}, "--stamp-todo must be a single colour"},
		{"stamp on its tint", func(t *testing.T, pkg fstest.MapFS) { setToken(t, pkg, "stamp-fill", "90%") },
			[]string{"contrast"}, "tinted"},
		{"stamp-fill not a percentage", func(t *testing.T, pkg fstest.MapFS) { setToken(t, pkg, "stamp-fill", "lots") },
			[]string{"contrast"}, "--stamp-fill = lots"},
		{"dark scheme only", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, cssFile, "color-scheme: dark;", "color-scheme: dark;\n  --muted: #222222;")
		}, []string{"contrast"}, "dark: --muted on --paper"},
		{"declared pair", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, "/* contrast: --ink on --paper 30 */")
		}, []string{"contrast"}, "--ink on --paper: "},
		{"declared pair, unknown token", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, "/* contrast: --nothing on --paper */")
		}, []string{"contrast"}, "--nothing is not defined"},

		{"px font size", func(t *testing.T, pkg fstest.MapFS) { appendCSS(pkg, ".x { font-size: 12px; }") },
			[]string{"font-size"}, "12px"},
		{"px in calc", func(t *testing.T, pkg fstest.MapFS) { appendCSS(pkg, ".x { font-size: calc(1rem + 2px) }") },
			[]string{"font-size"}, "calc(1rem + 2px)"},
		{"fractional px", func(t *testing.T, pkg fstest.MapFS) { appendCSS(pkg, ".x { font-size: .5px }") },
			[]string{"font-size"}, ".5px"},

		{"@import", func(t *testing.T, pkg fstest.MapFS) { appendCSS(pkg, `@import "other.css";`) },
			[]string{"import"}, ""},
		{"@IMPORT", func(t *testing.T, pkg fstest.MapFS) { appendCSS(pkg, `@IMPORT "other.css";`) },
			[]string{"import"}, ""},

		{"url outside the package", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, ".x { background: url(https://example.com/a.png); }")
		}, []string{"url"}, "want a file under"},
		{"URL in capitals", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, ".x { background: URL('https://example.com/a.png'); }")
		}, []string{"url"}, "want a file under"},
		{"url climbing out", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, ".x { background: url('fonts/../../a.png'); }")
		}, []string{"url"}, "want a file under"},
		{"url with ./", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, ".x { background: url('./assets/a.svg'); }")
		}, []string{"url"}, "want a file under"},
		{"url to a missing file", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, ".x { background: url('assets/a.svg'); }")
		}, []string{"url"}, "no such file"},
		{"url to a folder", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, ".x { background: url(fonts); }")
		}, []string{"url"}, "want a file under"},
		{"url hiding behind /*", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, ".x { background: url(fonts/a/*.png); } .y { background: url(https://example.com/b.png); } /* */")
		}, []string{"url"}, "url(https://example.com/b.png)"},

		{"escape in an at-rule name", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `@\69mport "https://example.com/a.css";`)
		}, []string{"escape"}, "only allowed in strings"},
		{"escape in a function name", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { background: u\72l(https://example.com/a.png); }`)
		}, []string{"escape"}, "only allowed in strings"},
		{"escape in a selector", func(t *testing.T, pkg fstest.MapFS) { appendCSS(pkg, `.a\/b { color: red; }`) },
			[]string{"escape"}, ""},
		{"escape in an unquoted url", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { background: url(fonts/\61.png); }`)
		}, []string{"escape", "url"}, "only allowed in strings"},
		{"escaped quote hiding an @import", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.a\" {} @import "https://example.com/a.css"; .b\" {}`)
		}, []string{"escape", "import"}, ""},

		{"image-set string", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { background: image-set("https://example.com/a.png" 1x); }`)
		}, []string{"url"}, "image-set(https://example.com/a.png): want a file under"},
		{"-webkit-image-set string", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { background: -webkit-image-set("https://example.com/a.png" 1x); }`)
		}, []string{"url"}, "image-set(https://example.com/a.png)"},
		{"image string", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { background: image(ltr 'https://example.com/a.png'); }`)
		}, []string{"url"}, "image(https://example.com/a.png)"},
		{"src string", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { background: SRC("https://example.com/a.png"); }`)
		}, []string{"url"}, "src(https://example.com/a.png)"},
		{"image-set second string", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { background: image-set("data:image/png;base64,AA" 1x, "https://example.com/a.png" 2x); }`)
		}, []string{"url"}, "image-set(https://example.com/a.png)"},
		{"image-set to a missing file", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { background: image-set("assets/a.png" type("image/png") 1x); }`)
		}, []string{"url"}, "image-set(assets/a.png): no such file"},
		{"image-set through var()", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { --a: "https://example.com/a.png"; background: image-set(var(--a) 1x); }`)
		}, []string{"load"}, "var() inside image-set()"},
		{"src through attr()", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { background: src(attr(data-a)); }`)
		}, []string{"load"}, "attr() inside src()"},

		{"manifest lists dark, no dark block", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, cssFile, "[data-skin='carnet'][data-scheme='dark'] {", ".other {")
		}, []string{"schemes"}, "no dark block"},
		{"dark block, manifest does not list it", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, manifestFile, `"light",`, `"light"`)
			swap(t, pkg, manifestFile, `"dark"`+"\n", "\n")
			text := string(pkg[manifestFile].Data)
			text = regexp.MustCompile(`,\s*"dark": \{[^}]*\}`).ReplaceAllString(text, "")
			pkg[manifestFile] = &fstest.MapFile{Data: []byte(text)}
		}, []string{"schemes"}, "does not list dark"},

		{"preview differs", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, manifestFile, `"paper": "#f4f1ea"`, `"paper": "#000000"`)
		}, []string{"preview"}, "preview.light.paper is #000000, --paper is [#f4f1ea]"},
		{"preview differs in dark", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, manifestFile, `"ink": "#ebe6da"`, `"ink": "#ffffff"`)
		}, []string{"preview"}, "preview.dark.ink"},
		{"preview of a var() token", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, manifestFile, `"bar": "#faf8f3"`, `"bar": "#eeeeee"`)
		}, []string{"preview"}, "--bar is [#faf8f3]"},
		{"preview not a gradient's stop", func(t *testing.T, pkg fstest.MapFS) {
			setToken(t, pkg, "bar", "linear-gradient(#eeeeee, #dddddd)")
			swap(t, pkg, cssFile, "color-scheme: dark;", "color-scheme: dark;\n  --bar: #22211d;")
		}, []string{"preview"}, "preview.light.bar"},

		{"font with no licence", func(t *testing.T, pkg fstest.MapFS) {
			delete(pkg, "fonts/ibm-plex-mono-OFL.txt")
		}, []string{"licence"}, "no licence file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pkg := carnetPackage(t)
			c.breakIt(t, pkg)
			findings := Validate(pkg)
			if got := rulesOf(findings); !slices.Equal(got, c.rules) {
				t.Errorf("rules %v, want %v: %v", got, c.rules, findings)
			}
			if c.message != "" && !slices.ContainsFunc(findings, func(f Finding) bool {
				return strings.Contains(f.Message, c.message)
			}) {
				t.Errorf("no finding says %q: %v", c.message, findings)
			}
		})
	}
}

func TestValidateAccepts(t *testing.T) {
	cases := []struct {
		name  string
		allow func(t *testing.T, pkg fstest.MapFS)
	}{
		{"carnet as it is", func(*testing.T, fstest.MapFS) {}},
		{"px elsewhere, 0 font size", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, ".x { font-size: 0; --gap: 4px; padding: 3px; } .y { font-size: 0px } .z { font-size: 0.75rem }")
		}},
		{"px font size in a comment or a string", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `/* font-size: 12px */ .x::after { content: 'font-size: 12px'; }`)
		}},
		{"@import in a comment or a string", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `/* @import "a.css"; */ .x::after { content: '@import'; }`)
		}},
		{"url in a comment or a string", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `/* url('https://example.com/a') */ .x::after { content: "url(https://example.com/b)"; }`)
		}},
		{"data: url holding a url(", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { background: url("data:image/svg+xml,%3Csvg fill='url(%23g)'/%3E"); }`)
		}},
		{"url with a fragment, unquoted, spaced", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { mask: url( fonts/ibm-plex-mono-OFL.txt#draw ); }`)
		}},
		{"the function of another name", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, `.x { background: my-url(https://example.com/a); }`)
		}},
		{"declared pair that holds", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, "/* contrast: --rule on --paper 1 */")
		}},
		{"3:1 is enough for focus", func(t *testing.T, pkg fstest.MapFS) {
			setToken(t, pkg, "focus", "#8a8a8a")
		}},
		{"preview in capitals", func(t *testing.T, pkg fstest.MapFS) {
			swap(t, pkg, manifestFile, `"paper": "#f4f1ea"`, `"paper": "#F4F1EA"`)
		}},
		{"preview is a gradient's stop", func(t *testing.T, pkg fstest.MapFS) {
			setToken(t, pkg, "bar", "linear-gradient(#eeeeee, #faf8f3)")
			swap(t, pkg, cssFile, "color-scheme: dark;", "color-scheme: dark;\n  --bar: #22211d;")
		}},
		{"a folder-wide licence", func(t *testing.T, pkg fstest.MapFS) {
			for _, name := range []string{"ibm-plex-mono", "ibm-plex-sans", "source-serif-4"} {
				delete(pkg, "fonts/"+name+"-OFL.txt")
			}
			pkg["fonts/OFL.txt"] = &fstest.MapFile{Data: []byte("licence")}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pkg := carnetPackage(t)
			c.allow(t, pkg)
			if findings := Validate(pkg); len(findings) != 0 {
				t.Errorf("findings: %v", findings)
			}
		})
	}
}

func TestValidateWhere(t *testing.T) {
	pkg := carnetPackage(t)
	text := string(pkg[cssFile].Data)
	wantLine := strings.Count(text, "\n") + 2 // appendCSS opens a new line.
	appendCSS(pkg, ".x { font-size: 12px; }")
	findings := Validate(pkg)
	want := Finding{Rule: "font-size", Message: "font size in px: 12px, use rem", File: cssFile, Line: wantLine}
	if !slices.Equal(findings, []Finding{want}) {
		t.Errorf("findings %v, want %v", findings, want)
	}

	pkg = carnetPackage(t)
	swap(t, pkg, manifestFile, `"paper": "#f4f1ea"`, `"paper": "#000000"`)
	findings = Validate(pkg)
	manifest := string(pkg[manifestFile].Data)
	wantLine = strings.Count(manifest[:strings.Index(manifest, `"preview"`)], "\n") + 1
	if len(findings) != 1 || findings[0].File != manifestFile || findings[0].Line != wantLine {
		t.Errorf("findings %v, want %s:%d", findings, manifestFile, wantLine)
	}
}

func TestValidateReportsEveryPair(t *testing.T) {
	pkg := carnetPackage(t)
	setToken(t, pkg, "muted", "#eeeeee")
	findings := Validate(pkg)
	for _, bg := range []string{"--paper:", "--paper-raised:", "--paper-sunk:"} {
		if !slices.ContainsFunc(findings, func(f Finding) bool {
			return strings.Contains(f.Message, "--muted on "+bg)
		}) {
			t.Errorf("nothing on %s: %v", bg, findings)
		}
	}
}

func TestParseColours(t *testing.T) {
	cases := []struct {
		value string
		want  []rgb
	}{
		{"#ffffff", []rgb{{255, 255, 255}}},
		{"#FFF", []rgb{{255, 255, 255}}},
		{"#102030", []rgb{{16, 32, 48}}},
		{"rgb(1, 2, 3)", []rgb{{1, 2, 3}}},
		{"rgba(1 2 3 / 50%)", []rgb{{1, 2, 3}}},
		{"linear-gradient(180deg, #000000 0%, #ffffff 100%)", []rgb{{0, 0, 0}, {255, 255, 255}}},
		{"#abcd", nil},
		{"inherit", nil},
	}
	for _, c := range cases {
		if got := parseColours(c.value); !slices.Equal(got, c.want) {
			t.Errorf("%q: %v, want %v", c.value, got, c.want)
		}
	}
}

func TestContrastRatio(t *testing.T) {
	if got := contrastRatio(rgb{0, 0, 0}, rgb{255, 255, 255}); math.Abs(got-21) > 1e-9 {
		t.Errorf("black on white: %v", got)
	}
	if got := contrastRatio(rgb{255, 255, 255}, rgb{0, 0, 0}); math.Abs(got-21) > 1e-9 {
		t.Errorf("white on black: %v", got)
	}
	if got := contrastRatio(rgb{118, 118, 118}, rgb{255, 255, 255}); math.Abs(got-4.54) > 0.01 {
		t.Errorf("#767676 on white: %v", got)
	}
	if got := over(rgb{100, 100, 100}, rgb{200, 200, 200}, 0.14); got != (rgb{186, 186, 186}) {
		t.Errorf("over: %v", got)
	}
}

func TestParseCSS(t *testing.T) {
	css := parseCSS(`.a { background: url('./x/a.svg'), url("data:image/svg+xml,%3Csvg fill='url(%23g)'/%3E"); }
/* url('./y/commented.svg') */
.b { mask: url( ./x/b.svg#draw ); }`)
	var got []string
	for _, u := range css.urls {
		got = append(got, u.target)
	}
	want := []string{"./x/a.svg", "data:image/svg+xml,%3Csvg fill='url(%23g)'/%3E", "./x/b.svg#draw"}
	if !slices.Equal(got, want) {
		t.Errorf("urls %q, want %q", got, want)
	}
	if line := css.line(css.urls[2].offset); line != 3 {
		t.Errorf("line %d, want 3", line)
	}
}

func TestTokenBlock(t *testing.T) {
	css := parseCSS(`/* [data-skin='x'] { --commented: 1 } */
:where(:root),
[data-skin="x"] {
  --a: #000; /* a comment; with a semicolon */
  --b: url('a;b.svg');
  --c:
    #111;
  .nested { --d: 1; }
  color-scheme: light;
  --e: 2
}
[data-skin='x'][data-scheme='dark'] { --a: #fff; }
[data-skin='x'] .y { --f: 1; }
`)
	block, _, ok, err := tokenBlock(css, "x", false)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	want := map[string]token{
		"a": {"#000", 4},
		"b": {"url('a;b.svg')", 5},
		"c": {"#111", 6},
		"e": {"2", 10},
	}
	if !maps.Equal(block, tokens(want)) {
		t.Errorf("first block %v, want %v", block, want)
	}
	dark, _, ok, err := tokenBlock(css, "x", true)
	if err != nil || !ok || dark["a"].value != "#fff" || len(dark) != 1 {
		t.Errorf("dark block %v %v %v", dark, ok, err)
	}
	if _, _, ok, err := tokenBlock(css, "other", false); ok || err != nil {
		t.Errorf("another skin: %v %v", ok, err)
	}
}
