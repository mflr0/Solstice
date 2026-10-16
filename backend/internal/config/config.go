// Package config reads Solstice's configuration file and writes it back.
//
// The file holds KEY=value lines and # comments: the format of systemd's
// EnvironmentFile and Docker's --env-file, so one file serves all three.
// Real environment variables override the file; command-line flags override
// both (the caller applies them).
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The keys Solstice reads.
const (
	KeyDB             = "SOLSTICE_DB"              // MariaDB URL: mysql://user:pass@host:port/db
	KeyData           = "SOLSTICE_DATA"            // data directory (attachments, setup code)
	KeyAddr           = "SOLSTICE_ADDR"            // listen address
	KeyTZ             = "SOLSTICE_TZ"              // time zone of dates typed in searches
	KeyBaseURL        = "SOLSTICE_BASE_URL"        // public address, e.g. https://tickets.example.org
	KeySecretKey      = "SOLSTICE_SECRET_KEY"      // optional; <data>/secret.key otherwise
	KeyTrustedProxies = "SOLSTICE_TRUSTED_PROXIES" // reverse proxies: IP addresses and CIDR ranges
	// KeyConfig names the file itself; it is only read from the environment.
	KeyConfig = "SOLSTICE_CONFIG"
)

// Keys are the settings a file may hold.
var Keys = []string{KeyDB, KeyData, KeyAddr, KeyTZ, KeyBaseURL, KeySecretKey, KeyTrustedProxies}

// Defaults for what nothing sets. There is deliberately no default database.
const (
	DefaultData = "data"
	DefaultAddr = ":8080"
)

// searchPaths are tried in order when no file is named. The first is also
// where a new file is written when none exists.
var searchPaths = []string{"solstice.conf", "/etc/solstice/solstice.conf"}

// Config is the configuration in effect.
type Config struct {
	// Path is the file read, or the one to write when none exists yet.
	Path string
	// Exists tells whether Path was found.
	Exists bool

	DB, Data, Addr, TZ, BaseURL, SecretKey, TrustedProxies string

	// FromEnv lists the keys set by an environment variable: writing them to
	// the file would change nothing.
	FromEnv map[string]bool
	// Unknown lists the keys of the file Solstice does not know (typos).
	Unknown []string
}

// Get returns a setting by key.
func (c *Config) Get(key string) string {
	switch key {
	case KeyDB:
		return c.DB
	case KeyData:
		return c.Data
	case KeyAddr:
		return c.Addr
	case KeyTZ:
		return c.TZ
	case KeyBaseURL:
		return c.BaseURL
	case KeySecretKey:
		return c.SecretKey
	case KeyTrustedProxies:
		return c.TrustedProxies
	}
	return ""
}

func (c *Config) set(key, value string) {
	switch key {
	case KeyDB:
		c.DB = value
	case KeyData:
		c.Data = value
	case KeyAddr:
		c.Addr = value
	case KeyTZ:
		c.TZ = value
	case KeyBaseURL:
		c.BaseURL = value
	case KeySecretKey:
		c.SecretKey = value
	case KeyTrustedProxies:
		c.TrustedProxies = value
	}
}

// Load reads the configuration. path is the file named on the command line
// ("" for none); then SOLSTICE_CONFIG; then ./solstice.conf and
// /etc/solstice/solstice.conf, the first that exists. A named file that does
// not exist yet is not an error: it is where the setup wizard will write.
// lookup reads the environment (os.LookupEnv).
func Load(path string, lookup func(string) (string, bool)) (*Config, error) {
	if path == "" {
		path, _ = lookup(KeyConfig)
	}
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
	c := &Config{Path: abs, FromEnv: map[string]bool{}}

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
	for k, v := range values {
		if slices.Contains(Keys, k) {
			c.set(k, v)
		} else {
			c.Unknown = append(c.Unknown, k)
		}
	}
	slices.Sort(c.Unknown)
	for _, k := range Keys {
		if v, ok := lookup(k); ok && v != "" {
			c.set(k, v)
			c.FromEnv[k] = true
		}
	}
	if c.Data == "" {
		c.Data = DefaultData
	}
	if c.Addr == "" {
		c.Addr = DefaultAddr
	}
	if _, err := ParseTrustedProxies(c.TrustedProxies); err != nil {
		return nil, err
	}
	return c, nil
}

// ParseTrustedProxies reads the trusted proxy list: IP addresses and CIDR
// ranges separated by commas, the reverse proxies whose X-Forwarded-For
// header Solstice may believe. An empty list trusts no proxy. An entry that
// is neither an address nor a range is an error naming it.
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
				return nil, fmt.Errorf("%s: %q is not a CIDR range", KeyTrustedProxies, part)
			}
			if a := p.Addr(); a.Is4In6() && p.Bits() >= 96 { // ::ffff:10.0.0.0/104 is 10.0.0.0/8
				p = netip.PrefixFrom(a.Unmap(), p.Bits()-96)
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not an IP address", KeyTrustedProxies, part)
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
