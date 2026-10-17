// Package tests holds the backend's integration tests. They drive the API
// from outside, as the interface and scripts do, against a Solstice database
// filled by the synthetic Jira import (scripts/e2e-import.sh sets it up and
// runs them). They exercise what must never break: who sees what, the issue
// lifecycle, the administration, e-mail, webhooks and the REST API.
//
//	SOLSTICE_TEST_DB=mysql://solstice:solstice@localhost:13306/solstice_e2e_pg go test ./tests/
//
// Without SOLSTICE_TEST_DB, every test skips itself. The tests change the
// database: give them a copy made for them, never a real instance.
package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/auth"
	"solstice/internal/config"
	"solstice/internal/db"
)

var (
	testDB  *sql.DB
	testURL string // the server most tests share
)

// csrfHeader is the anti-forgery header every write needs.
const csrfHeader = "X-Solstice"

// keyExpr is an issue's key in queries joining issues i and projects p.
const keyExpr = "CONCAT(p.pkey, '-', i.num)"

func TestMain(m *testing.M) {
	dsn := os.Getenv("SOLSTICE_TEST_DB")
	if dsn == "" {
		os.Exit(m.Run())
	}
	var err error
	testDB, err = db.Open(context.Background(), dsn, config.Defaults().DB.Pool())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	dataDir, err := os.MkdirTemp("", "solstice-api-test")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	srv := httptest.NewServer(api.New(testDB, dataDir, time.UTC, quiet()).Handler(http.NotFoundHandler()))
	testURL = srv.URL
	code := m.Run()
	srv.Close()
	os.RemoveAll(dataDir)
	os.Exit(code)
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// needDB skips a test without a database.
func needDB(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip("SOLSTICE_TEST_DB not set: integration test skipped")
	}
}

// newServer starts an API server of its own, for tests that lock addresses
// out (failed sign-ins) or change the server itself.
func newServer(t *testing.T) (*api.Server, string) {
	t.Helper()
	needDB(t)
	srv := api.New(testDB, t.TempDir(), time.UTC, quiet())
	ts := httptest.NewServer(srv.Handler(http.NotFoundHandler()))
	t.Cleanup(ts.Close)
	return srv, ts.URL
}

// ownServer is newServer for tests that only need its address.
func ownServer(t *testing.T) string {
	_, base := newServer(t)
	return base
}

// client is one browser: its cookies, on one server.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func anonymousAt(t *testing.T, base string) *client {
	needDB(t)
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: base, http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
}

func anonymous(t *testing.T) *client { return anonymousAt(t, testURL) }

func loginAt(t *testing.T, base, username, password string) *client {
	t.Helper()
	c := anonymousAt(t, base)
	if status, body := c.do("POST", "/api/login", map[string]string{"username": username, "password": password}); status != 200 {
		t.Fatalf("login %s: %d %s", username, status, body)
	}
	return c
}

func login(t *testing.T, username, password string) *client {
	t.Helper()
	return loginAt(t, testURL, username, password)
}

// request sends a JSON request with the anti-forgery header, in a language
// ("" for no Accept-Language).
func (c *client) request(lang, method, path string, body any) (int, []byte) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeader, "1")
	if lang != "" {
		req.Header.Set("Accept-Language", lang)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

func (c *client) do(method, path string, body any) (int, []byte) {
	c.t.Helper()
	return c.request("", method, path, body)
}

// get decodes a 200 answer into out (if not nil) and returns the status.
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

// call does a request that must answer want, and decodes the answer.
func (c *client) call(method, path string, body any, want int, out any) []byte {
	c.t.Helper()
	status, raw := c.do(method, path, body)
	if status != want {
		c.t.Fatalf("%s %s: %d, want %d\n%s", method, path, status, want, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			c.t.Fatalf("%s %s: %v\n%s", method, path, err, raw)
		}
	}
	return raw
}

// errorIn sends a request in a language and returns the status, the error
// message and the whole answer.
func (c *client) errorIn(lang, method, path string, body any) (int, string, map[string]any) {
	c.t.Helper()
	status, raw := c.request(lang, method, path, body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	msg, _ := out["error"].(string)
	return status, msg, out
}

// call sends a request with the client's cookies (c may be nil for none);
// setup adds headers or credentials.
func call(t *testing.T, c *client, base, method, path string, body any, setup func(*http.Request)) (int, http.Header, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, base+path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if setup != nil {
		setup(req)
	}
	hc := http.DefaultClient
	if c != nil {
		hc = c.http
	}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, out
}

func bearer(token string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

func basic(user, pass string) func(*http.Request) {
	return func(r *http.Request) { r.SetBasicAuth(user, pass) }
}

func header(k, v string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set(k, v) }
}

// obj decodes a JSON object, failing the test otherwise.
func obj(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("not a JSON object: %v\n%s", err, raw)
	}
	return m
}

// at walks a decoded JSON value: keys of objects, indexes of arrays.
func at(v any, path ...any) any {
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, _ := v.(map[string]any)
			v = m[k]
		case int:
			a, _ := v.([]any)
			if k >= len(a) {
				return nil
			}
			v = a[k]
		}
	}
	return v
}

func str(v any, path ...any) string {
	s, _ := at(v, path...).(string)
	return s
}

func num(v any, path ...any) int {
	f, _ := at(v, path...).(float64)
	return int(f)
}

// pick runs a query returning one value (an issue key, a user name…).
func pick(t *testing.T, query string, args ...any) string {
	t.Helper()
	needDB(t)
	var s string
	if err := testDB.QueryRow(query, args...).Scan(&s); err != nil {
		t.Fatalf("fixture lookup failed: %v\n%s", err, query)
	}
	return s
}

func userID(t *testing.T, username string) int64 {
	id, _ := strconv.ParseInt(pick(t, `SELECT id FROM users WHERE username = ?`, username), 10, 64)
	return id
}

func issueID(t *testing.T, key string) string {
	return pick(t, `SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id WHERE `+keyExpr+` = ?`, key)
}

// newToken creates a token for a user straight in the database.
func newToken(t *testing.T, username string, expires *time.Time) string {
	t.Helper()
	token, _, err := auth.NewToken(context.Background(), testDB, userID(t, username), "test "+username, expires)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// standardType is the first issue type of a project that is not a sub-task.
func standardType(t *testing.T, c *client, project string) string {
	t.Helper()
	var p struct {
		IssueTypes []typeRef `json:"issueTypes"`
	}
	c.call("GET", "/api/projects/"+project, nil, 200, &p)
	for _, it := range p.IssueTypes {
		if !it.Subtask {
			return it.ID
		}
	}
	t.Fatalf("%s has no standard issue type", project)
	return ""
}

// createIssue creates an issue of the first standard type of a project.
func createIssue(t *testing.T, c *client, project, summary, description string) string {
	t.Helper()
	var created struct{ Key string }
	c.call("POST", "/api/issues", map[string]any{"project": project, "type": standardType(t, c, project),
		"summary": summary, "description": description}, 201, &created)
	return created.Key
}

// unique gives names no other run of the tests used.
func unique(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano()%1e9)
}
