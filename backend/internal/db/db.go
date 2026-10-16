// Package db opens Solstice's MariaDB database and applies its migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Pool sizes the connection pool, as sql.DB's setters of the same names.
type Pool struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Open connects to mysql://user:pass@host:port/db (MariaDB). Every session
// runs in UTC so DATETIME columns always hold UTC. The one option accepted
// after "?" is tls (true, false, skip-verify or preferred) to encrypt the
// link to a remote server; any other option is an error. Errors never
// contain the URL, which holds the password.
func Open(ctx context.Context, rawURL string, pool Pool) (*sql.DB, error) {
	dsn, err := mysqlDSN(rawURL)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(pool.MaxOpenConns)
	db.SetMaxIdleConns(pool.MaxIdleConns)
	db.SetConnMaxLifetime(pool.ConnMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot reach the database: %w", err)
	}
	return db, nil
}

// mysqlDSN turns the URL of Solstice's database into a driver DSN.
func mysqlDSN(rawURL string) (string, error) {
	u, err := ParseURL(rawURL)
	if err != nil {
		return "", err
	}
	if u.Scheme != "mysql" && u.Scheme != "mariadb" {
		if u.Host == "" {
			return "", errors.New("invalid database URL: expected mysql://user:password@host:3306/database")
		}
		return "", fmt.Errorf("unsupported database %q: expected mysql:// or mariadb://", u.Scheme)
	}
	cfg := MySQLConfig(u)
	q, err := URLQuery(u)
	if err != nil {
		return "", err
	}
	if err := ApplyMySQLParams(cfg, q, ParamTLS); err != nil {
		return "", err
	}
	cfg.Loc = time.UTC
	cfg.Params = map[string]string{"time_zone": "'+00:00'", "charset": "utf8mb4"}
	return cfg.FormatDSN(), nil
}

// MySQLConfig is the driver configuration for a mysql:// URL: credentials,
// address (port 3306 by default), database and parsed times. Callers set the
// time zone and parameters.
func MySQLConfig(u *url.URL) *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.User = u.User.Username()
	cfg.Passwd, _ = u.User.Password()
	cfg.Net = "tcp"
	cfg.Addr = u.Host
	if u.Port() == "" {
		cfg.Addr = u.Host + ":3306"
	}
	cfg.DBName = strings.TrimPrefix(u.Path, "/")
	cfg.ParseTime = true
	return cfg
}

// Migrate applies the embedded migrations that have not run yet, in name order.
// MariaDB does not roll back DDL, so a failing migration is reported with the
// statement that failed and must be fixed by hand.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    VARCHAR(255) NOT NULL PRIMARY KEY,
		applied_at DATETIME(3) NOT NULL
	)`); err != nil {
		return err
	}
	applied := map[string]bool{}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimSuffix(strings.TrimPrefix(name, "migrations/"), ".sql")
		if applied[version] {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		for _, stmt := range splitStatements(string(body)) {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("migration %s failed: %w\n%s", version, err, stmt)
			}
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			version, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

// splitStatements cuts a migration file on semicolons that end a line.
// Line comments are dropped. Migrations never contain procedures or triggers,
// so nothing smarter is needed.
func splitStatements(src string) []string {
	var out []string
	var cur strings.Builder
	for line := range strings.Lines(src) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		cur.WriteString(line)
		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSuffix(strings.TrimSpace(cur.String()), ";")
			out = append(out, stmt)
			cur.Reset()
		}
	}
	if rest := strings.TrimSpace(cur.String()); rest != "" {
		out = append(out, rest)
	}
	return out
}
