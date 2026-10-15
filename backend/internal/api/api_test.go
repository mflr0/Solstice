package api

// Integration tests against a Solstice database filled by the synthetic Jira
// import (scripts/e2e-import.sh sets this up and runs them). They exercise
// what must never break: who sees what, old keys, live updates, and the
// basic issue lifecycle.
//
//	SOLSTICE_TEST_DB=mysql://solstice:solstice@localhost:13306/solstice_e2e_pg go test ./internal/api/

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"solstice/internal/db"
)

var (
	testDB  *sql.DB
	testURL string
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("SOLSTICE_TEST_DB")
	if dsn == "" {
		// Unit tests still run; integration tests skip themselves.
		os.Exit(m.Run())
	}
	ctx := context.Background()
	var err error
	testDB, err = db.Open(ctx, dsn)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	dataDir, err := os.MkdirTemp("", "solstice-api-test")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(New(testDB, dataDir, time.UTC, log).Handler(http.NotFoundHandler()))
	testURL = srv.URL
	code := m.Run()
	srv.Close()
	os.RemoveAll(dataDir)
	os.Exit(code)
}

type client struct {
	t    *testing.T
	http *http.Client
}

func anonymous(t *testing.T) *client {
	if testDB == nil {
		t.Skip("SOLSTICE_TEST_DB not set: integration test skipped")
	}
	jar, _ := cookiejar.New(nil)
	return &client{t: t, http: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
}

func login(t *testing.T, username, password string) *client {
	t.Helper()
	c := anonymous(t)
	if status, body := c.do("POST", "/api/login", map[string]string{"username": username, "password": password}); status != 200 {
		t.Fatalf("login %s: %d %s", username, status, body)
	}
	return c
}

func (c *client) do(method, path string, body any) (int, []byte) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, testURL+path, r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeader, "1")
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func (c *client) get(path string, out any) int {
	c.t.Helper()
	status, body := c.do("GET", path, nil)
	if status == 200 && out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			c.t.Fatalf("GET %s: %v\n%s", path, err, body)
		}
	}
	return status
}

// pick runs a query returning one string (an issue key, a user name…).
func pick(t *testing.T, query string, args ...any) string {
	t.Helper()
	if testDB == nil {
		t.Skip("SOLSTICE_TEST_DB not set: integration test skipped")
	}
	var s string
	if err := testDB.QueryRow(query, args...).Scan(&s); err != nil {
		t.Fatalf("fixture lookup failed: %v\n%s", err, query)
	}
	return s
}

const keyExpr = "CONCAT(p.pkey, '-', i.num)"

func TestLogin(t *testing.T) {
	login(t, "admin", "admin")             // Jira hash, admin's own password
	login(t, "marie.durand", "motdepasse") // Jira hash
	for _, bad := range [][2]string{
		{"admin", "Admin"},                // wrong password
		{"karim.benali", ""},              // LDAP user: no password, no application access
		{"emma.morel", "motdepasse"},      // deactivated in Jira
		{"ancien.collegue", "motdepasse"}, // deleted from Jira
	} {
		if status, _ := anonymous(t).do("POST", "/api/login", map[string]string{"username": bad[0], "password": bad[1]}); status != 401 {
			t.Errorf("login %s accepted (%d)", bad[0], status)
		}
	}
	if status := anonymous(t).get("/api/me", nil); status != 401 {
		t.Errorf("anonymous /api/me: %d", status)
	}
}

func TestCSRFHeaderRequired(t *testing.T) {
	c := login(t, "paul.martin", "motdepasse")
	req, _ := http.NewRequest("POST", testURL+"/api/issues", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST without %s: %d, want 403", csrfHeader, resp.StatusCode)
	}
}

func TestRestrictedProject(t *testing.T) {
	rhKey := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	outsider := login(t, "paul.martin", "motdepasse") // developer, not in "rh"
	insider := login(t, "chloe.simon", "motdepasse")  // in "rh"

	var projects []projectOut
	outsider.get("/api/projects", &projects)
	for _, p := range projects {
		if p.Key == "RH" {
			t.Fatal("paul.martin sees the RH project")
		}
	}
	if status := outsider.get("/api/issues/"+rhKey, nil); status != 404 {
		t.Errorf("paul.martin GET %s: %d, want 404", rhKey, status)
	}
	var list struct{ Total int }
	outsider.get("/api/issues?q=projet:RH", &list)
	if list.Total != 0 {
		t.Errorf("paul.martin finds %d RH issues", list.Total)
	}
	if status := insider.get("/api/issues/"+rhKey, nil); status != 200 {
		t.Errorf("chloe.simon GET %s: %d, want 200", rhKey, status)
	}
}

func TestSecurityLevels(t *testing.T) {
	// "Restreint": its reporter, marie.durand and the Administrators role only.
	restricted := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN security_levels l ON l.id = i.security_level_id JOIN users u ON u.id = i.reporter_id
		WHERE l.name = 'Restreint' AND u.username NOT IN ('paul.martin', 'marie.durand') AND u.active
		ORDER BY i.num LIMIT 1`)
	reporter := pick(t, `SELECT u.username FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN users u ON u.id = i.reporter_id WHERE `+keyExpr+` = ?`, restricted)

	for user, want := range map[string]int{"paul.martin": 404, "marie.durand": 200, reporter: 200} {
		if status := login(t, user, "motdepasse").get("/api/issues/"+restricted, nil); status != want {
			t.Errorf("%s GET %s: %d, want %d", user, restricted, status, want)
		}
	}
	var list struct{ Total int }
	login(t, "paul.martin", "motdepasse").get("/api/issues?q="+restricted, &list)
	if list.Total != 0 {
		t.Errorf("paul.martin finds %s by key", restricted)
	}

	// "Équipe": the developpeurs group only — not even the reporter otherwise.
	team := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN security_levels l ON l.id = i.security_level_id WHERE l.name = 'Équipe' ORDER BY i.num LIMIT 1`)
	if status := login(t, "paul.martin", "motdepasse").get("/api/issues/"+team, nil); status != 200 {
		t.Errorf("developer GET %s: %d", team, status)
	}
	if status := login(t, "chloe.simon", "motdepasse").get("/api/issues/"+team, nil); status != 404 {
		t.Errorf("non-developer GET %s: %d, want 404", team, status)
	}
}

func TestRestrictedComments(t *testing.T) {
	key := pick(t, `SELECT `+keyExpr+` FROM comments c JOIN issues i ON i.id = c.issue_id
		JOIN projects p ON p.id = i.project_id
		WHERE c.visibility_group = 'developpeurs' AND i.security_level_id IS NULL AND p.pkey <> 'RH'
		ORDER BY c.id LIMIT 1`)
	var dev, other []commentOut
	login(t, "paul.martin", "motdepasse").get("/api/issues/"+key+"/comments", &dev)
	login(t, "chloe.simon", "motdepasse").get("/api/issues/"+key+"/comments", &other)
	restricted := 0
	for _, c := range dev {
		if c.Visibility != nil && c.Visibility.Group == "developpeurs" {
			restricted++
		}
	}
	if restricted == 0 {
		t.Fatalf("developer does not see the restricted comment on %s", key)
	}
	for _, c := range other {
		if c.Visibility != nil && c.Visibility.Group == "developpeurs" {
			t.Fatalf("chloe.simon sees a comment restricted to developpeurs on %s", key)
		}
	}
	if len(other) != len(dev)-restricted {
		t.Errorf("chloe.simon sees %d comments, want %d", len(other), len(dev)-restricted)
	}
}

