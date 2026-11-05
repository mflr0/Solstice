package themes

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

const (
	manifestFile = "theme.json"
	cssFile      = "theme.css"
)

// Finding is one rule a theme package breaks.
type Finding struct {
	Rule    string `json:"rule"` // manifest, css, tokens, contrast, font-size, import, escape, url, load, schemes, preview, licence, size or zip
	Message string `json:"message"`
	File    string `json:"file"`           // path inside the package
	Line    int    `json:"line,omitempty"` // from 1; 0 when it concerns the whole file
}

func (f Finding) String() string {
	if f.Line == 0 {
		return fmt.Sprintf("%s: %s: %s", f.File, f.Rule, f.Message)
	}
	return fmt.Sprintf("%s:%d: %s: %s", f.File, f.Line, f.Rule, f.Message)
}

// Validate checks a theme package against the rules of ANATOMY.md and returns
// what it breaks, by file and line. Without a valid manifest it skips the
// rules that need the package's id or schemes.
func Validate(pkg fs.FS) []Finding {
	out := append(checkLicences(pkg), checkSize(pkg)...)
	raw, err := fs.ReadFile(pkg, manifestFile)
	var m Manifest
	if err == nil {
		m, err = ParseManifest(raw)
	}
	manifestOK := err == nil
	if !manifestOK {
		out = append(out, Finding{Rule: "manifest", Message: err.Error(), File: manifestFile})
	}
	data, err := fs.ReadFile(pkg, cssFile)
	if err != nil {
		out = append(out, Finding{Rule: "css", Message: err.Error(), File: cssFile})
		return sorted(out)
	}
	css := parseCSS(string(data))
	out = append(out, checkImports(css)...)
	out = append(out, checkEscapes(css)...)
	out = append(out, checkLoaders(css)...)
	out = append(out, checkFontSizes(css)...)
	out = append(out, checkURLs(pkg, css)...)
	if manifestOK {
		out = append(out, checkTokens(css, m, string(raw))...)
	}
	return sorted(out)
}

// checkSize holds a package to MaxPackageFiles and MaxPackageBytes.
func checkSize(pkg fs.FS) []Finding {
	var files, size int64
	err := fs.WalkDir(pkg, ".", func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files++
		size += info.Size()
		return nil
	})
	var out []Finding
	if err != nil {
		out = append(out, Finding{Rule: "size", Message: err.Error(), File: "."})
	}
	if files > MaxPackageFiles {
		out = append(out, Finding{Rule: "size", Message: fmt.Sprintf("%d files: the most a package may hold is %d", files, MaxPackageFiles), File: "."})
	}
	if size > MaxPackageBytes {
		out = append(out, Finding{Rule: "size", Message: fmt.Sprintf("%d bytes: the most a package may hold is %d", size, MaxPackageBytes), File: "."})
	}
	return out
}

func sorted(out []Finding) []Finding {
	slices.SortFunc(out, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Rule, b.Rule), cmp.Compare(a.Message, b.Message))
	})
	return slices.Compact(out)
}

// stylesheet is a CSS source in several forms of the same length, so an offset
// in one is the same place in the others.
type stylesheet struct {
	raw   string
	code  string // comments blanked
	plain string // comments, strings and url() targets blanked
	urls  []urlRef

	escapes  []int    // offsets of the backslashes outside strings and comments
	indirect []urlRef // calls inside a function that loads strings, which may hide a target
}

// urlRef is a file a stylesheet loads, through call.
type urlRef struct {
	call   string
	target string
	offset int
}

func (s stylesheet) line(offset int) int {
	return strings.Count(s.raw[:offset], "\n") + 1
}

