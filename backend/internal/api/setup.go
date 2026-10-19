package api

// The first-run wizard. A Solstice started without a database, or on a
// database without any account, serves only this wizard (the interface's
// /setup page and /api/setup/*), its static files and /healthz. Everything
// else redirects to /setup, or answers 503 under /api.
//
// The wizard is locked by a one-time code printed in the server log and
// written to <data>/setup-code: whoever can read the server's log or files
// is the administrator. Once the first administrator exists, Done switches
// the server to the application without a restart and the code file goes.

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math/big"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"solstice/internal/auth"
	"solstice/internal/config"
	"solstice/internal/db"
	"solstice/internal/i18n"
)

// SetupOptions configures the wizard.
type SetupOptions struct {
	// DB is the database, or nil when none is configured yet: the wizard
	// then asks for one.
	DB *sql.DB
	// Pool sizes the connection pool of the database the wizard opens.
	Pool db.Pool
	// DataDir is the data directory in use (absolute).
	DataDir string
	// ConfigPath is the configuration file the wizard writes.
	ConfigPath string
	// TrustedProxies are the reverse proxies whose forwarding header the
	// server believes (SOLSTICE_HTTP_TRUSTEDPROXIES).
	TrustedProxies []netip.Prefix
	// BaseURL and TZ are the values in effect.
	BaseURL, TZ string
	Version     string
	Log         *slog.Logger
	// Done is called once, when the first administrator exists. It must
	// switch the server to the application.
	Done func(SetupResult)
}

// SetupResult is what the wizard settled.
type SetupResult struct {
	DB      *sql.DB
	BaseURL string
	TZ      string // "" for the server's local time
	Admin   string // user name of the administrator
}

// Setup is the wizard's state. Its mutating steps run one at a time.
type Setup struct {
	opt     SetupOptions
	ctx     context.Context // lives as long as the server: imports run in it
	log     *slog.Logger
	code    string // the setup code, normalised: 8 characters, no dash
	limiter *auth.Limiter
	proxies []netip.Prefix // reverse proxies to believe; see clientIP

	mu       sync.Mutex
	db       *sql.DB
	server   string // database server and version, e.g. "MariaDB 11.4.2"
	tokens   map[string]bool
	baseURL  string
	tz       string
	pending  map[string]string // settings to keep in the configuration file
	lastSave *configSave
	imported bool // an import ran here: the accounts in the database are ours
	job      *importJob
	done     bool
}

// codeAlphabet leaves out 0/O, 1/I/L and U, which read alike.
const codeAlphabet = "23456789ABCDEFGHJKMNPQRSTVWXYZ"

// setupCookie keeps a browser that gave the code unlocked.
const setupCookie = "solstice_setup"

// NewSetup starts setup mode: it draws the setup code, writes it to
// <data>/setup-code and prints it in the log.
func NewSetup(ctx context.Context, opt SetupOptions) (*Setup, error) {
	if opt.Log == nil {
		opt.Log = slog.Default()
	}
	var code strings.Builder
	for range 8 {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		if err != nil {
			return nil, err
		}
		code.WriteByte(codeAlphabet[n.Int64()])
	}
	w := &Setup{
		opt: opt, ctx: ctx, log: opt.Log, code: code.String(), limiter: auth.NewLimiter(),
		db: opt.DB, tokens: map[string]bool{}, baseURL: opt.BaseURL, tz: opt.TZ,
		proxies: opt.TrustedProxies, pending: map[string]string{},
	}
	if w.db != nil {
		w.server, _ = serverVersion(ctx, w.db)
	}
	if err := os.MkdirAll(opt.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("data directory: %w", err)
	}
	if err := writeSecretFile(w.codeFile(), w.Code()+"\n"); err != nil {
		return nil, fmt.Errorf("writing the setup code: %w", err)
	}
	return w, nil
}

// Code returns the setup code as shown to people: 4F7K-9QX2.
func (w *Setup) Code() string { return w.code[:4] + "-" + w.code[4:] }

