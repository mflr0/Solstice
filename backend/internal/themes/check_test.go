package themes

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// zipOfPackage is a package as its zip.
func zipOfPackage(t *testing.T, pkg fstest.MapFS) []byte {
	t.Helper()
	var entries []zipEntry
	for name, f := range pkg {
		entries = append(entries, zipEntry{name: name, data: f.Data})
	}
	return zipOf(t, entries...)
}

// rewrite replaces every match of a pattern in a file of the package.
func rewrite(t *testing.T, pkg fstest.MapFS, file, pattern, with string) {
	t.Helper()
	re := regexp.MustCompile(pattern)
	if !re.Match(pkg[file].Data) {
		t.Fatalf("%s has no %s", file, pattern)
	}
	pkg[file] = &fstest.MapFile{Data: re.ReplaceAll(pkg[file].Data, []byte(with))}
}

func hasRule(findings []Finding, rule string) bool {
	return slices.ContainsFunc(findings, func(f Finding) bool { return f.Rule == rule })
}

func TestCheckCarnet(t *testing.T) {
	got, err := Check(context.Background(), zipOfPackage(t, carnetPackage(t)))
	if err != nil {
		t.Fatal(err)
	}
	if got.Refused || got.Findings == nil || len(got.Findings) != 0 || got.Manifest.ID != "carnet" {
		t.Fatalf("%+v", got.Findings)
	}
	for _, u := range parseCSS(got.CSS).urls {
		if !isDataURL(u.target) {
			t.Errorf("%s(%s) is not inlined", u.call, u.target)
		}
	}
	if !strings.Contains(got.CSS, "data:font/woff2;base64,") {
		t.Error("no font inlined")
	}
}

func TestCheckRefuses(t *testing.T) {
	for _, c := range []struct {
		name, rule string
		breakIt    func(t *testing.T, pkg fstest.MapFS)
	}{
		{"unreadable manifest", "manifest", func(t *testing.T, pkg fstest.MapFS) {
			pkg[manifestFile] = &fstest.MapFile{Data: []byte("{")}
		}},
		{"no manifest", "manifest", func(t *testing.T, pkg fstest.MapFS) { delete(pkg, manifestFile) }},
		{"newer contract", "manifest", func(t *testing.T, pkg fstest.MapFS) {
			rewrite(t, pkg, manifestFile, `"contract":\s*\d+`, fmt.Sprintf(`"contract": %d`, Contract+1))
		}},
		{"no stylesheet", "css", func(t *testing.T, pkg fstest.MapFS) { delete(pkg, cssFile) }},
		{"import", "import", func(t *testing.T, pkg fstest.MapFS) { appendCSS(pkg, "@import 'a.css';") }},
		{"url elsewhere", "url", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, "a { background: url(https://example.com/a.png); }")
		}},
		{"url to no file", "url", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, "a { background: url(assets/none.png); }")
		}},
		{"escape", "escape", func(t *testing.T, pkg fstest.MapFS) { appendCSS(pkg, `@\69mport 'a.css';`) }},
		{"loader", "load", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, "a { background: image-set(var(--x) 1x); }")
		}},
		{"with a finding that alone is accepted", "import", func(t *testing.T, pkg fstest.MapFS) {
			appendCSS(pkg, "a { font-size: 12px; }\n@import 'a.css';")
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			pkg := carnetPackage(t)
			c.breakIt(t, pkg)
			got, err := Check(context.Background(), zipOfPackage(t, pkg))
			if err != nil {
				t.Fatal(err)
			}
			if !got.Refused || !hasRule(got.Findings, c.rule) || got.CSS != "" {
				t.Errorf("refused %v, css %d bytes, findings %v", got.Refused, len(got.CSS), got.Findings)
			}
		})
	}
	t.Run("unzip limit", func(t *testing.T) {
		got, err := Check(context.Background(), zipOf(t, named("../x")))
		if err != nil {
			t.Fatal(err)
		}
		if !got.Refused || !hasRule(got.Findings, "zip") {
			t.Errorf("%+v", got)
		}
	})
	t.Run("not a zip", func(t *testing.T) {
		got, err := Check(context.Background(), []byte("nope"))
		if err != nil {
			t.Fatal(err)
		}
		if !got.Refused || !hasRule(got.Findings, "zip") {
			t.Errorf("%+v", got)
		}
	})
	t.Run("a damaged entry", func(t *testing.T) {
		data := zipOfPackage(t, carnetPackage(t))
		// The CRC in the last entry's central header: its bytes no longer match.
		i := strings.LastIndex(string(data), "PK\x01\x02")
		data[i+16] ^= 0xff
		got, err := Check(context.Background(), data)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Refused || !hasRule(got.Findings, "zip") {
			t.Errorf("refused %v, %v", got.Refused, got.Findings)
		}
	})
}

