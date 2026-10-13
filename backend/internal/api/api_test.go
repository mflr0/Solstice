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
		fmt.Println("SOLSTICE_TEST_DB not set: API integration tests skipped")
		os.Exit(0)
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
	} {
		status, body := c.do("GET", "/api/issues?q="+url.QueryEscape(q), nil)
		if status != 200 {
			t.Errorf("%s: %d %s", q, status, body)
		}
	}
}

func TestJQLErrorsAreExplained(t *testing.T) {
	c := login(t, "admin", "admin")
	for _, q := range []string{`project = "DEMO`, `couleur = bleu`, `status was Done`, `sprint = 3`} {
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
