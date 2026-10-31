package tests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"solstice/internal/config"
	"solstice/internal/db"
	"solstice/internal/devseed"
	"solstice/internal/sqlq"
)

// The dev seed on an empty database, twice: the second run adds nothing.
// Then a database with accounts but no seed administrator is refused.
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

	type state struct{ accounts, issues, comments, projects string }
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
		return state{strings.Join(accounts, " "), issues, comments, strings.Join(projects, " ")}
	}

	var out strings.Builder
	if err := devseed.Run(ctx, conn, devseed.Options{SMTP: "localhost:1025", Out: &out}); err != nil {
		t.Fatalf("first run: %v\n%s", err, out.String())
	}
	first := read()
	want := state{"alice:en giulia:it hans:de marie:fr priya:hi seed*:- tom:-", "12", "6",
		"DEMO/Products/false/3/3(2,1) MOB/Products/false/2/2(1,0) OLD/-/true/1/1(1,0) OPS/Internal/false/2/0(0,0)"}
	if first != want {
		t.Fatalf("after the first run: %+v, want %+v", first, want)
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