func parseCSS(src string) stylesheet {
	code, plain := []byte(src), []byte(src)
	blank := func(b []byte, from, to int) {
		for i := from; i < to; i++ {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
	}
	var urls, indirect []urlRef
	var escapes []int
	var calls []string // the functions open at i, innermost last
	for i := 0; i < len(src); {
		switch {
		case strings.HasPrefix(src[i:], "/*"):
			end := len(src)
			if n := strings.Index(src[i+2:], "*/"); n >= 0 {
				end = i + 2 + n + 2
			}
			blank(code, i, end)
			blank(plain, i, end)
			i = end
		case src[i] == '\\':
			// An escaped quote opens no string, nor an escaped "/*" a comment.
			escapes = append(escapes, i)
			i += 2
		case src[i] == '"' || src[i] == '\'':
			target, end := stringAt(src, i)
			if call := loaderOf(calls); call != "" {
				urls = append(urls, urlRef{call, target, i})
			}
			blank(plain, i, end)
			i = end
		case isURLCall(src, i):
			calls = append(calls, "url")
			start := i + len("url(")
			for start < len(src) && isCSSSpace(src[start]) {
				start++
			}
			if start < len(src) && (src[start] == '"' || src[start] == '\'') {
				target, end := stringAt(src, start)
				urls = append(urls, urlRef{"url", target, i})
				blank(plain, start, end)
				i = end
				break
			}
			// Unquoted, the target runs to the ")": a "/*" in it opens no comment.
			end := len(src)
			if n := strings.IndexByte(src[start:], ')'); n >= 0 {
				end = start + n
			}
			if n := strings.IndexByte(src[start:end], '\\'); n >= 0 {
				escapes = append(escapes, start+n)
			}
			urls = append(urls, urlRef{"url", strings.TrimSpace(src[start:end]), i})
			blank(plain, start, end)
			i = end
		case src[i] == '(':
			name := callName(src, i)
			if slices.ContainsFunc(calls, isLoader) && !isLoader(name) && name != "type" {
				indirect = append(indirect, urlRef{loaderOf(calls), name, i})
			}
			calls = append(calls, name)
			i++
		case src[i] == ')':
			if len(calls) > 0 {
				calls = calls[:len(calls)-1]
			}
			i++
		default:
			i++
		}
	}
	return stylesheet{raw: src, code: string(code), plain: string(plain), urls: urls, escapes: escapes, indirect: indirect}
}

// stringAt reads the CSS string opening at i: its text and the offset after
// it. A string stops at an unescaped newline, as it does in a browser.
func stringAt(src string, i int) (string, int) {
	quote := src[i]
	for j := i + 1; j < len(src); j++ {
		switch src[j] {
		case '\\':
			j++
		case quote:
			return src[i+1 : j], j + 1
		case '\n':
			return src[i+1 : j], j
		}
	}
	return src[i+1:], len(src)
}

// callName is the function a "(" at i opens, lowercase and without a vendor
// prefix; "" for a bare parenthesis.
func callName(src string, i int) string {
	start := i
	for start > 0 && isIdentByte(src[start-1]) {
		start--
	}
	name := strings.ToLower(src[start:i])
	if strings.HasPrefix(name, "-") && !strings.HasPrefix(name, "--") {
		if _, rest, ok := strings.Cut(name[1:], "-"); ok {
			return rest
		}
	}
	return name
}

// isLoader reports whether a function takes a bare string as a file to load.
func isLoader(name string) bool {
	return name == "image-set" || name == "image" || name == "src"
}

// loaderOf is the function that would load a string in these open calls: the
// outermost loader, unless the string belongs to type(), which names a format.
func loaderOf(calls []string) string {
	if len(calls) == 0 || calls[len(calls)-1] == "type" {
		return ""
	}
	if i := slices.IndexFunc(calls, isLoader); i >= 0 {
		return calls[i]
	}
	return ""
}

func isURLCall(src string, i int) bool {
	if len(src)-i < len("url(") || !strings.EqualFold(src[i:i+len("url(")], "url(") {
		return false
	}
	return i == 0 || !isIdentByte(src[i-1])
}

func isIdentByte(c byte) bool {
	return c == '-' || c == '_' || c >= 0x80 ||
		'0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

func isCSSSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f'
}

var importPattern = regexp.MustCompile(`(?i)@import\b`)

func checkImports(css stylesheet) []Finding {
	var out []Finding
	for _, m := range importPattern.FindAllStringIndex(css.plain, -1) {
		out = append(out, Finding{Rule: "import", Message: "@import is not allowed", File: cssFile, Line: css.line(m[0])})
	}
	return out
}

// checkEscapes refuses every CSS escape outside strings: "@\\69mport" and
// "u\\72l(" are @import and url() to a browser, but not to this scanner.
func checkEscapes(css stylesheet) []Finding {
	var out []Finding
	for _, offset := range css.escapes {
		out = append(out, Finding{Rule: "escape", Message: "CSS escapes are only allowed in strings", File: cssFile, Line: css.line(offset)})
	}
	return out
}

// checkLoaders refuses a call inside a function that loads strings: var() or
// attr() would hand it a target this scanner never sees. checkURLs reads the
// strings themselves.
func checkLoaders(css stylesheet) []Finding {
	var out []Finding
	for _, c := range css.indirect {
		msg := fmt.Sprintf("%s() inside %s(): want strings or url() there", c.target, c.call)
		out = append(out, Finding{Rule: "load", Message: msg, File: cssFile, Line: css.line(c.offset)})
	}
	return out
}

var (
	fontSizePattern = regexp.MustCompile(`(?i)font-size\s*:\s*([^;}]*)`)
	pxPattern       = regexp.MustCompile(`(?i)(\d*\.?\d+)px`)
)

func checkFontSizes(css stylesheet) []Finding {
	var out []Finding
	for _, m := range fontSizePattern.FindAllStringSubmatchIndex(css.plain, -1) {
		value := strings.TrimSpace(css.plain[m[2]:m[3]])
		for _, px := range pxPattern.FindAllStringSubmatch(value, -1) {
			if n, err := strconv.ParseFloat(px[1], 64); err == nil && n > 0 {
				out = append(out, Finding{Rule: "font-size", Message: "font size in px: " + value + ", use rem", File: cssFile, Line: css.line(m[0])})
				break
			}
		}
	}
	return out
}

func checkURLs(pkg fs.FS, css stylesheet) []Finding {
	var out []Finding
	for _, u := range css.urls {
		if len(u.target) >= 5 && strings.EqualFold(u.target[:5], "data:") {
			continue
		}
		msg := ""
		name, _, _ := strings.Cut(u.target, "#")
		if !strings.HasPrefix(name, "assets/") && !strings.HasPrefix(name, "fonts/") || strings.Contains(name, "..") {
			msg = u.call + "(" + u.target + "): want a file under assets/ or fonts/, or data:"
		} else if info, err := fs.Stat(pkg, name); err != nil || info.IsDir() {
			msg = u.call + "(" + u.target + "): no such file"
		}
		if msg != "" {
			out = append(out, Finding{Rule: "url", Message: msg, File: cssFile, Line: css.line(u.offset)})
		}
	}
	return out
}

// checkLicences wants, next to each font file, a licence: a .txt named
// <family>-<licence>, where the font's name starts with <family>-, or a .txt
// with no hyphen in its name, which covers the whole folder.
func checkLicences(pkg fs.FS) []Finding {
	var fonts []string
	licences := map[string][]string{} // folder → .txt names without extension
	err := fs.WalkDir(pkg, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		base := path.Base(name)
		switch strings.ToLower(path.Ext(name)) {
		case ".woff2", ".woff", ".ttf", ".otf":
			fonts = append(fonts, name)
		case ".txt":
			licences[path.Dir(name)] = append(licences[path.Dir(name)], strings.TrimSuffix(base, path.Ext(base)))
		}
		return nil
	})
	if err != nil {
		return []Finding{{Rule: "licence", Message: err.Error(), File: "."}}
	}
	var out []Finding
	for _, font := range fonts {
		covered := slices.ContainsFunc(licences[path.Dir(font)], func(licence string) bool {
			hyphen := strings.LastIndexByte(licence, '-')
			return hyphen < 0 || strings.HasPrefix(path.Base(font), licence[:hyphen+1])
		})
		if !covered {
			out = append(out, Finding{Rule: "licence", Message: "no licence file next to this font", File: font})
		}
	}
	return out
}

