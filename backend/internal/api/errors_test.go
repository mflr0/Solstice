package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// An error that is not meant for the user never reaches the client: the
// answer is the generic message, in the request's language.
func TestWriteErrorHidesInternalErrors(t *testing.T) {
	secret := errors.New("dial tcp 10.0.0.5:3306: connection refused")
	r := httptest.NewRequest("GET", "/api/anything", nil)
	r.Header.Set("Accept-Language", "fr")

	w := httptest.NewRecorder()
	writeError(w, r, secret)
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "10.0.0.5") ||
		!strings.Contains(w.Body.String(), "Erreur interne") {
		t.Errorf("interface error: %d %s", w.Code, w.Body)
	}

	w = httptest.NewRecorder()
	writeRESTError(w, r, secret)
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "10.0.0.5") ||
		!strings.Contains(w.Body.String(), "Erreur interne") {
		t.Errorf("REST error: %d %s", w.Code, w.Body)
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