func TestCheckAccepts(t *testing.T) {
	for _, c := range []struct {
		name, rule string
		breakIt    func(t *testing.T, pkg fstest.MapFS)
	}{
		{"contrast", "contrast", func(t *testing.T, pkg fstest.MapFS) { setToken(t, pkg, "ink", "#fffffe") }},
		{"tokens", "tokens", func(t *testing.T, pkg fstest.MapFS) {
			rewrite(t, pkg, cssFile, `--tone-epic:[^;]*;`, "")
		}},
		{"font-size", "font-size", func(t *testing.T, pkg fstest.MapFS) { appendCSS(pkg, "a { font-size: 12px; }") }},
		{"preview", "preview", func(t *testing.T, pkg fstest.MapFS) {
			rewrite(t, pkg, manifestFile, `"ink":\s*"#[0-9a-fA-F]{6}"`, `"ink": "#010203"`)
		}},
		{"schemes", "schemes", func(t *testing.T, pkg fstest.MapFS) {
			rewrite(t, pkg, cssFile, `\[data-scheme=['"]dark['"]\]`, "[data-scheme='night']")
		}},
		{"licence", "licence", func(t *testing.T, pkg fstest.MapFS) {
			for name := range pkg {
				if strings.HasSuffix(name, ".txt") {
					delete(pkg, name)
				}
			}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			pkg := carnetPackage(t)
			c.breakIt(t, pkg)
			got, err := Check(context.Background(), zipOfPackage(t, pkg))
			if err != nil {
				t.Fatal(err)
			}
			if got.Refused || !hasRule(got.Findings, c.rule) || got.CSS == "" || got.Manifest.ID != "carnet" {
				t.Errorf("refused %v, css %d bytes, findings %v", got.Refused, len(got.CSS), got.Findings)
			}
			// The split belongs to the check: an import refuses all of it.
			if findings := Validate(pkg); !hasRule(findings, c.rule) {
				t.Errorf("Validate: %v", findings)
			}
		})
	}
}

func TestInlineFiles(t *testing.T) {
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	pkg := fstest.MapFS{
		"assets/a.svg":  {Data: []byte("svg")},
		"assets/a.png":  {Data: []byte("png")},
		"assets/a.webp": {Data: []byte("webp")},
		"assets/a.jpg":  {Data: []byte("jpg")},
		"assets/B.PNG":  {Data: []byte("upper")},
		"assets/a.txt":  {Data: []byte("txt")},
		"fonts/a.woff2": {Data: []byte("font")},
	}
	svg := "data:image/svg+xml;base64," + b64("svg")
	png := "data:image/png;base64," + b64("png")
	for _, c := range []struct{ src, want string }{
		{`a { b: url(assets/a.svg); }`, `a { b: url(` + svg + `); }`},
		{`a { b: url("assets/a.png"); }`, `a { b: url("` + png + `"); }`},
		{`a { b: url( 'assets/a.png' ); }`, `a { b: url( '` + png + `' ); }`},
		{`a { b: url(  assets/a.png  ); }`, `a { b: url(  ` + png + `  ); }`},
		{`a { b: URL(assets/a.png); }`, `a { b: URL(` + png + `); }`},
		{`a { b: url(assets/a.webp); }`, `a { b: url(data:image/webp;base64,` + b64("webp") + `); }`},
		{`a { b: url(assets/a.jpg); }`, `a { b: url(data:image/jpeg;base64,` + b64("jpg") + `); }`},
		{`a { b: url(assets/B.PNG); }`, `a { b: url(data:image/png;base64,` + b64("upper") + `); }`},
		{`a { b: url(assets/a.txt); }`, `a { b: url(data:text/plain;base64,` + b64("txt") + `); }`},
		{`@font-face { src: url(fonts/a.woff2) format("woff2"); }`,
			`@font-face { src: url(data:font/woff2;base64,` + b64("font") + `) format("woff2"); }`},
		{`a { b: url(assets/a.svg#icon); }`, `a { b: url(` + svg + `#icon); }`},
		{`a { b: image-set("assets/a.png" 1x, url(assets/a.svg) 2x); }`, `a { b: image-set("` + png + `" 1x, url(` + svg + `) 2x); }`},
		{`a { b: -webkit-image-set("assets/a.png" type("image/png")); }`, `a { b: -webkit-image-set("` + png + `" type("image/png")); }`},
		{"a { b: url(assets/a.png), url(assets/a.png); }\nc { d: url(assets/a.svg); }",
			"a { b: url(" + png + "), url(" + png + "); }\nc { d: url(" + svg + "); }"},
		{`a { b: url(data:image/png;base64,AAAA); c: url(DATA:x); }`, `a { b: url(data:image/png;base64,AAAA); c: url(DATA:x); }`},
		{`/* url(assets/a.png) */ a { content: "assets/a.png"; }`, `/* url(assets/a.png) */ a { content: "assets/a.png"; }`},
	} {
		got, fits, err := inlineFiles(pkg, c.src)
		if err != nil || !fits {
			t.Errorf("%s: fits %v, %v", c.src, fits, err)
		}
		if got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.src, got, c.want)
		}
	}
}

