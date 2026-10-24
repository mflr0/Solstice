package api

import (
	"compress/gzip"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"solstice/internal/web"
)

// gzipPool keeps the writers: one allocates about 800 KB of tables.
var gzipPool = sync.Pool{New: func() any {
	zw, _ := gzip.NewWriterLevel(nil, 5)
	return zw
}}

// gzipSkip tells whether a path is left alone: live updates must flush as
// they go, files and avatars are already compressed or sandboxed and answer
// ranges.
func gzipSkip(path string) bool {
	return path == "/api/events" || strings.HasPrefix(path, "/api/attachments/") ||
		strings.HasPrefix(path, "/secure/")
}

// gzipHandler compresses the answers of next for clients that accept gzip:
// text types of at least web.GzipMinSize bytes, without a Content-Encoding of
// their own.
func gzipHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gzipSkip(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Add("Vary", "Accept-Encoding")
		if r.Method == http.MethodHead || r.Header.Get("Range") != "" || !web.AcceptsGzip(r.Header.Get("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipWriter{ResponseWriter: w}
		defer gw.finish()
		next.ServeHTTP(gw, r)
	})
}

// compressible tells whether a Content-Type names text that gzip shrinks.
func compressible(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	switch mediaType {
	case "application/json", "application/javascript", "application/xml", "image/svg+xml":
		return true
	}
	return strings.HasPrefix(mediaType, "text/") || strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml")
}

// gzipWriter holds the first web.GzipMinSize bytes of an answer back until it
// knows whether to compress it.
type gzipWriter struct {
	http.ResponseWriter
	status  int    // 0 until WriteHeader
	pending []byte // held back until the choice is made
	decided bool
	zw      *gzip.Writer // nil when the answer goes out as it is
}

func (g *gzipWriter) WriteHeader(status int) {
	if status < 200 {
		g.ResponseWriter.WriteHeader(status) // informational: the answer is still to come
		return
	}
	if g.status == 0 {
		g.status = status
	}
}

func (g *gzipWriter) Write(p []byte) (int, error) {
	if g.decided {
		if g.zw != nil {
			return g.zw.Write(p)
		}
		return g.ResponseWriter.Write(p)
	}
	h := g.Header()
	if n, err := strconv.Atoi(h.Get("Content-Length")); err == nil && n < web.GzipMinSize {
		g.passThrough()
		return g.ResponseWriter.Write(p)
	}
	g.pending = append(g.pending, p...)
	if len(g.pending) < web.GzipMinSize {
		return len(p), nil
	}
	g.start()
	return len(p), g.flushPending()
}

// start chooses to compress, if the answer is of a kind that allows it.
func (g *gzipWriter) start() {
	h := g.Header()
	status := g.status
	if status == 0 {
		status = http.StatusOK
	}
	if h.Get("Content-Encoding") != "" || !compressible(h.Get("Content-Type")) ||
		status == http.StatusNoContent || status == http.StatusNotModified || status == http.StatusPartialContent {
		g.passThrough()
		return
	}
	h.Del("Content-Length")
	h.Set("Content-Encoding", "gzip")
	if etag := h.Get("ETag"); etag != "" && !strings.HasPrefix(etag, "W/") {
		h.Set("ETag", "W/"+etag)
	}
	g.decided = true
	g.zw = gzipPool.Get().(*gzip.Writer)
	g.zw.Reset(g.ResponseWriter)
	g.sendHeader()
}

// passThrough chooses to send the answer as it is.
func (g *gzipWriter) passThrough() {
	g.decided = true
	g.sendHeader()
}

func (g *gzipWriter) sendHeader() {
	if g.status != 0 {
		g.ResponseWriter.WriteHeader(g.status)
	}
}

func (g *gzipWriter) flushPending() error {
	if len(g.pending) == 0 {
		return nil
	}
	p := g.pending
	g.pending = nil
	var err error
	if g.zw != nil {
		_, err = g.zw.Write(p)
	} else {
		_, err = g.ResponseWriter.Write(p)
	}
	return err
}

// Flush sends what is held, as it is when it is too short to compress.
func (g *gzipWriter) Flush() {
	if !g.decided {
		if len(g.pending) >= web.GzipMinSize {
			g.start()
		} else {
			g.passThrough()
		}
		g.flushPending()
	}
	if g.zw != nil {
		g.zw.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (g *gzipWriter) Unwrap() http.ResponseWriter { return g.ResponseWriter }

// finish ends the answer: what was held back is too short to compress, and
// a compressed stream gets its trailer.
func (g *gzipWriter) finish() {
	if !g.decided {
		g.passThrough()
		g.flushPending()
		return
	}
	if g.zw != nil {
		g.zw.Close()
		gzipPool.Put(g.zw)
		g.zw = nil
	}
}