func TestOldKeysResolve(t *testing.T) {
	c := login(t, "admin", "admin")
	for old, want := range map[string]string{
		"SITE-1": "WEB-1", // renamed project
		pick(t, `SELECT old_key FROM moved_issue_keys LIMIT 1`): pick(t, `SELECT `+keyExpr+` FROM moved_issue_keys m
			JOIN issues i ON i.id = m.issue_id JOIN projects p ON p.id = i.project_id LIMIT 1`), // moved issue
	} {
		var d struct{ Key string }
		if status := c.get("/api/issues/"+old, &d); status != 200 || d.Key != want {
			t.Errorf("GET %s: %d %q, want %s", old, status, d.Key, want)
		}
	}
}

func TestIssueLifecycle(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	var meta catalog
	paul.get("/api/meta", &meta)
	var project struct {
		IssueTypes []typeRef `json:"issueTypes"`
	}
	paul.get("/api/projects/DEMO", &project)
	var task string
	for _, it := range project.IssueTypes {
		if !it.Subtask {
			task = it.ID
			break
		}
	}

	status, body := paul.do("POST", "/api/issues", map[string]any{
		"project": "DEMO", "type": task, "summary": "Le panier se vide au rafraîchissement",
		"description": "h3. Étapes\n# ajouter un article\n# recharger la page", "labels": []string{"panier", "web"},
	})
	if status != 201 {
		t.Fatalf("create: %d %s", status, body)
	}
	var created struct{ Key string }
	json.Unmarshal(body, &created)

	var d issueDetail
	paul.get("/api/issues/"+created.Key, &d)
	if d.Summary != "Le panier se vide au rafraîchissement" || d.Reporter == nil || d.Reporter.Username != "paul.martin" ||
		d.Status.Category != "todo" || !d.Watchers.Me || len(d.Labels) != 2 {
		t.Fatalf("created issue looks wrong: %+v", d.issueSummary)
	}

	var done string
	for _, s := range d.Statuses {
		if s.Category == "done" {
			done = s.ID
		}
	}
	status, body = paul.do("PATCH", "/api/issues/"+created.Key, map[string]any{"status": done, "summary": "Panier vidé au rechargement"})
	if status != 200 {
		t.Fatalf("transition: %d %s", status, body)
	}
	json.Unmarshal(body, &d)
	if d.Status.ID != done || d.Resolution == nil || d.Resolved == nil {
		t.Fatalf("moving to a done status must resolve: status=%v resolution=%v", d.Status, d.Resolution)
	}
	var history []historyEntry
	paul.get("/api/issues/"+created.Key+"/history", &history)
	fields := map[string]bool{}
	for _, h := range history {
		for _, it := range h.Items {
			fields[it.Field] = true
		}
	}
	for _, f := range []string{"status", "resolution", "summary"} {
		if !fields[f] {
			t.Errorf("history lacks %s: %v", f, fields)
		}
	}

	// Comments: the author edits, someone else may not.
	status, body = paul.do("POST", "/api/issues/"+created.Key+"/comments", map[string]any{"body": "Reproduit sur Firefox."})
	if status != 201 {
		t.Fatalf("comment: %d %s", status, body)
	}
	var comment commentOut
	json.Unmarshal(body, &comment)
	if status, _ := login(t, "hugo.robert", "motdepasse").do("PATCH", fmt.Sprintf("/api/comments/%d", comment.ID),
		map[string]string{"body": "réécrit"}); status != 403 {
		t.Errorf("another user edited the comment: %d", status)
	}
	if status, _ := paul.do("PATCH", fmt.Sprintf("/api/comments/%d", comment.ID), map[string]string{"body": "Reproduit sur Firefox 131."}); status != 200 {
		t.Errorf("author could not edit the comment: %d", status)
	}

	// Attachments round-trip.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "trace.txt")
	fw.Write([]byte("panic: panier vide"))
	mw.Close()
	req, _ := http.NewRequest("POST", testURL+"/api/issues/"+created.Key+"/attachments", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(csrfHeader, "1")
	resp, err := paul.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	upload, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("upload: %d %s", resp.StatusCode, upload)
	}
	json.Unmarshal(upload, &d)
	if len(d.Attachments) != 1 {
		t.Fatalf("attachments: %+v", d.Attachments)
	}
	status, content := paul.do("GET", d.Attachments[0].URL, nil)
	if status != 200 || string(content) != "panic: panier vide" {
		t.Errorf("download: %d %q", status, content)
	}
	if status, _ := login(t, "chloe.simon", "motdepasse").do("DELETE", fmt.Sprintf("/api/attachments/%d", d.Attachments[0].ID), nil); status != 403 {
		t.Errorf("another user deleted the attachment: %d", status)
	}
}

func TestLiveEventsRespectPermissions(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	req, _ := http.NewRequest("GET", testURL+"/api/events", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := paul.http.Do(req.WithContext(ctx))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	events := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			if line, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				events <- line
			}
		}
	}()
	time.Sleep(200 * time.Millisecond) // let the subscription register

	// Global administrators have no implicit access to content: someone from
	// the rh group makes both changes.
	actor := login(t, "chloe.simon", "motdepasse")
	hidden := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL LIMIT 1`)
	visible := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'WEB' AND i.security_level_id IS NULL LIMIT 1`)
	for _, key := range []string{hidden, visible} {
		if status, body := actor.do("POST", "/api/issues/"+key+"/comments", map[string]string{"body": "relance"}); status != 201 {
			t.Fatalf("comment on %s: %d %s", key, status, body)
		}
	}
	select {
	case e := <-events:
		var ev struct{ Key, What string }
		json.Unmarshal([]byte(e), &ev)
		if ev.Key != visible || ev.What != "comment" {
			t.Fatalf("paul.martin received %s, want a comment event on %s (and nothing about %s)", e, visible, hidden)
		}
	case <-ctx.Done():
		t.Fatal("no live event received")
	}
}

// Regressions from the security review --------------------------------------

