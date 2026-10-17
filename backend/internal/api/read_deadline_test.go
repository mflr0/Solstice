package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// A body sent at once goes through; one that stops mid-way past the read
// deadline loses the connection.
func TestSlowBodyIsCut(t *testing.T) {
	read := make(chan error, 2)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setBodyReadDeadline(w, time.Now().Add(100*time.Millisecond))
		_, err := io.ReadAll(r.Body)
		read <- err
	}))
	defer ts.Close()

	if resp, err := ts.Client().Post(ts.URL, "text/plain", strings.NewReader("sent at once")); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
	}
	if err := <-read; err != nil {
		t.Fatalf("a quick body: %v", err)
	}

	pr, pw := io.Pipe()
	go func() {
		io.WriteString(pw, "start")
		time.Sleep(300 * time.Millisecond) // longer than the deadline
		io.WriteString(pw, ", never sent")
		pw.Close()
	}()
	if resp, err := ts.Client().Post(ts.URL, "text/plain", pr); err == nil {
		resp.Body.Close()
	}
	if err := <-read; !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("a slow body: %v, want a deadline error", err)
	}
}

// A handler that has read its whole body keeps working past the body read
// deadline: net/http clears the deadline when the body hits EOF.
func TestDeadlineClearedOnceBodyIsRead(t *testing.T) {
	done := make(chan error, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setBodyReadDeadline(w, time.Now().Add(100*time.Millisecond)) // as wrap does
		if _, err := io.ReadAll(r.Body); err != nil {
			done <- err
			return
		}
		time.Sleep(300 * time.Millisecond) // longer than the deadline
		done <- r.Context().Err()
	}))
	defer ts.Close()

	if resp, err := ts.Client().Post(ts.URL, "text/plain", strings.NewReader("sent at once")); err != nil {
		t.Fatal(err)
	} else {
		resp.Body.Close()
	}
	if err := <-done; err != nil {
		t.Fatalf("the request was cancelled after its body was read: %v", err)
	}
}
