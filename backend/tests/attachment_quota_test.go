package tests

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
)

// uploadText posts one file as a multipart attachment upload, and returns
// the status and the answer.
func uploadText(t *testing.T, c *client, key, name, content string) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(fw, content); err != nil {
		t.Fatal(err)
	}
	mw.Close()
	req, err := http.NewRequest("POST", c.base+"/api/issues/"+key+"/attachments", &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(csrfHeader, "1")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

// Once the attachments hold their whole storage quota, further uploads are
// refused; without a quota (the default), they are accepted.
func TestAttachmentStorageQuota(t *testing.T) {
	needDB(t)
	srv, base := newServer(t)
	srv.SetMaxStorage(1) // one byte: the first file passes, and fills it
	paul := loginAt(t, base, "paul.martin", "motdepasse")
	key := createIssue(t, paul, "DEMO", unique("quota"), "")

	if status, body := uploadText(t, paul, key, "first.txt", "petit"); status != 200 {
		t.Fatalf("first upload: %d %s, want 200", status, body)
	}
	status, body := uploadText(t, paul, key, "second.txt", "encore")
	if status != 507 {
		t.Fatalf("second upload: %d %s, want 507", status, body)
	}
	if msg := str(obj(t, body), "error"); msg != "The attachment storage quota is reached." {
		t.Errorf("second upload: %q", msg)
	}

	// A server without a quota accepts both.
	free := loginAt(t, ownServer(t), "paul.martin", "motdepasse")
	other := createIssue(t, free, "DEMO", unique("quota"), "")
	for _, name := range []string{"one.txt", "two.txt"} {
		if status, body := uploadText(t, free, other, name, "sans limite"); status != 200 {
			t.Fatalf("upload %s without a quota: %d %s, want 200", name, status, body)
		}
	}
}
