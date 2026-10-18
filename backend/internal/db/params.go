package db

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
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

// ApplyMySQLParams checks the query of a mysql:// URL against the parameter
// names in allow and applies the accepted ones to cfg through its typed
// fields: nothing is ever copied as a raw driver parameter, and local file
// loading stays disabled. A name that is not in allow, a repeated name or an
// invalid value is an error (*UnknownParamsError, *InvalidParamError).
func ApplyMySQLParams(cfg *mysql.Config, q url.Values, allow ...string) error {
	cfg.AllowAllFiles = false
	allowed := map[string]bool{}
	for _, name := range allow {
		allowed[name] = true
	}
	var unknown []string
	for name := range q {
		if !allowed[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		accepted := append([]string(nil), allow...)
		sort.Strings(accepted)
		return &UnknownParamsError{Names: unknown, Allowed: accepted}
	}
	names := make([]string, 0, len(q))
	for name := range q {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		values := q[name]
		if len(values) != 1 {
			return &InvalidParamError{Name: name, Hint: "give it once"}
		}
		v := values[0]
		switch name {
		case ParamTLS:
			switch strings.ToLower(v) {
			case "true", "skip-verify", "preferred":
				cfg.TLSConfig = strings.ToLower(v)
			case "false":
				cfg.TLSConfig = ""
			default:
				return &InvalidParamError{Name: name, Hint: "true, false, skip-verify or preferred"}
			}
		case ParamCharset:
			if !charsetPattern.MatchString(v) {
				return &InvalidParamError{Name: name, Hint: "letters, digits and underscores"}
			}
			if cfg.Params == nil {
				cfg.Params = map[string]string{}
			}
			cfg.Params["charset"] = v
		case ParamTimeout:
			d, err := parseDuration(v, 30*time.Minute)
			if err != nil {
				return &InvalidParamError{Name: name, Hint: "a duration between 1s and 30m, e.g. 10s"}
			}
			cfg.Timeout = d
		case ParamReadTimeout:
			d, err := parseDuration(v, 6*time.Hour)
			if err != nil {
				return &InvalidParamError{Name: name, Hint: "a duration between 1s and 6h, e.g. 5m"}
			}
			cfg.ReadTimeout = d
		}
	}
	return nil
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
