package themes

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// MaxInlinedCSSBytes caps the stylesheet Check answers: a browser parses all of
// it on every page load.
const MaxInlinedCSSBytes = 8 << 20

// advisoryRules are the findings a theme kept in its user's browser may have.
// They judge how it reads, and it misleads nobody else. Any other rule refuses
// it, including one added later.
var advisoryRules = map[string]bool{
	"contrast":  true,
	"tokens":    true,
	"font-size": true,
	"preview":   true,
	"schemes":   true,
	"licence":   true,
}

// checkSlots bounds the checks running at once: at the worst each takes a few
// hundred MB and a second of CPU, and anyone signed in can ask for one.
var checkSlots = make(chan struct{}, 2)

// Checked is what Check says about a package. A refused one has only Findings.
type Checked struct {
	Manifest Manifest  `json:"manifest"`
	Findings []Finding `json:"findings"`
	CSS      string    `json:"css"`
	Refused  bool      `json:"-"`
}

func refusal(findings []Finding) Checked {
	return Checked{Findings: findings, Refused: true}
}

// Check validates a package from its zip and inlines its files into its
// stylesheet. It keeps nothing: the zip is read in memory, as hostile.
func Check(ctx context.Context, data []byte) (Checked, error) {
	select {
	case checkSlots <- struct{}{}:
		defer func() { <-checkSlots }()
	case <-ctx.Done():
		return Checked{}, ctx.Err()
	}
	zr, findings := openZip(data)
	if len(findings) > 0 {
		return refusal(findings), nil
	}
	// Inflating them all once holds the zip to its limits, whatever Validate reads.
	findings, err := eachFile(zr, func(string, []byte) error { return nil })
	if err != nil {
		return Checked{}, err
	}
	if len(findings) > 0 {
		return refusal(findings), nil
	}
	findings = Validate(zr)
	for _, f := range findings {
		if !advisoryRules[f.Rule] {
			return refusal(findings), nil
		}
	}
	raw, err := fs.ReadFile(zr, manifestFile)
	if err != nil {
		return Checked{}, err
	}
	m, err := ParseManifest(raw)
	if err != nil {
		return Checked{}, err
	}
	src, err := fs.ReadFile(zr, cssFile)
	if err != nil {
		return Checked{}, err
	}
	css, fits, err := inlineFiles(zr, string(src))
	if err != nil {
		return Checked{}, err
	}
	if !fits {
		msg := fmt.Sprintf("over %d bytes with its files inlined: the most a stylesheet may be", MaxInlinedCSSBytes)
		return refusal(tidy(append(findings, Finding{Rule: "size", Message: msg, File: cssFile}))), nil
	}
	if findings == nil {
		findings = []Finding{}
	}
	return Checked{Manifest: m, Findings: findings, CSS: css}, nil
}

// inlineFiles replaces each file a stylesheet loads with a data: URL. Validate
// has already held the targets to files of the package. A fragment stays. fits
// is false over MaxInlinedCSSBytes, which it finds before building anything:
// a file can be named many times over.
func inlineFiles(pkg fs.FS, src string) (css string, fits bool, err error) {
	urls := parseCSS(src).urls
	inlined := map[string]string{}
	size := len(src)
	for _, u := range urls {
		if isDataURL(u.target) {
			continue
		}
		name, _, _ := strings.Cut(u.target, "#")
		if _, ok := inlined[name]; !ok {
			if inlined[name], err = dataURL(pkg, name); err != nil {
				return "", false, err
			}
		}
		if size += len(inlined[name]) - len(name); size > MaxInlinedCSSBytes {
			return "", false, nil
		}
	}
	var out strings.Builder
	out.Grow(size)
	last := 0
	for _, u := range urls {
		if isDataURL(u.target) {
			continue
		}
		name, _, _ := strings.Cut(u.target, "#")
		out.WriteString(src[last:u.at])
		out.WriteString(inlined[name])
		last = u.at + len(name)
	}
	out.WriteString(src[last:])
	return out.String(), true, nil
}

func dataURL(pkg fs.FS, name string) (string, error) {
	data, err := fs.ReadFile(pkg, name)
	if err != nil {
		return "", err
	}
	// Without parameters: a space would end an unquoted url().
	mediaType, _, _ := strings.Cut(fileTypes[strings.ToLower(path.Ext(name))], ";")
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
