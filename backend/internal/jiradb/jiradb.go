// Package jiradb opens a connection to a Jira Data Center database, on
// PostgreSQL or MySQL, from a URL.
//
// Jira stores timestamps differently per engine: TIMESTAMPTZ on PostgreSQL
// (absolute instants) but DATETIME on MySQL, i.e. the wall-clock time of the
// JVM that wrote it. Open therefore takes the time zone of the Jira server,
// which the MySQL driver uses to read (and write) DATETIME values.
package jiradb

import (
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver

	soldb "solstice/internal/db"
)

// Engine is the database engine behind a Jira instance.
type Engine int

const (
	Postgres Engine = iota
	MySQL
)

func (e Engine) String() string {
	if e == MySQL {
		return "mysql"
	}
	return "postgres"
}

// Placeholder returns the n-th (1-based) bind parameter marker.
func (e Engine) Placeholder(n int) string {
	if e == MySQL {
		return "?"
	}
	return "$" + strconv.Itoa(n)
}

// Open connects to postgres://… or mysql://user:pass@host:port/db.
func Open(rawURL string, loc *time.Location) (*sql.DB, Engine, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid database URL: %w", err)
	}
	switch u.Scheme {
	case "postgres", "postgresql":
		db, err := sql.Open("pgx", rawURL)
		return db, Postgres, err
	case "mysql", "mariadb":
		db, err := sql.Open("mysql", mysqlDSN(u, loc))
		return db, MySQL, err
	default:
		return nil, 0, fmt.Errorf("unsupported database %q: expected postgres:// or mysql://", u.Scheme)
	}
}

// mysqlDSN converts a mysql:// URL into a go-sql-driver DSN. Query
// parameters of the URL are passed through to the driver.
func mysqlDSN(u *url.URL, loc *time.Location) string {
	cfg := soldb.MySQLConfig(u)
	cfg.Loc = loc
	if cfg.Loc == nil {
		cfg.Loc = time.UTC
	}
	params := map[string]string{}
	for k, v := range u.Query() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	if len(params) > 0 {
		cfg.Params = params
	}
	return cfg.FormatDSN()
}
