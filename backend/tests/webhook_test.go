package tests

// Webhook deliveries: payload and signature, the JQL filter, retries and
// giving up, test sends, and the guard against metadata addresses.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"solstice/internal/events"
	"solstice/internal/mail/mailtest"
	"solstice/internal/secret"
	"solstice/internal/webhook"
)

// receiver records what webhooks receive and answers with a chosen status.
type receiver struct {
	mu     sync.Mutex
	status int
	got    []received
}

type received struct {
	header http.Header
	body   []byte
	path   string
}

func (rc *receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rc.mu.Lock()
	rc.got = append(rc.got, received{r.Header.Clone(), body, r.URL.Path})
	status := rc.status
	rc.mu.Unlock()
	w.WriteHeader(status)
}

func (rc *receiver) all() []received {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return append([]received(nil), rc.got...)
}

func TestDeliveries(t *testing.T) {
	needDB(t)
	conn := testDB
	mailtest.Lock(t, conn)
	ctx := context.Background()
	box, _ := secret.New(make([]byte, 32))
	// The stub matcher accepts issues whose JQL names their key.
	var issue *events.Issue
	match := func(_ context.Context, jql string, id int64) (bool, error) {
		return strings.Contains(jql, issue.Key), nil
	}
	s := webhook.New(conn, quiet(), func() (*secret.Box, error) { return box, nil }, match)
	s.BaseURL = func() string { return "https://tickets.exemple.fr" }
	clock := time.Now()
	s.Now = func() time.Time { return clock }

	rc := &receiver{status: 200}
	srv := httptest.NewServer(rc)
	defer srv.Close()

	var id int64
	if err := conn.QueryRow(`SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' ORDER BY i.num LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	var err error
	if issue, err = events.LoadIssue(ctx, conn, id); err != nil {
		t.Fatal(err)
	}
	secretText := "chut"
	signed := &webhook.Webhook{Name: "test-signed", URL: srv.URL + "/hook/${issue.key}", Events: []string{webhook.IssueUpdated, webhook.CommentCreated},
		Enabled: true}
	filtered := &webhook.Webhook{Name: "test-filtered", URL: srv.URL + "/other", Events: []string{webhook.IssueUpdated},
		JQL: "key = NOPE-1", Enabled: true}
	for _, w := range []*webhook.Webhook{signed, filtered} {
		var sec *string
		if w == signed {
			sec = &secretText
		}
		if err := s.Save(ctx, w, sec, 0); err != nil {
			t.Fatal(err)
		}
		defer s.Delete(ctx, w.ID)
	}
	ev := &events.Event{Kind: events.IssueUpdated, At: clock, IssueID: issue.ID, Key: issue.Key, Issue: issue,
		Changes: []events.Change{{Field: "status", From: events.Str("1"), FromString: events.Str("To do"),
			To: events.Str("3"), ToString: events.Str("In progress")}}}
	if !s.Wants(ev) {
		t.Fatal("an enabled webhook listens, but Wants is false")
	}
	if err := s.Handle(ctx, ev); err != nil {
		t.Fatal(err)
	}
	if n, err := s.DeliverDue(ctx); err != nil || n != 1 {
		t.Fatalf("DeliverDue: %d, %v (the JQL filter must stop the second webhook)", n, err)
	}
	got := rc.all()
	if len(got) != 1 || got[0].path != "/hook/"+issue.Key {
		t.Fatalf("received %+v", got)
	}
	mac := hmac.New(sha256.New, []byte(secretText))
	mac.Write(got[0].body)
	if sig := got[0].header.Get("X-Hub-Signature"); sig != "sha256="+hex.EncodeToString(mac.Sum(nil)) {
		t.Errorf("signature %q does not match the body", sig)
	}
	var p struct {
		WebhookEvent string `json:"webhookEvent"`
		Timestamp    int64  `json:"timestamp"`
		EventType    string `json:"issue_event_type_name"`
		Issue        struct {
			ID, Key, Self string
			Fields        struct {
				Summary string
				Status  struct {
					Name           string
					StatusCategory struct{ Key string }
				}
				Project struct{ Key string }
			}
		}
		Changelog struct {
			Items []struct{ Field, FromString, ToString string }
		}
	}
	if err := json.Unmarshal(got[0].body, &p); err != nil {
		t.Fatal(err)
	}
	if p.WebhookEvent != webhook.IssueUpdated || p.EventType != "issue_generic" || p.Timestamp != clock.UnixMilli() ||
		p.Issue.Key != issue.Key || p.Issue.Fields.Summary != issue.Summary || p.Issue.Fields.Project.Key != "DEMO" ||
		p.Issue.Self != "https://tickets.exemple.fr/rest/api/2/issue/"+p.Issue.ID ||
		len(p.Changelog.Items) != 1 || p.Changelog.Items[0].ToString != "In progress" || p.Issue.Fields.Status.StatusCategory.Key == "" {
		t.Errorf("payload: %s", got[0].body)
	}

	// The receiver fails: retried after a delay, then given up.
	rc.mu.Lock()
	rc.status = 503
	rc.mu.Unlock()
	s.Handle(ctx, ev)
	s.DeliverDue(ctx)
	log, _ := s.Deliveries(ctx, signed.ID, 10)
	if log[0].Status != "pending" || log[0].Attempts != 1 || log[0].StatusCode == nil || *log[0].StatusCode != 503 ||
		log[0].NextAttempt == nil || !log[0].NextAttempt.After(clock) {
		t.Fatalf("after a 503: %+v", log[0])
	}
	if n, _ := s.DeliverDue(ctx); n != 0 || len(rc.all()) != 2 {
		t.Fatal("retried before the delay")
	}
	for i := 0; i < webhook.MaxAttempts; i++ {
		clock = clock.Add(3 * time.Hour)
		s.DeliverDue(ctx)
	}
	log, _ = s.Deliveries(ctx, signed.ID, 10)
	if log[0].Status != "failed" || log[0].Attempts != webhook.MaxAttempts {
		t.Errorf("after %d failures: %+v", webhook.MaxAttempts, log[0])
	}

	// A 4xx is final at once.
	rc.mu.Lock()
	rc.status = 400
	rc.mu.Unlock()
	s.Handle(ctx, ev)
	s.DeliverDue(ctx)
	log, _ = s.Deliveries(ctx, signed.ID, 1)
	if log[0].Status != "failed" || log[0].Attempts != 1 {
		t.Errorf("after a 400: %+v", log[0])
	}

	// Test sends now, without retry.
	rc.mu.Lock()
	rc.status = 204
	rc.mu.Unlock()
	d, err := s.Test(ctx, signed.ID, 1)
	if err != nil || d.Status != "sent" || d.Event != webhook.TestEvent || d.StatusCode == nil || *d.StatusCode != 204 {
		t.Errorf("test delivery: %+v, %v", d, err)
	}

	// A webhook aimed at a metadata address never connects.
	meta := &webhook.Webhook{Name: "test-meta", URL: "http://169.254.169.254/latest", Events: []string{webhook.IssueUpdated}, Enabled: true}
	if err := s.Save(ctx, meta, nil, 0); err != nil {
		t.Fatal(err)
	}
	defer s.Delete(ctx, meta.ID)
	s.Handle(ctx, ev)
	s.DeliverDue(ctx)
	log, _ = s.Deliveries(ctx, meta.ID, 1)
	if len(log) != 1 || log[0].Status != "failed" || log[0].Error == nil || !strings.Contains(*log[0].Error, "refused") {
		t.Errorf("delivery to a metadata address: %+v", log)
	}
}
