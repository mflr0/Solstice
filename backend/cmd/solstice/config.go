package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"

	"solstice/internal/config"
	"solstice/internal/db"
)

// version is set at build time: go build -ldflags "-X main.version=1.2.0".
var version = "dev"

// loadConfig reads the configuration, sets up logging from it and reports
// unknown keys. Values are never logged: some are passwords.
func loadConfig() (*config.Config, error) {
	c, err := config.Load(os.Environ())
	if err != nil {
		return nil, err
	}
	setupLog(c.Log)
	for _, k := range c.Unknown {
		slog.Warn("unknown setting in the configuration file or the environment", "key", k, "file", c.Path)
	}
	return c, nil
}

func setupLog(l config.LogSettings) {
	if l.Format == config.LogJSON {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: l.Level})))
		return
	}
	// The default handler's text output, filtered.
	slog.SetLogLoggerLevel(l.Level)
}

// fixed tells whether the environment sets a key: the setup wizard cannot
// change it.
func fixed(c *config.Config, key string) bool { return c.Sources[key] == config.SourceEnv }

// configSources tells where each setting in effect comes from, for the
// administration's system page: env, file or default. baseURL and tz are
// the values in effect, which the setup wizard may have just written.
func configSources(c *config.Config, baseURL string, tz *time.Location) map[string]string {
	out := maps.Clone(c.Sources)
	if out[config.KeyBaseURL] == config.SourceDefault && baseURL != "" {
		out[config.KeyBaseURL] = config.SourceFile
	}
	if out[config.KeyTZ] == config.SourceDefault && tz != nil && tz != time.Local {
		out[config.KeyTZ] = config.SourceFile
	}
	return out
}

// openMigrated connects to Solstice's database and brings it up to date, for
// the commands that cannot run without one.
func openMigrated(ctx context.Context, c *config.Config) (*sql.DB, error) {
	if c.DB.URL == "" {
		return nil, fmt.Errorf("no database configured: set %s in the environment or in %s "+
			"(\"solstice serve\" with no database opens the setup wizard)", config.KeyDBURL, c.Path)
	}
	conn, err := db.Open(ctx, c.DB.URL, c.DB.Pool())
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// openDB connects to Solstice's database. The database may still be
// starting (containers started together): unreachable servers are retried
// for a minute; wrong credentials or URLs are not.
func openDB(ctx context.Context, rawURL string, pool db.Pool) (*sql.DB, error) {
	deadline := time.Now().Add(time.Minute)
	for {
		conn, err := db.Open(ctx, rawURL, pool)
		var oe *net.OpError
		retry := errors.As(err, &oe) || errors.Is(err, mysql.ErrInvalidConn) || errors.Is(err, driver.ErrBadConn)
		if err == nil || !retry || time.Now().After(deadline) {
			return conn, err
		}
		slog.Warn("database not reachable yet, retrying", "err", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// loadTZ reads a time zone name; "" is the system's.
func loadTZ(name string) (*time.Location, error) {
	if name == "" {
		return time.Local, nil
	}
	tz, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("unknown time zone %q: %w", name, err)
	}
	return tz, nil
}
