package themes

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"
)

// StarterName is the file name of the starter package's zip.
const StarterName = "theme-starter.zip"

const (
	starterID      = "my-theme"
	starterFont    = "fonts/ibm-plex-mono-latin-400-normal.woff2"
	starterLicence = "fonts/ibm-plex-mono-OFL.txt"
	starterChevron = "assets/chevron.svg"
	anatomyFile    = "ANATOMY.md"
	chevronSVG     = `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="6" viewBox="0 0 10 6">
  <path d="M1 1l4 4 4-4" fill="none" stroke="#8a8578" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
</svg>
`
)

// anatomy is the reference the starter's comments send its reader to.
//
//go:embed ANATOMY.md
var anatomy []byte

// starterGroups sorts Carnet's tokens into commented groups, in the order of
// ANATOMY.md. A token Carnet adds must join one: TestStarterDeclaresCarnetTokens fails until it does.
var starterGroups = []struct {
	comment string
	tokens  []string
}{
	{"Surfaces: behind the page, the page, raised and sunken parts.", []string{"desk", "paper", "paper-raised", "paper-sunk"}},
	{"Text: 4.5:1 on every surface.", []string{"ink", "ink-soft", "muted"}},
	{"Lines; --rule-stroke is a background, for section dividers.", []string{"rule", "rule-strong", "rule-stroke", "rule-height"}},
	{"Accent: --accent fills and marks, --accent-text is the accent as text, --sun the logo.", []string{"accent", "accent-text", "accent-wash", "sun", "margin-line", "focus"}},
	{"Main button.", []string{"primary", "primary-ink"}},
	{"Top bar.", []string{"bar", "bar-ink", "bar-ink-soft"}},
	{"Statuses: one ink per category, then their shape.", []string{"stamp-todo", "stamp-doing", "stamp-done", "stamp-font", "stamp-case", "stamp-tracking", "stamp-dot", "stamp-large-size", "stamp-large-tracking", "stamp-large-turn"}},
	{"Issue types.", []string{"tone-bug", "tone-story", "tone-epic", "tone-task", "tone-subtask"}},
	{"Depth.", []string{"shadow-pop", "backdrop", "ink-blend"}},
	{"Type: title, body and code roles.", []string{"font-title", "font-body", "font-mono", "font-size", "title-weight", "h1-weight", "title-tracking", "title-italic", "empty-font"}},
	{"Shapes.", []string{"frame", "radius-xs", "radius-s", "radius-m", "radius-l", "avatar-radius", "card-top"}},
	{"Page background, loading mark (none or inline-block).", []string{"page-texture", "page-texture-size", "loader"}},
}

// starterFontTokens replace the fonts of Carnet the starter doesn't carry.
var starterFontTokens = map[string]string{
	"font-title": "Georgia, serif",
	"font-body":  "system-ui, sans-serif",
}

// starterHooks are the sections of ANATOMY.md's hooks, in its order.
var starterHooks = []string{
	"Shell", "Top bar", "Logo", "Page", "Panels / windows", "Buttons", "Fields", "Tables", "Tabs", "Menus",
	"Dropdowns", "Dialogs", "Save bar", "Workflows", "Toasts", "Marks", "Empty and waiting", "Pager", "Search",
	"Wiki text", "Editor", "Boards", "Setup wizard", "Administration overview", "Gallery",
}

