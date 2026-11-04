// Package themes reads the theme packages built into the binary and serves
// their files.
//
// A package is a folder named after its id, holding theme.json (see
// Manifest), theme.css and optional assets/ and fonts/ folders.
package themes

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Contract is the version of the interface between the server and theme
// packages: the hooks and tokens a theme may rely on. A package declaring a
// higher one is refused.
const Contract = 1

// Manifest is a package's theme.json.
type Manifest struct {
	ID          string             `json:"id"`
	Contract    int                `json:"contract"`
	Version     string             `json:"version"`
	Name        map[string]string  `json:"name"`            // locale → text
	Description map[string]string  `json:"description"`     // locale → text
	About       map[string]string  `json:"about,omitempty"` // locale → text, optional
	Author      string             `json:"author"`
	Schemes     []string           `json:"schemes"`
	Preview     map[string]Preview `json:"preview"` // scheme → colours
}

// Preview holds the colours a theme's thumbnail is drawn with.
type Preview struct {
	Bar    string `json:"bar"`
	Paper  string `json:"paper"`
	Ink    string `json:"ink"`
	Accent string `json:"accent"`
}

var (
	idPattern     = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)
	colourPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	// From https://semver.org/#is-there-a-suggested-regular-expression-regex-to-check-a-semver-string.
	semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
		`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
		`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)
)

// ParseManifest reads and checks a theme.json.
func ParseManifest(data []byte) (Manifest, error) {
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return Manifest{}, errors.New("data after the manifest")
	}
	return m, m.check()
}

func (m Manifest) check() error {
	if !idPattern.MatchString(m.ID) {
		return fmt.Errorf("id %q: want 1 to 32 of a-z, 0-9 and -", m.ID)
	}
	if m.Contract < 1 || m.Contract > Contract {
		return fmt.Errorf("contract %d: this server supports 1 to %d", m.Contract, Contract)
	}
	if !semverPattern.MatchString(m.Version) {
		return fmt.Errorf("version %q: not a semantic version", m.Version)
	}
	for field, texts := range map[string]map[string]string{"name": m.Name, "description": m.Description, "about": m.About} {
		if field == "about" && texts == nil {
			continue
		}
		if texts["en"] == "" {
			return fmt.Errorf("%s: no English (en) text", field)
		}
		for locale, text := range texts {
			if locale == "" || strings.TrimSpace(text) == "" {
				return fmt.Errorf("%s: empty locale or text", field)
			}
		}
	}
	if strings.TrimSpace(m.Author) == "" {
		return errors.New("no author")
	}
	if !slices.Equal(m.Schemes, []string{"light"}) && !slices.Equal(m.Schemes, []string{"light", "dark"}) {
		return fmt.Errorf("schemes %q: want [light] or [light dark]", m.Schemes)
	}
	if len(m.Preview) != len(m.Schemes) {
		return errors.New("preview: want one entry for each scheme")
	}
	for _, scheme := range m.Schemes {
		p, ok := m.Preview[scheme]
		if !ok {
			return fmt.Errorf("preview: no %s colours", scheme)
		}
		for name, c := range map[string]string{"bar": p.Bar, "paper": p.Paper, "ink": p.Ink, "accent": p.Accent} {
			if !colourPattern.MatchString(c) {
				return fmt.Errorf("preview.%s.%s %q: want #rrggbb", scheme, name, c)
			}
		}
	}
	return nil
}

// fileTypes are the files a package may hold, by extension.
var fileTypes = map[string]string{
	".css":   "text/css; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".webp":  "image/webp",
	".gif":   "image/gif",
	".woff2": "font/woff2",
	".woff":  "font/woff",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".txt":   "text/plain; charset=utf-8",
}

type file struct {
	data        []byte
	contentType string
	etag        string
}

// Package is a theme package, its files held in memory.
type Package struct {
	Manifest Manifest
	files    map[string]file // theme.css, assets/…, fonts/…
}

// Set is the theme packages a server offers.
type Set struct {
	list []*Package // by id, or in the order of sortBy
	byID map[string]*Package
}