func TestInlineFilesCap(t *testing.T) {
	pkg := fstest.MapFS{"assets/a.png": {Data: make([]byte, 2<<20)}}
	ref := "url(assets/a.png)"
	encoded, err := dataURL(pkg, "assets/a.png")
	if err != nil {
		t.Fatal(err)
	}
	two := strings.Repeat(ref, 2)
	size := len(two) + 2*(len(encoded)-len("assets/a.png"))
	exact := two + strings.Repeat(" ", MaxInlinedCSSBytes-size)
	css, fits, err := inlineFiles(pkg, exact)
	if err != nil || !fits || len(css) != MaxInlinedCSSBytes {
		t.Errorf("at the cap: fits %v, %d bytes, %v", fits, len(css), err)
	}
	if _, fits, err := inlineFiles(pkg, exact+" "); err != nil || fits {
		t.Errorf("a byte over: fits %v, %v", fits, err)
	}
	// Each name is small, what they inline is not: the source alone says nothing.
	if _, fits, err := inlineFiles(pkg, strings.Repeat(ref, 1<<16)); err != nil || fits {
		t.Errorf("one file named many times: fits %v, %v", fits, err)
	}
}

func TestCheckCSSOverTheCap(t *testing.T) {
	pkg := carnetPackage(t)
	// Noise, so that the file stays under the zip's ratio limit.
	noise := make([]byte, 900_000)
	for i := range noise {
		noise[i] = byte(i * 2654435761 >> 13)
	}
	pkg["assets/noise.png"] = &fstest.MapFile{Data: noise}
	appendCSS(pkg, "a { background: "+strings.Repeat("url(assets/noise.png), ", 10)+"none; font-size: 12px; }")
	got, err := Check(context.Background(), zipOfPackage(t, pkg))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Refused || !hasRule(got.Findings, "size") || !hasRule(got.Findings, "font-size") || got.CSS != "" {
		t.Errorf("refused %v, css %d bytes, findings %v", got.Refused, len(got.CSS), got.Findings)
	}
}

