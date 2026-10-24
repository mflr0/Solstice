package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"solstice/internal/web"
)

func serveGzip(t *testing.T, path, acceptEncoding, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		io.WriteString(w, body)
	}))
	r := httptest.NewRequest("GET", path, nil)
	if acceptEncoding != "" {
		r.Header.Set("Accept-Encoding", acceptEncoding)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestGzipCompressesWhenAsked(t *testing.T) {
	body := `{"key":"` + strings.Repeat("DEMO-1 ", 500) + `"}`
	w := serveGzip(t, "/api/issues", "gzip, deflate", "application/json; charset=utf-8", body)
	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q", got)
	}
	if got := w.Header().Get("Content-Length"); got != "" {
		t.Errorf("Content-Length = %q, want none", got)
	}
	if got := w.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("Vary = %q", got)
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != body {
		t.Errorf("the body does not decompress to the same bytes (%d, want %d)", len(plain), len(body))
	}
	if w.Body.Len() >= len(body) {
		t.Errorf("compressed %d bytes, not under %d", w.Body.Len(), len(body))
	}
}

func TestGzipNotWhenNotAsked(t *testing.T) {
	body := strings.Repeat("a", 5000)
	for _, accept := range []string{"", "identity", "deflate", "gzip;q=0", "br"} {
		w := serveGzip(t, "/api/issues", accept, "application/json", body)
		if got := w.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("Accept-Encoding %q: Content-Encoding = %q", accept, got)
		}
		if w.Body.String() != body {
			t.Errorf("Accept-Encoding %q: the body changed", accept)
		}
		if got := w.Header().Get("Vary"); got != "Accept-Encoding" {
			t.Errorf("Accept-Encoding %q: Vary = %q", accept, got)
		}
	}
	if w := serveGzip(t, "/api/issues", "*", "application/json", body); w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Accept-Encoding * should allow gzip")
	}
}

func TestGzipNotForSmallAnswers(t *testing.T) {
	body := strings.Repeat("a", web.GzipMinSize-1)
	w := serveGzip(t, "/api/me", "gzip", "application/json", body)
	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q", got)
	}
	if w.Body.String() != body {
		t.Error("the body changed")
	}
	if got := w.Header().Get("Content-Length"); got != strconv.Itoa(len(body)) {
		t.Errorf("Content-Length = %q", got)
	}
	// Without a declared length, the first write is held back, then sent.
	h := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, "{}")
	}))
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusCreated || rec.Body.String() != "{}" || rec.Header().Get("Content-Encoding") != "" {
		t.Errorf("small answer: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Encoding"))
	}
}

func TestGzipNotForOtherTypes(t *testing.T) {
	body := strings.Repeat("a", 5000)
	for _, typ := range []string{"image/png", "application/octet-stream", "application/zip", ""} {
		if w := serveGzip(t, "/api/x", "gzip", typ, body); w.Header().Get("Content-Encoding") != "" {
			t.Errorf("%q was compressed", typ)
		}
	}
	for _, typ := range []string{"text/csv; charset=utf-8", "text/html", "text/javascript", "text/css", "image/svg+xml", "text/plain"} {
		if w := serveGzip(t, "/api/x", "gzip", typ, body); w.Header().Get("Content-Encoding") != "gzip" {
			t.Errorf("%q was not compressed", typ)
		}
	}
}

func TestGzipNotForStreamsFilesAndAvatars(t *testing.T) {
	body := strings.Repeat("a", 5000)
	for _, path := range []string{"/api/events", "/api/attachments/3", "/secure/attachment/3/a.txt", "/secure/useravatar"} {
		w := serveGzip(t, path, "gzip", "text/plain", body)
		if w.Header().Get("Content-Encoding") != "" || w.Body.String() != body {
			t.Errorf("%s was compressed", path)
		}
	}
}

// Server-sent events must reach the client as they are written.
func TestGzipEventsFlush(t *testing.T) {
	h := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "retry: 3000\n\n")
		w.(http.Flusher).Flush()
	}))
	r := httptest.NewRequest("GET", "/api/events", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if !w.Flushed || w.Body.String() != "retry: 3000\n\n" || w.Header().Get("Content-Encoding") != "" {
		t.Errorf("flushed=%v body=%q", w.Flushed, w.Body.String())
	}
}

func TestGzipKeepsOwnEncoding(t *testing.T) {
	h := gzipHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(bytes.Repeat([]byte("a"), 5000))
	}))
	r := httptest.NewRequest("GET", "/app.js", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Body.Len() != 5000 {
		t.Errorf("a body already encoded was compressed again (%d bytes)", w.Body.Len())
	}
}
