package jiradb

import (
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	soldb "solstice/internal/db"
)

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestMySQLDSNRefusesDriverOptions(t *testing.T) {
	refused := []string{
		"allowAllFiles=true",
		"allowAllFiles=false",
		"allowCleartextPasswords=true",
		"allowFallbackToPlaintext=true",
		"allowOldPasswords=true",
		"multiStatements=true",
		"clientFoundRows=true",
		"maxAllowedPacket=0",
		"sql_mode=ANSI",
		"time_zone=%27%2B01%3A00%27",
		"tls=true&allowAllFiles=true",
	}
	for _, q := range refused {
		u := mustParse(t, "mysql://jira:secret@db.example.org:3306/jira?"+q)
		dsn, err := mysqlDSN(u, time.UTC)
		var unknown *soldb.UnknownParamsError
		if !errors.As(err, &unknown) {
			t.Errorf("%s: error %v, want an unknown option error", q, err)
			continue
		}
		if dsn != "" {
			t.Errorf("%s: DSN %q returned with the error", q, dsn)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: error mentions the password: %v", q, err)
		}
		for _, name := range unknown.Names {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("%s: error does not name %q: %v", q, name, err)
			}
		}
	}
}

func TestMySQLDSNKeepsAcceptedOptions(t *testing.T) {
	u := mustParse(t, "mysql://jira:secret@db.example.org/jira?tls=skip-verify&timeout=10s&readTimeout=5m&charset=utf8mb4")
	dsn, err := mysqlDSN(u, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"tls=skip-verify", "timeout=10s", "readTimeout=5m", "charset=utf8mb4", "db.example.org:3306"} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN %q lacks %q", dsn, want)
		}
	}
	if strings.Contains(strings.ToLower(dsn), "allowallfiles") {
		t.Errorf("DSN enables local files: %q", dsn)
	}
	for _, v := range []string{"true", "false", "skip-verify", "preferred"} {
		if _, err := mysqlDSN(mustParse(t, "mysql://u:p@h/d?tls="+v), nil); err != nil {
			t.Errorf("tls=%s refused: %v", v, err)
		}
	}
}

func TestMySQLDSNInvalidValues(t *testing.T) {
	for _, q := range []string{"tls=maybe", "tls=", "tls=true&tls=false", "charset=utf8;DROP", "timeout=forever", "timeout=0s",
		"readTimeout=-5s"} {
		_, err := mysqlDSN(mustParse(t, "mysql://u:secret@h/d?"+q), nil)
		var invalid *soldb.InvalidParamError
		if !errors.As(err, &invalid) {
			t.Errorf("%s: error %v, want an invalid value error", q, err)
			continue
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "DROP") {
			t.Errorf("%s: error repeats the value: %v", q, err)
		}
	}
}

func TestMySQLDSNWithoutOptions(t *testing.T) {
	dsn, err := mysqlDSN(mustParse(t, "mysql://jira:secret@db.example.org:3307/jira"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "jira:secret@tcp(db.example.org:3307)/jira") || strings.Contains(dsn, "tls=") {
		t.Errorf("unexpected DSN %q", dsn)
	}
}

func TestPostgresURLRefusesFileAndCommandOptions(t *testing.T) {
	for _, q := range []string{
		"sslkey=/etc/solstice/secret.key",
		"sslcert=/etc/passwd",
		"sslrootcert=/etc/passwd",
		"sslpassword=x",
		"passfile=/home/solstice/.pgpass",
		"service=jira",
		"servicefile=/tmp/svc",
		"options=-c%20log_statement%3Dall",
		"host=/var/run/postgresql",
		"target_session_attrs=read-write",
		"sslmode=require&sslkey=/x",
	} {
		_, _, err := Open("postgres://jira:secret@db.example.org/jira?"+q, nil)
		var unknown *soldb.UnknownParamsError
		if !errors.As(err, &unknown) {
			t.Errorf("%s: error %v, want an unknown option error", q, err)
			continue
		}
		if strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: error mentions the password: %v", q, err)
		}
	}
}

