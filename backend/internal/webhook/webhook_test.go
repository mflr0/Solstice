package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"solstice/internal/db"
	"solstice/internal/events"
	"solstice/internal/mail/mailtest"
	"solstice/internal/secret"
)

func TestBlocked(t *testing.T) {
	for addr, want := range map[string]bool{
		"169.254.169.254":        true, // AWS, GCP, Azure, Oracle metadata
		"169.254.170.2":          true, // ECS task metadata
		"100.100.100.200":        true, // Alibaba metadata
		"fd00:ec2::254":          true, // AWS metadata over IPv6
		"fe80::1":                true,
		"fe80::1%eth0":           true, // zoned: still link-local
		"::ffff:169.254.169.254": true, // IPv4-mapped
		"64:ff9b::a9fe:a9fe":     true, // NAT64
		"::a9fe:a9fe":            true, // IPv4-compatible
		"0.0.0.0":                true,
		"::":                     true,
		"224.0.0.1":              true,
		"ff02::1":                true,
		"255.255.255.255":        true,
		"127.0.0.1":              false, // the local host: allowed (internal services)
		"::1":                    false,
		"10.1.2.3":               false, // private networks: allowed
		"192.168.1.10":           false,
		"172.16.0.5":             false,
		"fd12:3456::1":           false,
		"::ffff:10.0.0.1":        false,
		"64:ff9b::7f00:1":        false,
		"8.8.8.8":                false,
		"2001:4860:4860::8888":   false,
		"169.253.255.255":        false,
		"169.255.0.0":            false,
		"fd00:ec2::253":          false,
	} {
		ip, err := netip.ParseAddr(addr)
		if err != nil {
			t.Fatal(err)
		}
		if got := Blocked(ip); got != want {
			t.Errorf("Blocked(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestCheckURL(t *testing.T) {
	ctx := context.Background()
	for raw, want := range map[string]error{
		"https://hooks.exemple.fr/solstice":        nil,
		"http://10.0.0.5:8080/jira-hook":           nil,
		"http://localhost:9000/x":                  nil,
		"https://exemple.fr/hook/${issue.key}?p=1": nil,
		"http://169.254.169.254/latest/meta-data":  ErrBlocked,
		"http://[fd00:ec2::254]/latest":            ErrBlocked,
		"http://[::ffff:169.254.169.254]/":         ErrBlocked,
		"http://0.0.0.0:80/":                       ErrBlocked,
		"file:///etc/passwd":                       ErrScheme,
		"gopher://exemple.fr/":                     ErrScheme,
		"ftp://exemple.fr/":                        ErrScheme,
		"javascript:alert(1)":                      ErrScheme,
		"exemple.fr/hook":                          ErrScheme,
		"https:///nohost":                          ErrHost,
	} {
		if got := CheckURL(ctx, raw); !errors.Is(got, want) {
			t.Errorf("CheckURL(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestClientRefusesAtConnection(t *testing.T) {
	c := guardedClient()
	for _, target := range []string{"http://169.254.169.254/latest/meta-data/", "http://[fd00:ec2::254]/"} {
		if strings.Contains(target, "[") && !hasIPv6() {
			t.Logf("skipping %s: this machine has no IPv6", target)
			continue
		}
		_, err := c.Get(target)
		var be *blockedError
		if !errors.As(err, &be) {
			t.Errorf("GET %s: %v, want refused by the guard", target, err)
		}
	}
	// A name resolving to the local host is fine (internal integrations).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	resp, err := c.Get(strings.Replace(srv.URL, "127.0.0.1", "localhost", 1))
	if err != nil {
		t.Fatalf("local receiver: %v", err)
	}
	resp.Body.Close()
}

func TestJiraDuration(t *testing.T) {
	for s, want := range map[int64]string{0: "0m", 60: "1m", 5400: "1h 30m", 8 * 3600: "1d", 8*3600*5 + 3600: "1w 1h"} {
		if got := jiraDuration(s); got != want {
			t.Errorf("jiraDuration(%d) = %q, want %q", s, got, want)
		}
	}
}

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

func testDB(t *testing.T) *sql.DB {
	dsn := os.Getenv("SOLSTICE_TEST_DB")
	if dsn == "" {
		t.Skip("SOLSTICE_TEST_DB not set: integration test skipped")
	}
	conn, err := db.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestDeliveries(t *testing.T) {
	conn := testDB(t)
	mailtest.Lock(t, conn)
	ctx := context.Background()
	box, _ := secret.New(make([]byte, 32))
	// The stub matcher accepts issues whose JQL names their key.
	var issue *events.Issue
	match := func(_ context.Context, jql string, id int64) (bool, error) {
		return strings.Contains(jql, issue.Key), nil
	}
	s := New(conn, slog.New(slog.NewTextHandler(io.Discard, nil)), func() (*secret.Box, error) { return box, nil }, match)
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
	signed := &Webhook{Name: "test-signed", URL: srv.URL + "/hook/${issue.key}", Events: []string{IssueUpdated, CommentCreated},
		Enabled: true}
	filtered := &Webhook{Name: "test-filtered", URL: srv.URL + "/other", Events: []string{IssueUpdated},
		JQL: "key = NOPE-1", Enabled: true}
	for _, w := range []*Webhook{signed, filtered} {
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
	if p.WebhookEvent != IssueUpdated || p.EventType != "issue_generic" || p.Timestamp != clock.UnixMilli() ||
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
	for i := 0; i < MaxAttempts; i++ {
		clock = clock.Add(3 * time.Hour)
		s.DeliverDue(ctx)
	}
	log, _ = s.Deliveries(ctx, signed.ID, 10)
	if log[0].Status != "failed" || log[0].Attempts != MaxAttempts {
		t.Errorf("after %d failures: %+v", MaxAttempts, log[0])
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
	if err != nil || d.Status != "sent" || d.Event != TestEvent || d.StatusCode == nil || *d.StatusCode != 204 {
		t.Errorf("test delivery: %+v, %v", d, err)
	}

	// A webhook aimed at a metadata address never connects.
	meta := &Webhook{Name: "test-meta", URL: "http://169.254.169.254/latest", Events: []string{IssueUpdated}, Enabled: true}
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

// hasIPv6 tells whether this machine can open IPv6 sockets.
func hasIPv6() bool {
	l, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// While a replacing import runs, a delivery tick does nothing: it does not
// touch the database (there is none here, so touching it would panic).
func TestDeliverDueSkippedWhilePaused(t *testing.T) {
	s := New(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	s.Paused = func() bool { return true }
	if n, err := s.DeliverDue(context.Background()); n != 0 || err != nil {
		t.Fatalf("paused: delivered %d, err %v", n, err)
	}
	s.Paused = func() bool { return false }
	defer func() {
		if recover() == nil {
			t.Error("not paused: the tick did not reach the database")
		}
	}()
	s.DeliverDue(context.Background())
}
