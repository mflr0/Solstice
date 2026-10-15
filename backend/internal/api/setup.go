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
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"solstice/internal/auth"
	"solstice/internal/config"
	"solstice/internal/i18n"
)

// SetupOptions configures the wizard.
type SetupOptions struct {
	// DB is the database, or nil when none is configured yet: the wizard
	// then asks for one.
	DB *sql.DB
	// DBFixed tells that the database comes from the environment or the
	// command line, so the wizard cannot change it.
	DBFixed bool
	// DataDir is the data directory in use (absolute); DataFixed as DBFixed.
	DataDir   string
	DataFixed bool
	// ConfigPath is the configuration file the wizard writes.
	ConfigPath string
	// BaseURL and TZ are the values in effect; *Fixed when the environment sets them.
	BaseURL, TZ           string
	BaseURLFixed, TZFixed bool
	Version               string
	Log                   *slog.Logger
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
		pending: map[string]string{},
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
		where += " (ou l'adresse de ce serveur)"
	}
	if w.baseURL != "" {
		where = w.baseURL + "/setup"
	}
	lines := []string{
		"Solstice n'est pas encore installé.",
		"Ouvrez " + where,
		"et saisissez ce code pour commencer :",
		"",
		"    Code d'installation : " + w.Code(),
		"",
		"Le code est aussi dans " + w.codeFile() + ".",
		"Il change à chaque démarrage tant que l'installation n'est pas faite.",
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
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
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
		return fail(http.StatusNotFound, "Adresse inconnue.")
	}, false))
	notInstalled := w.wrap(func(http.ResponseWriter, *http.Request) error {
		return &httpError{status: http.StatusServiceUnavailable, extra: map[string]any{"setup": true},
			msg: i18n.M("Solstice n'est pas encore installé : terminez l'installation à l'adresse /setup.")}
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
			writeError(rw, r, failf(http.StatusForbidden, "Requête refusée (en-tête %s manquant).", csrfHeader))
			return
		}
		if needCode && !w.unlocked(r) {
			writeError(rw, r, fail(http.StatusUnauthorized, "Saisissez d'abord le code d'installation."))
			return
		}
		if err := h(rw, r); err != nil {
			var he *httpError
			if !errors.As(err, &he) {
				if errors.Is(err, context.Canceled) {
					return
				}
				w.log.Error("setup request failed", "method", r.Method, "path", r.URL.Path, "err", err)
				err = fail(http.StatusInternalServerError, "Erreur interne, l'incident a été journalisé.")
			}
			writeError(rw, r, err)
		}
	})
}

func tokenKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (w *Setup) unlocked(r *http.Request) bool {
	c, err := r.Cookie(setupCookie)
	if err != nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return !w.done && w.tokens[tokenKey(c.Value)]
}

var errInstalled = fail(http.StatusConflict, "Solstice est déjà installé.")

// unlock checks the setup code, throttled per client address like
// passwords, and keeps the browser unlocked with a cookie.
func (w *Setup) unlock(rw http.ResponseWriter, r *http.Request) error {
	var in struct {
		Code string `json:"code"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	key := "setup:" + clientIP(r)
	if d := w.limiter.Locked(key); d > 0 {
		return failf(http.StatusTooManyRequests, "Trop de tentatives : réessayez dans %d s.", int(d.Seconds())+1)
	}
	typed := strings.Map(func(c rune) rune {
		if c == '-' || c == ' ' {
			return -1
		}
		return c
	}, strings.ToUpper(strings.TrimSpace(in.Code)))
	if subtle.ConstantTimeCompare([]byte(typed), []byte(w.code)) != 1 {
		w.limiter.Failure(key)
		w.log.Warn("wrong setup code", "client", clientIP(r))
		return fail(http.StatusForbidden, "Code d'installation incorrect.")
	}
	w.limiter.Success(key)
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	w.mu.Lock()
	if w.done {
		w.mu.Unlock()
		return errInstalled
	}
	w.tokens[tokenKey(token)] = true
	w.mu.Unlock()
	// Secure only when this request is: the wizard often runs over plain
	// http before the HTTPS proxy of the public address exists.
	http.SetCookie(rw, &http.Cookie{Name: setupCookie, Value: token, Path: "/api/setup", HttpOnly: true,
		Secure: secureRequest(r, ""), SameSite: http.SameSiteStrictMode})
	w.log.Info("setup wizard unlocked", "client", clientIP(r))
	rw.WriteHeader(http.StatusNoContent)
	return nil
}

// secureRequest tells whether cookies must be marked Secure: the request
// came over HTTPS, directly or through a proxy, or the public address is HTTPS.
func secureRequest(r *http.Request, baseURL string) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" || strings.HasPrefix(baseURL, "https://")
}

type setupStatus struct {
	Setup    bool   `json:"setup"`
	Unlocked bool   `json:"unlocked"`
	Version  string `json:"version"`
	Database struct {
		Configured bool   `json:"configured"`
		Fixed      bool   `json:"fixed"`
		Server     string `json:"server,omitempty"`
	} `json:"database"`
	// Only once unlocked:
	DataDir      string      `json:"dataDir,omitempty"`
	ConfigPath   string      `json:"configPath,omitempty"`
	BaseURL      string      `json:"baseURL,omitempty"`
	BaseURLFixed bool        `json:"baseURLFixed,omitempty"`
	TZ           string      `json:"tz,omitempty"`
	TZFixed      bool        `json:"tzFixed,omitempty"`
	Imported     bool        `json:"imported,omitempty"`
	Import       *importView `json:"import,omitempty"`
	Config       *configSave `json:"config,omitempty"`
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
	out.Database.Fixed = w.opt.DBFixed
	if unlocked {
		out.Database.Server = w.server
		out.DataDir, out.ConfigPath = w.opt.DataDir, w.opt.ConfigPath
		out.BaseURL, out.BaseURLFixed = w.baseURL, w.opt.BaseURLFixed
		out.TZ, out.TZFixed = w.tz, w.opt.TZFixed
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

// SetBaseURL sets the public address of the instance (SOLSTICE_BASE_URL):
// when it is https, session cookies are always Secure.
func (s *Server) SetBaseURL(u string) { s.baseURL = strings.TrimSuffix(u, "/") }

// setupDone answers the wizard's status once the instance is set up.
func (s *Server) setupDone(w http.ResponseWriter, _ *http.Request) error {
	return writeJSON(w, http.StatusOK, map[string]bool{"setup": false})
}