// token is a custom property: its value and the line declaring it.
type token struct {
	value string
	line  int
}

type tokens map[string]token // name, without the leading --

var declarationPattern = regexp.MustCompile(`^\s*--([\w-]+)\s*:((?s:.*))$`)

// tokenBlock finds the block of the selector, a skin's first or dark block:
// the offset of its "{" and the custom properties declared directly in it, not
// in nested rules. ok is false when no such block exists.
func tokenBlock(css stylesheet, id string, dark bool) (block tokens, offset int, ok bool, err error) {
	selector := `\[data-skin=['"]` + regexp.QuoteMeta(id) + `['"]\]`
	if dark {
		selector += `\[data-scheme=['"]dark['"]\]`
	}
	m := regexp.MustCompile(`(?m)(?:^|,)\s*` + selector + `\s*\{`).FindStringIndex(css.code)
	if m == nil {
		return nil, 0, false, nil
	}
	open := m[1] - 1
	block = tokens{}
	flush := func(start, end int) {
		d := declarationPattern.FindStringSubmatchIndex(css.code[start:end])
		if d != nil {
			block[css.code[start+d[2]:start+d[3]]] = token{
				value: strings.TrimSpace(css.code[start+d[4] : start+d[5]]),
				line:  css.line(start + d[2]),
			}
		}
	}
	depth, start := 0, open+1
	for i := open + 1; i < len(css.plain); i++ {
		switch css.plain[i] {
		case '(', '{':
			depth++
		case ')':
			depth--
		case '}':
			if depth == 0 {
				flush(start, i)
				return block, open, true, nil
			}
			if depth--; depth == 0 {
				start = i + 1
			}
		case ';':
			if depth == 0 {
				flush(start, i)
				start = i + 1
			}
		}
	}
	return nil, open, true, errors.New("unclosed block")
}

