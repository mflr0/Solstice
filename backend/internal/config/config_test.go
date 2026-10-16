package config

import (
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func env(vars map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := vars[k]
		return v, ok
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestParse(t *testing.T) {
	got, err := Parse(strings.NewReader(`# a comment
SOLSTICE_DB=mysql://u:p%40ss@db:3306/solstice

  SOLSTICE_TZ = Europe/Paris
export SOLSTICE_ADDR=:9000
SOLSTICE_DATA="/var/lib/solstice"
SOLSTICE_BASE_URL='https://tickets.example.org'
SOLSTICE_TZ=UTC
EMPTY=
`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"SOLSTICE_DB":       "mysql://u:p%40ss@db:3306/solstice",
		"SOLSTICE_TZ":       "UTC", // the later line wins
		"SOLSTICE_ADDR":     ":9000",
		"SOLSTICE_DATA":     "/var/lib/solstice",
		"SOLSTICE_BASE_URL": "https://tickets.example.org",
		"EMPTY":             "",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	for _, bad := range []string{"no equals sign", "=value", "1KEY=x", "KEY WITH SPACE=x"} {
		if _, err := Parse(strings.NewReader(bad)); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestLoadFileThenEnvironment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "solstice.conf")
	write(t, path, "SOLSTICE_DB=mysql://file@db/solstice\nSOLSTICE_TZ=Europe/Paris\nSOLSTICE_TYPO=1\n")

	c, err := Load(path, env(map[string]string{"SOLSTICE_TZ": "Asia/Kolkata", "SOLSTICE_ADDR": ""}))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Exists || c.Path != path {
		t.Errorf("path %q exists=%v", c.Path, c.Exists)
	}
	if c.DB != "mysql://file@db/solstice" || c.FromEnv[KeyDB] {
		t.Errorf("DB %q fromEnv=%v: the file's value should hold", c.DB, c.FromEnv[KeyDB])
	}
	if c.TZ != "Asia/Kolkata" || !c.FromEnv[KeyTZ] {
		t.Errorf("TZ %q fromEnv=%v: the environment should override the file", c.TZ, c.FromEnv[KeyTZ])
	}
	// An empty variable does not count as set.
	if c.Addr != DefaultAddr || c.FromEnv[KeyAddr] {
		t.Errorf("addr %q fromEnv=%v", c.Addr, c.FromEnv[KeyAddr])
	}
	if c.Data != DefaultData {
		t.Errorf("data %q, want the default", c.Data)
	}
	if !reflect.DeepEqual(c.Unknown, []string{"SOLSTICE_TYPO"}) {
		t.Errorf("unknown keys %v", c.Unknown)
	}
}

func TestLoadNoDatabaseByDefault(t *testing.T) {
	t.Chdir(t.TempDir())
	saved := searchPaths
	searchPaths = []string{"solstice.conf", filepath.Join(t.TempDir(), "absent.conf")}
	defer func() { searchPaths = saved }()

	c, err := Load("", env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.DB != "" {
		t.Errorf("a database URL out of nowhere: %q", c.DB)
	}
	if c.Exists || filepath.Base(c.Path) != "solstice.conf" || !filepath.IsAbs(c.Path) {
		t.Errorf("path %q exists=%v: want ./solstice.conf, absolute, to be written", c.Path, c.Exists)
	}
}

func TestLoadLookupOrder(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	etc := filepath.Join(t.TempDir(), "etc-solstice.conf")
	saved := searchPaths
	searchPaths = []string{"solstice.conf", etc}
	defer func() { searchPaths = saved }()

	// Only the system-wide file: it is read.
	write(t, etc, "SOLSTICE_DB=mysql://etc/db\n")
	c, err := Load("", env(nil))
	if err != nil || c.DB != "mysql://etc/db" || c.Path != etc {
		t.Fatalf("system file: %v %q %q", err, c.DB, c.Path)
	}

	// A file in the working directory comes first.
	write(t, filepath.Join(cwd, "solstice.conf"), "SOLSTICE_DB=mysql://cwd/db\n")
	if c, err = Load("", env(nil)); err != nil || c.DB != "mysql://cwd/db" {
		t.Fatalf("working directory file: %v %q", err, c.DB)
	}

	// SOLSTICE_CONFIG beats both, and the flag beats SOLSTICE_CONFIG.
	named := filepath.Join(t.TempDir(), "named.conf")
	write(t, named, "SOLSTICE_DB=mysql://named/db\n")
	flagged := filepath.Join(t.TempDir(), "flag.conf")
	write(t, flagged, "SOLSTICE_DB=mysql://flag/db\n")
	if c, err = Load("", env(map[string]string{KeyConfig: named})); err != nil || c.DB != "mysql://named/db" {
		t.Fatalf("SOLSTICE_CONFIG: %v %q", err, c.DB)
	}
	if c, err = Load(flagged, env(map[string]string{KeyConfig: named})); err != nil || c.DB != "mysql://flag/db" {
		t.Fatalf("-config: %v %q", err, c.DB)
	}

	// A named file that does not exist yet is where the wizard will write.
	missing := filepath.Join(t.TempDir(), "new.conf")
	if c, err = Load(missing, env(nil)); err != nil || c.Exists || c.Path != missing || c.DB != "" {
		t.Fatalf("missing named file: %v %+v", err, c)
	}

	// A broken file is an error, not a silent default.
	write(t, flagged, "garbage\n")
	if _, err = Load(flagged, env(nil)); err == nil {
		t.Fatal("a malformed file was accepted")
	}
}

func TestUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "solstice.conf")

	// A new file: header, keys in order, owner-only.
	if err := Update(path, map[string]string{KeyTZ: "Europe/Paris", KeyDB: "mysql://u:p@db:3306/s"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", info.Mode().Perm())
	}
	got, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(got), "#") || !strings.HasSuffix(string(got), "SOLSTICE_DB=mysql://u:p@db:3306/s\nSOLSTICE_TZ=Europe/Paris\n") {
		t.Errorf("new file:\n%s", got)
	}

	// An existing file keeps its comments and other keys; a set key is replaced in place.
	write(t, path, "# mine\nSOLSTICE_ADDR=:9000\nSOLSTICE_TZ=UTC\n# end\nSOLSTICE_TZ=Asia/Kolkata\n")
	if err := Update(path, map[string]string{KeyTZ: "Europe/Rome", KeyBaseURL: "https://t.example"}); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	want := "# mine\nSOLSTICE_ADDR=:9000\nSOLSTICE_TZ=Europe/Rome\n# end\nSOLSTICE_BASE_URL=https://t.example\n"
	if string(got) != want {
		t.Errorf("updated file:\n%s\nwant:\n%s", got, want)
	}
	values, err := Parse(strings.NewReader(string(got)))
	if err != nil || values[KeyTZ] != "Europe/Rome" || values[KeyAddr] != ":9000" {
		t.Errorf("re-read: %v %v", values, err)
	}

	// Values that systemd and Docker would read differently are refused.
	for _, bad := range []string{"a b ", "with\"quote", "back\\slash", "dollar$x", "new\nline"} {
		if err := Update(path, map[string]string{KeyTZ: bad}); err == nil {
			t.Errorf("%q written", bad)
		}
	}
	if err := Update(path, map[string]string{"OTHER": "x"}); err == nil {
		t.Error("unknown key written")
	}

	// A directory that cannot be written to is reported, not ignored.
	if err := Update(filepath.Join(t.TempDir(), "absent", "solstice.conf"), map[string]string{KeyTZ: "UTC"}); err == nil {
		t.Error("writing into a missing directory succeeded")
	}
}

func TestLines(t *testing.T) {
	got := Lines(map[string]string{KeyTZ: "UTC", KeyDB: "mysql://x"})
	if got != "SOLSTICE_DB=mysql://x\nSOLSTICE_TZ=UTC\n" {
		t.Errorf("got %q", got)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	// An address is a range of one; a range is masked; a mapped address unmapped.
	got, err := ParseTrustedProxies("192.0.2.1, 10.0.0.0/8 ,2001:db8::1, ::ffff:127.0.0.1, 10.1.2.3/16, ::ffff:172.16.0.0/108")
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("2001:db8::1/128"), netip.MustParsePrefix("127.0.0.1/32"),
		netip.MustParsePrefix("10.1.0.0/16"), netip.MustParsePrefix("172.16.0.0/12")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// An empty list trusts no proxy; empty entries are skipped.
	for _, in := range []string{"", "  "} {
		if ps, err := ParseTrustedProxies(in); err != nil || len(ps) != 0 {
			t.Errorf("ParseTrustedProxies(%q) = %v, %v, want nothing", in, ps, err)
		}
	}
	if ps, err := ParseTrustedProxies("192.0.2.1,"); err != nil || len(ps) != 1 {
		t.Errorf("a trailing comma: %v %v", ps, err)
	}

	// An entry Solstice cannot use names the setting it came from.
	for _, bad := range []string{"example.org", "10.0.0.0/33", "192.0.2.1:80"} {
		if _, err := ParseTrustedProxies(bad); err == nil || !strings.Contains(err.Error(), KeyTrustedProxies) {
			t.Errorf("ParseTrustedProxies(%q) = %v, want an error naming %s", bad, err, KeyTrustedProxies)
		}
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "solstice.conf")
	write(t, path, "SOLSTICE_TRUSTED_PROXIES=127.0.0.1,10.0.0.0/8\n")
	c, err := Load(path, env(nil))
	if err != nil || c.TrustedProxies != "127.0.0.1,10.0.0.0/8" {
		t.Fatalf("file: %v %q", err, c.TrustedProxies)
	}
	c, err = Load(path, env(map[string]string{KeyTrustedProxies: "2001:db8::1"}))
	if err != nil || c.TrustedProxies != "2001:db8::1" {
		t.Fatalf("environment: %v %q", err, c.TrustedProxies)
	}

	// A list Solstice cannot parse stops it at start-up.
	write(t, path, "SOLSTICE_TRUSTED_PROXIES=not-an-address\n")
	if _, err = Load(path, env(nil)); err == nil || !strings.Contains(err.Error(), KeyTrustedProxies) {
		t.Errorf("file: %v, want an error naming %s", err, KeyTrustedProxies)
	}
	if _, err = Load(filepath.Join(t.TempDir(), "absent.conf"),
		env(map[string]string{KeyTrustedProxies: "10.0.0.0/40"})); err == nil || !strings.Contains(err.Error(), KeyTrustedProxies) {
		t.Errorf("environment: %v, want an error naming %s", err, KeyTrustedProxies)
	}
}
