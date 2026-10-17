// Package config reads Solstice's configuration file and writes it back.
//
// The file holds KEY=value lines and # comments: the format of systemd's
// EnvironmentFile and Docker's --env-file, so one file serves all three.
// Real environment variables override the file.
//
// Each field of Settings is one key, named after its path: SOLSTICE, then
// the field names upper-cased, joined by "_" (HTTP.BaseURL is
// SOLSTICE_HTTP_BASEURL). Adding a field adds a key.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"solstice/internal/db"
)

// Prefix starts every key.
const Prefix = "SOLSTICE"

// The keys other packages name; TestKeyNames checks them against Settings.
const (
	KeyDBURL   = "SOLSTICE_DB_URL"
	KeyData    = "SOLSTICE_DATA"
	KeyTZ      = "SOLSTICE_TZ"
	KeyBaseURL = "SOLSTICE_HTTP_BASEURL"
	// KeyConfig names the file itself; it is only read from the environment.
	KeyConfig = "SOLSTICE_CONFIG"
)

// Settings are what the file and the environment set.
type Settings struct {
	// Data is the data directory (attachments, secret key, setup code).
	Data string
	// TZ is the time zone of dates typed in searches; nil for the system's.
	TZ *time.Location
	// SecretKey seals stored secrets; empty for <data>/secret.key.
	SecretKey string
	// Install tells how Solstice was installed ("docker"), for the update page.
	Install string

	DB   DBSettings
	Log  LogSettings
	HTTP HTTPSettings
}