// Load reads every folder of root as a theme package.
func Load(root fs.FS) (*Set, error) {
	entries, err := fs.ReadDir(root, ".")
	if err != nil {
		return nil, err
	}
	s := &Set{byID: map[string]*Package{}}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir, _ := fs.Sub(root, e.Name())
		p, err := loadPackage(dir)
		if err != nil {
			return nil, fmt.Errorf("theme %s: %w", e.Name(), err)
		}
		if p.Manifest.ID != e.Name() {
			return nil, fmt.Errorf("theme %s: id %q is not the folder's name", e.Name(), p.Manifest.ID)
		}
		s.list = append(s.list, p)
		s.byID[p.Manifest.ID] = p
	}
	return s, nil
}

func loadPackage(dir fs.FS) (*Package, error) {
	raw, err := fs.ReadFile(dir, "theme.json")
	if err != nil {
		return nil, err
	}
	m, err := ParseManifest(raw)
	if err != nil {
		return nil, fmt.Errorf("theme.json: %w", err)
	}
	p := &Package{Manifest: m, files: map[string]file{}}
	names := []string{"theme.css"}
	for _, sub := range []string{"assets", "fonts"} {
		err := fs.WalkDir(dir, sub, func(name string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				names = append(names, name)
			}
			return err
		})
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	for _, name := range names {
		contentType, ok := fileTypes[strings.ToLower(path.Ext(name))]
		if !ok {
			return nil, fmt.Errorf("%s: unsupported file type", name)
		}
		data, err := fs.ReadFile(dir, name)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		p.files[name] = file{data: data, contentType: contentType, etag: `"` + hex.EncodeToString(sum[:16]) + `"`}
	}
	return p, nil
}

// sortBy puts the packages in the order of ids, which must name each once.
func (s *Set) sortBy(ids []string) error {
	if len(ids) != len(s.list) {
		return fmt.Errorf("order lists %d themes, there are %d", len(ids), len(s.list))
	}
	list := make([]*Package, 0, len(ids))
	for _, id := range ids {
		p, ok := s.byID[id]
		if !ok || slices.Contains(list, p) {
			return fmt.Errorf("order: %q is not a theme, or is listed twice", id)
		}
		list = append(list, p)
	}
	s.list = list
	return nil
}

// builtin holds a folder per package and order.json, the ids in the order
// the theme menus list them.
//
//go:embed builtin
var builtin embed.FS

// Builtin returns the packages embedded in the binary, in the order of
// order.json. It panics on an invalid one: TestBuiltin catches that before a
// release.
func Builtin() *Set {
	root, err := fs.Sub(builtin, "builtin")
	if err != nil {
		panic(err)
	}
	s, err := Load(root)
	if err != nil {
		panic(err)
	}
	raw, err := fs.ReadFile(root, "order.json")
	if err != nil {
		panic(err)
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		panic(fmt.Errorf("order.json: %w", err))
	}
	if err := s.sortBy(ids); err != nil {
		panic(err)
	}
	return s
}

// Index is the answer of GET /api/themes.
type Index struct {
	Contract int        `json:"contract"`
	Themes   []Manifest `json:"themes"`
}

// Index lists the packages' manifests, by id.
func (s *Set) Index() Index {
	out := Index{Contract: Contract, Themes: make([]Manifest, len(s.list))}
	for i, p := range s.list {
		out.Themes[i] = p.Manifest
	}
	return out
}

// Serve answers a file of package id; false, with nothing written, when
// there is no such package or file.
func (s *Set) Serve(w http.ResponseWriter, r *http.Request, id, name string) bool {
	p, ok := s.byID[id]
	if !ok {
		return false
	}
	f, ok := p.files[name]
	if !ok {
		return false
	}
	h := w.Header()
	h.Set("Content-Type", f.contentType)
	h.Set("X-Content-Type-Options", "nosniff")
	// An SVG opened on its own is a document: it must not run script.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	h.Set("ETag", f.etag)
	h.Set("Cache-Control", "public, no-cache")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(f.data))
	return true
}
