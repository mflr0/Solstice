package themes

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// Export returns the zip of a theme's package, as an import reads it, and a
// name for the file. The package keeps its manifest's id, whatever id the theme
// is served under. A disabled theme counts as absent unless withDisabled is set.
func (r *Registry) Export(ctx context.Context, id string, withDisabled bool) (name string, data []byte, err error) {
	if p, ok := r.builtin.byID[id]; ok {
		if r.db != nil && !withDisabled {
			disabled, err := r.disabledBuiltin(ctx)
			if err != nil {
				return "", nil, err
			}
			if slices.Contains(disabled, id) {
				return "", nil, ErrNotFound
			}
		}
		data, err = zipPackage(p.dir)
		return exportName(p.Manifest), data, err
	}
	if r.db == nil {
		return "", nil, ErrNotFound
	}
	// Held while reading, so a replace can't mix two versions in the zip.
	r.mu.Lock()
	defer r.mu.Unlock()
	t, err := r.storedOne(ctx, id)
	if err != nil {
		return "", nil, err
	}
	if t.manifest.ID == "" || (!t.enabled && !withDisabled) {
		return "", nil, ErrNotFound
	}
	root, err := os.OpenRoot(filepath.Join(r.dir, id))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, err
	}
	defer root.Close()
	data, err = zipPackage(root.FS())
	return exportName(t.manifest), data, err
}

func exportName(m Manifest) string { return m.ID + "-" + m.Version + ".zip" }

// zipPackage packs the regular files of a package, always in the same bytes.
func zipPackage(pkg fs.FS) ([]byte, error) {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	err := fs.WalkDir(pkg, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		content, err := fs.ReadFile(pkg, name)
		if err != nil {
			return err
		}
		method := zip.Deflate
		// The import refuses a big entry that inflates too much: don't compress it.
		if len(content) > ratioFloor {
			method = zip.Store
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			return err
		}
		_, err = w.Write(content)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