func (w *Setup) codeFile() string { return filepath.Join(w.opt.DataDir, "setup-code") }

// DB returns the database, once known.
func (w *Setup) DB() *sql.DB {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.db
}

// Banner is the framed message telling the administrator where to go and
// which code to type, for the server log.
func (w *Setup) Banner(addr string) string {
	where := "http://localhost" + addr + "/setup"
	if strings.HasPrefix(addr, ":") {
		where += " (or this server's address)"
	}
	if w.baseURL != "" {
		where = w.baseURL + "/setup"
	}
	lines := []string{
		"Solstice is not installed yet.",
		"Open " + where,
		"and type this code to start:",
		"",
		"    Setup code: " + w.Code(),
		"",
		"The code is also in " + w.codeFile() + ".",
		"It changes on every start until setup is done.",
	}
	width := 0
	for _, l := range lines {
		width = max(width, utf8.RuneCountInString(l))
	}
	var sb strings.Builder
	rule := "+" + strings.Repeat("-", width+4) + "+\n"
	sb.WriteString("\n" + rule)
	for _, l := range lines {
		sb.WriteString("|  " + l + strings.Repeat(" ", width-utf8.RuneCountInString(l)) + "  |\n")
	}
	sb.WriteString(rule)
	return sb.String()
}

// writeSecretFile writes a file only its owner can read.
func writeSecretFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- path built by the server, not from user input
	if err != nil {
		return err
	}
	if err := f.Chmod(0o600); err != nil { // an older file may have looser rights
		f.Close()
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Handler serves setup mode. static serves the interface; isAsset tells its
// files (scripts, styles, fonts) from its pages.
func (w *Setup) Handler(static http.Handler, isAsset func(string) bool) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/setup/status", w.wrap(w.status, false))
	mux.Handle("POST /api/setup/unlock", w.wrap(w.unlock, false))
	mux.Handle("POST /api/setup/database/test", w.wrap(w.testDatabase, true))
	mux.Handle("POST /api/setup/database", w.wrap(w.saveDatabase, true))
	mux.Handle("POST /api/setup/instance", w.wrap(w.saveInstance, true))
	mux.Handle("POST /api/setup/source/test", w.wrap(w.testSource, true))
	mux.Handle("POST /api/setup/import", w.wrap(w.startImport, true))
	mux.Handle("GET /api/setup/import", w.wrap(w.importState, true))
	mux.Handle("GET /api/setup/accounts", w.wrap(w.importedAccounts, true))
	mux.Handle("POST /api/setup/admin", w.wrap(w.createAdmin, true))
	mux.Handle("/api/setup/", w.wrap(func(http.ResponseWriter, *http.Request) error {
		return fail(http.StatusNotFound, i18n.RequestUnknownAddress)
	}, false))
	notInstalled := w.wrap(func(http.ResponseWriter, *http.Request) error {
		return &httpError{status: http.StatusServiceUnavailable, extra: map[string]any{"setup": true},
			msg: i18n.M(i18n.SetupNotDone)}
	}, false)
	mux.Handle("/api/", notInstalled)
	mux.Handle("/rest/", notInstalled)
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/setup" || strings.HasPrefix(p, "/setup/") || isAsset(p):
			static.ServeHTTP(rw, r)
		case r.Method == http.MethodGet || r.Method == http.MethodHead:
			http.Redirect(rw, r, "/setup", http.StatusSeeOther)
		default:
			notInstalled.ServeHTTP(rw, r)
		}
	})
	return securityHeaders(mux)
}

// wrap is Server.wrap for the wizard: the CSRF header on writes, the setup
// code where needed, and errors written in the request's language.
func (w *Setup) wrap(h handler, needCode bool) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get(csrfHeader) == "" {
			writeError(rw, r, failf(http.StatusForbidden, i18n.RequestMissingHeader, csrfHeader))
			return
		}
		if needCode && !w.unlocked(r) {
			writeError(rw, r, fail(http.StatusUnauthorized, i18n.SetupCodeRequired))
			return
		}
		if err := h(rw, r); err != nil {
			var he *httpError
			if !errors.As(err, &he) {
				if errors.Is(err, context.Canceled) {
					return
				}
				w.log.Error("setup request failed", "method", r.Method, "path", r.URL.Path, "err", err)
				err = fail(http.StatusInternalServerError, i18n.RequestInternalError)
			}
			writeError(rw, r, err)
		}
	})
}

