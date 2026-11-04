package themes

import (
	"cmp"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"solstice/internal/sqlq"
)

// The caps of a package.
const (
	MaxPackageFiles = 100
	MaxPackageBytes = 10 << 20
)

// Registry is the themes a server offers: the built-in ones, then those
// stored on the instance. It reads the stored ones on each request, so a
// change to them needs no reload.
type Registry struct {
	builtin *Set
	db      *sql.DB // nil for the built-in themes only
	dir     string  // <data>/themes, a folder per stored theme
	log     *slog.Logger
}

// NewRegistry returns the registry of an instance. With a nil db, as in the
// setup wizard, which runs before the migrations, it offers the built-in
// themes only.
func NewRegistry(db *sql.DB, dataDir string, log *slog.Logger) *Registry {
	return &Registry{builtin: Builtin(), db: db, dir: filepath.Join(dataDir, "themes"), log: log}
}

// storedTheme is a row of the themes table.
type storedTheme struct {
	id       string // the served id, which may not be the manifest's
	manifest Manifest
	enabled  bool
}

// merged lists the built-in themes, then the enabled stored ones by name. A
// built-in theme hides a stored one of the same id: a new release can bring
// an id an instance already serves.
func merged(builtin *Set, stored []storedTheme) Index {
	out := builtin.Index()
	var extra []Manifest
	for _, t := range stored {
		if _, taken := builtin.byID[t.id]; t.enabled && !taken {
			m := t.manifest
			m.ID = t.id
			extra = append(extra, m)
		}
	}
	slices.SortFunc(extra, func(a, b Manifest) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name["en"]), strings.ToLower(b.Name["en"])), cmp.Compare(a.ID, b.ID))
	})
	out.Themes = append(out.Themes, extra...)
	return out
}

// Index lists the themes on offer.
func (r *Registry) Index(ctx context.Context) (Index, error) {
	if r.db == nil {
		return r.builtin.Index(), nil
	}
	var stored []storedTheme
	err := sqlq.Each(ctx, r.db, `SELECT id, manifest, enabled FROM themes`, func(rows *sql.Rows) error {
		var t storedTheme
		var raw string
		if err := rows.Scan(&t.id, &raw, &t.enabled); err != nil {
			return err
		}
		m, err := ParseManifest([]byte(raw))
		if err != nil {
			// One bad row must not take every theme down with it.
			r.log.Warn("stored theme skipped", "theme", t.id, "error", err)
			return nil
		}
		t.manifest = m
		stored = append(stored, t)
		return nil
	})
	if err != nil {
		return Index{}, err
	}
	return merged(r.builtin, stored), nil
}

// Serve answers a file of theme id; false, with nothing written, when there
// is no such enabled theme or file.
func (r *Registry) Serve(w http.ResponseWriter, req *http.Request, id, name string) (bool, error) {
	if _, ok := r.builtin.byID[id]; ok {
		return r.builtin.Serve(w, req, id, name), nil
	}
	if r.db == nil || !idPattern.MatchString(id) {
		return false, nil
	}
	var enabled bool
	err := r.db.QueryRowContext(req.Context(), `SELECT enabled FROM themes WHERE id = ?`, id).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !enabled) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return r.serveStored(w, req, id, name)
}

// serveStored answers a file of a stored theme, from the same paths and types
// as a built-in package. OpenInRoot keeps it inside the theme's folder.
func (r *Registry) serveStored(w http.ResponseWriter, req *http.Request, id, name string) (bool, error) {
	contentType, ok := fileTypes[strings.ToLower(path.Ext(name))]
	if !ok || !fs.ValidPath(name) || (name != cssFile && !strings.HasPrefix(name, "assets/") && !strings.HasPrefix(name, "fonts/")) {
		return false, nil
	}
	f, err := os.OpenInRoot(filepath.Join(r.dir, id), name)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			r.log.Warn("stored theme file refused", "theme", id, "file", name, "error", err)
		}
		return false, nil
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() {
		return false, err
	}
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return false, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	serveFile(w, req, name, contentType, `"`+hex.EncodeToString(sum.Sum(nil)[:16])+`"`, f)
	return true, nil
}