func TestSubtaskInheritsConfidentiality(t *testing.T) {
	parent := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN security_levels l ON l.id = i.security_level_id
		WHERE l.name = 'Restreint' AND i.parent_id IS NULL ORDER BY i.num LIMIT 1`)
	subtaskType := pick(t, `SELECT t.issue_type_id FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND it.subtask LIMIT 1`)
	marie := login(t, "marie.durand", "motdepasse") // member of the level
	status, body := marie.do("POST", "/api/issues", map[string]any{
		"project": "DEMO", "type": subtaskType, "summary": "Sous-tâche confidentielle", "parent": parent,
	})
	if status != 201 {
		t.Fatalf("create sub-task: %d %s", status, body)
	}
	var created struct{ Key string }
	json.Unmarshal(body, &created)
	if status := login(t, "paul.martin", "motdepasse").get("/api/issues/"+created.Key, nil); status != 404 {
		t.Fatalf("paul.martin sees a sub-task of a confidential issue (%d)", status)
	}
}

func TestHistoryHidesInvisibleIssues(t *testing.T) {
	// A WEB issue that paul.martin sees, linked by chloe.simon to an RH issue he does not.
	visible := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'WEB' AND i.security_level_id IS NULL ORDER BY i.num DESC LIMIT 1`)
	hidden := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL ORDER BY i.num DESC LIMIT 1`)
	linkType := pick(t, `SELECT id FROM link_types ORDER BY position LIMIT 1`)
	if status, body := login(t, "chloe.simon", "motdepasse").do("POST", "/api/issues/"+visible+"/links",
		map[string]any{"type": json.Number(linkType), "direction": "outward", "target": hidden}); status != 200 {
		t.Fatalf("link: %d %s", status, body)
	}
	_, raw := login(t, "paul.martin", "motdepasse").do("GET", "/api/issues/"+visible+"/history", nil)
	if strings.Contains(string(raw), hidden) {
		t.Fatalf("paul.martin's history of %s mentions %s", visible, hidden)
	}
	_, raw = login(t, "chloe.simon", "motdepasse").do("GET", "/api/issues/"+visible+"/history", nil)
	if !strings.Contains(string(raw), hidden) {
		t.Fatalf("chloe.simon's history of %s lacks the link to %s", visible, hidden)
	}
}

func TestNegatedFilterKeepsUnassigned(t *testing.T) {
	c := login(t, "admin", "admin")
	var all, unassigned, notMarie struct{ Total int }
	c.get("/api/issues?q=projet:DEMO", &all)
	c.get("/api/issues?q=projet:DEMO%20@personne", &unassigned)
	c.get("/api/issues?q=projet:DEMO%20-@marie", &notMarie)
	var marie struct{ Total int }
	c.get("/api/issues?q=projet:DEMO%20@marie", &marie)
	if unassigned.Total == 0 || notMarie.Total != all.Total-marie.Total {
		t.Fatalf("all=%d marie=%d not-marie=%d unassigned=%d", all.Total, marie.Total, notMarie.Total, unassigned.Total)
	}
}

func TestLoginThrottling(t *testing.T) {
	if testDB == nil {
		t.Skip("SOLSTICE_TEST_DB not set: integration test skipped")
	}
	// Its own server, so the lock does not affect the other tests.
	srv := httptest.NewServer(New(testDB, t.TempDir(), time.UTC, slog.New(slog.NewTextHandler(io.Discard, nil))).
		Handler(http.NotFoundHandler()))
	defer srv.Close()
	try := func(password string) int {
		req, _ := http.NewRequest("POST", srv.URL+"/api/login",
			strings.NewReader(`{"username":"hugo.robert","password":"`+password+`"}`))
		req.Header.Set(csrfHeader, "1")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	for i := range 5 {
		if status := try("faux"); status != 401 {
			t.Fatalf("attempt %d: %d, want 401", i+1, status)
		}
	}
	if status := try("motdepasse"); status != http.StatusTooManyRequests {
		t.Fatalf("right password after 5 failures: %d, want 429", status)
	}
}

// JQL -----------------------------------------------------------------------

func runSearch(t *testing.T, c *client, q string) (int, int, string) {
	t.Helper()
	var res struct {
		Total int
		Mode  string
	}
	status := c.get("/api/issues?q="+url.QueryEscape(q), &res)
	return status, res.Total, res.Mode
}

func TestJQLMatchesSimpleSearch(t *testing.T) {
	c := login(t, "paul.martin", "motdepasse")
	for _, pair := range [][2]string{
		{`project = DEMO AND assignee is EMPTY`, `projet:DEMO @personne`},
		{`project = WEB AND statusCategory != Done`, `projet:WEB est:ouvert`},
		{`assignee = currentUser()`, `@moi`},
		{`reporter = currentUser() AND resolution = Unresolved`, `par:@moi est:ouvert`},
		{`labels = backend`, `#backend`},
	} {
		_, jqlTotal, mode := runSearch(t, c, pair[0])
		_, simpleTotal, _ := runSearch(t, c, pair[1])
		if mode != "jql" || jqlTotal != simpleTotal {
			t.Errorf("%q (%s) = %d, %q = %d", pair[0], mode, jqlTotal, pair[1], simpleTotal)
		}
	}
	if _, n, _ := runSearch(t, c, `project = RH`); n != 0 {
		t.Errorf("JQL lets paul.martin see %d RH issues", n)
	}
}

func TestJQLQueriesRun(t *testing.T) {
	c := login(t, "admin", "admin")
	for _, q := range []string{
		`project in (DEMO, WEB) AND status in ("In Progress", "In Review") ORDER BY priority DESC`,
		`priority not in (Low, Lowest) AND type = Bug`,
		`created >= -30d OR updated >= startOfMonth()`,
		`resolved >= startOfYear() AND resolution is not EMPTY`,
		`due <= endOfWeek("+1w") AND due is not EMPTY`,
		`assignee in membersOf("developpeurs") AND reporter != currentUser()`,
		`component = API OR fixVersion is EMPTY`,
		`affectedVersion is not EMPTY`,
		`labels in (backend, urgent) OR labels is EMPTY`,
		`watcher = currentUser() OR voter = currentUser()`,
		`text ~ "panier" OR summary ~ "export" OR description !~ "test"`,
		`comment ~ "relance"`,
		`key in (DEMO-1, WEB-2) OR issue > DEMO-100`,
		`parent = DEMO-3 OR parent is not EMPTY`,
		`"Story Points" >= 3 AND cf[10100] < 13`,
		`"Environnement client" = Production`,
		`"Référent métier" = currentUser()`,
		`"Date de recette" < now()`,
		`"Epic Link" = DEMO-1`,
		`"Thèmes" is not EMPTY`,
		`NOT (status = Done OR status = Closed) ORDER BY created ASC, key DESC`,
		`ORDER BY updated`,
		`sprint in openSprints() ORDER BY Rank ASC`,
		`sprint = "Sprint 5" OR sprint in closedSprints() OR sprint is EMPTY`,
		`fixVersion in unreleasedVersions() OR fixVersion is EMPTY`,
		`fixVersion in (earliestUnreleasedVersion(DEMO), latestReleasedVersion(DEMO))`,
		`issuetype in standardIssueTypes() AND level is EMPTY`,
		`"Epic Link" is not EMPTY ORDER BY "Epic Link", "Story Points" DESC, fixVersion, component`,
		`timespent > 2h OR originalEstimate >= 1d OR remainingEstimate < 30`,
		`key in issueHistory()`,
	} {
		status, body := c.do("GET", "/api/issues?q="+url.QueryEscape(q), nil)
		if status != 200 {
			t.Errorf("%s: %d %s", q, status, body)
		}
	}
}

func TestJQLErrorsAreExplained(t *testing.T) {
	c := login(t, "admin", "admin")
	for _, q := range []string{`project = "DEMO`, `couleur = bleu`, `summary was x`, `rank = 3`} {
		status, body := c.do("GET", "/api/issues?jql="+url.QueryEscape(q), nil)
		if status != 400 || !strings.Contains(string(body), "JQL") {
			t.Errorf("%s: %d %s", q, status, body)
		}
	}
}

// Saved filters ---------------------------------------------------------------

func filterNames(t *testing.T, c *client) map[string]filterOut {
	t.Helper()
	var list []filterOut
	if status := c.get("/api/filters", &list); status != 200 {
		t.Fatalf("GET /api/filters: %d", status)
	}
	out := map[string]filterOut{}
	for _, f := range list {
		out[f.Name] = f
	}
	return out
}

func TestFilterVisibility(t *testing.T) {
	dev := filterNames(t, login(t, "paul.martin", "motdepasse")) // developpeurs, not rh
	hr := filterNames(t, login(t, "lea.richard", "motdepasse"))  // rh, owner of the RH filter
	if _, ok := dev["RH — à traiter"]; ok {
		t.Error("paul.martin sees the RH filter")
	}
	if f, ok := hr["RH — à traiter"]; !ok || !f.Mine || !f.CanEdit {
		t.Errorf("lea.richard's own filter: %+v", f)
	}
	if _, ok := dev["Créés cette semaine"]; !ok {
		t.Error("a developer does not see the filter shared with developpeurs")
	}
	if _, ok := hr["Créés cette semaine"]; ok {
		t.Error("lea.richard sees the filter shared with developpeurs")
	}
	if f, ok := dev["Mes tickets ouverts"]; !ok || !f.Favourite || f.Mine || f.CanEdit {
		t.Errorf("filter shared with everyone, favourite of paul.martin: %+v", f)
	}
}

