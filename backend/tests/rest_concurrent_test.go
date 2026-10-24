package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sync"
	"testing"

	"solstice/internal/jirafixture"
)

// TestRESTConcurrentCreations sends simultaneous work logs and attachments by
// one user to one issue: each answer must name the record of its own request.
func TestRESTConcurrentCreations(t *testing.T) {
	paul := login(t, "paul.martin", jirafixture.Password)
	token := newToken(t, "paul.martin", nil)
	key := createIssue(t, paul, "DEMO", unique("Simultaneous creations "), "")
	send := func(req *http.Request) (int, []byte, error) {
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		return resp.StatusCode, raw, err
	}
	worklog := func(i int) error {
		comment := fmt.Sprintf("log %d", i)
		b, _ := json.Marshal(map[string]any{"timeSpent": "1m", "comment": comment})
		req, _ := http.NewRequest("POST", testURL+"/rest/api/2/issue/"+key+"/worklog", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		status, raw, err := send(req)
		if err != nil {
			return err
		}
		var out struct{ Comment string }
		json.Unmarshal(raw, &out)
		if status != 201 || out.Comment != comment {
			return fmt.Errorf("worklog %q: %d %s", comment, status, raw)
		}
		return nil
	}
	attach := func(i int) error {
		name := fmt.Sprintf("file-%d.txt", i)
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write([]byte(name))
		mw.Close()
		req, _ := http.NewRequest("POST", testURL+"/rest/api/2/issue/"+key+"/attachments", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("X-Atlassian-Token", "no-check")
		status, raw, err := send(req)
		if err != nil {
			return err
		}
		var out []struct{ Filename string }
		json.Unmarshal(raw, &out)
		if status != 200 || len(out) != 1 || out[0].Filename != name {
			return fmt.Errorf("attach %q: %d %s", name, status, raw)
		}
		return nil
	}

	const n = 16
	var wg sync.WaitGroup
	errs := make(chan error, 2*n)
	for i := range n {
		wg.Go(func() { errs <- worklog(i) })
		wg.Go(func() { errs <- attach(i) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}
