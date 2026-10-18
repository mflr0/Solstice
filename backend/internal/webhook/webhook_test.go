package webhook

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"solstice/internal/events"
	"strings"
	"testing"
	"time"
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

// A changed or deleted work log fires Jira's events, and is also an issue
// update.
func TestWorklogEvents(t *testing.T) {
	for kind, want := range map[events.Kind][]string{
		events.WorklogUpdated: {WorklogUpdated, IssueUpdated},
		events.WorklogDeleted: {WorklogDeleted, IssueUpdated},
	} {
		got := namesOf(kind)
		if !slices.Equal(got, want) {
			t.Errorf("namesOf(%s) = %v, want %v", kind, got, want)
		}
		for _, name := range want {
			if !slices.Contains(Events, name) {
				t.Errorf("%s is not in the list of events", name)
			}
		}
	}
	if got := issueEventType(&events.Event{Kind: events.WorklogUpdated}); got != "issue_worklog_updated" {
		t.Errorf("updated work log: %s", got)
	}
	if got := issueEventType(&events.Event{Kind: events.WorklogDeleted}); got != "issue_worklog_deleted" {
		t.Errorf("deleted work log: %s", got)
	}
	b := &builder{base: "https://solstice.example", users: map[int64]*events.User{7: {ID: 7, Username: "marie.durand"}}}
	created := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	w := b.worklog(12, &events.Worklog{ID: 3, AuthorID: 7, Seconds: 3600, Created: created})
	if w["updated"] != jtime(created) {
		t.Errorf("a work log never changed was updated at %v", w["updated"])
	}
	changed := created.Add(time.Hour)
	w = b.worklog(12, &events.Worklog{ID: 3, AuthorID: 7, UpdatedBy: 7, Seconds: 7200, Created: created, Updated: changed})
	if w["updated"] != jtime(changed) || w["timeSpentSeconds"] != int64(7200) {
		t.Errorf("a changed work log: %v", w)
	}
}

func TestIssueMovedEventType(t *testing.T) {
	moved := &events.Event{Kind: events.IssueUpdated, Changes: []events.Change{
		{Field: "Key"}, {Field: "project"}, {Field: "resolution", To: new(string)}}}
	if got := issueEventType(moved); got != "issue_moved" {
		t.Errorf("a move: %s", got)
	}
	edited := &events.Event{Kind: events.IssueUpdated, Changes: []events.Change{{Field: "summary"}}}
	if got := issueEventType(edited); got != "issue_updated" {
		t.Errorf("an edit: %s", got)
	}
}

// A sprint that is changed or deleted fires its own event.
func TestSprintEvents(t *testing.T) {
	for kind, want := range map[events.Kind]string{events.SprintUpdated: SprintUpdated, events.SprintDeleted: SprintDeleted} {
		if got := namesOf(kind); !slices.Equal(got, []string{want}) {
			t.Errorf("namesOf(%s) = %v, want [%s]", kind, got, want)
		}
		if !slices.Contains(Events, want) {
			t.Errorf("%s is not in the list of events", want)
		}
	}
}