func (w *Setup) unlocked(r *http.Request) bool {
	c, err := r.Cookie(setupCookie)
	if err != nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return !w.done && w.tokens[auth.TokenHash(c.Value)]
}

var errInstalled = fail(http.StatusConflict, i18n.SetupDone)

// unlock checks the setup code, throttled per client address like
// passwords, and keeps the browser unlocked with a cookie.
func (w *Setup) unlock(rw http.ResponseWriter, r *http.Request) error {
	var in struct {
		Code string `json:"code"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	key := "setup:" + clientIP(r, w.proxies)
	if d := w.limiter.Locked(key); d > 0 {
		return tooManyAttempts(d)
	}
	typed := strings.Map(func(c rune) rune {
		if c == '-' || c == ' ' {
			return -1
		}
		return c
	}, strings.ToUpper(strings.TrimSpace(in.Code)))
	if subtle.ConstantTimeCompare([]byte(typed), []byte(w.code)) != 1 {
		w.limiter.Failure(key)
		w.log.Warn("wrong setup code", "client", clientIP(r, w.proxies))
		return fail(http.StatusForbidden, i18n.SetupCodeIncorrect)
	}
	w.limiter.Success(key)
	token, err := auth.RandomToken()
	if err != nil {
		return err
	}
	w.mu.Lock()
	if w.done {
		w.mu.Unlock()
		return errInstalled
	}
	w.tokens[auth.TokenHash(token)] = true
	w.mu.Unlock()
	// Secure only when this request is: the wizard often runs over plain
	// http before the HTTPS proxy of the public address exists.
	http.SetCookie(rw, &http.Cookie{Name: setupCookie, Value: token, Path: "/api/setup", HttpOnly: true,
		Secure: secureRequest(r, "", w.proxies, w.log), SameSite: http.SameSiteStrictMode})
	w.log.Info("setup wizard unlocked", "client", clientIP(r, w.proxies))
	rw.WriteHeader(http.StatusNoContent)
	return nil
}

// secureRequest tells whether cookies must be marked Secure: the request
// came over HTTPS, directly or through a trusted proxy, or the public
// address is HTTPS. A https claim from any other peer is not believed and
// logged, as it is probably a proxy missing from the list.
func secureRequest(r *http.Request, baseURL string, trusted []netip.Prefix, log *slog.Logger) bool {
	if r.TLS != nil || strings.HasPrefix(baseURL, "https://") {
		return true
	}
	if r.Header.Get("X-Forwarded-Proto") != "https" {
		return false
	}
	addr, ok := peerAddr(r.RemoteAddr)
	if ok && isTrusted(trusted, addr) {
		return true
	}
	warnUntrustedProto(log, r.RemoteAddr)
	return false
}

type setupStatus struct {
	Setup    bool   `json:"setup"`
	Unlocked bool   `json:"unlocked"`
	Version  string `json:"version"`
	Database struct {
		Configured bool   `json:"configured"`
		Server     string `json:"server,omitempty"`
	} `json:"database"`
	// Only once unlocked:
	DataDir    string      `json:"dataDir,omitempty"`
	ConfigPath string      `json:"configPath,omitempty"`
	BaseURL    string      `json:"baseURL,omitempty"`
	TZ         string      `json:"tz,omitempty"`
	Imported   bool        `json:"imported,omitempty"`
	Import     *importView `json:"import,omitempty"`
	Config     *configSave `json:"config,omitempty"`
}

// status tells the wizard where things stand. Before the code is given it
// only says whether setup is on and whether a database is set.
func (w *Setup) status(rw http.ResponseWriter, r *http.Request) error {
	if err := w.checkForeignAccounts(r.Context()); err != nil && !errors.Is(err, errInstalled) {
		return err
	}
	unlocked := w.unlocked(r)
	w.mu.Lock()
	defer w.mu.Unlock()
	out := setupStatus{Setup: !w.done, Unlocked: unlocked, Version: w.opt.Version}
	if w.done {
		return writeJSON(rw, http.StatusOK, out)
	}
	out.Database.Configured = w.db != nil
	if unlocked {
		out.Database.Server = w.server
		out.DataDir, out.ConfigPath = w.opt.DataDir, w.opt.ConfigPath
		out.BaseURL, out.TZ = w.baseURL, w.tz
		out.Imported = w.imported
		out.Config = w.lastSave
		if w.job != nil {
			v := w.job.view(locale(r))
			out.Import = &v
		}
	}
	return writeJSON(rw, http.StatusOK, out)
}

// checkForeignAccounts ends setup mode when accounts appeared in the
// database by other means than this wizard (solstice user add, an import
// from the command line): the instance is set up.
func (w *Setup) checkForeignAccounts(ctx context.Context) error {
	w.mu.Lock()
	conn, imported, done := w.db, w.imported, w.done
	w.mu.Unlock()
	if done {
		return errInstalled
	}
	if conn == nil || imported {
		return nil
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	w.log.Info("accounts exist: setup mode ends")
	w.finish("")
	return errInstalled
}

// begin starts a mutating step: it takes the lock (released by the caller)
// and refuses once setup is over or when accounts were made elsewhere.
func (w *Setup) begin(ctx context.Context) error {
	if err := w.checkForeignAccounts(ctx); err != nil {
		return err
	}
	w.mu.Lock()
	if w.done {
		w.mu.Unlock()
		return errInstalled
	}
	return nil
}

// finish ends setup mode: the code no longer opens anything, its file goes,
// and the server switches to the application. Called with w.mu unlocked.
func (w *Setup) finish(admin string) {
	w.mu.Lock()
	if w.done {
		w.mu.Unlock()
		return
	}
	w.done = true
	clear(w.tokens)
	res := SetupResult{DB: w.db, BaseURL: w.baseURL, TZ: w.tz, Admin: admin}
	w.mu.Unlock()
	if err := os.Remove(w.codeFile()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		w.log.Warn("could not delete the setup code file", "path", w.codeFile(), "err", err)
	}
	w.log.Info("setup complete", "admin", admin)
	if w.opt.Done != nil {
		w.opt.Done(res)
	}
}

// configSave tells what became of the configuration file.
type configSave struct {
	Path    string `json:"path"`
	Written bool   `json:"written"`
	// When the file could not be written: the lines to put in it by hand.
	Lines string `json:"lines,omitempty"`
	Error string `json:"error,omitempty"`
}

// save records settings and writes them to the configuration file. A file
// that cannot be written is not an error: the wizard shows the lines to
// paste. Called with w.mu held.
func (w *Setup) save(values map[string]string) *configSave {
	for k, v := range values {
		w.pending[k] = v
	}
	out := &configSave{Path: w.opt.ConfigPath, Written: true}
	if err := config.Update(w.opt.ConfigPath, w.pending); err != nil {
		w.log.Warn("could not write the configuration file", "path", w.opt.ConfigPath, "err", err)
		out.Written, out.Lines, out.Error = false, config.Lines(w.pending), err.Error()
	}
	w.lastSave = out
	return out
}

// SetBaseURL sets the public address of the instance (SOLSTICE_HTTP_BASEURL):
// when it is https, session cookies are always Secure.
// It is an initial value: one saved in the administration wins.
func (s *Server) SetBaseURL(u string) {
	s.baseURL = strings.TrimSuffix(u, "/")
	s.forgetGeneral()
}

// setupDone answers the wizard's status once the instance is set up.
func (s *Server) setupDone(w http.ResponseWriter, _ *http.Request) error {
	return writeJSON(w, http.StatusOK, map[string]bool{"setup": false})
}