func TestFilterLifecycle(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	status, body := paul.do("POST", "/api/filters", map[string]any{
		"name": "Mes bugs", "query": "assignee = currentUser() AND type = Bug",
	})
	if status != 201 {
		t.Fatalf("create: %d %s", status, body)
	}
	var f filterOut
	json.Unmarshal(body, &f)
	if status, _ := paul.do("POST", "/api/filters", map[string]any{"name": "Cassé", "query": "project = "}); status != 400 {
		t.Errorf("invalid JQL accepted: %d", status)
	}
	rh := pick(t, `SELECT id FROM user_groups WHERE name = 'rh'`)
	if status, _ := paul.do("PATCH", "/api/filters/"+strconv.FormatInt(f.ID, 10), map[string]any{
		"shares": []map[string]any{{"kind": "group", "id": json.Number(rh)}},
	}); status != 400 {
		t.Errorf("sharing with a group one is not in: %d", status)
	}
	if status, _ := login(t, "hugo.robert", "motdepasse").do("DELETE", "/api/filters/"+strconv.FormatInt(f.ID, 10), nil); status != 404 {
		t.Errorf("someone else deleted a private filter: %d", status)
	}
	if status, _ := paul.do("PUT", "/api/filters/"+strconv.FormatInt(f.ID, 10)+"/favourite", nil); status != 204 {
		t.Errorf("favourite: %d", status)
	}
	if !filterNames(t, paul)["Mes bugs"].Favourite {
		t.Error("favourite not recorded")
	}
	if status, _ := paul.do("DELETE", "/api/filters/"+strconv.FormatInt(f.ID, 10), nil); status != 204 {
		t.Errorf("delete: %d", status)
	}
}

// Boards and sprints --------------------------------------------------------------

type boardCard struct {
	Key    string `json:"key"`
	Column int64  `json:"column"`
	Lane   *int64 `json:"lane"`
	Status struct {
		ID       string `json:"id"`
		Category string `json:"category"`
	} `json:"status"`
	Epic   *struct{ Key string } `json:"epic"`
	Parent *struct{ Key string } `json:"parent"`
	Sprint *int64                `json:"sprint"`
}

type boardDetail struct {
	Board struct {
		ID   int64  `json:"id"`
		Kind string `json:"kind"`
	} `json:"board"`
	Columns []struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Statuses []struct {
			ID string `json:"id"`
		} `json:"statuses"`
	} `json:"columns"`
	Sprints []struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		State string `json:"state"`
	} `json:"sprints"`
	Can struct {
		Manage bool `json:"manage"`
	} `json:"can"`
}

func boardID(t *testing.T, name string) string {
	return pick(t, `SELECT CAST(id AS CHAR) FROM boards WHERE name = ?`, name)
}

func TestBoardVisibility(t *testing.T) {
	names := func(c *client) map[string]bool {
		var list []struct{ Name string }
		c.get("/api/boards", &list)
		out := map[string]bool{}
		for _, b := range list {
			out[b.Name] = true
		}
		return out
	}
	dev := names(login(t, "paul.martin", "motdepasse"))
	hr := names(login(t, "chloe.simon", "motdepasse"))
	if !dev["DEMO — Scrum"] || !dev["WEB — Kanban"] || dev["RH"] {
		t.Errorf("paul.martin's boards: %v", dev)
	}
	if !hr["RH"] {
		t.Errorf("chloe.simon does not see the RH board: %v", hr)
	}
	if status := login(t, "paul.martin", "motdepasse").get("/api/boards/"+boardID(t, "RH"), nil); status != 404 {
		t.Errorf("paul.martin GET RH board: %d", status)
	}
}

func TestScrumBoard(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	id := boardID(t, "DEMO — Scrum")
	var b boardDetail
	if status := marie.get("/api/boards/"+id, &b); status != 200 {
		t.Fatalf("GET board: %d", status)
	}
	if b.Board.Kind != "scrum" || len(b.Columns) != 4 || !b.Can.Manage {
		t.Fatalf("board: %+v", b)
	}
	var active int64
	for _, sp := range b.Sprints {
		if sp.State == "active" {
			active = sp.ID
		}
	}
	if active == 0 {
		t.Fatal("no active sprint")
	}
	var cards struct {
		Cards  []boardCard
		Sprint *struct{ ID int64 }
	}
	marie.get("/api/boards/"+id+"/cards", &cards)
	want := pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM sprint_issues WHERE sprint_id = ?`, active)
	if cards.Sprint == nil || cards.Sprint.ID != active || strconv.Itoa(len(cards.Cards)) != want {
		t.Fatalf("active sprint shows %d cards, want %s", len(cards.Cards), want)
	}
	for _, c := range cards.Cards {
		if c.Column == 0 {
			t.Errorf("%s has no column", c.Key)
		}
	}

	var backlog struct {
		Sprints []struct {
			Sprint struct{ ID int64 }
			Cards  []boardCard
		}
		Backlog []boardCard
	}
	marie.get("/api/boards/"+id+"/backlog", &backlog)
	if len(backlog.Sprints) < 2 || len(backlog.Backlog) == 0 {
		t.Fatalf("backlog: %d sprints, %d issues", len(backlog.Sprints), len(backlog.Backlog))
	}
	for _, c := range backlog.Backlog {
		if c.Status.Category == "done" || c.Parent != nil {
			t.Errorf("%s should not be in the backlog", c.Key)
		}
	}

	// Plan the first backlog issue into the future sprint, then take it back.
	var future int64
	for _, sp := range b.Sprints {
		if sp.State == "future" {
			future = sp.ID
		}
	}
	key := backlog.Backlog[0].Key
	if status, body := marie.do("POST", "/api/boards/"+id+"/move", map[string]any{"issue": key, "sprint": future}); status != 204 {
		t.Fatalf("move to sprint: %d %s", status, body)
	}
	if n := pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM sprint_issues si JOIN issues i ON i.id = si.issue_id
		JOIN projects p ON p.id = i.project_id WHERE si.sprint_id = ? AND `+keyExpr+` = ?`, future, key); n != "1" {
		t.Fatalf("%s not in sprint %d", key, future)
	}
	marie.do("POST", "/api/boards/"+id+"/move", map[string]any{"issue": key, "sprint": 0})

	// Ranking: put the last backlog issue first.
	last := backlog.Backlog[len(backlog.Backlog)-1].Key
	if status, body := marie.do("POST", "/api/boards/"+id+"/move", map[string]any{"issue": last, "before": key}); status != 204 {
		t.Fatalf("rank: %d %s", status, body)
	}
	marie.get("/api/boards/"+id+"/backlog", &backlog)
	if backlog.Backlog[0].Key != last {
		t.Errorf("after ranking, the backlog starts with %s, want %s", backlog.Backlog[0].Key, last)
	}

	// Sprint lifecycle: one active sprint at a time; completing moves the rest.
	if status, _ := marie.do("POST", fmt.Sprintf("/api/sprints/%d/start", future), map[string]any{}); status != 409 {
		t.Errorf("second active sprint: %d, want 409", status)
	}
	if status, _ := login(t, "paul.martin", "motdepasse").do("POST", fmt.Sprintf("/api/sprints/%d/complete", active),
		map[string]any{"moveTo": future}); status != 403 {
		t.Errorf("a developer completed the sprint: %d", status)
	}
	status, body := marie.do("POST", fmt.Sprintf("/api/sprints/%d/complete", active), map[string]any{"moveTo": future})
	if status != 200 {
		t.Fatalf("complete: %d %s", status, body)
	}
	if status, body := marie.do("POST", fmt.Sprintf("/api/sprints/%d/start", future), map[string]any{}); status != 204 {
		t.Fatalf("start: %d %s", status, body)
	}
	if status, _ := marie.do("POST", "/api/boards/"+id+"/sprints", map[string]any{"name": "Sprint 7"}); status != 201 {
		t.Errorf("create sprint: %d", status)
	}
}