func TestPostgresURLKeepsAcceptedOptions(t *testing.T) {
	got, err := postgresURL(mustParse(t, "postgres://jira:se%40cret@db.example.org:5432/jira?sslmode=verify-full&connect_timeout=10"))
	if err != nil {
		t.Fatal(err)
	}
	u := mustParse(t, got)
	if u.Query().Get("sslmode") != "verify-full" || u.Query().Get("connect_timeout") != "10" ||
		u.Host != "db.example.org:5432" || u.Path != "/jira" {
		t.Errorf("unexpected URL %q", got)
	}
	if pw, _ := u.User.Password(); pw != "se@cret" {
		t.Errorf("password changed: %q", got)
	}
	db, eng, err := Open("postgres://jira:secret@db.example.org/jira?sslmode=require", nil)
	if err != nil || eng != Postgres {
		t.Fatalf("Open: %v %v", eng, err)
	}
	db.Close()
	for _, q := range []string{"sslmode=sometimes", "connect_timeout=abc", "connect_timeout=0", "sslmode=require&sslmode=disable"} {
		_, err := postgresURL(mustParse(t, "postgres://u:secret@h/d?"+q))
		var invalid *soldb.InvalidParamError
		if !errors.As(err, &invalid) {
			t.Errorf("%s: error %v, want an invalid value error", q, err)
		}
	}
}

// With several bad options, the error names the first in sorted order, on
// every run (the query is a map, walked in random order).
func TestPostgresURLReportsTheFirstBadOption(t *testing.T) {
	for range 50 {
		_, err := postgresURL(mustParse(t, "postgres://u:secret@h/d?sslmode=bad&connect_timeout=0"))
		var invalid *soldb.InvalidParamError
		if !errors.As(err, &invalid) || invalid.Name != "connect_timeout" {
			t.Fatalf("error %v, want connect_timeout invalid", err)
		}
		_, err = postgresURL(mustParse(t, "postgres://u:secret@h/d?sslkey=/x&passfile=/y&service=z"))
		var unknown *soldb.UnknownParamsError
		if !errors.As(err, &unknown) || !slices.Equal(unknown.Names, []string{"passfile", "service", "sslkey"}) {
			t.Fatalf("error %v, want passfile, service and sslkey unknown", err)
		}
	}
}

// Quote keeps the case on both engines: Active Objects tables are upper-case
// on PostgreSQL, and "lead" is a reserved word on MySQL.
func TestQuote(t *testing.T) {
	cases := []struct {
		eng  Engine
		name string
		want string
	}{
		{MySQL, "AO_60DB71_SPRINT", "`AO_60DB71_SPRINT`"},
		{Postgres, "AO_60DB71_SPRINT", `"AO_60DB71_SPRINT"`}, // AO tables stay upper-case on PostgreSQL
		{MySQL, "lead", "`lead`"},
		{Postgres, "lead", `"lead"`},
	}
	for _, c := range cases {
		if got := c.eng.Quote(c.name); got != c.want {
			t.Errorf("Quote(%s, %q) = %q, want %q", c.eng, c.name, got, c.want)
		}
	}
}

// QuoteFolded folds to lower case on PostgreSQL only, where Jira creates its
// tables unquoted.
func TestQuoteFolded(t *testing.T) {
	cases := []struct {
		eng  Engine
		name string
		want string
	}{
		{MySQL, "OS_CURRENTSTEP", "`OS_CURRENTSTEP`"}, // MySQL table names are case-sensitive
		{Postgres, "OS_CURRENTSTEP", `"os_currentstep"`},
		{MySQL, "local", "`local`"},
		{Postgres, "local", `"local"`},
		{Postgres, "pk_jiraissue", `"pk_jiraissue"`},
	}
	for _, c := range cases {
		if got := c.eng.QuoteFolded(c.name); got != c.want {
			t.Errorf("QuoteFolded(%s, %q) = %q, want %q", c.eng, c.name, got, c.want)
		}
	}
}

func TestOpenErrorsNeverContainThePassword(t *testing.T) {
	for _, raw := range []string{
		"mysql://u:pa%zzss-secret@localhost/db",
		"postgres://u:pa%zzss-secret@localhost/db",
		"oracle://u:secret@localhost/db",
		"u:secret@localhost/db",
		"mysql://u:secret@localhost/db?allowAllFiles=true",
	} {
		_, _, err := Open(raw, nil)
		if err == nil {
			t.Errorf("%s: no error", raw)
			continue
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "zz") {
			t.Errorf("%s: error contains the password: %v", raw, err)
		}
	}
}
