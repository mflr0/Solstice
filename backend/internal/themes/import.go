package themes

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"solstice/internal/settings"
	"solstice/internal/sqlq"
)

const (
	// FallbackID is the theme users go back to: it can't be disabled.
	FallbackID = "carnet"
	// MaxZipBytes caps a package's zip. Stored files plus the zip's own headers
	// fit in it.
	MaxZipBytes = MaxPackageBytes + 1<<20

	// An entry over ratioFloor bytes may not inflate more than maxZipRatio
	// times. Small files escape it: a short repetitive stylesheet does that
	// honestly, and MaxPackageBytes bounds them anyway.
	maxZipRatio = 100
	ratioFloor  = 1 << 20
	maxNameLen  = 200
	// Findings an unreadable zip reports, as its names come back in the answer.
	maxZipFindings = 50
)

// Errors of the theme administration.
var (
	ErrNotFound = errors.New("no such theme")
	ErrBuiltin  = errors.New("a built-in theme can't be replaced or deleted")
	ErrFallback = errors.New("the fallback theme can't be disabled")
)

// Origins of a theme.
const (
	OriginBuiltin  = "builtin"
	OriginImported = "imported"
)

// Entry is a theme as the administration sees it, enabled or not. ID is the
// served id, ManifestID the one in its package.
type Entry struct {
	Manifest
	ManifestID string     `json:"manifestId"`
	Origin     string     `json:"origin"`
	Enabled    bool       `json:"enabled"`
	ImportedBy string     `json:"importedBy,omitempty"`
	ImportedAt *time.Time `json:"importedAt,omitempty"`
}

func builtinEntry(p *Package, disabled []string) Entry {
	return Entry{Manifest: p.Manifest, ManifestID: p.Manifest.ID, Origin: OriginBuiltin, Enabled: !slices.Contains(disabled, p.Manifest.ID)}
}

func (t storedTheme) entry() Entry {
	m := t.manifest
	m.ID = t.id
	e := Entry{Manifest: m, ManifestID: t.manifest.ID, Origin: OriginImported, Enabled: t.enabled, ImportedBy: t.importedBy}
	if !t.importedAt.IsZero() {
		e.ImportedAt = &t.importedAt
	}
	return e
}

// List returns every theme the administration can act on: the built-in ones,
// then the stored ones by name. A stored theme a built-in hides isn't one.
func (r *Registry) List(ctx context.Context) ([]Entry, error) {
	disabled, err := r.disabledBuiltin(ctx)
	if err != nil {
		return nil, err
	}
	stored, err := r.stored(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(r.builtin.list)+len(stored))
	for _, p := range r.builtin.list {
		out = append(out, builtinEntry(p, disabled))
	}
	slices.SortFunc(stored, byName)
	for _, t := range stored {
		if _, hidden := r.builtin.byID[t.id]; !hidden {
			out = append(out, t.entry())
		}
	}
	return out, nil
}

// Import stores a package from its zip, under its manifest's id or, when that
// is taken, a suffixed one. It stores nothing when there are findings.
func (r *Registry) Import(ctx context.Context, data []byte, by string) (Entry, []Finding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	work, err := r.workDir()
	if err != nil {
		return Entry{}, nil, err
	}
	defer os.RemoveAll(work)
	unpacked := filepath.Join(work, "new")
	m, raw, findings, err := openPackage(data, unpacked)
	if err != nil || len(findings) > 0 {
		return Entry{}, findings, err
	}
	taken := map[string]bool{}
	err = sqlq.Each(ctx, r.db, `SELECT id FROM themes`, func(rows *sql.Rows) error {
		var id string
		err := rows.Scan(&id)
		taken[id] = true
		return err
	})
	if err != nil {
		return Entry{}, nil, err
	}
	id := freeID(m.ID, func(id string) bool {
		_, builtin := r.builtin.byID[id]
		return builtin || taken[id]
	})
	folder := filepath.Join(r.dir, id)
	// The row says what exists: a folder without one is what a crash left.
	if err := os.RemoveAll(folder); err != nil {
		return Entry{}, nil, err
	}
	if err := os.Rename(unpacked, folder); err != nil {
		return Entry{}, nil, err
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `INSERT INTO themes (id, manifest_id, manifest, version, enabled, imported_by, imported_at)
		VALUES (?, ?, ?, ?, TRUE, ?, ?)`, id, m.ID, raw, m.Version, by, now)
	if err != nil {
		os.RemoveAll(folder)
		return Entry{}, nil, err
	}
	return storedTheme{id: id, manifest: m, enabled: true, importedBy: by, importedAt: now}.entry(), nil, nil
}

