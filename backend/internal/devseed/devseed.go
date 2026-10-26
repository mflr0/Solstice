// Package devseed fills a development database: an administrator, mock users
// in every interface language, a demo project with issues, and e-mail sent to
// a local Mailpit. Only cmd/devseed uses it, so its passwords never ship in
// the solstice binary.
package devseed

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strconv"
	"time"

	"solstice/internal/api"
	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/issue"
)

// Admin is the administrator the seed creates; its presence marks a
// database as a development one.
const (
	Admin         = "seed"
	AdminPassword = "seed-dev-only" // #nosec G101 -- development account, documented in the README
	UserPassword  = "dev-password"  // #nosec G101 -- development accounts, documented in the README
)

// User is a mock account.
type User struct {
	Username, Name string
	Locale         string // "" follows the browser
}

// Users are the mock accounts, one per interface language and one without.
var Users = []User{
	{"alice", "Alice Martin", "en"},
	{"marie", "Marie Dubois", "fr"},
	{"hans", "Hans Müller", "de"},
	{"giulia", "Giulia Rossi", "it"},
	{"priya", "Priya Sharma", "hi"},
	{"tom", "Tom Baker", ""},
}

// Project is the key of the demo project.
const Project = "DEMO"

// ErrNotDev is returned for a database with accounts but no seed
// administrator: probably a real instance.
var ErrNotDev = errors.New("this database has accounts but no \"" + Admin + "\" administrator: " +
	"it does not look like a development database, nothing was changed")

// Options configures Run.
type Options struct {
	// SMTP is Mailpit's host:port, set as the mail server when none is.
	SMTP string
	// Out receives one line per thing created.
	Out io.Writer
}