// Starter returns the zip of the starter package: Carnet's tokens and preview
// under placeholders, an example font and asset, the hooks as empty headings,
// and ANATOMY.md.
var Starter = sync.OnceValues(func() ([]byte, error) {
	dir := "builtin/" + FallbackID + "/"
	raw, err := fs.ReadFile(builtin, dir+manifestFile)
	if err != nil {
		return nil, err
	}
	carnet, err := ParseManifest(raw)
	if err != nil {
		return nil, err
	}
	manifest, err := json.MarshalIndent(Manifest{
		ID:          starterID,
		Contract:    Contract,
		Version:     "0.1.0",
		Name:        map[string]string{"en": "My theme", "fr": "Mon thème"},
		Description: map[string]string{"en": "A few words on the look.", "fr": "Quelques mots sur son allure."},
		About:       map[string]string{"en": "One sentence for the theme gallery.", "fr": "Une phrase pour la galerie des thèmes."},
		Author:      "Your name",
		Schemes:     carnet.Schemes,
		Preview:     carnet.Preview,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	raw, err = fs.ReadFile(builtin, dir+cssFile)
	if err != nil {
		return nil, err
	}
	css, err := starterCSS(parseCSS(string(raw)))
	if err != nil {
		return nil, err
	}
	font, err := fs.ReadFile(builtin, dir+starterFont)
	if err != nil {
		return nil, err
	}
	licence, err := fs.ReadFile(builtin, dir+starterLicence)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, f := range []struct {
		name    string
		content []byte
	}{
		{manifestFile, append(manifest, '\n')},
		{cssFile, []byte(css)},
		{anatomyFile, anatomy},
		{starterChevron, []byte(chevronSVG)},
		{starterFont, font},
		{starterLicence, licence},
	} {
		if err := addToZip(zw, f.name, f.content); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
})

// starterCSS writes the starter's stylesheet, with the token values of carnet.
func starterCSS(carnet stylesheet) (string, error) {
	light, _, ok, err := tokenBlock(carnet, FallbackID, false)
	if err != nil || !ok {
		return "", fmt.Errorf("%s: no token block: %v", FallbackID, err)
	}
	dark, _, ok, err := tokenBlock(carnet, FallbackID, true)
	if err != nil || !ok {
		return "", fmt.Errorf("%s: no dark token block: %v", FallbackID, err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `/*
 * A starter theme. It passes the checks as it is, with the colours of Carnet.
 *
 * Choose an id: replace %[1]s here and in theme.json. Then change the tokens,
 * and copy --bar, --paper, --ink and --accent of each scheme to the preview of
 * theme.json. Every rule is in ANATOMY.md.
 */

/* Fonts ------------------------------------------------------------------- */

/* One @font-face per file, each with its licence next to it in fonts/. */

@font-face {
  font-family: 'IBM Plex Mono';
  font-style: normal;
  font-weight: 400;
  font-display: swap;
  src: url('%[2]s') format('woff2');
}

/* Tokens ------------------------------------------------------------------ */

/* Every token: this block must declare them all. */

`, starterID, starterFont)
	writeStarterBlock(&b, "[data-skin='"+starterID+"']", light, "light")
	b.WriteString("\n/* Only the colours that change in the dark. Delete this block, and \"dark\" from\n   theme.json, for a light-only theme. */\n\n")
	writeStarterBlock(&b, "[data-skin='"+starterID+"'][data-scheme='dark']", dark, "dark")
	fmt.Fprintf(&b, `
/* Assets ------------------------------------------------------------------ */

/* The dropdowns' chevron: the base doesn't draw one. */

[data-skin='%[1]s'] .select {
  background-image: url('%[2]s');
}

/* Hooks ------------------------------------------------------------------- */

/* The classes of each section are in ANATOMY.md, under Hooks. Delete the
   sections you don't use. */
`, starterID, starterChevron)
	for _, hook := range starterHooks {
		fmt.Fprintf(&b, "\n/* %s */\n", hook)
	}
	return b.String(), nil
}

func writeStarterBlock(b *strings.Builder, selector string, block tokens, scheme string) {
	b.WriteString(selector + " {\n")
	for _, group := range starterGroups {
		comment := false
		for _, name := range group.tokens {
			t, ok := block[name]
			if !ok {
				continue
			}
			if !comment {
				fmt.Fprintf(b, "  /* %s */\n", group.comment)
				comment = true
			}
			value := t.value
			if font, ok := starterFontTokens[name]; ok {
				value = font
			}
			fmt.Fprintf(b, "  --%s: %s;\n", name, value)
		}
		if comment {
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(b, "  color-scheme: %s;\n}\n", scheme)
}