func TestCheckWaitsForASlot(t *testing.T) {
	for range cap(checkSlots) {
		checkSlots <- struct{}{}
	}
	defer func() {
		for range cap(checkSlots) {
			<-checkSlots
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Check(ctx, nil); err != context.Canceled {
		t.Errorf("err = %v", err)
	}
}

// A package is at most MaxPackageBytes of stylesheet, sent by anyone signed in:
// each of these took minutes to hours while a rule was quadratic.
func TestValidateHostileStylesheets(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	repeat := func(s string) string { return strings.Repeat(s, MaxPackageBytes/len(s)) }
	block := func(body string) string { return "[data-skin='carnet'] {\n" + body + "}\n" }
	var chain strings.Builder
	for i := range 100_000 {
		fmt.Fprintf(&chain, "--t%d: var(--t%d);\n", i, i+1)
	}
	for _, c := range []struct{ name, css string }{
		{"nested parentheses", repeat("(")},
		{"parentheses in a loader", "a { b: image-set(" + repeat("(")},
		{"strings in parentheses", repeat("('a'")},
		{"strings in a loader", "a { b: image-set(" + repeat("'a' ")},
		{"imports", repeat("@import;\n")},
		{"escapes", repeat("\\a\n")},
		{"font sizes", repeat("font-size:1px;")},
		{"urls", repeat("url(x)\n")},
		{"long var() chain, compared often", block(chain.String()) + repeat("/* contrast: --t0 on --t1 */\n")[:MaxPackageBytes/2]},
		{"many colours against many", block("--a: "+repeat("#123 ")[:MaxPackageBytes/3]+";\n--b: "+repeat("#456 ")[:MaxPackageBytes/3]+";\n") + "/* contrast: --a on --b */\n"},
		{"long token name, quoted often", block("--"+strings.Repeat("a", 1<<20)+": red;\n") + repeat("/* contrast: --"+strings.Repeat("a", 1<<20)+" on --ink */\n")},
	} {
		t.Run(c.name, func(t *testing.T) {
			pkg := carnetPackage(t)
			pkg[cssFile] = &fstest.MapFile{Data: []byte(c.css)}
			start := time.Now()
			findings := Validate(pkg)
			took := time.Since(start)
			size := 0
			for _, f := range findings {
				size += len(f.Message)
			}
			t.Logf("%v, %d findings, %d bytes of messages", took.Round(time.Millisecond), len(findings), size)
			if took > time.Minute {
				t.Errorf("took %v", took)
			}
			if size > 1<<20 {
				t.Errorf("%d bytes of messages", size)
			}
		})
	}
}

func TestValidateBounds(t *testing.T) {
	rules := func(css string) map[string]int {
		pkg := carnetPackage(t)
		appendCSS(pkg, css)
		got := map[string]int{}
		for _, f := range Validate(pkg) {
			got[f.Rule]++
		}
		return got
	}
	for _, c := range []struct {
		name, css, rule string
		want            int
	}{
		{"imports", strings.Repeat("@import;\n", maxRuleFindings+5), "import", maxRuleFindings},
		{"escapes", strings.Repeat("\\a\n", maxRuleFindings+5), "escape", maxRuleFindings},
		{"loaders", "a { b: image-set(" + strings.Repeat("var(--x)\n", maxRuleFindings+5) + "); }", "load", maxRuleFindings},
		{"urls", strings.Repeat("url(x)\n", maxRuleFindings+5), "url", maxRuleFindings},
		{"font sizes", strings.Repeat("font-size:1px;\n", maxRuleFindings+5), "font-size", maxRuleFindings},
	} {
		if got := rules(c.css)[c.rule]; got != c.want {
			t.Errorf("%s: %d findings, want %d", c.name, got, c.want)
		}
	}

	for urls, want := range map[int]bool{maxURLs: false, maxURLs + 1: true} {
		pkg := carnetPackage(t)
		pkg[cssFile] = &fstest.MapFile{Data: []byte(strings.Repeat("url(data:x)\n", urls))}
		if got := hasRule(Validate(pkg), "url"); got != want {
			t.Errorf("%d urls: a finding = %v", urls, got)
		}
	}

	contrast := func(t *testing.T, css, want string, found bool) {
		t.Helper()
		pkg := carnetPackage(t)
		appendCSS(pkg, css)
		text := fmt.Sprint(Validate(pkg))
		if strings.Contains(text, want) != found {
			t.Errorf("%q in findings = %v: %.300s", want, !found, text)
		}
	}
	contrast(t, strings.Repeat("/* contrast: --ink on --paper */\n", maxContrastPairs+1), "contrast comments", true)
	contrast(t, "", "contrast comments", false)

	chainOf := func(n int) tokens {
		out := tokens{"end": {value: "#000000"}}
		last := "end"
		for i := range n {
			name := fmt.Sprintf("t%d", i)
			out[name] = token{value: "var(--" + last + ")"}
			last = name
		}
		out["start"] = out[last]
		return out
	}
	if _, err := chainOf(maxTokenChain-1).colours("start", nil); err != nil {
		t.Errorf("a chain within the limit: %v", err)
	}
	if _, err := chainOf(maxTokenChain+1).colours("start", nil); err == nil {
		t.Error("a chain over the limit")
	}
	many := func(n int) tokens { return tokens{"a": {value: strings.Repeat("#123456 ", n)}} }
	if found, err := many(maxTokenColours).colours("a", nil); err != nil || len(found) != maxTokenColours {
		t.Errorf("as many colours as allowed: %d, %v", len(found), err)
	}
	if _, err := many(maxTokenColours+1).colours("a", nil); err == nil {
		t.Error("too many colours")
	}
	if _, err := (tokens{"a": {value: strings.Repeat(" ", maxColourValue) + "#123456"}}).colours("a", nil); err == nil {
		t.Error("too long a value")
	}

	long := tidy([]Finding{{Rule: "url", Message: strings.Repeat("x", maxMessageLen+1)}})
	if len(long[0].Message) > maxMessageLen+len("…") {
		t.Errorf("message of %d bytes", len(long[0].Message))
	}
}
