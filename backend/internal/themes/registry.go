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
	"sync"
	"time"

	"solstice/internal/settings"
	"solstice/internal/sqlq"
)

// The caps of a package.
const (
	MaxPackageFiles = 100
	MaxPackageBytes = 10 << 20
)

const (
	maxIDLen = 32
	// disabledSetting holds the ids of the built-in themes turned off, in JSON.
	disabledSetting = "themes.disabled"
)

// Registry is the themes a server offers: the built-in ones, then those
// stored on the instance. It reads the stored ones on each request, so a
// change to them needs no reload.
type Registry struct {
	builtin *Set
	db      *sql.DB // nil for the built-in themes only
	dir     string  // <data>/themes, a folder per stored theme
	log     *slog.Logger
	mu      sync.Mutex // one change at a time to the stored themes and their files
}

// NewRegistry returns the registry of an instance. With a nil db, as in the
// setup wizard, which runs before the migrations, it offers the built-in
// themes only.
func NewRegistry(db *sql.DB, dataDir string, log *slog.Logger) *Registry {
	return &Registry{builtin: Builtin(), db: db, dir: filepath.Join(dataDir, "themes"), log: log}
}

// storedTheme is a row of the themes table.
type storedTheme struct {
	id         string // the served id, which may not be the manifest's
	manifest   Manifest
	enabled    bool
	importedBy string
	importedAt time.Time
}

func byName(a, b storedTheme) int {
	return cmp.Or(cmp.Compare(strings.ToLower(a.manifest.Name["en"]), strings.ToLower(b.manifest.Name["en"])), cmp.Compare(a.id, b.id))
}

// merged lists the built-in themes not turned off, then the enabled stored ones
// by name. A built-in theme hides a stored one of the same id: a new release
// can bring an id an instance already serves.
func merged(builtin *Set, disabled []string, stored []storedTheme) Index {
	out := builtin.Index()
	out.Themes = slices.DeleteFunc(out.Themes, func(m Manifest) bool { return slices.Contains(disabled, m.ID) })
	slices.SortFunc(stored, byName)
	var extra []Manifest
	for _, t := range stored {
		if _, taken := builtin.byID[t.id]; t.enabled && !taken {
			m := t.manifest
			m.ID = t.id
			extra = append(extra, m)
		}
	}
	out.Themes = append(out.Themes, extra...)
	return out
}

// Index lists the themes on offer.
func (r *Registry) Index(ctx context.Context) (Index, error) {
	if r.db == nil {
		return r.builtin.Index(), nil
	}
	disabled, err := r.disabledBuiltin(ctx)
	if err != nil {
		return Index{}, err
	}
	stored, err := r.stored(ctx)
	if err != nil {
		return Index{}, err
	}
	return merged(r.builtin, disabled, stored), nil
}

// disabledBuiltin returns the ids of the built-in themes turned off.
func (r *Registry) disabledBuiltin(ctx context.Context) ([]string, error) {
	var ids []string
	_, err := settings.New(r.db).GetJSON(ctx, disabledSetting, &ids)
	return ids, err
}

const storedColumns = `id, manifest, enabled, COALESCE(imported_by, ''), imported_at`

func scanStored(rows interface{ Scan(...any) error }) (t storedTheme, raw string, err error) {
	err = rows.Scan(&t.id, &raw, &t.enabled, &t.importedBy, &t.importedAt)
	return t, raw, err
}

// stored reads the themes table.
func (r *Registry) stored(ctx context.Context) ([]storedTheme, error) {
	var out []storedTheme
	err := sqlq.Each(ctx, r.db, `SELECT `+storedColumns+` FROM themes`, func(rows *sql.Rows) error {
		t, raw, err := scanStored(rows)
		if err != nil {
			return err
		}
		m, err := ParseManifest([]byte(raw))
		if err != nil {
			// One bad row must not take every theme down with it.
			r.log.Warn("stored theme skipped", "theme", t.id, "error", err)
			return nil
		}
		t.manifest = m
		out = append(out, t)
		return nil
	})
	return out, err
}

// storedOne reads a row of the themes table. A manifest it can't read leaves
// the theme's manifest empty: such a theme can still be turned off or deleted.
func (r *Registry) storedOne(ctx context.Context, id string) (storedTheme, error) {
	t, raw, err := scanStored(r.db.QueryRowContext(ctx, `SELECT `+storedColumns+` FROM themes WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return storedTheme{}, ErrNotFound
	}
	if err != nil {
		return storedTheme{}, err
	}
	t.manifest, _ = ParseManifest([]byte(raw))
	return t, nil
}

// Serve answers a file of theme id; false, with nothing written, when there
// is no such file or theme, or the theme is disabled and mayDisabled, asked only
// then, doesn't let this request see it. A nil mayDisabled lets no request.
func (r *Registry) Serve(w http.ResponseWriter, req *http.Request, id, name string, mayDisabled func(*http.Request) bool) (bool, error) {
	if _, ok := r.builtin.byID[id]; ok {
		if r.db != nil {
			disabled, err := r.disabledBuiltin(req.Context())
			if err != nil {
				return false, err
			}
			if slices.Contains(disabled, id) && !serveDisabled(w, req, mayDisabled) {
				return false, nil
			}
		}
		return r.builtin.Serve(w, req, id, name), nil
	}
	if r.db == nil || !idPattern.MatchString(id) {
		return false, nil
	}
	var enabled bool
	err := r.db.QueryRowContext(req.Context(), `SELECT enabled FROM themes WHERE id = ?`, id).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !enabled && !serveDisabled(w, req, mayDisabled) {
		return false, nil
	}
	return r.serveStored(w, req, id, name)
}

// serveDisabled says whether this request may see a disabled theme. Its answer
// depends on who asks, so no shared cache may keep it.
func serveDisabled(w http.ResponseWriter, req *http.Request, mayDisabled func(*http.Request) bool) bool {
	if mayDisabled == nil || !mayDisabled(req) {
		return false
	}
	w.Header().Set("Cache-Control", "private, no-cache")
	return true
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
