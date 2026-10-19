package db

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"
)

func TestMySQLDSNKeepsTLS(t *testing.T) {
	for value, want := range map[string]string{
		"true": "tls=true", "skip-verify": "tls=skip-verify", "preferred": "tls=preferred",
	} {
		dsn, err := mysqlDSN("mysql://solstice:secret@db.example.org:3306/solstice?tls=" + value)
		if err != nil {
			t.Fatalf("tls=%s: %v", value, err)
		}
		if !strings.Contains(dsn, want) {
			t.Errorf("tls=%s: DSN %q lacks %q", value, dsn, want)
		}
		if !strings.Contains(dsn, "time_zone") || !strings.Contains(dsn, "charset=utf8mb4") {
			t.Errorf("tls=%s: session settings lost: %q", value, dsn)
		}
	}
	dsn, err := mysqlDSN("mariadb://solstice:secret@db/solstice?tls=false")
	if err != nil || strings.Contains(dsn, "tls=") {
		t.Errorf("tls=false: %q, %v", dsn, err)
	}
	dsn, err = mysqlDSN("mysql://solstice:secret@db/solstice")
	if err != nil || strings.Contains(dsn, "tls=") {
		t.Errorf("no option: %q, %v", dsn, err)
	}
}

func TestMySQLDSNRefusesOtherOptions(t *testing.T) {
	for _, q := range []string{"allowAllFiles=true", "charset=latin1", "time_zone=%27%2B05%3A00%27", "multiStatements=true",
		"tls=true&timeout=1s"} {
		_, err := mysqlDSN("mysql://solstice:secret@db/solstice?" + q)
		var unknown *UnknownParamsError
		if !errors.As(err, &unknown) {
			t.Errorf("%s: %v, want an unknown option error", q, err)
		} else if strings.Contains(err.Error(), "secret") {
			t.Errorf("%s: error contains the password: %v", q, err)
		}
	}
	for _, q := range []string{"tls=sometimes", "tls=", "tls=true&tls=false"} {
		_, err := mysqlDSN("mysql://solstice:secret@db/solstice?" + q)
		var invalid *InvalidParamError
		if !errors.As(err, &invalid) {
			t.Errorf("%s: %v, want an invalid value error", q, err)
		}
	}
}

// Whatever is wrong with the URL, the error never repeats the password.
func TestOpenErrorsNeverContainThePassword(t *testing.T) {
	for _, raw := range []string{
		"mysql://solstice:pa%zzss-secret@localhost/db",
		"mysql://solstice:secret@localhost/db?tls=%zz",
		"mysql://solstice:secret@localhost/db?tls=nope",
		"mysql://solstice:secret@localhost/db?allowAllFiles=true",
		"postgres://solstice:secret@localhost/db",
		"solstice:secret@localhost/db",
		"solstice:secret",
	} {
		_, err := Open(context.Background(), raw, Pool{})
		if err == nil {
			t.Errorf("%s: no error", raw)
			continue
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "zz") {
			t.Errorf("%s: error contains the password: %v", raw, err)
		}
	}
}

func TestCheckParams(t *testing.T) {
	var applied []string
	params := map[string]Param{
		"a": {"only ok", func(v string) bool { applied = append(applied, "a"); return v == "ok" }},
		"b": {"only ok", func(v string) bool { applied = append(applied, "b"); return v == "ok" }},
	}
	for _, c := range []struct {
		query       string
		unknown     []string // the names of an *UnknownParamsError
		invalid     string   // the name of an *InvalidParamError
		hint        string
		wantApplied []string
	}{
		{query: "", wantApplied: nil},
		{query: "b=ok&a=ok", wantApplied: []string{"a", "b"}},
		{query: "z=1&a=ok&y=2", unknown: []string{"y", "z"}},
		{query: "b=1&b=2&a=bad", invalid: "b", hint: "give it once"},
		{query: "b=bad&a=bad", invalid: "a", hint: "only ok", wantApplied: []string{"a"}},
	} {
		applied = nil
		q, err := url.ParseQuery(c.query)
		if err != nil {
			t.Fatal(err)
		}
		err = CheckParams(q, params)
		var unknown *UnknownParamsError
		var invalid *InvalidParamError
		switch {
		case c.unknown != nil:
			if !errors.As(err, &unknown) || !slices.Equal(unknown.Names, c.unknown) ||
				!slices.Equal(unknown.Allowed, []string{"a", "b"}) {
				t.Errorf("%q: %v, want unknown %v", c.query, err, c.unknown)
			}
		case c.invalid != "":
			if !errors.As(err, &invalid) || invalid.Name != c.invalid || invalid.Hint != c.hint {
				t.Errorf("%q: %v, want %s invalid (%s)", c.query, err, c.invalid, c.hint)
			}
		case err != nil:
			t.Errorf("%q: %v", c.query, err)
		}
		if !slices.Equal(applied, c.wantApplied) {
			t.Errorf("%q: applied %v, want %v", c.query, applied, c.wantApplied)
		}
	}
}