// Replace swaps the files of a stored theme for a new version of the same
// manifest id. The theme keeps its served id and whether it is enabled.
func (r *Registry) Replace(ctx context.Context, id string, data []byte, by string) (e Entry, oldVersion string, findings []Finding, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.builtin.byID[id]; ok {
		return Entry{}, "", nil, ErrBuiltin
	}
	var manifestID string
	var enabled bool
	err = r.db.QueryRowContext(ctx, `SELECT manifest_id, version, enabled FROM themes WHERE id = ?`, id).Scan(&manifestID, &oldVersion, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, "", nil, ErrNotFound
	}
	if err != nil {
		return Entry{}, "", nil, err
	}
	work, err := r.workDir()
	if err != nil {
		return Entry{}, "", nil, err
	}
	defer os.RemoveAll(work)
	unpacked, previous := filepath.Join(work, "new"), filepath.Join(work, "old")
	m, raw, findings, err := openPackage(data, unpacked)
	if err != nil || len(findings) > 0 {
		return Entry{}, "", findings, err
	}
	if m.ID != manifestID {
		return Entry{}, "", []Finding{{Rule: "manifest", File: manifestFile,
			Message: fmt.Sprintf("id %q: this theme's is %q, import the package to add a new theme", m.ID, manifestID)}}, nil
	}
	folder := filepath.Join(r.dir, id)
	if err := os.Rename(folder, previous); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Entry{}, "", nil, err
	}
	restore := func() {
		os.RemoveAll(folder)
		os.Rename(previous, folder)
	}
	if err := os.Rename(unpacked, folder); err != nil {
		restore()
		return Entry{}, "", nil, err
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `UPDATE themes SET manifest = ?, version = ?, imported_by = ?, imported_at = ? WHERE id = ?`,
		raw, m.Version, by, now, id)
	if err != nil {
		restore()
		return Entry{}, "", nil, err
	}
	return storedTheme{id: id, manifest: m, enabled: enabled, importedBy: by, importedAt: now}.entry(), oldVersion, nil, nil
}

// Delete removes a stored theme, and returns what it was.
func (r *Registry) Delete(ctx context.Context, id string) (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.builtin.byID[id]; ok {
		return Entry{}, ErrBuiltin
	}
	t, err := r.storedOne(ctx, id)
	if err != nil {
		return Entry{}, err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM themes WHERE id = ?`, id); err != nil {
		return Entry{}, err
	}
	// The row is gone, so the theme is: the next import of this id clears a folder left here.
	if err := os.RemoveAll(filepath.Join(r.dir, id)); err != nil {
		r.log.Warn("deleted theme's files kept", "theme", id, "error", err)
	}
	return t.entry(), nil
}

// SetEnabled turns a theme on or off, and says whether that changed anything.
func (r *Registry) SetEnabled(ctx context.Context, id string, on bool) (Entry, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.builtin.byID[id]; ok {
		if id == FallbackID && !on {
			return Entry{}, false, ErrFallback
		}
		disabled, err := r.disabledBuiltin(ctx)
		if err != nil {
			return Entry{}, false, err
		}
		if slices.Contains(disabled, id) != on {
			return builtinEntry(p, disabled), false, nil
		}
		disabled = slices.DeleteFunc(disabled, func(d string) bool { return d == id })
		if !on {
			disabled = append(disabled, id)
		}
		if err := settings.New(r.db).SetJSON(ctx, disabledSetting, disabled); err != nil {
			return Entry{}, false, err
		}
		return builtinEntry(p, disabled), true, nil
	}
	t, err := r.storedOne(ctx, id)
	if err != nil {
		return Entry{}, false, err
	}
	if t.enabled == on {
		return t.entry(), false, nil
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE themes SET enabled = ? WHERE id = ?`, on, id); err != nil {
		return Entry{}, false, err
	}
	t.enabled = on
	return t.entry(), true, nil
}

// workDir makes a folder next to the stored themes, so a rename brings its
// content in at once. No id starts with a dot, so nothing serves it.
func (r *Registry) workDir() (string, error) {
	if err := os.MkdirAll(r.dir, 0o750); err != nil {
		return "", err
	}
	return os.MkdirTemp(r.dir, ".import-*")
}

// freeID returns want if it isn't taken, else want with the first free suffix,
// cut to stay a valid id.
func freeID(want string, taken func(string) bool) string {
	if !taken(want) {
		return want
	}
	for n := 2; ; n++ {
		suffix := "-" + strconv.Itoa(n)
		id := want[:min(len(want), maxIDLen-len(suffix))] + suffix
		if !taken(id) {
			return id
		}
	}
}

// openPackage unzips data into dir, a folder it creates, and validates the
// result. raw is the manifest as the package has it.
func openPackage(data []byte, dir string) (m Manifest, raw string, findings []Finding, err error) {
	findings, err = unzipPackage(data, dir)
	if err != nil || len(findings) > 0 {
		return Manifest{}, "", findings, err
	}
	pkg := os.DirFS(dir)
	if findings = Validate(pkg); len(findings) > 0 {
		return Manifest{}, "", findings, nil
	}
	content, err := fs.ReadFile(pkg, manifestFile)
	if err != nil {
		return Manifest{}, "", nil, err
	}
	m, err = ParseManifest(content)
	return m, string(content), nil, err
}

