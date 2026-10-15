// Package web serves the compiled Svelte interface embedded in the binary.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// The frontend build (pnpm build) writes into dist; .gitkeep keeps the
// directory present so the binary compiles before the first build.
//
//go:embed all:dist
var dist embed.FS

// IsAsset tells whether a request path names a file of the build (scripts,
// styles, fonts…) rather than a page of the interface.
func IsAsset(urlPath string) bool {
	name := strings.TrimPrefix(path.Clean(urlPath), "/")
	if name == "" || name == "index.html" {
		return false
	}
	info, err := fs.Stat(dist, "dist/"+name)
	return err == nil && !info.IsDir()
}

// Handler serves static files, and index.html for every other path so the
// interface's own routes (/browse/DEMO-1, /projects/…) work on reload.
func Handler() http.Handler {
	root, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	index, err := fs.ReadFile(root, "index.html")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "Interface not built: run \"pnpm build\" in frontend/ "+
				"(or \"pnpm dev\" for development).", http.StatusServiceUnavailable)
		})
	}
	files := http.FileServerFS(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(root, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					// Vite fingerprints asset names, so they never change.
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(index)
	})
}
