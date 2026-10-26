package main

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"solstice/internal/config"
)

func TestRunUserRefusals(t *testing.T) {
	t.Setenv(config.KeyConfig, filepath.Join(t.TempDir(), "absent.conf"))
	t.Setenv(config.KeyDBURL, "")
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no action", nil, "usage: solstice user"},
		{"unknown action", []string{"delete", "-username", "me"}, "usage: solstice user"},
		{"add without username", []string{"add", "-name", "Me"}, "-username is required"},
		{"passwd without username", []string{"passwd"}, "-username is required"},
		{"add without database", []string{"add", "-username", "me", "-name", "Me", "-admin"}, "no database configured"},
		{"passwd without database", []string{"passwd", "-username", "me"}, "no database configured"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runUser(context.Background(), tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want an error containing %q", err, tc.want)
			}
		})
	}
}

func TestCheckAccount(t *testing.T) {
	const pw = "long enough pw"
	for _, tc := range []struct {
		name   string
		action string
		in     account
		min    int
		want   account
		err    string
	}{
		{"documented add", "add", account{login: "me", name: "Me", password: pw}, 12,
			account{login: "me", name: "Me", password: pw}, ""},
		{"login lowercased and trimmed, name defaults to it", "add", account{login: " Marie.D@ox ", name: "  ", password: pw}, 12,
			account{login: "marie.d@ox", name: "marie.d@ox", password: pw}, ""},
		{"name and e-mail trimmed", "add", account{login: "me", name: " Me ", email: " me@example.org ", password: pw}, 12,
			account{login: "me", name: "Me", email: "me@example.org", password: pw}, ""},
		{"login with a space", "add", account{login: "bob smith", password: pw}, 12, account{}, "invalid -username"},
		{"login starting with a dot", "add", account{login: ".me", password: pw}, 12, account{}, "invalid -username"},
		{"login of 101 characters", "add", account{login: strings.Repeat("a", 101), password: pw}, 12, account{}, "invalid -username"},
		{"empty login", "add", account{login: " ", password: pw}, 12, account{}, "invalid -username"},
		{"e-mail with a name", "add", account{login: "me", email: `"Me" <me@example.org>`, password: pw}, 12, account{}, "invalid -email"},
		{"not an e-mail", "add", account{login: "me", email: "me", password: pw}, 12, account{}, "invalid -email"},
		{"password below the minimum", "add", account{login: "me", password: "short"}, 12, account{}, "at least 12 characters"},
		{"password below a raised minimum", "add", account{login: "me", password: pw}, 20, account{}, "at least 20 characters"},
		{"password is the login", "add", account{login: "longusername1", password: "LongUsername1"}, 12, account{}, "cannot be the username"},
		{"passwd keeps a login the add rule refuses", "passwd", account{login: "Old Login", password: pw}, 12,
			account{login: "old login", password: pw}, ""},
		{"passwd password below the minimum", "passwd", account{login: "me", password: "short"}, 12, account{}, "at least 12 characters"},
		{"passwd password is the login", "passwd", account{login: "longusername1", password: "longusername1"}, 12, account{}, "cannot be the username"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := checkAccount(tc.action, tc.in, tc.min)
			if tc.err == "" {
				if err != nil || got != tc.want {
					t.Errorf("got %+v, %v; want %+v", got, err, tc.want)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("got %+v, %v; want an error containing %q", got, err, tc.err)
			}
		})
	}
}

// The usage text lists every setting, and each default it states is the one the loader applies.
func TestUsageMatchesConfig(t *testing.T) {
	keyLine := regexp.MustCompile(`^  (SOLSTICE_[A-Z_]+)\s+(\S.*)$`)
	stated := map[string]string{}
	var key string
	for _, line := range strings.Split(usage, "\n") {
		if m := keyLine.FindStringSubmatch(line); m != nil {
			key = m[1]
			stated[key] = m[2]
		} else if key != "" && strings.HasPrefix(line, "    ") {
			stated[key] += " " + strings.TrimSpace(line)
		} else {
			key = ""
		}
	}
	var listed []string
	for k := range stated {
		listed = append(listed, k)
	}
	slices.Sort(listed)
	want := slices.Clone(config.Keys)
	slices.Sort(want)
	if !reflect.DeepEqual(listed, want) {
		t.Fatalf("the usage text lists %v, config.Keys is %v", listed, want)
	}

	load := func(env ...string) config.Settings {
		t.Helper()
		c, err := config.Load(append(env, config.KeyConfig+"="+filepath.Join(t.TempDir(), "absent.conf")))
		if err != nil {
			t.Fatal(err)
		}
		c.Data = filepath.Clean(c.Data)
		return c.Settings
	}
	def := config.Defaults()
	def.Data = filepath.Clean(def.Data)
	defaults := regexp.MustCompile(`\(default ([^)]+)\)`)
	for _, k := range config.Keys {
		env := []string{}
		// No stated default, or a description in place of a value: the loader's is unset.
		if m := defaults.FindStringSubmatch(stated[k]); m != nil && m[1] != "none" && !strings.HasPrefix(m[1], "<") {
			env = append(env, k+"="+m[1])
		}
		if got := load(env...); !reflect.DeepEqual(got, def) {
			t.Errorf("%s: the usage text says %q, which does not load as the default", k, stated[k])
		}
	}
}

// A taken port stops serve before the setup code is drawn and printed.
func TestServeOnTakenPort(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	data := t.TempDir()
	t.Setenv(config.KeyConfig, filepath.Join(t.TempDir(), "absent.conf"))
	t.Setenv(config.KeyDBURL, "")
	t.Setenv(config.KeyData, data)
	t.Setenv("SOLSTICE_HTTP_ADDR", taken.Addr().String())

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := os.Stderr
	os.Stderr = w
	err = runServe(context.Background(), nil)
	os.Stderr = stderr
	w.Close()
	printed, _ := io.ReadAll(r)

	if err == nil || !strings.Contains(err.Error(), "address already in use") {
		t.Errorf("runServe on a taken port: %v", err)
	}
	if strings.Contains(string(printed), "Setup code") {
		t.Errorf("the setup code was printed before the port failed:\n%s", printed)
	}
	if _, err := os.Stat(filepath.Join(data, "setup-code")); err == nil {
		t.Error("a setup code file was written")
	}
}
