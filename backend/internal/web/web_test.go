package web

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

func TestAcceptsGzip(t *testing.T) {
	for header, want := range map[string]bool{
		"": false, "gzip": true, "GZIP": true, "gzip, deflate, br": true, "deflate": false,
		"*": true, "gzip;q=0": false, "gzip;q=0.5": true, "*;q=0": false, "gzip;q=0, *": false, "br, *;q=0.1": true,
	} {
		if got := AcceptsGzip(header); got != want {
			t.Errorf("AcceptsGzip(%q) = %v, want %v", header, got, want)
		}
	}
}

func TestGzippedOnce(t *testing.T) {
	script := make([]byte, 5000)
	for i := range script {
		script[i] = "var a = 1;\n"[i%11]
	}
	root := fstest.MapFS{
		"app.js":    {Data: script},
		"small.css": {Data: []byte("a{}")},
		"font.woff": {Data: script},
	}
	var cache = new(sync.Map)
	first := gzipped(cache, root, "app.js")
	if first == nil || len(first) >= len(script) {
		t.Fatalf("app.js not compressed: %d bytes", len(first))
	}
	if second := gzipped(cache, root, "app.js"); &second[0] != &first[0] {
		t.Error("app.js was compressed again")
	}
	if gzipped(cache, root, "small.css") != nil {
		t.Error("a file under 1 KB was compressed")
	}
	if gzipped(cache, root, "font.woff") != nil {
		t.Error("a font was compressed")
	}
}

func TestServeGzipped(t *testing.T) {
	r := httptest.NewRequest("GET", "/assets/app.js", nil)
	w := httptest.NewRecorder()
	serveGzipped(w, r, "text/javascript; charset=utf-8", []byte("x"))
	if w.Header().Get("Content-Encoding") != "gzip" || w.Header().Get("Vary") != "Accept-Encoding" ||
		w.Header().Get("Content-Length") != "1" || w.Code != http.StatusOK {
		t.Errorf("headers: %v", w.Header())
	}
}

func decompress(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(plain)
}

// The embedded build: with gzip asked, a script comes compressed and
// decompresses to the original; without it, as it is.
func TestHandlerGzip(t *testing.T) {
	entries, err := dist.ReadDir("dist/assets")
	if err != nil {
		t.Skip("interface not built")
	}
	var script string
	for _, e := range entries {
		if info, err := e.Info(); err == nil && strings.HasSuffix(e.Name(), ".js") && info.Size() >= GzipMinSize {
			script = "/assets/" + e.Name()
		}
	}
	if script == "" {
		t.Skip("interface not built")
	}
	h := Handler()
	plain := httptest.NewRecorder()
	h.ServeHTTP(plain, httptest.NewRequest("GET", script, nil))
	if plain.Header().Get("Content-Encoding") != "" || plain.Header().Get("Vary") != "Accept-Encoding" {
		t.Errorf("without gzip: %v", plain.Header())
	}

	r := httptest.NewRequest("GET", script, nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Header().Get("Content-Encoding") != "gzip" || w.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("with gzip: %v", w.Header())
	}
	if w.Header().Get("Cache-Control") == "" || !strings.Contains(w.Header().Get("Content-Type"), "javascript") {
		t.Errorf("headers lost: %v", w.Header())
	}
	if got := decompress(t, w); got != plain.Body.String() {
		t.Error("the body does not decompress to the same bytes")
	}
}