func TestKanbanMoveFollowsWorkflow(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	id := boardID(t, "WEB — Kanban")
	var b boardDetail
	marie.get("/api/boards/"+id, &b)
	var cards struct{ Cards []boardCard }
	marie.get("/api/boards/"+id+"/cards", &cards)
	if len(cards.Cards) == 0 {
		t.Fatal("empty kanban board")
	}
	lanes := 0
	for _, c := range cards.Cards {
		if c.Lane != nil {
			lanes++
		}
	}
	if lanes != len(cards.Cards) {
		t.Errorf("%d of %d cards have a swimlane", lanes, len(cards.Cards))
	}
	// A "to do" card moved to the "in progress" column takes that column's status.
	var todo, doing int64
	for _, c := range b.Columns {
		switch c.Name {
		case "To do":
			todo = c.ID
		case "In progress":
			doing = c.ID
		}
	}
	var key string
	for _, c := range cards.Cards {
		if c.Column == todo {
			key = c.Key
			break
		}
	}
	if key == "" {
		t.Skip("no card in the first column")
	}
	if status, body := marie.do("POST", "/api/boards/"+id+"/move", map[string]any{"issue": key, "column": doing}); status != 204 {
		t.Fatalf("move: %d %s", status, body)
	}
	var d issueDetail
	marie.get("/api/issues/"+key, &d)
	if d.Status.Category != "doing" {
		t.Errorf("%s is %s after moving to En cours", key, d.Status.Name)
	}
}

func TestEpicField(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	epic := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN issue_types t ON t.id = i.issue_type_id WHERE p.pkey = 'DEMO' AND t.name = 'Epic' ORDER BY i.id LIMIT 1`)
	story := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN issue_types t ON t.id = i.issue_type_id WHERE p.pkey = 'DEMO' AND t.name = 'Story' AND i.epic_id IS NULL
		ORDER BY i.id LIMIT 1`)
	if status, body := marie.do("PATCH", "/api/issues/"+story, map[string]any{"epic": epic}); status != 200 {
		t.Fatalf("set epic: %d %s", status, body)
	}
	var d issueDetail
	marie.get("/api/issues/"+story, &d)
	if d.Epic == nil || d.Epic.Key != epic {
		t.Fatalf("%s's epic: %+v, want %s", story, d.Epic, epic)
	}
	if status, _ := marie.do("PATCH", "/api/issues/"+story, map[string]any{"epic": story}); status != 400 {
		t.Errorf("a story as epic: %d, want 400", status)
	}
	if status, _ := marie.do("PATCH", "/api/issues/"+epic, map[string]any{"epic": epic}); status != 400 {
		t.Errorf("an epic in an epic: %d, want 400", status)
	}
	marie.do("PATCH", "/api/issues/"+story, map[string]any{"epic": nil})
	marie.get("/api/issues/"+story, &d)
	if d.Epic != nil {
		t.Errorf("epic still set after clearing: %+v", d.Epic)
	}
}

func TestBoardSettings(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	id := boardID(t, "WEB — Kanban")
	var b boardDetail
	marie.get("/api/boards/"+id, &b)
	if len(b.Columns) < 2 {
		t.Fatalf("columns: %+v", b.Columns)
	}
	// Merge the first two columns, cap the result at 3 and add a quick filter.
	first, second := b.Columns[0], b.Columns[1]
	statuses := []string{}
	for _, s := range append(first.Statuses, second.Statuses...) {
		statuses = append(statuses, s.ID)
	}
	columns := []map[string]any{{"id": first.ID, "name": "Entrée", "max": 3, "statuses": statuses}}
	for _, c := range b.Columns[2:] {
		ids := []string{}
		for _, s := range c.Statuses {
			ids = append(ids, s.ID)
		}
		columns = append(columns, map[string]any{"id": c.ID, "name": c.Name, "statuses": ids})
	}
	status, body := marie.do("PUT", "/api/boards/"+id, map[string]any{
		"columns":      columns,
		"quickFilters": []map[string]any{{"name": "Mes tickets", "query": "assignee = currentUser()"}},
		"doneCutoff":   "2w",
	})
	if status != 200 {
		t.Fatalf("update: %d %s", status, body)
	}
	marie.get("/api/boards/"+id, &b)
	if len(b.Columns) != len(columns) || b.Columns[0].Name != "Entrée" || len(b.Columns[0].Statuses) != len(statuses) {
		t.Errorf("columns after update: %+v", b.Columns)
	}
	if n := pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM board_columns WHERE id = ?`, second.ID); n != "0" {
		t.Errorf("the dropped column is still there")
	}

	// Bad input is refused and changes nothing.
	for _, bad := range []map[string]any{
		{"columns": []map[string]any{{"name": "A", "statuses": []string{statuses[0]}}, {"name": "B", "statuses": []string{statuses[0]}}}},
		{"quickFilters": []map[string]any{{"name": "Cassé", "query": "assignee = = moi"}}},
		{"doneCutoff": "deux semaines"},
		{"swimlanes": "planètes"},
	} {
		if status, _ := marie.do("PUT", "/api/boards/"+id, bad); status != 400 {
			t.Errorf("PUT %v: %d, want 400", bad, status)
		}
	}
	// paul.martin leads WEB and owns its board; jeanne.dupont is a developer there.
	if status, _ := login(t, "jeanne.dupont", "motdepasse").do("PUT", "/api/boards/"+id, map[string]any{"name": "x"}); status != 403 {
		t.Errorf("a developer changed the board: %d", status)
	}
}

func TestCreateAndDeleteBoard(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	status, body := marie.do("POST", "/api/boards", map[string]any{"kind": "kanban", "project": "OPS"})
	if status != 201 {
		t.Fatalf("create: %d %s", status, body)
	}
	var created struct{ ID int64 }
	json.Unmarshal(body, &created)
	path := fmt.Sprintf("/api/boards/%d", created.ID)
	var b boardDetail
	marie.get(path, &b)
	if len(b.Columns) != 3 {
		t.Fatalf("new board columns: %+v", b.Columns)
	}
	var cards struct{ Cards []boardCard }
	marie.get(path+"/cards", &cards)
	want := pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.pkey = 'OPS'`)
	if strconv.Itoa(len(cards.Cards)) != want {
		t.Errorf("new board shows %d cards, want %s", len(cards.Cards), want)
	}
	if status, _ := login(t, "paul.martin", "motdepasse").do("POST", "/api/boards", map[string]any{"kind": "kanban", "project": "OPS"}); status != 403 {
		t.Errorf("a developer created a board: %d", status)
	}
	if status, _ := marie.do("DELETE", path, nil); status != 204 {
		t.Fatalf("delete: %d", status)
	}
	if status := marie.get(path, nil); status != 404 {
		t.Errorf("deleted board still answers %d", status)
	}
}

type workflowList struct {
	Workflows []struct {
		ID          int64
		Name        string
		Default     bool
		Initial     struct{ ID string }
		Statuses    []struct{ ID, Name string }
		Transitions []struct {
			ID   int64
			Name string
			From *struct{ ID string }
			To   struct{ ID string }
		}
		IssueTypes []struct{ ID, Name string }
		Usage      map[string]int
	}
	Can struct{ Edit bool }
}

