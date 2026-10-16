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
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"

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

// The URL options accepted for the Jira database. The URL is typed in a form
// by an administrator and the connection is made by the Solstice process, so
// only options that are checked and mapped to typed driver settings get
// through; the others (MySQL: allowAllFiles, multiStatements, unknown names
// that the driver would send as SET statements; PostgreSQL: sslkey, sslcert,
// sslrootcert, passfile, service, servicefile, sslpassword, options, ...) are
// refused with an error naming them.
//
// MySQL: tls (encryption of the link), timeout and readTimeout (a slow or
// distant Jira server) and charset (Jira databases created in a legacy
// character set).
// PostgreSQL: sslmode (encryption of the link) and connect_timeout.
var (
	mysqlParams    = []string{soldb.ParamTLS, soldb.ParamCharset, soldb.ParamTimeout, soldb.ParamReadTimeout}
	postgresParams = []string{"sslmode", "connect_timeout"}
)

var sslModes = map[string]bool{"disable": true, "allow": true, "prefer": true, "require": true,
	"verify-ca": true, "verify-full": true}

// Open connects to postgres://… or mysql://user:pass@host:port/db. Options in
// the query of the URL are limited to a short list (see above); an error
// never contains the password.
func Open(rawURL string, loc *time.Location) (*sql.DB, Engine, error) {
	u, err := soldb.ParseURL(rawURL)
	if err != nil {
		return nil, 0, err
	}
	switch u.Scheme {
	case "postgres", "postgresql":
		connStr, err := postgresURL(u)
		if err != nil {
			return nil, 0, err
		}
		cfg, err := pgx.ParseConfigWithOptions(connStr, pgx.ParseConfigOptions{
			ParseConfigOptions: pgconn.ParseConfigOptions{ConnStringAllowedKeys: append([]string{
				"host", "port", "user", "password", "database"}, postgresParams...)},
		})
		if err != nil {
			return nil, 0, fmt.Errorf("invalid PostgreSQL database URL")
		}
		return stdlib.OpenDB(*cfg), Postgres, nil
	case "mysql", "mariadb":
		dsn, err := mysqlDSN(u, loc)
		if err != nil {
			return nil, 0, err
		}
		db, err := sql.Open("mysql", dsn)
		return db, MySQL, err
	default:
		if u.Host == "" {
			return nil, 0, fmt.Errorf("invalid database URL: expected postgres:// or mysql://")
		}
		return nil, 0, fmt.Errorf("unsupported database %q: expected postgres:// or mysql://", u.Scheme)
	}
}

// postgresURL rebuilds a postgres:// URL from its parts, keeping only the
// accepted options.
func postgresURL(u *url.URL) (string, error) {
	q, err := soldb.URLQuery(u)
	if err != nil {
		return "", err
	}
	var unknown []string
	for name := range q {
		if !slices.Contains(postgresParams, name) {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		accepted := append([]string(nil), postgresParams...)
		sort.Strings(accepted)
		return "", &soldb.UnknownParamsError{Names: unknown, Allowed: accepted}
	}
	keep := url.Values{}
	for name, values := range q {
		if len(values) != 1 {
			return "", &soldb.InvalidParamError{Name: name, Hint: "give it once"}
		}
		switch v := values[0]; name {
		case "sslmode":
			if !sslModes[v] {
				return "", &soldb.InvalidParamError{Name: name,
					Hint: "disable, allow, prefer, require, verify-ca or verify-full"}
			}
		case "connect_timeout":
			if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 600 {
				return "", &soldb.InvalidParamError{Name: name, Hint: "a number of seconds between 1 and 600"}
			}
		}
		keep.Set(name, values[0])
	}
	clean := url.URL{Scheme: "postgres", User: u.User, Host: u.Host, Path: u.Path, RawQuery: keep.Encode()}
	return clean.String(), nil
}

// mysqlDSN converts a mysql:// URL into a go-sql-driver DSN, built from
// validated parts only: the query of the URL is limited to mysqlParams and
// local file loading is off.
func mysqlDSN(u *url.URL, loc *time.Location) (string, error) {
	cfg := soldb.MySQLConfig(u)
	cfg.Loc = loc
	if cfg.Loc == nil {
		cfg.Loc = time.UTC
	}
	q, err := soldb.URLQuery(u)
	if err != nil {
		return "", err
	}
	if err := soldb.ApplyMySQLParams(cfg, q, mysqlParams...); err != nil {
		return "", err
	}
	return cfg.FormatDSN(), nil
}
