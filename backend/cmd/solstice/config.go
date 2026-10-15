package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"

	"solstice/internal/config"
	"solstice/internal/db"
)

// version is set at build time: go build -ldflags "-X main.version=1.2.0".
var version = "dev"

// settingFlags are the flags every command shares; they win over the
// environment and the configuration file.
type settingFlags struct {
	config, db, data *string
}

func commonFlags(fs *flag.FlagSet, withData bool) *settingFlags {
	f := &settingFlags{
		config: fs.String("config", "", "configuration file (default: $SOLSTICE_CONFIG, else ./solstice.conf, else /etc/solstice/solstice.conf)"),
		db:     fs.String("db", "", "MariaDB URL, mysql://user:password@host:3306/database (default: $SOLSTICE_DB or the configuration file)"),
	}
	if withData {
		f.data = fs.String("data", "", "data directory (default: $SOLSTICE_DATA, the configuration file, or ./data)")
	}
	return f
}

// load reads the configuration and applies the flags. fixed tells which
// settings the setup wizard cannot change: those given by the environment
// or on the command line.
func (f *settingFlags) load() (c *config.Config, fixed map[string]bool, err error) {
	c, err = config.Load(*f.config, os.LookupEnv)
	if err != nil {
		return nil, nil, err
	}
	fixed = map[string]bool{}
	for k, v := range c.FromEnv {
		fixed[k] = v
	}
	if *f.db != "" {
		c.DB, fixed[config.KeyDB] = *f.db, true
	}
	if f.data != nil && *f.data != "" {
		c.Data, fixed[config.KeyData] = *f.data, true
	}
	for _, k := range c.Unknown {
		slog.Warn("unknown setting in the configuration file", "key", k, "file", c.Path)
	}
	// Packages that read the environment themselves (the secret key…) see
	// the file's values too.
	for _, k := range config.Keys {
		if v := c.Get(k); v != "" && os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
	return c, fixed, nil
}

// configSources tells where each setting in effect comes from, for the
// administration's system page: env, flag, file or default. baseURL and tz
// are the values in effect, which the setup wizard may have just written.
func configSources(c *config.Config, fixed map[string]bool, baseURL string, tz *time.Location) map[string]string {
	defaults := map[string]string{config.KeyData: config.DefaultData, config.KeyAddr: config.DefaultAddr}
	out := map[string]string{}
	for _, k := range config.Keys {
		v := c.Get(k)
		switch {
		case c.FromEnv[k]:
			out[k] = "env"
		case fixed[k]:
			out[k] = "flag"
		case v == "" || v == defaults[k]:
			out[k] = "default"
		default:
			out[k] = "file"
		}
	}
	if out[config.KeyBaseURL] == "default" && baseURL != "" {
		out[config.KeyBaseURL] = "file"
	}
	if out[config.KeyTZ] == "default" && tz != nil && tz != time.Local {
		out[config.KeyTZ] = "file"
	}
	return out
}

// requireDB is for the commands that cannot run without a database.
func requireDB(c *config.Config) error {
	if c.DB == "" {
		return fmt.Errorf("no database configured: set SOLSTICE_DB, pass -db, or put SOLSTICE_DB in %s "+
			"(\"solstice serve\" with no database opens the setup wizard)", c.Path)
	}
	return nil
}

// openDB connects to Solstice's database. The database may still be
// starting (containers started together): unreachable servers are retried
// for a minute; wrong credentials or URLs are not.
func openDB(ctx context.Context, rawURL string) (*sql.DB, error) {
	deadline := time.Now().Add(time.Minute)
	for {
		conn, err := db.Open(ctx, rawURL)
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