func TestWorkflowEditor(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	var wl workflowList
	if status := marie.get("/api/projects/OPS/workflows", &wl); status != 200 || !wl.Can.Edit || len(wl.Workflows) != 1 {
		t.Fatalf("GET workflows: %d %+v", status, wl)
	}
	wf := wl.Workflows[0]
	if wf.Usage["4"] == 0 {
		t.Fatalf("no issue in Reopened to strand: %v", wf.Usage)
	}

	// A new status, created once and found again by name.
	var review struct{ ID, Category string }
	status, body := marie.do("POST", "/api/statuses", map[string]any{"name": "Validation", "category": "doing", "project": "OPS"})
	if status != 201 {
		t.Fatalf("create status: %d %s", status, body)
	}
	json.Unmarshal(body, &review)
	if status, _ := marie.do("POST", "/api/statuses", map[string]any{"name": "validation", "category": "doing", "project": "OPS"}); status != 200 {
		t.Errorf("same status again: %d, want 200", status)
	}

	// Drop Reopened, add Validation between In Progress and Resolved.
	statuses := []string{"1", "3", review.ID, "5", "6"}
	transitions := []map[string]any{
		{"name": "Démarrer", "from": "1", "to": "3"},
		{"name": "Faire valider", "from": "3", "to": review.ID},
		{"name": "Valider", "from": review.ID, "to": "5"},
		{"name": "Fermer", "to": "6"},
	}
	put := map[string]any{"name": "Exploitation", "initial": "1", "statuses": statuses, "transitions": transitions}
	status, body = marie.do("PUT", fmt.Sprintf("/api/workflows/%d", wf.ID), put)
	if status != 409 || !strings.Contains(string(body), `"stranded"`) {
		t.Fatalf("dropping a used status without remap: %d %s", status, body)
	}
	reopened := pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'OPS' AND i.status_id = '4'`)
	put["remap"] = map[string]string{"4": "1"}
	if status, body = marie.do("PUT", fmt.Sprintf("/api/workflows/%d", wf.ID), put); status != 200 {
		t.Fatalf("PUT with remap: %d %s", status, body)
	}
	var after workflowList
	json.Unmarshal(body, &after)
	if got := after.Workflows[0]; got.Name != "Exploitation" || len(got.Statuses) != 5 || len(got.Transitions) != 4 || got.Usage["4"] != 0 {
		t.Errorf("after update: %+v", got)
	}
	if n := pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM change_items ci JOIN change_groups cg ON cg.id = ci.group_id
		JOIN issues i ON i.id = cg.issue_id JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'OPS' AND ci.field = 'status' AND ci.old_value = '4' AND ci.new_value = '1'`); n != reopened {
		t.Errorf("%s history entries for the remapped issues, want %s", n, reopened)
	}

	// An In Progress issue can now go to Validation.
	key := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'OPS' AND i.status_id = '3' ORDER BY i.id LIMIT 1`)
	var d issueDetail
	marie.get("/api/issues/"+key, &d)
	found := false
	for _, tr := range d.Transitions {
		found = found || tr.To.ID == review.ID
	}
	if !found {
		t.Errorf("%s cannot reach Validation: %+v", key, d.Transitions)
	}

	// Bad workflows are refused.
	for _, bad := range []map[string]any{
		{"name": "x", "initial": "1", "statuses": []string{}},
		{"name": "x", "initial": "5", "statuses": []string{"1"}},
		{"name": "x", "initial": "1", "statuses": []string{"1", "1"}},
		{"name": "x", "initial": "1", "statuses": []string{"1", "3"}, "transitions": []map[string]any{{"name": "t", "from": "1", "to": "6"}}},
	} {
		if status, _ := marie.do("PUT", fmt.Sprintf("/api/workflows/%d", wf.ID), bad); status != 400 {
			t.Errorf("PUT %v: %d, want 400", bad, status)
		}
	}

	// A second workflow for bugs, then removed again.
	status, body = marie.do("POST", "/api/projects/OPS/workflows", map[string]any{"name": "Incidents"})
	if status != 201 {
		t.Fatalf("create workflow: %d %s", status, body)
	}
	var created struct{ ID int64 }
	json.Unmarshal(body, &created)
	bug := pick(t, `SELECT id FROM issue_types WHERE name = 'Bug'`)
	put = map[string]any{"name": "Incidents", "initial": "1", "statuses": statuses, "issueTypes": []string{bug}}
	if status, body = marie.do("PUT", fmt.Sprintf("/api/workflows/%d", created.ID), put); status != 200 {
		t.Fatalf("map bugs: %d %s", status, body)
	}
	if status, _ := marie.do("DELETE", fmt.Sprintf("/api/workflows/%d", wf.ID), nil); status != 409 {
		t.Errorf("deleting the default workflow: %d, want 409", status)
	}
	if status, _ := marie.do("DELETE", fmt.Sprintf("/api/workflows/%d", created.ID), nil); status != 204 {
		t.Errorf("delete: %d", status)
	}
	if n := pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM workflow_issue_types WHERE workflow_id = ?`, created.ID); n != "0" {
		t.Errorf("type mapping left behind")
	}

	// Only administrators edit.
	jeanne := login(t, "jeanne.dupont", "motdepasse")
	if status, _ := jeanne.do("PUT", fmt.Sprintf("/api/workflows/%d", wf.ID), put); status != 403 && status != 404 {
		t.Errorf("a developer edited a workflow: %d", status)
	}
	if status, _ := jeanne.do("POST", "/api/statuses", map[string]any{"name": "Pirate", "category": "todo", "project": "OPS"}); status != 403 {
		t.Errorf("a developer created a status: %d", status)
	}
}

