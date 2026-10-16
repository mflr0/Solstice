package db

import (
	"context"
	"errors"
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
		_, err := Open(context.Background(), raw)
		if err == nil {
			t.Errorf("%s: no error", raw)
			continue
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "zz") {
			t.Errorf("%s: error contains the password: %v", raw, err)
		}
	}
}