// carnetTokens is the tokens every package must declare in its first block.
var carnetTokens = sync.OnceValue(func() tokens {
	data, err := fs.ReadFile(builtin, "builtin/carnet/"+cssFile)
	if err != nil {
		panic(err)
	}
	block, _, ok, err := tokenBlock(parseCSS(string(data)), "carnet", false)
	if err != nil || !ok {
		panic(fmt.Errorf("carnet: no token block: %v", err))
	}
	return block
})

// checkTokens runs the rules that read the token blocks.
func checkTokens(css stylesheet, m Manifest, manifest string) []Finding {
	base, baseAt, ok, err := tokenBlock(css, m.ID, false)
	if err != nil {
		return []Finding{{Rule: "tokens", Message: err.Error(), File: cssFile, Line: css.line(baseAt)}}
	}
	if !ok {
		return []Finding{{Rule: "tokens", Message: fmt.Sprintf("no [data-skin='%s'] block", m.ID), File: cssFile}}
	}
	dark, darkAt, hasDark, err := tokenBlock(css, m.ID, true)
	if err != nil {
		return []Finding{{Rule: "tokens", Message: err.Error(), File: cssFile, Line: css.line(darkAt)}}
	}

	var out []Finding
	for name := range carnetTokens() {
		if _, ok := base[name]; !ok {
			out = append(out, Finding{Rule: "tokens", Message: "--" + name + " is not declared", File: cssFile, Line: css.line(baseAt)})
		}
	}
	if slices.Contains(m.Schemes, "dark") != hasDark {
		msg := "schemes lists dark, but theme.css has no dark block"
		if hasDark {
			msg = "theme.css has a dark block, but schemes does not list dark"
		}
		out = append(out, Finding{Rule: "schemes", Message: msg, File: manifestFile, Line: jsonLine(manifest, "schemes")})
	}

	out = append(out, checkContrast(css, base, "light")...)
	out = append(out, checkPreview(base, m.Preview["light"], "light", manifest)...)
	if hasDark {
		merged := maps.Clone(base)
		maps.Copy(merged, dark)
		out = append(out, checkContrast(css, merged, "dark")...)
		if p, ok := m.Preview["dark"]; ok {
			out = append(out, checkPreview(merged, p, "dark", manifest)...)
		}
	}
	return out
}

// jsonLine is the line of a key's first occurrence in a manifest.
func jsonLine(manifest, key string) int {
	i := strings.Index(manifest, `"`+key+`"`)
	if i < 0 {
		return 0
	}
	return strings.Count(manifest[:i], "\n") + 1
}

func checkPreview(t tokens, p Preview, scheme, manifest string) []Finding {
	var out []Finding
	for _, c := range []struct{ name, value string }{{"bar", p.Bar}, {"paper", p.Paper}, {"ink", p.Ink}, {"accent", p.Accent}} {
		want, err := t.colours(c.name, nil)
		if err != nil {
			continue // checkContrast reports it, or checkTokens the token's absence.
		}
		got := parseColours(c.value)
		// A gradient's preview is one of its stops.
		if len(got) == 1 && slices.Contains(want, got[0]) {
			continue
		}
		out = append(out, Finding{
			Rule:    "preview",
			Message: fmt.Sprintf("preview.%s.%s is %s, --%s is %s", scheme, c.name, c.value, c.name, want),
			File:    manifestFile,
			Line:    jsonLine(manifest, "preview"),
		})
	}
	return out
}

type rgb [3]int

func (c rgb) String() string { return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2]) }

var (
	hexPattern = regexp.MustCompile(`(?i)#([0-9a-f]{3}|[0-9a-f]{6})\b`)
	rgbPattern = regexp.MustCompile(`(?i)rgba?\(\s*(\d+)[\s,]+(\d+)[\s,]+(\d+)`)
	varPattern = regexp.MustCompile(`^var\(--([\w-]+)\)$`)
)

// parseColours reads every colour of a value: one, or a gradient's stops.
func parseColours(value string) []rgb {
	var out []rgb
	for _, m := range hexPattern.FindAllStringSubmatch(value, -1) {
		h := m[1]
		if len(h) == 3 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		n, _ := strconv.ParseUint(h, 16, 32)
		out = append(out, rgb{int(n >> 16), int(n >> 8 & 0xff), int(n & 0xff)})
	}
	for _, m := range rgbPattern.FindAllStringSubmatch(value, -1) {
		var c rgb
		for i := range c {
			c[i], _ = strconv.Atoi(m[i+1])
		}
		out = append(out, c)
	}
	return out
}

