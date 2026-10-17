package tests

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// uploadText posts one file as a multipart attachment upload, and returns
// the status and the answer.
func uploadText(t *testing.T, c *client, key, name, content string) (int, []byte) {
	t.Helper()
	return uploadTexts(t, c, key, [2]string{name, content})
}

// uploadTexts posts several files (name, content) in one multipart request.
func uploadTexts(t *testing.T, c *client, key string, files ...[2]string) (int, []byte) {
	t.Helper()
	return postFiles(t, c, key, false, files...)
}

// postFiles posts the files; chunked sends the body without a length, so the
// server cannot tell its size beforehand.
func postFiles(t *testing.T, c *client, key string, chunked bool, files ...[2]string) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, f := range files {
		fw, err := mw.CreateFormFile("file", f[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(fw, f[1]); err != nil {
			t.Fatal(err)
		}
	}
	mw.Close()
	var body io.Reader = &buf
	if chunked {
		body = io.MultiReader(&buf)
	}
	req, err := http.NewRequest("POST", c.base+"/api/issues/"+key+"/attachments", body)
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
	answer, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, answer
}

// storedFiles counts the attachments of an issue.
func storedFiles(t *testing.T, key string) string {
	t.Helper()
	return pick(t, `SELECT COUNT(*) FROM attachments WHERE issue_id = ?`, issueID(t, key))
}

// storedBytes is the size of all the attachments in the database, which is
// what the quota counts.
func storedBytes(t *testing.T) int64 {
	t.Helper()
	n, err := strconv.ParseInt(pick(t, `SELECT COALESCE(SUM(size), 0) FROM attachments`), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// Once the attachments hold their whole storage quota, further uploads are
// refused; without a quota (the default), they are accepted.
func TestAttachmentStorageQuota(t *testing.T) {
	needDB(t)
	srv, base := newServer(t)
	srv.SetMaxStorage(storedBytes(t) + 1000)
	paul := loginAt(t, base, "paul.martin", "motdepasse")
	key := createIssue(t, paul, "DEMO", unique("quota"), "")

	if status, body := uploadText(t, paul, key, "first.txt", strings.Repeat("a", 600)); status != 200 {
		t.Fatalf("first upload: %d %s, want 200", status, body)
	}
	status, body := uploadText(t, paul, key, "second.txt", strings.Repeat("b", 600))
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

// A request that cannot fit in the quota is refused whole: the files before
// the one that does not fit are not stored either.
func TestAttachmentQuotaRefusesWholeRequest(t *testing.T) {
	needDB(t)
	srv, base := newServer(t)
	srv.SetMaxStorage(storedBytes(t) + 1000)
	paul := loginAt(t, base, "paul.martin", "motdepasse")
	key := createIssue(t, paul, "DEMO", unique("quota"), "")

	status, body := uploadTexts(t, paul, key,
		[2]string{"one.txt", strings.Repeat("a", 600)}, [2]string{"two.txt", strings.Repeat("b", 600)})
	if status != 507 {
		t.Fatalf("two files over the quota: %d %s, want 507", status, body)
	}
	if n := storedFiles(t, key); n != "0" {
		t.Errorf("%s files stored, want none", n)
	}
	var history []historyEntry
	paul.get("/api/issues/"+key+"/history", &history)
	if historyHas(history, "Attachment") {
		t.Errorf("history records the refused upload: %+v", history)
	}
}

// A file cannot take the storage past the quota, even when the request does
// not say how long it is.
func TestAttachmentQuotaStopsAnOversizedPart(t *testing.T) {
	needDB(t)
	srv, base := newServer(t)
	srv.SetMaxStorage(storedBytes(t) + 1000)
	paul := loginAt(t, base, "paul.martin", "motdepasse")
	key := createIssue(t, paul, "DEMO", unique("quota"), "")

	status, body := postFiles(t, paul, key, true, [2]string{"big.txt", strings.Repeat("a", 1500)})
	if status != 507 {
		t.Fatalf("a file over the quota: %d %s, want 507", status, body)
	}
	if msg := str(obj(t, body), "error"); msg != "The attachment storage quota is reached." {
		t.Errorf("a file over the quota: %q", msg)
	}
	if n := storedFiles(t, key); n != "0" {
		t.Errorf("%s files stored, want none", n)
	}
	if status, body := postFiles(t, paul, key, true, [2]string{"small.txt", strings.Repeat("a", 900)}); status != 200 {
		t.Fatalf("a file within the quota: %d %s, want 200", status, body)
	}
}
