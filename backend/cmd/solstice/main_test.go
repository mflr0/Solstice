package main

import (
	"context"
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