// History (WAS, CHANGED) is checked against intervals rebuilt here from the
// change log, independently of the SQL the compiler writes.
func TestJQLHistory(t *testing.T) {
	ctx := context.Background()
	type change struct {
		at       time.Time
		old, new sql.NullString
		author   int64
	}
	type issue struct {
		key     string
		created time.Time
		creator int64
		current sql.NullString
		changes []change
	}
	load := func(field, current string) map[int64]*issue {
		out := map[int64]*issue{}
		rows, err := testDB.QueryContext(ctx, `SELECT i.id, `+keyExpr+`, i.created_at, COALESCE(i.creator_id, 0), `+current+`
			FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.pkey = 'DEMO'`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id int64
			is := &issue{}
			if err := rows.Scan(&id, &is.key, &is.created, &is.creator, &is.current); err != nil {
				t.Fatal(err)
			}
			out[id] = is
		}
		rows.Close()
		rows, err = testDB.QueryContext(ctx, `SELECT g.issue_id, g.created_at, ci.old_value, ci.new_value, COALESCE(g.author_id, 0)
			FROM change_groups g JOIN change_items ci ON ci.group_id = g.id WHERE ci.field = ? ORDER BY g.created_at, g.id`, field)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id int64
			var c change
			if err := rows.Scan(&id, &c.at, &c.old, &c.new, &c.author); err != nil {
				t.Fatal(err)
			}
			if is := out[id]; is != nil {
				is.changes = append(is.changes, c)
			}
		}
		rows.Close()
		return out
	}
	type interval struct {
		value      sql.NullString
		start, end time.Time // zero end: still held
	}
	intervals := func(is *issue) []interval {
		if len(is.changes) == 0 {
			return []interval{{is.current, is.created, time.Time{}}}
		}
		out := []interval{{is.changes[0].old, is.created, is.changes[0].at}}
		for i, c := range is.changes {
			iv := interval{value: c.new, start: c.at}
			if i+1 < len(is.changes) {
				iv.end = is.changes[i+1].at
			}
			out = append(out, iv)
		}
		return out
	}
	keys := func(issues map[int64]*issue, keep func(*issue) bool) []string {
		out := []string{}
		for _, is := range issues {
			if keep(is) {
				out = append(out, is.key)
			}
		}
		sort.Strings(out)
		return out
	}
	c := login(t, "admin", "admin")
	search := func(q string) []string {
		t.Helper()
		var res struct {
			Issues []struct{ Key string }
		}
		if status := c.get("/api/issues?size=200&q="+url.QueryEscape("project = DEMO AND "+q), &res); status != 200 {
			t.Fatalf("%s: %d", q, status)
		}
		got := []string{}
		for _, is := range res.Issues {
			got = append(got, is.Key)
		}
		sort.Strings(got)
		return got
	}
	// The oracle knows every issue; the search only those admin may see.
	visible := search("created is not EMPTY")
	check := func(q string, want []string) {
		t.Helper()
		want = slices.DeleteFunc(want, func(k string) bool { _, ok := slices.BinarySearch(visible, k); return !ok })
		if got := search(q); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %d issues, want %d\n got %v\nwant %v", q, len(got), len(want), got, want)
		}
	}
	is := func(v sql.NullString, id string) bool { return v.Valid && v.String == id }

	status := load("status", "i.status_id")
	inProgress := pick(t, `SELECT id FROM statuses WHERE name = 'In Progress'`)
	inReview := pick(t, `SELECT id FROM statuses WHERE name = 'In Review'`)
	// A moment with history on both sides of it.
	var times []time.Time
	for _, i := range status {
		for _, c := range i.changes {
			times = append(times, c.at)
		}
	}
	if len(times) < 10 {
		t.Fatalf("only %d status changes in DEMO", len(times))
	}
	sort.Slice(times, func(a, b int) bool { return times[a].Before(times[b]) })
	at := times[len(times)/2].Add(time.Minute).Round(time.Minute)
	when := at.Format("2006-01-02 15:04")
	day := at.Format(time.DateOnly)
	dayStart, _ := time.ParseInLocation(time.DateOnly, day, time.UTC)
	dayEnd := dayStart.AddDate(0, 0, 1)
	held := func(i *issue, ok func(interval) bool) bool { return slices.ContainsFunc(intervals(i), ok) }

	check(`status was "In Progress"`, keys(status, func(i *issue) bool {
		return held(i, func(iv interval) bool { return is(iv.value, inProgress) })
	}))
	check(`status was not "In Progress"`, keys(status, func(i *issue) bool {
		return !held(i, func(iv interval) bool { return is(iv.value, inProgress) })
	}))
	check(`status was "In Progress" before "`+when+`"`, keys(status, func(i *issue) bool {
		return held(i, func(iv interval) bool { return is(iv.value, inProgress) && iv.start.Before(at) })
	}))
	check(`status was "In Progress" after "`+when+`"`, keys(status, func(i *issue) bool {
		return held(i, func(iv interval) bool { return is(iv.value, inProgress) && (iv.end.IsZero() || iv.end.After(at)) })
	}))
	check(`status was in ("In Progress", "In Review") on "`+day+`"`, keys(status, func(i *issue) bool {
		return held(i, func(iv interval) bool {
			return (is(iv.value, inProgress) || is(iv.value, inReview)) && iv.start.Before(dayEnd) &&
				(iv.end.IsZero() || !iv.end.Before(dayStart))
		})
	}))
	check(`status changed to "In Progress" after "`+when+`"`, keys(status, func(i *issue) bool {
		return slices.ContainsFunc(i.changes, func(c change) bool { return is(c.new, inProgress) && c.at.After(at) })
	}))
	check(`status changed from "In Progress" during ("2020-01-01", "`+day+`")`, keys(status, func(i *issue) bool {
		return slices.ContainsFunc(i.changes, func(c change) bool { return is(c.old, inProgress) && c.at.Before(dayEnd) })
	}))

	// Users: the log keeps Jira user keys (JIRAUSER…); queries use user names.
	assignee := load("assignee", "(SELECT u.user_key FROM users u WHERE u.id = i.assignee_id)")
	var name, ukey string
	if err := testDB.QueryRowContext(ctx, `SELECT u.username, u.user_key FROM users u JOIN change_items ci
		ON ci.new_value = u.user_key WHERE ci.field = 'assignee' AND u.user_key <> u.username
		GROUP BY u.id ORDER BY COUNT(*) DESC LIMIT 1`).Scan(&name, &ukey); err != nil {
		t.Fatal(err)
	}
	check(`assignee was "`+name+`"`, keys(assignee, func(i *issue) bool {
		return held(i, func(iv interval) bool { return is(iv.value, ukey) })
	}))
	check(`assignee changed to "`+name+`"`, keys(assignee, func(i *issue) bool {
		return slices.ContainsFunc(i.changes, func(c change) bool { return is(c.new, ukey) })
	}))
	check(`assignee changed from EMPTY`, keys(assignee, func(i *issue) bool {
		return slices.ContainsFunc(i.changes, func(c change) bool { return !c.old.Valid })
	}))
	var author string
	var authorID int64
	if err := testDB.QueryRowContext(ctx, `SELECT u.id, u.username FROM change_groups g JOIN change_items ci ON ci.group_id = g.id
		JOIN users u ON u.id = g.author_id WHERE ci.field = 'status' GROUP BY u.id ORDER BY COUNT(*) DESC LIMIT 1`).
		Scan(&authorID, &author); err != nil {
		t.Fatal(err)
	}
	check(`status changed by "`+author+`"`, keys(status, func(i *issue) bool {
		return slices.ContainsFunc(i.changes, func(c change) bool { return c.author == authorID })
	}))

	for _, q := range []string{`summary was "x"`, `status changed after EMPTY`, `status was Done by EMPTY`, `fixVersion was EMPTY`} {
		if status, _ := c.do("GET", "/api/issues?jql="+url.QueryEscape(q), nil); status != 400 {
			t.Errorf("%s: %d, want 400", q, status)
		}
	}
}

func TestSuggestions(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	var labels []string
	if status := paul.get("/api/labels?q=BACK", &labels); status != 200 || !slices.Contains(labels, "backend") {
		t.Errorf("labels matching BACK: %d %v", status, labels)
	}
	var none []string
	paul.get("/api/labels?project=RH", &none)
	if len(none) != 0 {
		t.Errorf("paul.martin sees RH labels: %v", none)
	}
	var sprints []struct {
		Name, State, BoardName string
	}
	if status := paul.get("/api/sprints", &sprints); status != 200 || len(sprints) == 0 || sprints[0].State != "active" {
		t.Fatalf("sprints: %d %+v", status, sprints)
	}
	for _, sp := range sprints {
		if sp.BoardName == "RH" {
			t.Errorf("paul.martin sees the RH board's sprint %s", sp.Name)
		}
	}
}

