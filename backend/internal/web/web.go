// Package web serves the compiled Svelte interface embedded in the binary.
package web

import (
	"bytes"
	"compress/gzip"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
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

// GzipMinSize is the size under which a file or an answer is not worth
// compressing: the header and the CPU would cost more than the bytes saved.
const GzipMinSize = 1024

// packed is a file compressed once, on its first request.
type packed struct {
	once sync.Once
	body []byte // nil when the file does not shrink or is not text
}

// compressible tells whether a file type is text that gzip shrinks. Fonts
// and images of the build are compressed formats already, except SVG.
func compressible(name string) bool {
	switch path.Ext(name) {
	case ".js", ".mjs", ".css", ".html", ".json", ".svg", ".txt", ".map", ".xml", ".webmanifest":
		return true
	}
	return false
}

// gzipped is the compressed form of a file, kept in memory after the first
// request; nil when the file is left as it is.
func gzipped(cache *sync.Map, root fs.FS, name string) []byte {
	v, _ := cache.LoadOrStore(name, &packed{})
	p := v.(*packed)
	p.once.Do(func() {
		if !compressible(name) {
			return
		}
		raw, err := fs.ReadFile(root, name)
		if err != nil || len(raw) < GzipMinSize {
			return
		}
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		zw.Write(raw)
		zw.Close()
		if buf.Len() < len(raw) {
			p.body = buf.Bytes()
		}
	})
	return p.body
}

// AcceptsGzip tells whether an Accept-Encoding header allows gzip: gzip, or *,
// unless given a quality of 0.
func AcceptsGzip(header string) bool {
	ok := false
	for part := range strings.SplitSeq(header, ",") {
		name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "gzip" && name != "*" {
			continue
		}
		q := 1.0
		if k, v, found := strings.Cut(strings.TrimSpace(params), "="); found && strings.EqualFold(strings.TrimSpace(k), "q") {
			q, _ = strconv.ParseFloat(strings.TrimSpace(v), 64)
		}
		if name == "gzip" {
			return q > 0
		}
		ok = q > 0
	}
	return ok
}

// serveGzipped answers with the cached compressed bytes of a file.
func serveGzipped(w http.ResponseWriter, r *http.Request, contentType string, body []byte) {
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Content-Encoding", "gzip")
	h.Add("Vary", "Accept-Encoding")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method != http.MethodHead {
		w.Write(body)
	}
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
	var cache sync.Map // file name → *packed
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(root, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					// Vite fingerprints asset names, so they never change.
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				w.Header().Add("Vary", "Accept-Encoding")
				if AcceptsGzip(r.Header.Get("Accept-Encoding")) && r.Header.Get("Range") == "" {
					if body := gzipped(&cache, root, name); body != nil {
						serveGzipped(w, r, mime.TypeByExtension(path.Ext(name)), body)
						return
					}
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Add("Vary", "Accept-Encoding")
		if AcceptsGzip(r.Header.Get("Accept-Encoding")) {
			if body := gzipped(&cache, root, "index.html"); body != nil {
				serveGzipped(w, r, "text/html; charset=utf-8", body)
				return
			}
		}
		w.Write(index)
	})
}
