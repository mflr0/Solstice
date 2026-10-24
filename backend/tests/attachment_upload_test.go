package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/jirafixture"
)

// A later file over the upload limit takes the earlier ones of the request
// with it.
func TestAttachmentUploadFailureRemovesEarlierFiles(t *testing.T) {
	needDB(t)
	dataDir := t.TempDir()
	srv := api.New(testDB, dataDir, time.UTC, quiet())
	srv.SetMaxUpload(64 << 10)
	ts := httptest.NewServer(srv.Handler(http.NotFoundHandler()))
	t.Cleanup(ts.Close)
	paul := loginAt(t, ts.URL, "paul.martin", jirafixture.Password)
	key := createIssue(t, paul, "DEMO", unique("upload"), "")

	status, body := uploadTexts(t, paul, key,
		[2]string{"small.txt", strings.Repeat("a", 10<<10)}, [2]string{"big.txt", strings.Repeat("b", 100<<10)})
	if status != 413 {
		t.Fatalf("upload over the limit: %d %s, want 413", status, body)
	}
	if msg := str(obj(t, body), "error"); msg != "File too large (100 MB at most)." {
		t.Errorf("upload over the limit: %q", msg)
	}
	if n := storedFiles(t, key); n != "0" {
		t.Errorf("%s attachment rows left, want none", n)
	}
	left, err := os.ReadDir(filepath.Join(dataDir, "attachments"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range left {
		t.Errorf("file left on disk: %s", f.Name())
	}
	var history []historyEntry
	paul.get("/api/issues/"+key+"/history", &history)
	if historyHas(history, "Attachment") {
		t.Errorf("history records the failed upload: %+v", history)
	}
}