// Issue types leaving a workflow go back to the default one: their issues in
// a status the default lacks must be placed (409, then remapDefault).
func TestWorkflowTypesLeaving(t *testing.T) {
	ctx := context.Background()
	marie := login(t, "marie.durand", "motdepasse")
	var blocked struct{ ID string }
	_, body := marie.do("POST", "/api/statuses", map[string]any{"name": "Bloqué", "category": "doing", "project": "WEB"})
	json.Unmarshal(body, &blocked)
	bug := pick(t, `SELECT id FROM issue_types WHERE name = 'Bug'`)
	key := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'WEB' AND i.issue_type_id = ? ORDER BY i.id LIMIT 1`, bug)

	// A "Bugs" workflow with a Bloqué status, and one bug sitting in it.
	setup := func() int64 {
		status, body := marie.do("POST", "/api/projects/WEB/workflows", map[string]any{"name": "Bugs"})
		if status != 201 {
			t.Fatalf("create: %d %s", status, body)
		}
		var created struct{ ID int64 }
		json.Unmarshal(body, &created)
		put := map[string]any{"name": "Bugs", "initial": "10000", "statuses": []string{"10000", "3", "10002", blocked.ID, "10001"},
			"issueTypes": []string{bug}}
		if status, body := marie.do("PUT", fmt.Sprintf("/api/workflows/%d", created.ID), put); status != 200 {
			t.Fatalf("map bugs: %d %s", status, body)
		}
		if _, err := testDB.ExecContext(ctx, `UPDATE issues i JOIN projects p ON p.id = i.project_id SET i.status_id = ?
			WHERE `+keyExpr+` = ?`, blocked.ID, key); err != nil {
			t.Fatal(err)
		}
		return created.ID
	}
	statusOf := func() string {
		return pick(t, `SELECT i.status_id FROM issues i JOIN projects p ON p.id = i.project_id WHERE `+keyExpr+` = ?`, key)
	}

	id := setup()
	put := map[string]any{"name": "Bugs", "initial": "10000", "statuses": []string{"10000", "3", "10002", blocked.ID, "10001"},
		"issueTypes": []string{}}
	status, body := marie.do("PUT", fmt.Sprintf("/api/workflows/%d", id), put)
	if status != 409 || !strings.Contains(string(body), `"strandedDefault"`) || !strings.Contains(string(body), `"defaultWorkflow"`) {
		t.Fatalf("unmapping bugs: %d %s", status, body)
	}
	put["remapDefault"] = map[string]string{blocked.ID: "10000"}
	if status, body := marie.do("PUT", fmt.Sprintf("/api/workflows/%d", id), put); status != 200 {
		t.Fatalf("unmapping with remapDefault: %d %s", status, body)
	}
	if st := statusOf(); st != "10000" {
		t.Errorf("%s is in %s after going back to the default workflow", key, st)
	}

	// Deleting the workflow behaves the same.
	marie.do("PUT", fmt.Sprintf("/api/workflows/%d", id), map[string]any{"name": "Bugs", "initial": "10000",
		"statuses": []string{"10000", "3", "10002", blocked.ID, "10001"}, "issueTypes": []string{bug}})
	testDB.ExecContext(ctx, `UPDATE issues i JOIN projects p ON p.id = i.project_id SET i.status_id = ? WHERE `+keyExpr+` = ?`,
		blocked.ID, key)
	if status, body := marie.do("DELETE", fmt.Sprintf("/api/workflows/%d", id), nil); status != 409 ||
		!strings.Contains(string(body), `"strandedDefault"`) {
		t.Fatalf("delete without remap: %d %s", status, body)
	}
	if status, body := marie.do("DELETE", fmt.Sprintf("/api/workflows/%d", id),
		map[string]any{"remapDefault": map[string]string{blocked.ID: "3"}}); status != 204 {
		t.Fatalf("delete with remap: %d %s", status, body)
	}
	if st := statusOf(); st != "3" {
		t.Errorf("%s is in %s after its workflow was deleted", key, st)
	}

	// The catalogue: global administrators rename and delete unused statuses.
	paul := login(t, "paul.martin", "motdepasse")
	if status, _ := paul.do("PATCH", "/api/statuses/"+blocked.ID, map[string]any{"name": "x"}); status != 403 {
		t.Errorf("a project lead renamed a shared status: %d", status)
	}
	if status, _ := marie.do("PATCH", "/api/statuses/"+blocked.ID, map[string]any{"name": "In Progress"}); status != 409 {
		t.Errorf("renaming to an existing name: %d, want 409", status)
	}
	if status, body := marie.do("PATCH", "/api/statuses/"+blocked.ID, map[string]any{"name": "En attente", "category": "todo"}); status != 200 ||
		!strings.Contains(string(body), "En attente") {
		t.Errorf("rename: %d %s", status, body)
	}
	if status, _ := marie.do("DELETE", "/api/statuses/3", nil); status != 409 {
		t.Errorf("deleting a status in use: %d, want 409", status)
	}
	if status, body := marie.do("DELETE", "/api/statuses/"+blocked.ID, nil); status != 204 {
		t.Errorf("deleting an unused status: %d %s", status, body)
	}
}

func TestSearchHelpers(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	var epics []struct{ Key, Project string }
	if status := paul.get("/api/epics?project=DEMO", &epics); status != 200 || len(epics) == 0 {
		t.Fatalf("epics: %d %v", status, epics)
	}
	var fields []struct{ Name, Kind, JQL string }
	paul.get("/api/fields", &fields)
	if !slices.ContainsFunc(fields, func(f struct{ Name, Kind, JQL string }) bool {
		return f.Name == "Story Points" && f.Kind == "float" && f.JQL == `"Story Points"`
	}) {
		t.Errorf("fields: %+v", fields)
	}
	var versions []struct {
		Name    string
		Project struct{ Key string }
	}
	paul.get("/api/versions", &versions)
	if len(versions) == 0 || slices.ContainsFunc(versions, func(v struct {
		Name    string
		Project struct{ Key string }
	}) bool {
		return v.Project.Key == "RH"
	}) {
		t.Errorf("versions seen by paul.martin: %+v", versions)
	}
	var comps []struct{ Name string }
	if status := paul.get("/api/components?project=DEMO", &comps); status != 200 || len(comps) == 0 {
		t.Errorf("components: %d %v", status, comps)
	}

	// Facets count the same issues the search lists.
	var facets struct {
		Total  int
		Facets map[string]map[string]int
	}
	q := url.QueryEscape("project = DEMO")
	if status := paul.get("/api/issues/facets?jql="+q+"&fields=status,assignee,labels,nope", &facets); status != 200 {
		t.Fatalf("facets: %d", status)
	}
	_, total, _ := runSearch(t, paul, "project = DEMO")
	sum := 0
	for _, n := range facets.Facets["status"] {
		sum += n
	}
	if facets.Total != total || sum != total || facets.Facets["assignee"] == nil || facets.Facets["labels"] == nil {
		t.Errorf("facets: total %d (search %d), status sum %d, %+v", facets.Total, total, sum, facets.Facets)
	}
	if _, ok := facets.Facets["nope"]; ok {
		t.Error("an unknown facet was answered")
	}

	// JQL errors say where.
	status, body := paul.do("GET", "/api/issues?jql="+url.QueryEscape("project = DEMO AND couleur = bleu"), nil)
	if status != 400 || !strings.Contains(string(body), `"pos":19`) {
		t.Errorf("JQL error: %d %s", status, body)
	}
}

func TestJQLParticipant(t *testing.T) {
	c := login(t, "marie.durand", "motdepasse")
	_, roles, _ := runSearch(t, c, `assignee = paul.martin OR reporter = paul.martin OR creator = paul.martin OR `+
		`watcher = paul.martin OR commenter = paul.martin`)
	status, all, mode := runSearch(t, c, `participant = paul.martin`)
	if status != 200 || mode != "jql" || roles == 0 || all < roles {
		t.Errorf("participant = paul.martin: %d (%s) %d issues, roles alone %d", status, mode, all, roles)
	}
	_, none, _ := runSearch(t, c, `participant != paul.martin`)
	_, total, _ := runSearch(t, c, `created is not EMPTY`)
	if all+none != total {
		t.Errorf("participant = and != paul.martin: %d + %d, want %d", all, none, total)
	}
}