// DBSettings are Solstice's database and its connection pool.
type DBSettings struct {
	// URL is mysql://user:password@host:3306/database, optionally ?tls=true.
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Pool returns the pool settings for db.Open.
func (d DBSettings) Pool() db.Pool {
	return db.Pool{MaxOpenConns: d.MaxOpenConns, MaxIdleConns: d.MaxIdleConns, ConnMaxLifetime: d.ConnMaxLifetime}
}

// LogSettings are what is logged and how.
type LogSettings struct {
	Level  slog.Level
	Format LogFormat
}

// HTTPSettings are the web server's.
type HTTPSettings struct {
	// Addr is the listen address.
	Addr string
	// BaseURL is the public address, e.g. https://tickets.example.org.
	BaseURL string
	// TrustedProxies are the reverse proxies whose X-Forwarded-For header
	// Solstice believes.
	TrustedProxies Proxies
	// ShutdownTimeout is how long requests and workers get to finish on stop.
	ShutdownTimeout time.Duration
	// MaxUpload caps a single upload request.
	MaxUpload ByteSize
	// MaxStorage caps the total size of all attachments; 0 for no limit.
	MaxStorage ByteSize
}

// Defaults returns the settings nothing sets. There is deliberately no
// default database.
func Defaults() Settings {
	return Settings{
		Data: "data",
		DB: DBSettings{
			MaxOpenConns:    20,
			MaxIdleConns:    10,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Log: LogSettings{
			Level:  slog.LevelInfo,
			Format: LogText,
		},
		HTTP: HTTPSettings{
			Addr:            ":8080",
			ShutdownTimeout: 10 * time.Second,
			MaxUpload:       100 << 20,
		},
	}
}

// Keys are the settings a file may hold, in Settings order.
var Keys = func() []string {
	var out []string
	for _, f := range fields(&Settings{}) {
		out = append(out, f.key)
	}
	return out
}()

// toolEnv are SOLSTICE_ variables of the scripts and tests, not settings:
// they are not reported as unknown.
var toolEnv = []string{KeyConfig, "SOLSTICE_TEST_DB", "SOLSTICE_TEST_EMPTY_DB", "SOLSTICE_BACKEND",
	"SOLSTICE_VERSION", "SOLSTICE_PORT", "SOLSTICE_RELEASE_KEY"}

// searchPaths are tried in order when no file is named. The first is also
// where a new file is written when none exists.
var searchPaths = []string{"solstice.conf", "/etc/solstice/solstice.conf"}

// Config is the configuration in effect.
type Config struct {
	Settings

	// Path is the file read, or the one to write when none exists yet.
	Path string
	// Exists tells whether Path was found.
	Exists bool
	// Unknown lists the keys of the file, and the SOLSTICE_ variables,
	// Solstice does not know (typos, renamed settings).
	Unknown []string
}

// Load reads the configuration. env is the environment, as os.Environ
// returns it. The file is the one SOLSTICE_CONFIG names, else
// ./solstice.conf or /etc/solstice/solstice.conf, the first that exists. A
// named file that does not exist yet is not an error: it is where the setup
// wizard will write. An empty value counts as unset.
func Load(env []string) (*Config, error) {
	vars := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			vars[k] = v
		}
	}
	path := vars[KeyConfig]
	if path == "" {
		path = searchPaths[0]
		for _, p := range searchPaths {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	c := &Config{Settings: Defaults(), Path: abs}

	values := map[string]string{}
	f, err := os.Open(abs) // #nosec G304 -- path built by the server, not from user input
	switch {
	case err == nil:
		values, err = Parse(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", abs, err)
		}
		c.Exists = true
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}

	for _, f := range fields(&c.Settings) {
		raw, inFile := vars[f.key], false
		if raw == "" {
			raw, inFile = values[f.key], true
		}
		if raw == "" {
			continue
		}
		if err := f.set(raw); err != nil {
			if inFile {
				return nil, fmt.Errorf("%s in %s: %w", f.key, abs, err)
			}
			return nil, fmt.Errorf("%s: %w", f.key, err)
		}
	}
	for k := range values {
		if !slices.Contains(Keys, k) {
			c.Unknown = append(c.Unknown, k)
		}
	}
	for k := range vars {
		if strings.HasPrefix(k, Prefix+"_") && !slices.Contains(Keys, k) && !slices.Contains(toolEnv, k) &&
			!slices.Contains(c.Unknown, k) {
			c.Unknown = append(c.Unknown, k)
		}
	}
	slices.Sort(c.Unknown)
	if err := c.check(); err != nil {
		return nil, err
	}
	return c, nil
}

// check rejects values that parse but cannot work.
func (s *Settings) check() error {
	for _, r := range []struct {
		bad  bool
		p    any
		want string
	}{
		{s.DB.MaxOpenConns < 1, &s.DB.MaxOpenConns, "at least 1"},
		{s.DB.MaxIdleConns < 0 || s.DB.MaxIdleConns > s.DB.MaxOpenConns, &s.DB.MaxIdleConns,
			"between 0 and " + keyOf(s, &s.DB.MaxOpenConns)},
		{s.DB.ConnMaxLifetime <= 0, &s.DB.ConnMaxLifetime, "a positive duration"},
		{s.HTTP.ShutdownTimeout <= 0, &s.HTTP.ShutdownTimeout, "a positive duration"},
		{s.HTTP.MaxUpload <= 0, &s.HTTP.MaxUpload, "a positive size"},
	} {
		if r.bad {
			return fmt.Errorf("%s: must be %s", keyOf(s, r.p), r.want)
		}
	}
	return nil
}

// ParseTrustedProxies reads the trusted proxy list: IP addresses and CIDR
// ranges separated by commas, the reverse proxies whose X-Forwarded-For
// header Solstice may believe. An empty list trusts no proxy. An entry that
// is neither an address nor a range is an error quoting it.
func ParseTrustedProxies(v string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			p, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("%q is not a CIDR range", part)
			}
			if a := p.Addr(); a.Is4In6() && p.Bits() >= 96 { // ::ffff:10.0.0.0/104 is 10.0.0.0/8
				p = netip.PrefixFrom(a.Unmap(), p.Bits()-96)
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("%q is not an IP address", part)
		}
		a = a.Unmap() // an IPv4 address written mapped, e.g. ::ffff:127.0.0.1
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}

var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Parse reads KEY=value lines. Blank lines and lines starting with # are
// skipped; "export " before a key is allowed; a value wrapped in matching
// single or double quotes loses them. Later lines win.
func Parse(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		if !ok || !keyPattern.MatchString(key) {
			return nil, fmt.Errorf("line %d: expected KEY=value", n)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		out[key] = value
	}
	return out, sc.Err()
}

// CheckValue tells whether a value can be written as is: systemd and Docker
// read env files differently (quotes, backslashes, $), so only values that
// mean the same to both are accepted.
func CheckValue(v string) error {
	if v != strings.TrimSpace(v) {
		return errors.New("leading or trailing spaces")
	}
	for _, r := range v {
		if r < 0x20 || r == 0x7f || strings.ContainsRune("\"'\\$`", r) {
			return fmt.Errorf("character %q not allowed", r)
		}
	}
	return nil
}

const header = `# Configuration de Solstice : des lignes CLÉ=valeur, lues par « solstice serve ».
# Le même fichier sert d'EnvironmentFile à systemd et de fichier env à Docker.
# Les variables d'environnement l'emportent sur ce qui est écrit ici.
`

// Lines renders settings as file lines, in Keys order, for an administrator
// to paste when the file cannot be written.
func Lines(values map[string]string) string {
	var sb strings.Builder
	for _, k := range Keys {
		if v, ok := values[k]; ok {
			sb.WriteString(k + "=" + v + "\n")
		}
	}
	return sb.String()
}

// Update sets keys in the file at path, keeping its other lines and
// comments, and creates it if needed. The file is replaced atomically and
// readable by its owner only (it holds the database password).
func Update(path string, values map[string]string) error {
	for k, v := range values {
		if !slices.Contains(Keys, k) {
			return fmt.Errorf("unknown key %s", k)
		}
		if err := CheckValue(v); err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}
	old, err := os.ReadFile(path) // #nosec G304 -- path built by the server, not from user input
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var sb strings.Builder
	if len(old) == 0 {
		sb.WriteString(header)
	}
	written := map[string]bool{}
	for line := range strings.Lines(string(old)) {
		trimmed := strings.TrimSpace(line)
		key, _, ok := strings.Cut(trimmed, "=")
		key = strings.TrimSpace(strings.TrimPrefix(key, "export "))
		if v, set := values[key]; ok && set && !strings.HasPrefix(trimmed, "#") {
			if !written[key] {
				sb.WriteString(key + "=" + v + "\n")
				written[key] = true
			}
			continue
		}
		sb.WriteString(strings.TrimRight(line, "\r\n") + "\n")
	}
	for _, k := range Keys {
		if v, ok := values[k]; ok && !written[k] {
			sb.WriteString(k + "=" + v + "\n")
		}
	}
	return writeAtomic(path, []byte(sb.String()))
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".solstice.conf-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name) // a no-op once renamed
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
