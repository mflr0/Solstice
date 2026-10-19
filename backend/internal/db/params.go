package db

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

// The query parameters of a mysql:// URL that Solstice accepts. The URL comes
// from a configuration file or, for the Jira source, from a form: its
// parameters must never reach the driver unchecked, since the driver
// understands options that read local files (allowAllFiles), weaken
// authentication (allowCleartextPasswords, allowFallbackToPlaintext), run
// several statements (multiStatements) or, for any unknown name, send
// "SET name=value" to the server.
const (
	// ParamTLS selects the encryption of the link: true (verified), false,
	// skip-verify (encrypted, certificate not checked) or preferred
	// (encrypted when the server offers it, not verified).
	ParamTLS = "tls"
	// ParamCharset is the connection character set (letters, digits and
	// underscores only).
	ParamCharset = "charset"
	// ParamTimeout is the time allowed to connect, e.g. 10s.
	ParamTimeout = "timeout"
	// ParamReadTimeout is the time allowed for the server to answer, e.g. 5m.
	ParamReadTimeout = "readTimeout"
)

// UnknownParamsError lists URL parameters that are not accepted.
type UnknownParamsError struct {
	Names   []string // sorted
	Allowed []string // sorted
}

func (e *UnknownParamsError) Error() string {
	return fmt.Sprintf("database URL option %s is not accepted (accepted: %s)",
		strings.Join(quoteAll(e.Names), ", "), strings.Join(e.Allowed, ", "))
}

// InvalidParamError reports a parameter whose value is not accepted. It never
// repeats the value.
type InvalidParamError struct {
	Name string
	Hint string // what is accepted
}

func (e *InvalidParamError) Error() string {
	return fmt.Sprintf("database URL option %q has an invalid value (%s)", e.Name, e.Hint)
}

func quoteAll(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = fmt.Sprintf("%q", n)
	}
	return out
}

var charsetPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)

// Param is an accepted URL parameter: Apply checks a value and records it,
// and returns false when the value is not accepted.
type Param struct {
	Hint  string // what is accepted, for the error
	Apply func(v string) bool
}

// CheckParams checks the query of a database URL against params, by name in
// sorted order: a name that is not in params is an *UnknownParamsError, a
// repeated name or a value that Apply refuses an *InvalidParamError.
func CheckParams(q url.Values, params map[string]Param) error {
	var unknown []string
	for name := range q {
		if _, ok := params[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		accepted := make([]string, 0, len(params))
		for name := range params {
			accepted = append(accepted, name)
		}
		sort.Strings(accepted)
		return &UnknownParamsError{Names: unknown, Allowed: accepted}
	}
	names := make([]string, 0, len(q))
	for name := range q {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if len(q[name]) != 1 {
			return &InvalidParamError{Name: name, Hint: "give it once"}
		}
	}
	for _, name := range names {
		if p := params[name]; !p.Apply(q.Get(name)) {
			return &InvalidParamError{Name: name, Hint: p.Hint}
		}
	}
	return nil
}

// ApplyMySQLParams checks the query of a mysql:// URL against the parameter
// names in allow and applies the accepted ones to cfg through its typed
// fields: nothing is ever copied as a raw driver parameter, and local file
// loading stays disabled. A name that is not in allow, a repeated name or an
// invalid value is an error (*UnknownParamsError, *InvalidParamError).
func ApplyMySQLParams(cfg *mysql.Config, q url.Values, allow ...string) error {
	cfg.AllowAllFiles = false
	all := map[string]Param{
		ParamTLS: {"true, false, skip-verify or preferred", func(v string) bool {
			switch v = strings.ToLower(v); v {
			case "true", "skip-verify", "preferred":
				cfg.TLSConfig = v
			case "false":
				cfg.TLSConfig = ""
			default:
				return false
			}
			return true
		}},
		ParamCharset: {"letters, digits and underscores", func(v string) bool {
			if !charsetPattern.MatchString(v) {
				return false
			}
			if cfg.Params == nil {
				cfg.Params = map[string]string{}
			}
			cfg.Params["charset"] = v
			return true
		}},
		ParamTimeout: {"a duration between 1s and 30m, e.g. 10s", func(v string) bool {
			d, err := parseDuration(v, 30*time.Minute)
			if err != nil {
				return false
			}
			cfg.Timeout = d
			return true
		}},
		ParamReadTimeout: {"a duration between 1s and 6h, e.g. 5m", func(v string) bool {
			d, err := parseDuration(v, 6*time.Hour)
			if err != nil {
				return false
			}
			cfg.ReadTimeout = d
			return true
		}},
	}
	params := map[string]Param{}
	for _, name := range allow {
		params[name] = all[name]
	}
	return CheckParams(q, params)
}

// The query parameters of a postgres:// URL that Solstice can accept: the
// others (sslkey, passfile, service, options, ...) read local files or change
// the session.
const (
	// ParamSSLMode selects the encryption of the link, as libpq names it.
	ParamSSLMode = "sslmode"
	// ParamConnectTimeout is the time allowed to connect, in seconds.
	ParamConnectTimeout = "connect_timeout"
)

var postgresParams = map[string]Param{
	ParamSSLMode: {"disable, allow, prefer, require, verify-ca or verify-full", func(v string) bool {
		return slices.Contains([]string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}, v)
	}},
	ParamConnectTimeout: {"a number of seconds between 1 and 600", func(v string) bool {
		n, err := strconv.Atoi(v)
		return err == nil && n >= 1 && n <= 600
	}},
}

// CheckPostgresParams checks the query of a postgres:// URL against the
// parameter names in allow; the values are left in q. A name that is not in
// allow, a repeated name or an invalid value is an error
// (*UnknownParamsError, *InvalidParamError).
func CheckPostgresParams(q url.Values, allow ...string) error {
	params := map[string]Param{}
	for _, name := range allow {
		params[name] = postgresParams[name]
	}
	return CheckParams(q, params)
}

func parseDuration(s string, max time.Duration) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d < time.Second || d > max {
		return 0, errors.New("out of range")
	}
	return d, nil
}

// ParseURL parses a database URL. Unlike url.Parse, its error never contains
// the text, which holds the password.
func ParseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("invalid database URL: it cannot be parsed " +
			"(percent-encode special characters of the user name and password, e.g. @ as %40)")
	}
	return u, nil
}

// URLQuery returns the query parameters of u. Unlike u.Query, a malformed
// query (a stray semicolon or percent sign) is an error instead of being
// dropped silently.
func URLQuery(u *url.URL) (url.Values, error) {
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, &InvalidParamError{Name: "?", Hint: "the options after ? are malformed"}
	}
	return q, nil
}