// Run seeds conn, a migrated database. It only adds what is missing, so it
// can run on every start.
func Run(ctx context.Context, conn *sql.DB, opt Options) error {
	if opt.Out == nil {
		opt.Out = io.Discard
	}
	say := func(format string, args ...any) { fmt.Fprintf(opt.Out, format+"\n", args...) }
	var accounts, admins int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(CASE WHEN username = ? THEN 1 END) FROM users`,
		Admin).Scan(&accounts, &admins); err != nil {
		return err
	}
	if accounts > 0 && admins == 0 {
		return ErrNotDev
	}
	if admins == 0 {
		if err := createAdmin(ctx, conn); err != nil {
			return err
		}
		say("created the administrator %s / %s", Admin, AdminPassword)
	}

	dataDir, err := os.MkdirTemp("", "solstice-devseed")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(api.New(conn, dataDir, time.UTC, log).Handler(http.NotFoundHandler()))
	defer srv.Close()
	admin, err := login(ctx, srv.URL, Admin, AdminPassword)
	if err != nil {
		return err
	}

	ids := map[string]int64{}
	for _, u := range Users {
		id, created, err := ensureUser(ctx, conn, admin, u)
		if err != nil {
			return fmt.Errorf("user %s: %w", u.Username, err)
		}
		ids[u.Username] = id
		if created {
			say("created the user %s / %s", u.Username, UserPassword)
		}
	}
	if opt.SMTP != "" {
		set, err := ensureMail(admin, opt.SMTP)
		if err != nil {
			return fmt.Errorf("mail settings: %w", err)
		}
		if set {
			say("e-mail goes to %s", opt.SMTP)
		}
	}
	created, err := ensureProject(ctx, conn, srv.URL, admin, ids)
	if err != nil {
		return fmt.Errorf("demo project: %w", err)
	}
	if created > 0 {
		say("created %d issues in the project %s", created, Project)
	}
	return nil
}

// createAdmin writes the administrator directly, as "solstice user add"
// does: the API needs an account to sign in with.
func createAdmin(ctx context.Context, conn *sql.DB) error {
	hash, err := auth.HashPassword(AdminPassword)
	if err != nil {
		return err
	}
	res, err := conn.ExecContext(ctx, `INSERT INTO users (username, display_name, email, password_hash, is_admin)
		VALUES (?, ?, ?, ?, TRUE)`, Admin, "Seed Admin", Admin+"@example.org", hash)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for _, action := range []string{audit.UserCreate, audit.AdminGrant} {
		if err := audit.Record(ctx, conn, audit.Entry{Action: action,
			Target:  audit.Target{Type: audit.TargetUser, ID: strconv.FormatInt(id, 10), Label: Admin},
			Details: map[string]any{"via": "dev seed"}}); err != nil {
			return err
		}
	}
	return nil
}

// ensureUser creates u through the administration and sets its language as
// the person would, unless the account exists.
func ensureUser(ctx context.Context, conn *sql.DB, admin *client, u User) (id int64, created bool, err error) {
	err = conn.QueryRowContext(ctx, `SELECT id FROM users WHERE username = ?`, u.Username).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	var out struct{ ID int64 }
	if err := admin.call("POST", "/api/admin/users", map[string]any{"username": u.Username, "name": u.Name,
		"email": u.Username + "@example.org", "password": UserPassword, "admin": false}, &out); err != nil {
		return 0, false, err
	}
	if u.Locale != "" {
		c, err := login(ctx, admin.base, u.Username, UserPassword)
		if err != nil {
			return 0, false, err
		}
		if err := c.call("PATCH", "/api/me", map[string]any{"locale": u.Locale}, nil); err != nil {
			return 0, false, err
		}
	}
	return out.ID, true, nil
}

// ensureMail points e-mail at Mailpit when no mail server is set.
func ensureMail(admin *client, hostPort string) (bool, error) {
	var cur struct{ Host string }
	if err := admin.call("GET", "/api/admin/mail", nil, &cur); err != nil {
		return false, err
	}
	if cur.Host != "" {
		return false, nil
	}
	host, rawPort, err := net.SplitHostPort(hostPort)
	if err != nil {
		return false, err
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return false, err
	}
	return true, admin.call("PUT", "/api/admin/mail", map[string]any{"enabled": true, "host": host, "port": port,
		"security": "none", "username": "", "password": nil, "from": "solstice@example.org", "fromName": "Solstice"}, nil)
}

// demoIssue is a demo issue: who reports it, who works on it, how far it got.
type demoIssue struct {
	reporter, assignee string
	summary            string
	status             issue.StatusCategory // where it ends; "" leaves it in the first status
	comment            string               // by the assignee
}

var issues = []demoIssue{
	{"alice", "alice", "Sign-in page: remember the last username", issue.Doing, "Working on it, the cookie is set."},
	{"alice", "", "Board columns overflow on small screens", "", ""},
	{"marie", "marie", "Traduire les libellés du tableau de bord", issue.Done, "C'est fait, à relire."},
	{"marie", "hans", "Export CSV : les dates sont en UTC", "", ""},
	{"hans", "hans", "Suche nach Umlauten findet nichts", issue.Doing, "Die Sortierung der Datenbank ist schuld."},
	{"hans", "giulia", "Attachment preview for PDF files", "", ""},
	{"giulia", "giulia", "Notifiche duplicate per i commenti", issue.Done, "Risolto con il raggruppamento."},
	{"giulia", "", "Keyboard shortcut to assign to me", "", ""},
	{"priya", "priya", "हिंदी में तारीख़ का प्रारूप", issue.Doing, "अंक अभी अंग्रेज़ी में दिखते हैं।"},
	{"priya", "tom", "Due dates in the past are not highlighted", "", ""},
	{"tom", "tom", "Upgrade the release checklist", issue.Done, "Checklist updated in the wiki."},
	{"tom", "alice", "Slow search on projects with many labels", "", ""},
}

// ensureProject creates the demo project unless it exists, then the demo
// issues it lacks (by summary), each by its reporter. It returns how many
// issues it created.
func ensureProject(ctx context.Context, conn *sql.DB, base string, admin *client, ids map[string]int64) (int, error) {
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE pkey = ?`, Project).Scan(&n); err != nil {
		return 0, err
	}
	if n == 0 {
		if err := admin.call("POST", "/api/projects", map[string]any{"key": Project, "name": "Demo",
			"description": "Sample project of the development database.", "template": "software",
			"lead": ids["alice"], "access": map[string]any{"holder": "everyone"}}, nil); err != nil {
			return 0, err
		}
	}
	var p struct {
		IssueTypes []struct {
			ID      string
			Subtask bool
		}
	}
	if err := admin.call("GET", "/api/projects/"+Project, nil, &p); err != nil {
		return 0, err
	}
	var typ string
	for _, t := range p.IssueTypes {
		if !t.Subtask {
			typ = t.ID
			break
		}
	}
	if typ == "" {
		return 0, errors.New("the project has no standard issue type")
	}
	clients := map[string]*client{}
	as := func(username string) (*client, error) {
		if c, ok := clients[username]; ok {
			return c, nil
		}
		c, err := login(ctx, base, username, UserPassword)
		clients[username] = c
		return c, err
	}
	created := 0
	for _, is := range issues {
		var exists int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues i JOIN projects p ON p.id = i.project_id
			WHERE p.pkey = ? AND i.summary = ?`, Project, is.summary).Scan(&exists); err != nil {
			return 0, err
		}
		if exists > 0 {
			continue
		}
		reporter, err := as(is.reporter)
		if err != nil {
			return 0, err
		}
		fields := map[string]any{"project": Project, "type": typ, "summary": is.summary}
		if is.assignee != "" {
			fields["assignee"] = ids[is.assignee]
		}
		var made struct{ Key string }
		if err := reporter.call("POST", "/api/issues", fields, &made); err != nil {
			return 0, fmt.Errorf("%q: %w", is.summary, err)
		}
		if is.status != "" {
			if err := moveTo(reporter, made.Key, is.status); err != nil {
				return 0, fmt.Errorf("%s: %w", made.Key, err)
			}
		}
		if is.comment != "" {
			assignee, err := as(is.assignee)
			if err != nil {
				return 0, err
			}
			if err := assignee.call("POST", "/api/issues/"+made.Key+"/comments",
				map[string]any{"body": is.comment}, nil); err != nil {
				return 0, fmt.Errorf("%s: %w", made.Key, err)
			}
		}
		created++
	}
	return created, nil
}

// moveTo moves an issue to the first status of a category.
func moveTo(c *client, key string, category issue.StatusCategory) error {
	var d struct {
		Statuses []struct{ ID, Category string }
	}
	if err := c.call("GET", "/api/issues/"+key, nil, &d); err != nil {
		return err
	}
	for _, s := range d.Statuses {
		if issue.StatusCategory(s.Category) == category {
			return c.call("PATCH", "/api/issues/"+key, map[string]any{"status": s.ID}, nil)
		}
	}
	return fmt.Errorf("no %s status", category)
}

// client is a signed-in session on the in-process server.
type client struct {
	base string
	http *http.Client
}

func login(ctx context.Context, base, username, password string) (*client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	c := &client{base: base, http: &http.Client{Jar: jar}}
	if err := c.call("POST", "/api/login", map[string]string{"username": username, "password": password}, nil); err != nil {
		return nil, fmt.Errorf("signing in as %s: %w", username, err)
	}
	return c, nil
}

// call sends a JSON request and decodes the answer into out, when given.
func (c *client) call(method, path string, body, out any) error {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Solstice", "1")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %d %s", method, path, res.StatusCode, bytes.TrimSpace(raw))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}
