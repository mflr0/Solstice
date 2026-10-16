package config

import (
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func env(vars map[string]string) []string {
	var out []string
	for k, v := range vars {
		out = append(out, k+"="+v)
	}
	return out
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestParse(t *testing.T) {
	got, err := Parse(strings.NewReader(`# a comment
SOLSTICE_DB_URL=mysql://u:p%40ss@db:3306/solstice

  SOLSTICE_TZ = Europe/Paris
export SOLSTICE_HTTP_ADDR=:9000
SOLSTICE_DATA="/var/lib/solstice"
SOLSTICE_HTTP_BASEURL='https://tickets.example.org'
SOLSTICE_TZ=UTC
EMPTY=
`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"SOLSTICE_DB_URL":       "mysql://u:p%40ss@db:3306/solstice",
		"SOLSTICE_TZ":           "UTC", // the later line wins
		"SOLSTICE_HTTP_ADDR":    ":9000",
		"SOLSTICE_DATA":         "/var/lib/solstice",
		"SOLSTICE_HTTP_BASEURL": "https://tickets.example.org",
		"EMPTY":                 "",
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
	write(t, path, "SOLSTICE_DB_URL=mysql://file@db/solstice\nSOLSTICE_TZ=Europe/Paris\nSOLSTICE_TYPO=1\nSOLSTICE_DB_MAXOPENCONNS=30\n")

	c, err := Load(env(map[string]string{KeyConfig: path, KeyTZ: "Asia/Kolkata", "SOLSTICE_HTTP_ADDR": "",
		"SOLSTICE_DB_MAXIDLECONNS": "5", "SOLSTICE_ENVTYPO": "x", "SOLSTICE_TEST_DB": "mysql://test", "OTHER": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Exists || c.Path != path {
		t.Errorf("path %q exists=%v", c.Path, c.Exists)
	}
	if c.DB.URL != "mysql://file@db/solstice" {
		t.Errorf("DB %q: the file's value should hold", c.DB.URL)
	}
	if c.TZ.String() != "Asia/Kolkata" {
		t.Errorf("TZ %v: the environment should override the file", c.TZ)
	}
	if c.DB.MaxOpenConns != 30 || c.DB.MaxIdleConns != 5 {
		t.Errorf("pool %+v", c.DB)
	}
	// An empty variable does not count as set.
	def := Defaults()
	if c.HTTP.Addr != def.HTTP.Addr {
		t.Errorf("addr %q", c.HTTP.Addr)
	}
	if c.Data != def.Data {
		t.Errorf("data %q, want the default", c.Data)
	}
	// Unknown keys of the file and of the environment; the scripts' variables are not settings.
	if !reflect.DeepEqual(c.Unknown, []string{"SOLSTICE_ENVTYPO", "SOLSTICE_TYPO"}) {
		t.Errorf("unknown keys %v", c.Unknown)
	}
}

func TestKeys(t *testing.T) {
	want := []string{"SOLSTICE_DATA", "SOLSTICE_TZ", "SOLSTICE_SECRETKEY", "SOLSTICE_INSTALL",
		"SOLSTICE_DB_URL", "SOLSTICE_DB_MAXOPENCONNS", "SOLSTICE_DB_MAXIDLECONNS", "SOLSTICE_DB_CONNMAXLIFETIME",
		"SOLSTICE_LOG_LEVEL", "SOLSTICE_LOG_FORMAT",
		"SOLSTICE_HTTP_ADDR", "SOLSTICE_HTTP_BASEURL", "SOLSTICE_HTTP_TRUSTEDPROXIES", "SOLSTICE_HTTP_SHUTDOWNTIMEOUT",
		"SOLSTICE_HTTP_MAXUPLOAD"}
	if !reflect.DeepEqual(Keys, want) {
		t.Errorf("got %v\nwant %v", Keys, want)
	}
	for _, k := range []string{KeyDBURL, KeyData, KeyTZ, KeyBaseURL} {
		if !slices.Contains(Keys, k) {
			t.Errorf("%s is not a setting", k)
		}
	}
}

func TestLoadEverySetting(t *testing.T) {
	c, err := Load(env(map[string]string{
		KeyConfig:                       filepath.Join(t.TempDir(), "absent.conf"),
		"SOLSTICE_DATA":                 "/srv/solstice",
		"SOLSTICE_TZ":                   "Europe/Paris",
		"SOLSTICE_SECRETKEY":            "k",
		"SOLSTICE_INSTALL":              "docker",
		"SOLSTICE_DB_URL":               "mysql://u:p@db/s",
		"SOLSTICE_DB_MAXOPENCONNS":      "40",
		"SOLSTICE_DB_MAXIDLECONNS":      "0",
		"SOLSTICE_DB_CONNMAXLIFETIME":   "1h",
		"SOLSTICE_LOG_LEVEL":            "debug",
		"SOLSTICE_LOG_FORMAT":           "JSON",
		"SOLSTICE_HTTP_ADDR":            "127.0.0.1:9000",
		"SOLSTICE_HTTP_BASEURL":         "https://t.example",
		"SOLSTICE_HTTP_TRUSTEDPROXIES":  "10.0.0.0/8",
		"SOLSTICE_HTTP_SHUTDOWNTIMEOUT": "30s",
		"SOLSTICE_HTTP_MAXUPLOAD":       "1GiB",
	}))
	if err != nil {
		t.Fatal(err)
	}
	paris, _ := time.LoadLocation("Europe/Paris")
	want := Settings{
		Data: "/srv/solstice", TZ: paris, SecretKey: "k", Install: "docker",
		DB:  DBSettings{URL: "mysql://u:p@db/s", MaxOpenConns: 40, MaxIdleConns: 0, ConnMaxLifetime: time.Hour},
		Log: LogSettings{Level: slog.LevelDebug, Format: LogJSON},
		HTTP: HTTPSettings{Addr: "127.0.0.1:9000", BaseURL: "https://t.example",
			TrustedProxies: Proxies{netip.MustParsePrefix("10.0.0.0/8")}, ShutdownTimeout: 30 * time.Second, MaxUpload: 1 << 30},
	}
	if !reflect.DeepEqual(c.Settings, want) {
		t.Errorf("got %+v\nwant %+v", c.Settings, want)
	}
}

func TestLoadInvalid(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "absent.conf")
	for key, value := range map[string]string{
		"SOLSTICE_TZ":                   "Mars/Olympus",
		"SOLSTICE_DB_MAXOPENCONNS":      "many",
		"SOLSTICE_DB_CONNMAXLIFETIME":   "5",
		"SOLSTICE_LOG_LEVEL":            "loud",
		"SOLSTICE_LOG_FORMAT":           "xml",
		"SOLSTICE_HTTP_TRUSTEDPROXIES":  "example.org",
		"SOLSTICE_HTTP_SHUTDOWNTIMEOUT": "-1s",
		"SOLSTICE_HTTP_MAXUPLOAD":       "100 parsecs",
	} {
		_, err := Load(env(map[string]string{KeyConfig: absent, key: value}))
		if err == nil || !strings.HasPrefix(err.Error(), key+":") {
			t.Errorf("%s=%s: %v, want an error naming the key", key, value, err)
		}
	}
	// The pool: idle connections cannot outnumber open ones.
	_, err := Load(env(map[string]string{KeyConfig: absent, "SOLSTICE_DB_MAXOPENCONNS": "5", "SOLSTICE_DB_MAXIDLECONNS": "6"}))
	if err == nil || !strings.Contains(err.Error(), "SOLSTICE_DB_MAXIDLECONNS") {
		t.Errorf("idle > open: %v", err)
	}
	// An error in the file names the file; no error repeats a value, which may be a password.
	path := filepath.Join(t.TempDir(), "solstice.conf")
	write(t, path, "SOLSTICE_DB_MAXOPENCONNS=s3cret\n")
	if _, err := Load(env(map[string]string{KeyConfig: path})); err == nil || !strings.Contains(err.Error(), path) ||
		strings.Contains(err.Error(), "s3cret") {
		t.Errorf("file: %v", err)
	}
}

func TestByteSize(t *testing.T) {
	for in, want := range map[string]ByteSize{
		"0": 0, "512": 512, "100B": 100, "1kb": 1000, "100MB": 100e6, "100 MiB": 100 << 20, "2GiB": 2 << 30, "1TB": 1e12,
	} {
		var b ByteSize
		if err := b.UnmarshalText([]byte(in)); err != nil || b != want {
			t.Errorf("%q: %d %v, want %d", in, b, err, want)
		}
	}
	for _, bad := range []string{"", "MB", "-1MB", "1.5GB", "10 parsecs", "9999999999TiB"} {
		var b ByteSize
		if err := b.UnmarshalText([]byte(bad)); err == nil {
			t.Errorf("%q accepted: %d", bad, b)
		}
	}
}

func TestLoadNoDatabaseByDefault(t *testing.T) {
	t.Chdir(t.TempDir())
	saved := searchPaths
	searchPaths = []string{"solstice.conf", filepath.Join(t.TempDir(), "absent.conf")}
	defer func() { searchPaths = saved }()

	c, err := Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.DB.URL != "" {
		t.Errorf("a database URL out of nowhere: %q", c.DB.URL)
	}
	if c.TZ != nil {
		t.Errorf("a time zone out of nowhere: %v", c.TZ)
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
	write(t, etc, "SOLSTICE_DB_URL=mysql://etc/db\n")
	c, err := Load(nil)
	if err != nil || c.DB.URL != "mysql://etc/db" || c.Path != etc {
		t.Fatalf("system file: %v %q %q", err, c.DB.URL, c.Path)
	}

	// A file in the working directory comes first.
	write(t, filepath.Join(cwd, "solstice.conf"), "SOLSTICE_DB_URL=mysql://cwd/db\n")
	if c, err = Load(nil); err != nil || c.DB.URL != "mysql://cwd/db" {
		t.Fatalf("working directory file: %v %q", err, c.DB.URL)
	}

	// SOLSTICE_CONFIG beats both.
	named := filepath.Join(t.TempDir(), "named.conf")
	write(t, named, "SOLSTICE_DB_URL=mysql://named/db\n")
	if c, err = Load(env(map[string]string{KeyConfig: named})); err != nil || c.DB.URL != "mysql://named/db" {
		t.Fatalf("SOLSTICE_CONFIG: %v %q", err, c.DB.URL)
	}

	// A named file that does not exist yet is where the wizard will write.
	missing := filepath.Join(t.TempDir(), "new.conf")
	if c, err = Load(env(map[string]string{KeyConfig: missing})); err != nil || c.Exists || c.Path != missing || c.DB.URL != "" {
		t.Fatalf("missing named file: %v %+v", err, c)
	}

	// A broken file is an error, not a silent default.
	write(t, named, "garbage\n")
	if _, err = Load(env(map[string]string{KeyConfig: named})); err == nil {
		t.Fatal("a malformed file was accepted")
	}
}

func TestUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "solstice.conf")

	// A new file: header, keys in order, owner-only.
	if err := Update(path, map[string]string{KeyTZ: "Europe/Paris", KeyDBURL: "mysql://u:p@db:3306/s"}); err != nil {
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
	if !strings.HasPrefix(string(got), "#") || !strings.HasSuffix(string(got), "SOLSTICE_TZ=Europe/Paris\nSOLSTICE_DB_URL=mysql://u:p@db:3306/s\n") {
		t.Errorf("new file:\n%s", got)
	}

	// An existing file keeps its comments and other keys; a set key is replaced in place.
	write(t, path, "# mine\nSOLSTICE_HTTP_ADDR=:9000\nSOLSTICE_TZ=UTC\n# end\nSOLSTICE_TZ=Asia/Kolkata\n")
	if err := Update(path, map[string]string{KeyTZ: "Europe/Rome", KeyBaseURL: "https://t.example"}); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	want := "# mine\nSOLSTICE_HTTP_ADDR=:9000\nSOLSTICE_TZ=Europe/Rome\n# end\nSOLSTICE_HTTP_BASEURL=https://t.example\n"
	if string(got) != want {
		t.Errorf("updated file:\n%s\nwant:\n%s", got, want)
	}
	values, err := Parse(strings.NewReader(string(got)))
	if err != nil || values[KeyTZ] != "Europe/Rome" || values["SOLSTICE_HTTP_ADDR"] != ":9000" {
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
	got := Lines(map[string]string{KeyTZ: "UTC", KeyDBURL: "mysql://x"})
	if got != "SOLSTICE_TZ=UTC\nSOLSTICE_DB_URL=mysql://x\n" {
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

	// An entry Solstice cannot use is quoted.
	for _, bad := range []string{"example.org", "10.0.0.0/33", "192.0.2.1:80"} {
		if _, err := ParseTrustedProxies(bad); err == nil || !strings.Contains(err.Error(), bad) {
			t.Errorf("ParseTrustedProxies(%q) = %v, want an error quoting it", bad, err)
		}
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	const key = "SOLSTICE_HTTP_TRUSTEDPROXIES"
	path := filepath.Join(t.TempDir(), "solstice.conf")
	write(t, path, key+"=127.0.0.1,10.0.0.0/8\n")
	c, err := Load(env(map[string]string{KeyConfig: path}))
	want := Proxies{netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("10.0.0.0/8")}
	if err != nil || !reflect.DeepEqual(c.HTTP.TrustedProxies, want) {
		t.Fatalf("file: %v %v", err, c.HTTP.TrustedProxies)
	}
	c, err = Load(env(map[string]string{KeyConfig: path, key: "2001:db8::1"}))
	if err != nil || !reflect.DeepEqual(c.HTTP.TrustedProxies, Proxies{netip.MustParsePrefix("2001:db8::1/128")}) {
		t.Fatalf("environment: %v %v", err, c.HTTP.TrustedProxies)
	}

	// A list Solstice cannot parse stops it at start-up.
	write(t, path, key+"=not-an-address\n")
	if _, err = Load(env(map[string]string{KeyConfig: path})); err == nil || !strings.Contains(err.Error(), key) {
		t.Errorf("file: %v, want an error naming %s", err, key)
	}
}
