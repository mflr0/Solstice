package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// An error that is not meant for the user never reaches the client: the
// answer is the generic message, in the request's language.
func TestWriteErrorHidesInternalErrors(t *testing.T) {
	secret := func(http.ResponseWriter, *http.Request) error {
		return errors.New("dial tcp 10.0.0.5:3306: connection refused")
	}
	s := &Server{log: slog.New(slog.DiscardHandler)}
	for name, h := range map[string]http.Handler{
		"interface": s.wrap(secret, false), "REST": s.rest(secret, false),
		"setup wizard": (&Setup{log: s.log}).wrap(secret, false),
	} {
		r := httptest.NewRequest("GET", "/api/anything", nil)
		r.Header.Set("Accept-Language", "fr")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "10.0.0.5") ||
			!strings.Contains(w.Body.String(), "Erreur interne") {
			t.Errorf("%s: %d %s", name, w.Code, w.Body)
		}
	}
}

// The log of an unexpected error names the route, never the path, which can
// hold a secret (a reset token).
func TestWriteErrorLogsRouteNotPath(t *testing.T) {
	var logged bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, nil))
	s := &Server{log: log}
	boom := func(http.ResponseWriter, *http.Request) error { return errors.New("boom") }
	for name, h := range map[string]http.Handler{
		"interface": s.wrap(boom, false), "REST": s.rest(boom, false), "setup wizard": (&Setup{log: log}).wrap(boom, false),
	} {
		logged.Reset()
		r := httptest.NewRequest("GET", "/api/auth/reset/s3cret-token", nil)
		r.Pattern = "GET /api/auth/reset/{token}"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "boom") {
			t.Errorf("%s: %d %s", name, w.Code, w.Body)
		}
		if out := logged.String(); strings.Contains(out, "s3cret-token") || !strings.Contains(out, "/api/auth/reset/{token}") ||
			!strings.Contains(out, "boom") {
			t.Errorf("%s log: %s", name, out)
		}
	}
}

// lockOut makes the address of r count five failures for key.
func lockOut(srv *Server, key string) {
	for range 5 {
		srv.limiter.Failure(key)
	}
}

// A 429 tells how long to wait, in a header and in the body.
func TestTooManyAttemptsRetryAfter(t *testing.T) {
	srv := quietServer()
	r := fromAddress("198.51.100.7")
	userKey, ipKey := srv.throttleKeys(r, "hugo.robert")
	lockOut(srv, ipKey)
	lockOut(srv, userKey)

	// An interface route.
	login := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"username":"hugo.robert","password":"x"}`))
	login.RemoteAddr = r.RemoteAddr
	login.Header.Set(csrfHeader, "1")
	w := httptest.NewRecorder()
	srv.wrap(srv.login, false).ServeHTTP(w, login)
	var body struct {
		Error      string `json:"error"`
		RetryAfter int    `json:"retryAfter"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusTooManyRequests || body.RetryAfter < 1 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("login: %d %v %s", w.Code, w.Header(), w.Body)
	}
	if got, want := w.Header().Get("Retry-After"), strconv.Itoa(body.RetryAfter); got != want {
		t.Errorf("login Retry-After %q, retryAfter %s", got, want)
	}

	// A REST route.
	get := httptest.NewRequest("GET", "/rest/api/2/myself", nil)
	get.RemoteAddr = r.RemoteAddr
	get.SetBasicAuth("hugo.robert", "x")
	w = httptest.NewRecorder()
	srv.rest(func(http.ResponseWriter, *http.Request) error { return nil }, true).ServeHTTP(w, get)
	var rest struct {
		RetryAfter int `json:"retryAfter"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rest); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusTooManyRequests || rest.RetryAfter < 1 || w.Header().Get("Retry-After") != strconv.Itoa(rest.RetryAfter) {
		t.Fatalf("REST: %d %v %s", w.Code, w.Header(), w.Body)
	}
}

// A JSON body over the size limit is refused with 413, in the request's
// language, not with 400.
func TestOversizedJSONBody(t *testing.T) {
	srv := quietServer()
	big := `{"username":"` + strings.Repeat("a", 4<<20) + `"}`

	post := func(h http.Handler) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/x", strings.NewReader(big))
		r.Header.Set(csrfHeader, "1")
		r.Header.Set("Accept-Language", "fr")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	// An interface route (readJSON).
	if w := post(srv.wrap(srv.login, false)); w.Code != http.StatusRequestEntityTooLarge ||
		!strings.Contains(w.Body.String(), "trop volumineuse (4096 Kio au maximum)") {
		t.Errorf("interface: %d %s", w.Code, w.Body)
	}
	// A REST route (readRESTJSON).
	read := func(w http.ResponseWriter, r *http.Request) error {
		var v map[string]any
		return readRESTJSON(r, &v)
	}
	if w := post(srv.rest(read, false)); w.Code != http.StatusRequestEntityTooLarge ||
		!strings.Contains(w.Body.String(), "trop volumineuse (4096 Kio au maximum)") {
		t.Errorf("REST: %d %s", w.Code, w.Body)
	}
	// The message gives the limit of the route that refused the body.
	small := httptest.NewRecorder()
	_, err := io.ReadAll(http.MaxBytesReader(small, io.NopCloser(strings.NewReader(strings.Repeat("a", 1<<17))), 1<<16))
	writeError(small, httptest.NewRequest("POST", "/x", nil), badBody(err))
	if small.Code != http.StatusRequestEntityTooLarge || !strings.Contains(small.Body.String(), "64 KiB") {
		t.Errorf("64 KiB limit: %d %s", small.Code, small.Body)
	}
	// A body within the limit but not JSON is still 400.
	r := httptest.NewRequest("POST", "/x", strings.NewReader("{"))
	r.Header.Set(csrfHeader, "1")
	w := httptest.NewRecorder()
	srv.wrap(srv.login, false).ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("malformed: %d %s", w.Code, w.Body)
	}
}