func luminance(c rgb) float64 {
	lin := func(v int) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c[0]) + 0.7152*lin(c[1]) + 0.0722*lin(c[2])
}

func contrastRatio(a, b rgb) float64 {
	hi, lo := luminance(a), luminance(b)
	if hi < lo {
		hi, lo = lo, hi
	}
	return (hi + 0.05) / (lo + 0.05)
}

// colours resolves a token, through var() references, to its colours.
func (t tokens) colours(name string, seen []string) ([]rgb, error) {
	tok, ok := t[name]
	if !ok {
		return nil, fmt.Errorf("--%s is not defined", name)
	}
	if ref := varPattern.FindStringSubmatch(tok.value); ref != nil {
		if slices.Contains(seen, ref[1]) {
			return nil, fmt.Errorf("--%s refers to itself", name)
		}
		return t.colours(ref[1], append(seen, name))
	}
	found := parseColours(tok.value)
	if len(found) == 0 {
		return nil, fmt.Errorf("--%s = %s is not a colour", name, tok.value)
	}
	return found, nil
}

// colour resolves a token that must be one colour, not a gradient.
func (t tokens) colour(name string) (rgb, error) {
	found, err := t.colours(name, nil)
	if err != nil {
		return rgb{}, err
	}
	if len(found) > 1 {
		return rgb{}, fmt.Errorf("--%s must be a single colour", name)
	}
	return found[0], nil
}

var contrastPair = regexp.MustCompile(`/\*\s*contrast:\s*--([\w-]+)\s+on\s+--([\w-]+)(?:\s+([\d.]+))?\s*\*/`)

const (
	textContrast    = 4.5
	nonTextContrast = 3 // focus rings and frames
)

// checkContrast checks the WCAG AA pairs the interface draws, plus those the
// stylesheet declares, in one scheme.
func checkContrast(css stylesheet, t tokens, scheme string) []Finding {
	var out []Finding
	report := func(line int, format string, args ...any) {
		out = append(out, Finding{Rule: "contrast", Message: scheme + ": " + fmt.Sprintf(format, args...), File: cssFile, Line: line})
	}
	// check tests fg on bg. A line of 0 means fg's own.
	check := func(fg, bg string, minimum float64, line int) {
		if line == 0 {
			line = t[fg].line
		}
		bgColours, err := t.colours(bg, nil)
		if err != nil {
			report(line, "%v", err)
			return
		}
		fgColours, err := t.colours(fg, nil)
		if err != nil {
			report(line, "%v", err)
			return
		}
		for _, f := range fgColours {
			for _, b := range bgColours {
				if c := contrastRatio(f, b); c < minimum {
					report(line, "--%s on --%s: %.2f, want %g", fg, bg, c, minimum)
				}
			}
		}
	}

	for _, bg := range []string{"paper", "paper-raised", "paper-sunk"} {
		for _, fg := range []string{"ink", "ink-soft", "muted"} {
			check(fg, bg, textContrast, 0)
		}
	}
	stampInks := []string{"stamp-todo", "stamp-doing", "stamp-done"}
	for _, fg := range stampInks {
		if _, err := t.colour(fg); err != nil {
			report(t[fg].line, "%v", err)
		}
	}
	for _, bg := range []string{"paper", "paper-raised"} {
		check("accent-text", bg, textContrast, 0)
		for _, fg := range stampInks {
			check(fg, bg, textContrast, 0)
		}
	}
	for _, fg := range []string{"tone-bug", "tone-story", "tone-task", "tone-subtask"} {
		check(fg, "paper-raised", textContrast, 0)
	}
	check("paper", "tone-epic", textContrast, 0)
	check("primary-ink", "primary", textContrast, 0)
	check("bar-ink", "bar", textContrast, 0)
	check("bar-ink-soft", "bar", textContrast, 0)
	check("focus", "paper", nonTextContrast, 0)
	check("accent", "paper", nonTextContrast, 0)

	for _, m := range contrastPair.FindAllStringSubmatchIndex(css.raw, -1) {
		line := css.line(m[0])
		minimum := textContrast
		if m[6] >= 0 {
			n, err := strconv.ParseFloat(css.raw[m[6]:m[7]], 64)
			if err != nil {
				report(line, "minimum %s is not a number", css.raw[m[6]:m[7]])
				continue
			}
			minimum = n
		}
		check(css.raw[m[2]:m[3]], css.raw[m[4]:m[5]], minimum, line)
	}
	return out
}
