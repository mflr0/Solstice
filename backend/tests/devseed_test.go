package tests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/config"
	"solstice/internal/db"
	"solstice/internal/devseed"
	"solstice/internal/sqlq"
)

// The dev seed on an empty database, twice: the second run adds nothing.
// The counts pin what the issues cover: every type, epics and sub-tasks,
// links, work logs, attachments and custom field values; and the groups,
// the security level, group grants and recipients, the restricted comment
// and work log, the custom workflow and the webhook. Through the API, tom
// sees neither the confidential issue nor what is restricted, a developer
// does. Then a database with accounts but no seed administrator is refused.
func TestDevSeed(t *testing.T) {
	dsn := emptyDB(t)
	ctx := context.Background()
	conn, err := db.Open(ctx, dsn, config.Defaults().DB.Pool())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}

	type state struct{ accounts, issues, comments, projects, types, extras, boards, access string }
	read := func() state {
		var accounts []string
		if err := sqlq.Each(ctx, conn, `SELECT username, COALESCE(locale, '-'), is_admin FROM users ORDER BY username`,
			func(rows *sql.Rows) error {
				var name, locale string
				var admin bool
				if err := rows.Scan(&name, &locale, &admin); err != nil {
					return err
				}
				if admin {
					name += "*"
				}
				accounts = append(accounts, name+":"+locale)
				return nil
			}); err != nil {
			t.Fatal(err)
		}
		var issues, comments string
		if err := conn.QueryRowContext(ctx, `SELECT CAST(COUNT(*) AS CHAR), (SELECT CAST(COUNT(*) AS CHAR) FROM comments)
			FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.pkey = ?`, devseed.Project).Scan(&issues, &comments); err != nil {
			t.Fatal(err)
		}
		// key/category/archived/components/versions (released, archived)
		var projects []string
		if err := sqlq.Each(ctx, conn, `SELECT p.pkey, COALESCE(c.name, '-'), p.archived,
			(SELECT COUNT(*) FROM components WHERE project_id = p.id),
			(SELECT COUNT(*) FROM versions WHERE project_id = p.id),
			(SELECT COUNT(*) FROM versions WHERE project_id = p.id AND released),
			(SELECT COUNT(*) FROM versions WHERE project_id = p.id AND archived)
			FROM projects p LEFT JOIN project_categories c ON c.id = p.category_id ORDER BY p.pkey`,
			func(rows *sql.Rows) error {
				var key, cat string
				var archived bool
				var components, versions, released, archivedVersions int
				if err := rows.Scan(&key, &cat, &archived, &components, &versions, &released, &archivedVersions); err != nil {
					return err
				}
				projects = append(projects, fmt.Sprintf("%s/%s/%t/%d/%d(%d,%d)", key, cat, archived, components,
					versions, released, archivedVersions))
				return nil
			}); err != nil {
			t.Fatal(err)
		}
		// key: type=count …, per project
		var types []string
		if err := sqlq.Each(ctx, conn, `SELECT p.pkey, t.name, COUNT(*) FROM issues i
			JOIN projects p ON p.id = i.project_id JOIN issue_types t ON t.id = i.issue_type_id
			GROUP BY p.pkey, t.name ORDER BY p.pkey, t.name`, func(rows *sql.Rows) error {
			var key, typ string
			var n int
			if err := rows.Scan(&key, &typ, &n); err != nil {
				return err
			}
			types = append(types, fmt.Sprintf("%s:%s=%d", key, typ, n))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		var extras []string
		for _, c := range []struct{ name, query string }{
			{"subtasks", `SELECT COUNT(*) FROM issues WHERE parent_id IS NOT NULL`},
			{"in-epics", `SELECT COUNT(*) FROM issues WHERE epic_id IS NOT NULL`},
			{"priorities", `SELECT COUNT(DISTINCT priority_id) FROM issues`},
			{"resolved", `SELECT COUNT(*) FROM issues WHERE resolution_id IS NOT NULL`},
			{"overdue", `SELECT COUNT(*) FROM issues WHERE due_date < CURRENT_DATE`},
			{"estimated", `SELECT COUNT(*) FROM issues WHERE original_estimate IS NOT NULL`},
			{"labels", `SELECT COUNT(*) FROM labels WHERE field_id IS NULL`},
			{"components", `SELECT COUNT(*) FROM issue_components`},
			{"fix-versions", `SELECT COUNT(*) FROM issue_versions WHERE kind = 'fix'`},
			{"links", `SELECT COUNT(*) FROM issue_links`},
			{"cross-links", `SELECT COUNT(*) FROM issue_links l JOIN issues s ON s.id = l.source_id
				JOIN issues d ON d.id = l.target_id WHERE s.project_id <> d.project_id`},
			{"link-types", `SELECT COUNT(DISTINCT type_id) FROM issue_links`},
			{"worklogs", `SELECT COUNT(*) FROM worklogs`},
			{"worklog-authors", `SELECT COUNT(DISTINCT author_id) FROM worklogs`},
			{"attachments", `SELECT COUNT(*) FROM attachments WHERE size > 0`},
			{"votes", `SELECT COUNT(*) FROM votes`},
			{"watchers", `SELECT COUNT(*) FROM watchers`},
			{"fields", `SELECT COUNT(*) FROM custom_fields`},
			{"field-types", `SELECT COUNT(DISTINCT type_key) FROM custom_fields`},
			{"field-values", `SELECT COUNT(*) FROM (SELECT DISTINCT issue_id, field_id FROM custom_field_values
				UNION SELECT DISTINCT issue_id, field_id FROM labels WHERE field_id IS NOT NULL) v`},
			{"reporters", `SELECT COUNT(DISTINCT reporter_id) FROM issues`},
		} {
			var n int
			if err := conn.QueryRowContext(ctx, c.query).Scan(&n); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}
			extras = append(extras, fmt.Sprintf("%s=%d", c.name, n))
		}
		// kind/columns/sprints closed, active, future/issues on the board's projects
		var boards []string
		if err := sqlq.Each(ctx, conn, `SELECT b.kind, (SELECT COUNT(*) FROM board_columns WHERE board_id = b.id),
			(SELECT COUNT(*) FROM sprints WHERE board_id = b.id AND state = 'closed'),
			(SELECT COUNT(*) FROM sprints WHERE board_id = b.id AND state = 'active'),
			(SELECT COUNT(*) FROM sprints WHERE board_id = b.id AND state = 'future'),
			(SELECT COUNT(DISTINCT si.issue_id) FROM sprint_issues si JOIN sprints sp ON sp.id = si.sprint_id
				WHERE sp.board_id = b.id)
			FROM boards b ORDER BY b.kind`,
			func(rows *sql.Rows) error {
				var kind string
				var columns, closed, active, future, inSprints int
				if err := rows.Scan(&kind, &columns, &closed, &active, &future, &inSprints); err != nil {
					return err
				}
				boards = append(boards, fmt.Sprintf("%s/%d/%d,%d,%d/%d", kind, columns, closed, active, future, inSprints))
				return nil
			}); err != nil {
			t.Fatal(err)
		}
		// One string column per row, rows joined by spaces.
		var access []string
		for _, q := range []string{
			`SELECT CONCAT(g.name, '=', GROUP_CONCAT(u.username ORDER BY u.username)) FROM user_groups g
				JOIN group_members m ON m.group_id = g.id JOIN users u ON u.id = m.user_id GROUP BY g.name ORDER BY g.name`,
			`SELECT CONCAT(p.pkey, '/', l.name, '=', GROUP_CONCAT(m.holder, ':', COALESCE(g.name, '-')), '/',
				(SELECT COUNT(*) FROM issues WHERE security_level_id = l.id))
				FROM security_levels l JOIN projects p ON p.id = l.project_id
				JOIN security_level_members m ON m.level_id = l.id LEFT JOIN user_groups g ON g.id = m.group_id
				GROUP BY p.pkey, l.id, l.name ORDER BY p.pkey, l.name`,
			`SELECT CONCAT(p.pkey, ':', g.name, '=', GROUP_CONCAT(pp.permission ORDER BY pp.permission))
				FROM project_permissions pp JOIN projects p ON p.id = pp.project_id JOIN user_groups g ON g.id = pp.group_id
				GROUP BY p.pkey, g.name ORDER BY p.pkey, g.name`,
			`SELECT CONCAT(p.pkey, ':', r.event, '>', g.name) FROM project_notification_recipients r
				JOIN projects p ON p.id = r.project_id JOIN user_groups g ON g.id = r.group_id ORDER BY 1`,
			`SELECT CONCAT('restricted-comments=', COUNT(*)) FROM comments WHERE visibility_group IS NOT NULL`,
			`SELECT CONCAT('restricted-worklogs=', COUNT(*)) FROM worklogs WHERE visibility_group IS NOT NULL`,
			`SELECT CONCAT(p.pkey, '/', w.name, '=', (SELECT GROUP_CONCAT(s.name ORDER BY ws.position, s.name)
				FROM workflow_statuses ws JOIN statuses s ON s.id = ws.status_id WHERE ws.workflow_id = w.id), '/',
				(SELECT GROUP_CONCAT(t.name ORDER BY t.name) FROM workflow_issue_types wt
				JOIN issue_types t ON t.id = wt.issue_type_id WHERE wt.workflow_id = w.id))
				FROM workflows w JOIN projects p ON p.id = w.project_id WHERE NOT w.is_default ORDER BY 1`,
			`SELECT CONCAT('webhook:', name, '>', url) FROM webhooks ORDER BY 1`,
		} {
			if err := sqlq.Each(ctx, conn, q, func(rows *sql.Rows) error {
				var v string
				err := rows.Scan(&v)
				access = append(access, v)
				return err
			}); err != nil {
				t.Fatalf("%v\n%s", err, q)
			}
		}
		return state{strings.Join(accounts, " "), issues, comments, strings.Join(projects, " "),
			strings.Join(types, " "), strings.Join(extras, " "), strings.Join(boards, " "), strings.Join(access, " ")}
	}

	var out strings.Builder
	if err := devseed.Run(ctx, conn, devseed.Options{SMTP: "localhost:1025", Out: &out}); err != nil {
		t.Fatalf("first run: %v\n%s", err, out.String())
	}
	first := read()
	want := state{"alice:en giulia:it hans:de marie:fr priya:hi seed*:- tom:-", "41", "15",
		"DEMO/Products/false/3/3(2,1) MOB/Products/false/2/2(1,0) OLD/-/true/1/1(1,0) OPS/Internal/false/2/0(0,0)",
		"DEMO:Bug=8 DEMO:Epic=2 DEMO:Story=24 DEMO:Sub-task=3 DEMO:Task=4 " +
			"MOB:Bug=1 MOB:Epic=1 MOB:Story=1 MOB:Sub-task=1 MOB:Task=10 OPS:Bug=1 OPS:Epic=1 OPS:Task=6",
		"subtasks=4 in-epics=5 priorities=5 resolved=24 overdue=3 estimated=9 labels=16 components=17 " +
			"fix-versions=12 links=8 cross-links=3 link-types=4 worklogs=13 worklog-authors=6 attachments=4 " +
			"votes=12 watchers=67 fields=13 field-types=13 field-values=49 reporters=6",
		"kanban/4/0,0,0/0 scrum/3/2,1,1/18",
		"developers=alice,giulia,hans,priya office=marie,tom DEMO/Developers only=group:developers/1 " +
			"DEMO:developers=moderate,security OPS:created>office restricted-comments=1 restricted-worklogs=1 " +
			"OPS/Purchases=To Do,In Progress,Done,Awaiting approval/Task webhook:Local listener (not running)>http://127.0.0.1:9/solstice"}
	if first != want {
		t.Fatalf("after the first run: %+v, want %+v", first, want)
	}
	// The reports need a history: closed sprints in the past, in order, and
	// no change dated after now.
	var badSprints, future int
	if err := conn.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM sprints WHERE state <> 'future' AND (start_at >= UTC_TIMESTAMP() - INTERVAL 1 DAY
			OR (state = 'closed' AND (completed_at >= UTC_TIMESTAMP() - INTERVAL 1 DAY OR completed_at <= start_at)))),
		(SELECT COUNT(*) FROM change_groups WHERE created_at > UTC_TIMESTAMP())`).Scan(&badSprints, &future); err != nil {
		t.Fatal(err)
	}
	if badSprints != 0 || future != 0 {
		t.Errorf("%d sprints not dated in the past, %d changes dated in the future", badSprints, future)
	}
	var host string
	if err := conn.QueryRowContext(ctx, `SELECT value FROM settings WHERE name = 'mail'`).Scan(&host); err != nil ||
		!strings.Contains(host, `"localhost"`) {
		t.Errorf("mail settings: %q %v", host, err)
	}

	out.Reset()
	if err := devseed.Run(ctx, conn, devseed.Options{SMTP: "localhost:1025", Out: &out}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if again := read(); again != first {
		t.Errorf("the second run changed the database: %+v, was %+v", again, first)
	}
	if out.Len() != 0 {
		t.Errorf("the second run created something:\n%s", out.String())
	}

	// JQL on what the seed set, custom fields included. The database is
	// created with a collation other than the connection's, as in CI.
	ts := httptest.NewServer(api.New(conn, t.TempDir(), time.UTC, quiet()).Handler(http.NotFoundHandler()))
	defer ts.Close()
	jar, _ := cookiejar.New(nil)
	alice := &client{t: t, base: ts.URL, http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
	if status, body := alice.do("POST", "/api/login", map[string]string{"username": "alice", "password": devseed.UserPassword}); status != 200 {
		t.Fatalf("login alice: %d %s", status, body)
	}
	as := func(username string) *client {
		jar, _ := cookiejar.New(nil)
		c := &client{t: t, base: ts.URL, http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
		if status, body := c.do("POST", "/api/login", map[string]string{"username": username,
			"password": devseed.UserPassword}); status != 200 {
			t.Fatalf("login %s: %d %s", username, status, body)
		}
		return c
	}

	// What a group restricts, seen by tom (in no group of the level) and by
	// hans (a developer).
	var confidential, umlaut string
	key := `(SELECT CONCAT(p.pkey, '-', i.num) FROM issues i JOIN projects p ON p.id = i.project_id WHERE i.summary = ?)`
	if err := conn.QueryRowContext(ctx, `SELECT `+key+`, `+key, "Session cookie sent to other subdomains",
		"Suche nach Umlauten findet nichts").Scan(&confidential, &umlaut); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		user                  string
		issue, comments, logs int
	}{{"tom", 404, 2, 3}, {"hans", 200, 3, 4}} {
		u := as(c.user)
		var comments []struct{ Body string }
		var logs []struct{ ID int64 }
		if status := u.get("/api/issues/"+confidential, nil); status != c.issue {
			t.Errorf("%s reads the confidential issue %s: %d, want %d", c.user, confidential, status, c.issue)
		}
		if status := u.get("/api/issues/"+umlaut+"/comments", &comments); status != 200 || len(comments) != c.comments {
			t.Errorf("%s reads the comments of %s: %d, %d comments, want %d", c.user, umlaut, status, len(comments),
				c.comments)
		}
		if status := u.get("/api/issues/"+umlaut+"/worklogs", &logs); status != 200 || len(logs) != c.logs {
			t.Errorf("%s reads the work logs of %s: %d, %d work logs, want %d", c.user, umlaut, status, len(logs),
				c.logs)
		}
	}
	for q, want := range map[string]int{
		`labels = auth`:                      2,
		`priority = "Very high"`:             3,
		`"Severity" = Blocker`:               2,
		`"QA owner" = giulia`:                1,
		`"Story points" > 4`:                 13,
		`project = DEMO ORDER BY "Severity"`: 41,
	} {
		var res struct{ Total int }
		if status := alice.get("/api/issues?q="+url.QueryEscape(q), &res); status != 200 || res.Total != want {
			t.Errorf("%s: %d, %d issues, want %d", q, status, res.Total, want)
		}
	}

	if _, err := conn.ExecContext(ctx, `UPDATE users SET username = 'not-seed' WHERE username = ?`, devseed.Admin); err != nil {
		t.Fatal(err)
	}
	if err := devseed.Run(ctx, conn, devseed.Options{}); !errors.Is(err, devseed.ErrNotDev) {
		t.Errorf("a database without the seed administrator: %v, want ErrNotDev", err)
	}
	names := strings.Fields(read().accounts)
	if slices.ContainsFunc(names, func(n string) bool { return strings.HasPrefix(n, devseed.Admin+"*") }) {
		t.Error("the refused run created the administrator")
	}
}