func zipFinding(name, format string, args ...any) Finding {
	if len(name) > maxNameLen {
		name = name[:maxNameLen]
	}
	return Finding{Rule: "zip", Message: fmt.Sprintf(format, args...), File: name}
}

// unzipPackage writes the files of a package's zip into dir, which it creates.
// It reads the zip as hostile: a finding, and possibly a part of the files in
// dir, when any entry or limit fails. err is for what fails on this side.
func unzipPackage(data []byte, dir string) ([]Finding, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	// ErrInsecurePath, under one GODEBUG setting, still gives the reader: the
	// names are checked below whatever the setting.
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return []Finding{zipFinding(".", "not a zip file: %v", err)}, nil
	}
	var out []Finding
	var files int
	var declared uint64
	seen := map[string]bool{}
	for _, f := range zr.File {
		if len(out) == maxZipFindings {
			break
		}
		if problem := refusedEntry(f); problem != "" {
			out = append(out, zipFinding(f.Name, "%s", problem))
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}
		files++
		declared += min(f.UncompressedSize64, MaxPackageBytes+1)
		// Lower-cased: on a case-insensitive disk these would be one file.
		if key := strings.ToLower(f.Name); seen[key] {
			out = append(out, zipFinding(f.Name, "this name appears twice"))
		} else {
			seen[key] = true
		}
	}
	if files > MaxPackageFiles {
		out = append(out, zipFinding(".", "%d files: the most a package may hold is %d", files, MaxPackageFiles))
	}
	if declared > MaxPackageBytes {
		out = append(out, zipFinding(".", "over %d bytes once unpacked: the most a package may hold", MaxPackageBytes))
	}
	if len(out) > 0 {
		return out, nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	var written int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		content, finding := inflate(f, MaxPackageBytes-written)
		if finding != nil {
			return []Finding{*finding}, nil
		}
		written += int64(len(content))
		file := filepath.Join(dir, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
			return nil, err
		}
		if err := os.WriteFile(file, content, 0o600); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// inflate reads an entry, trusting the bytes that come out and not the sizes
// its headers declare. room is what the package may still grow by.
func inflate(f *zip.File, room int64) ([]byte, *Finding) {
	refuse := func(format string, args ...any) ([]byte, *Finding) {
		finding := zipFinding(f.Name, format, args...)
		return nil, &finding
	}
	rc, err := f.Open()
	if err != nil {
		return refuse("can't be read: %v", err)
	}
	defer rc.Close()
	content, err := io.ReadAll(io.LimitReader(rc, room+1))
	if err != nil {
		return refuse("can't be read: %v", err)
	}
	if int64(len(content)) > room {
		return refuse("over %d bytes once unpacked: the most a package may hold", MaxPackageBytes)
	}
	if len(content) > ratioFloor && uint64(len(content)) > maxZipRatio*max(f.CompressedSize64, 1) {
		return refuse("compressed over %d times: the most a package may be", maxZipRatio)
	}
	return content, nil
}

// refusedEntry says why a zip entry can't be part of a package, or nothing.
func refusedEntry(f *zip.File) string {
	name := strings.TrimSuffix(f.Name, "/")
	mode := f.Mode()
	switch {
	case !fs.ValidPath(name) || name == "." || strings.Contains(name, `\`):
		return "not a relative path inside the package"
	case len(name) > maxNameLen || strings.ContainsFunc(name, unicode.IsControl):
		return "not a usable file name"
	case mode&fs.ModeSymlink != 0:
		return "links are not allowed"
	case mode.IsDir():
		if name != "assets" && name != "fonts" {
			return "only the folders assets and fonts are allowed"
		}
		return ""
	case !mode.IsRegular():
		return "not a regular file"
	case !importable(name):
		return "this file is not allowed: theme.json, theme.css, ANATOMY.md, assets/*.svg|png|webp|jpg, fonts/*.woff2 and licence files only"
	}
	return ""
}

var rootLicence = regexp.MustCompile(`(?i)^(licen[cs]e|copying)(\.txt|\.md)?$`)

// importable says whether a package from elsewhere may hold this file. It is
// narrower than fileTypes, which also covers what the built-in packages may
// hold. A licence file is a .txt next to what it covers, as the validator reads
// it, or a usual licence name at the root. ANATOMY.md comes with the starter, so
// that zipping its folder works; like a root licence, it is never served.
func importable(name string) bool {
	dir, base := path.Split(name)
	ext := strings.ToLower(path.Ext(base))
	switch dir {
	case "":
		return name == manifestFile || name == cssFile || name == anatomyFile || rootLicence.MatchString(name)
	case "assets/":
		return slices.Contains([]string{".svg", ".png", ".webp", ".jpg", ".txt"}, ext)
	case "fonts/":
		return ext == ".woff2" || ext == ".txt"
	}
	return false
}
