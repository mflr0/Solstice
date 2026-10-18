package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/config"
	"solstice/internal/db"
	"solstice/internal/i18n"
	"solstice/internal/importer"
	"solstice/internal/jiradb"
	"solstice/internal/sqlq"
)

// The database step ---------------------------------------------------------

type dbForm struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password"`
}

var (
	hostPattern   = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,253}$`)
	dbNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

// url builds the database URL. User and password are percent-encoded
// beyond what URLs require, so the line reads the same to systemd and Docker.
func (f *dbForm) url() (string, error) {
	f.Host = strings.TrimSpace(f.Host)
	f.Name = strings.TrimSpace(f.Name)
	f.User = strings.TrimSpace(f.User)
	if f.Port == 0 {
		f.Port = 3306
	}
	switch {
	case !hostPattern.MatchString(f.Host):
		return "", fail(http.StatusBadRequest, i18n.SetupHostInvalid)
	case f.Port < 1 || f.Port > 65535:
		return "", fail(http.StatusBadRequest, i18n.SetupPortInvalid)
	case !dbNamePattern.MatchString(f.Name):
		return "", fail(http.StatusBadRequest, i18n.SetupDatabaseNameInvalid)
	case f.User == "":
		return "", fail(http.StatusBadRequest, i18n.SetupDatabaseUserRequired)
	}
	return "mysql://" + strictEscape(f.User) + ":" + strictEscape(f.Password) + "@" +
		net.JoinHostPort(f.Host, strconv.Itoa(f.Port)) + "/" + f.Name, nil
}

func strictEscape(s string) string {
	var sb strings.Builder
	for _, b := range []byte(s) {
		if b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || strings.IndexByte("-._~", b) >= 0 {
			sb.WriteByte(b)
		} else {
			fmt.Fprintf(&sb, "%%%02X", b)
		}
	}
	return sb.String()
}

type dbProbe struct {
	Server   string `json:"server"`   // e.g. "MariaDB 11.4.2"
	Tables   int    `json:"tables"`   // tables already in the database
	Accounts int    `json:"accounts"` // accounts, when it already holds a Solstice instance
}

// probeDatabase connects and checks the server and the database: version,
// character set, and that it is empty or Solstice's. The connection is
// returned open on success.
func probeDatabase(ctx context.Context, f dbForm, rawURL string, pool db.Pool) (*sql.DB, *dbProbe, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := db.Open(ctx, rawURL, pool)
	if err != nil {
		return nil, nil, connectError(err, net.JoinHostPort(f.Host, strconv.Itoa(f.Port)), f.Name, f.User)
	}
	p, err := checkDatabase(ctx, conn, f.Name)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, p, nil
}

func checkDatabase(ctx context.Context, conn *sql.DB, name string) (*dbProbe, error) {
	p := &dbProbe{}
	server, ok := serverVersion(ctx, conn)
	p.Server = server
	if !ok {
		return nil, failf(http.StatusBadRequest, i18n.SetupDatabaseTooOld, server)
	}
	var charset string
	if err := conn.QueryRowContext(ctx, `SELECT DEFAULT_CHARACTER_SET_NAME FROM information_schema.SCHEMATA
		WHERE SCHEMA_NAME = DATABASE()`).Scan(&charset); err != nil {
		return nil, err
	}
	if charset != "utf8mb4" {
		return nil, failf(http.StatusBadRequest, i18n.SetupDatabaseCharset,
			name, charset, name)
	}
	var ours int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(table_name = 'schema_migrations'), 0)
		FROM information_schema.tables WHERE table_schema = DATABASE()`).Scan(&p.Tables, &ours); err != nil {
		return nil, err
	}
	if p.Tables > 0 && ours == 0 {
		return nil, failf(http.StatusBadRequest, i18n.SetupDatabaseNotEmpty, name)
	}
	if ours > 0 {
		// A Solstice database: an existing instance if it has accounts.
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&p.Accounts); err != nil {
			p.Accounts = 0 // not migrated far enough to have accounts yet
		}
	}
	return p, nil
}

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?`)

// serverVersion names the database server ("MariaDB 11.4.2", "MySQL 8.4.0")
// and tells whether Solstice supports it: MariaDB 10.6+, MySQL 8.0+.
func serverVersion(ctx context.Context, conn *sql.DB) (string, bool) {
	var v string
	if err := conn.QueryRowContext(ctx, `SELECT VERSION()`).Scan(&v); err != nil {
		return "", false
	}
	product, minMajor, minMinor := "MySQL", 8, 0
	if strings.Contains(strings.ToLower(v), "mariadb") {
		product, minMajor, minMinor = "MariaDB", 10, 6
	}
	m := versionPattern.FindStringSubmatch(v)
	if m == nil {
		return product + " " + v, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return product + " " + m[0], major > minMajor || major == minMajor && minor >= minMinor
}

// connectError explains a failed connection to the Solstice database.
func connectError(err error, addr, name, user string) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		switch me.Number {
		case 1045:
			return fail(http.StatusBadRequest, i18n.SetupDatabaseAuth)
		case 1044:
			return failf(http.StatusBadRequest, i18n.SetupDatabaseAccess, user, name)
		case 1049:
			return failf(http.StatusBadRequest, i18n.SetupDatabaseMissing, name)
		}
		return failf(http.StatusBadRequest, i18n.SetupDatabaseConnect, me.Message)
	}
	if unreachable(err) {
		return failf(http.StatusBadRequest, i18n.SetupDatabaseUnreachable, addr)
	}
	return failf(http.StatusBadRequest, i18n.SetupDatabaseConnect, err.Error())
}

func unreachable(err error) bool {
	var ne net.Error
	var oe *net.OpError
	var de *net.DNSError
	return errors.As(err, &ne) || errors.As(err, &oe) || errors.As(err, &de) ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, mysql.ErrInvalidConn)
}

// testDatabase tries the connection without keeping anything.
func (w *Setup) testDatabase(rw http.ResponseWriter, r *http.Request) error {
	var f dbForm
	if err := readJSON(r, &f); err != nil {
		return err
	}
	rawURL, err := f.url()
	if err != nil {
		return err
	}
	conn, p, err := probeDatabase(r.Context(), f, rawURL, w.opt.Pool)
	if err != nil {
		return err
	}
	conn.Close()
	return writeJSON(rw, http.StatusOK, p)
}

// saveDatabase connects, creates the schema and writes the configuration.
// A database that already holds an instance (accounts) ends setup: Solstice
// simply uses it.
func (w *Setup) saveDatabase(rw http.ResponseWriter, r *http.Request) error {
	var f dbForm
	if err := readJSON(r, &f); err != nil {
		return err
	}
	rawURL, err := f.url()
	if err != nil {
		return err
	}
	if err := w.begin(r.Context()); err != nil {
		return err
	}
	locked := true
	defer func() {
		if locked {
			w.mu.Unlock()
		}
	}()
	if w.imported {
		return fail(http.StatusConflict, i18n.SetupDatabaseImported)
	}
	conn, p, err := probeDatabase(r.Context(), f, rawURL, w.opt.Pool)
	if err != nil {
		return err
	}
	if err := db.Migrate(r.Context(), conn); err != nil {
		conn.Close()
		w.log.Error("creating the tables failed", "err", err)
		return failf(http.StatusInternalServerError, i18n.SetupTablesFailed, i18n.M(i18n.RequestInternalError))
	}
	if w.db != nil && w.db != conn {
		w.db.Close()
	}
	w.db, w.server = conn, p.Server
	saved := w.save(map[string]string{config.KeyDBURL: rawURL, config.KeyData: w.opt.DataDir})
	out := map[string]any{"server": p.Server, "config": saved, "existing": p.Accounts > 0}
	if p.Accounts > 0 {
		locked = false
		w.mu.Unlock()
		w.log.Info("the database holds an instance already: setup mode ends", "accounts", p.Accounts)
		w.finish("")
	}
	return writeJSON(rw, http.StatusOK, out)
}

// The instance step -----------------------------------------------------------

// normalizeBaseURL checks a public address: http or https, a host, no path.
func normalizeBaseURL(s string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" ||
		config.CheckValue(u.Host) != nil {
		return "", fail(http.StatusBadRequest, i18n.SetupPublicURLInvalid)
	}
	return strings.ToLower(u.Scheme + "://" + u.Host), nil
}

// checkTZ accepts IANA time zone names (Europe/Paris), not "Local".
func checkTZ(name string) error {
	name = strings.TrimSpace(name)
	if _, err := time.LoadLocation(name); err != nil || name == "" || name == "Local" || config.CheckValue(name) != nil {
		return failf(http.StatusBadRequest, i18n.CommonTimeZoneUnknown, name)
	}
	return nil
}

func (w *Setup) saveInstance(rw http.ResponseWriter, r *http.Request) error {
	var in struct {
		BaseURL string `json:"baseURL"`
		TZ      string `json:"tz"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if err := w.begin(r.Context()); err != nil {
		return err
	}
	defer w.mu.Unlock()
	u, err := normalizeBaseURL(in.BaseURL)
	if err != nil {
		return err
	}
	if err := checkTZ(in.TZ); err != nil {
		return err
	}
	w.baseURL, w.tz = u, strings.TrimSpace(in.TZ)
	// The data directory is written too: kept whatever the working directory of the next start.
	saved := w.save(map[string]string{config.KeyData: w.opt.DataDir, config.KeyBaseURL: w.baseURL, config.KeyTZ: w.tz})
	return writeJSON(rw, http.StatusOK, map[string]any{"baseURL": w.baseURL, "tz": w.tz, "config": saved})
}

// The Jira import ---------------------------------------------------------------

type sourceForm struct {
	URL      string `json:"url"`
	JiraHome string `json:"jiraHome"`
	TZ       string `json:"tz"` // the Jira server's time zone
}

type sourceProbe struct {
	Engine   string `json:"engine"`
	Projects int    `json:"projects"`
	Issues   int    `json:"issues"`
	Users    int    `json:"users"`
	// Attachments tells whether the Jira home holds attachment files.
	Attachments bool   `json:"attachments"`
	Warning     string `json:"warning,omitempty"`
}

// probeSource connects to the Jira database and looks at the Jira home.
func probeSource(ctx context.Context, f *sourceForm, lang string) (*sourceProbe, error) {
	f.URL, f.JiraHome, f.TZ = strings.TrimSpace(f.URL), strings.TrimSpace(f.JiraHome), strings.TrimSpace(f.TZ)
	if err := checkTZ(f.TZ); err != nil {
		return nil, err
	}
	loc, _ := time.LoadLocation(f.TZ)
	u, err := url.Parse(f.URL)
	if err != nil || u.Host == "" {
		return nil, fail(http.StatusBadRequest, i18n.ImportDatabaseURLInvalid)
	}
	src, engine, err := jiradb.Open(f.URL, loc)
	if err != nil {
		return nil, sourceURLError(err)
	}
	defer src.Close()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := src.PingContext(ctx); err != nil {
		return nil, sourceError(err, u)
	}
	p := &sourceProbe{Engine: engine.String()}
	for table, n := range map[string]*int{"project": &p.Projects, "jiraissue": &p.Issues, "cwd_user": &p.Users} {
		if err := src.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(n); err != nil {
			return nil, failf(http.StatusBadRequest, i18n.ImportNotJira, table)
		}
	}
	if f.JiraHome != "" {
		if !filepath.IsAbs(f.JiraHome) {
			return nil, fail(http.StatusBadRequest, i18n.ImportHomeNotAbsolute)
		}
		if info, err := os.Stat(f.JiraHome); err != nil || !info.IsDir() {
			return nil, failf(http.StatusBadRequest, i18n.ImportHomeNotFound, f.JiraHome)
		}
		if info, err := os.Stat(filepath.Join(f.JiraHome, "data", "attachments")); err == nil && info.IsDir() {
			p.Attachments = true
		} else {
			p.Warning = i18n.M(i18n.ImportNoAttachments,
				f.JiraHome).In(lang)
		}
	}
	return p, nil
}

// sourceURLError explains a Jira database address that was refused: an
// option that is not accepted is named, anything else is a malformed address.
func sourceURLError(err error) error {
	var unknown *db.UnknownParamsError
	var invalid *db.InvalidParamError
	switch {
	case errors.As(err, &unknown):
		return failf(http.StatusBadRequest, i18n.ImportOptionRefused,
			strings.Join(unknown.Names, ", "), strings.Join(unknown.Allowed, ", "))
	case errors.As(err, &invalid):
		return failf(http.StatusBadRequest, i18n.ImportOptionInvalid, invalid.Name)
	}
	return fail(http.StatusBadRequest, i18n.ImportDatabaseURLInvalid)
}

// sourceError explains a failed connection to the Jira database.
func sourceError(err error, u *url.URL) error {
	name := strings.TrimPrefix(u.Path, "/")
	var me *mysql.MySQLError
	var pe *pgconn.PgError
	switch {
	case errors.As(err, &me) && me.Number == 1045,
		errors.As(err, &pe) && (pe.Code == "28P01" || pe.Code == "28000"):
		return fail(http.StatusBadRequest, i18n.ImportDatabaseAuth)
	case errors.As(err, &me) && me.Number == 1049, errors.As(err, &pe) && pe.Code == "3D000":
		return failf(http.StatusBadRequest, i18n.ImportDatabaseMissing, name)
	case errors.As(err, &me):
		return failf(http.StatusBadRequest, i18n.ImportDatabaseOpen, me.Message)
	case errors.As(err, &pe):
		return failf(http.StatusBadRequest, i18n.ImportDatabaseOpen, pe.Message)
	case unreachable(err):
		return failf(http.StatusBadRequest, i18n.ImportDatabaseUnreachable, u.Host)
	}
	return failf(http.StatusBadRequest, i18n.ImportDatabaseOpen, err.Error())
}

func (w *Setup) testSource(rw http.ResponseWriter, r *http.Request) error {
	var f sourceForm
	if err := readJSON(r, &f); err != nil {
		return err
	}
	p, err := probeSource(r.Context(), &f, locale(r))
	if err != nil {
		return err
	}
	return writeJSON(rw, http.StatusOK, p)
}

// importJob is an import running in the background, followed by polling.
type importJob struct {
	mu       sync.Mutex
	state    string // running, done, failed
	progress importer.Progress
	warnings []string
	err      error
	started  time.Time
	finished time.Time
}

type importView struct {
	State    string          `json:"state"`
	Phase    string          `json:"phase"`
	Done     int             `json:"done"`
	Total    int             `json:"total"`
	Rows     int             `json:"rows"`
	Steps    []importer.Step `json:"steps"`
	Warnings []string        `json:"warnings"`
	Error    string          `json:"error,omitempty"`
	Seconds  int             `json:"seconds"`
}

func (j *importJob) view(lang string) importView {
	j.mu.Lock()
	defer j.mu.Unlock()
	v := importView{State: j.state, Phase: j.progress.Phase, Done: j.progress.Done, Total: j.progress.Total,
		Rows: j.progress.Rows, Steps: j.progress.Steps, Warnings: j.warnings}
	if v.Steps == nil {
		v.Steps = []importer.Step{}
	}
	if v.Warnings == nil {
		v.Warnings = []string{}
	}
	end := j.finished
	if end.IsZero() {
		end = time.Now()
	}
	v.Seconds = int(end.Sub(j.started).Seconds())
	if j.err != nil {
		v.Error = i18n.M(i18n.ImportFailed, j.err.Error()).In(lang)
	}
	return v
}

func (j *importJob) running() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.state == "running"
}

// covers the code around RunGuarded: a panic there fails a job still running
// instead of stopping the server.
func (j *importJob) crashed(log *slog.Logger) {
	r := recover()
	if r == nil {
		return
	}
	log.Error("import panicked", "panic", r, "stack", string(debug.Stack()))
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.state == "running" {
		j.state, j.err = "failed", fmt.Errorf("import stopped by an internal error: %v", r)
		j.finished = time.Now()
	}
}

// startImport copies a Jira instance into the database, in the background.
// It replaces whatever an earlier attempt of this wizard left.
func (w *Setup) startImport(rw http.ResponseWriter, r *http.Request) error {
	var f sourceForm
	if err := readJSON(r, &f); err != nil {
		return err
	}
	if _, err := probeSource(r.Context(), &f, locale(r)); err != nil {
		return err
	}
	if err := w.begin(r.Context()); err != nil {
		return err
	}
	defer w.mu.Unlock()
	if w.db == nil {
		return fail(http.StatusConflict, i18n.SetupDatabaseRequired)
	}
	if w.job != nil && w.job.running() {
		return fail(http.StatusConflict, i18n.ImportRunning)
	}
	loc, _ := time.LoadLocation(f.TZ)
	job := &importJob{state: "running", started: time.Now()}
	w.job, w.imported = job, true
	conn := w.db
	w.log.Info("import from Jira started", "jira_home", f.JiraHome)
	go func() {
		defer job.crashed(w.log)
		rep, err := importer.RunGuarded(w.ctx, conn, importer.Options{
			SourceURL: f.URL, SourceTZ: loc, JiraHome: f.JiraHome, DataDir: w.opt.DataDir, Replace: true, Log: w.log,
			Progress: func(p importer.Progress) {
				job.mu.Lock()
				job.progress = p
				job.mu.Unlock()
			},
		})
		job.mu.Lock()
		defer job.mu.Unlock()
		job.finished = time.Now()
		if rep != nil {
			job.progress.Steps, job.warnings = rep.Steps, rep.Warnings
		}
		if err != nil {
			job.state, job.err = "failed", err
			w.log.Error("import from Jira failed", "err", err)
			return
		}
		job.state = "done"
		w.log.Info("import from Jira done", "duration", job.finished.Sub(job.started).Round(time.Second))
	}()
	return writeJSON(rw, http.StatusAccepted, job.view(locale(r)))
}

func (w *Setup) importState(rw http.ResponseWriter, r *http.Request) error {
	w.mu.Lock()
	job := w.job
	w.mu.Unlock()
	if job == nil {
		return fail(http.StatusNotFound, i18n.ImportNotStarted)
	}
	return writeJSON(rw, http.StatusOK, job.view(locale(r)))
}

type setupAccount struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Admin    bool   `json:"admin"` // administrator in Jira already
}

// importedAccounts lists the imported accounts that can sign in (active,
// with their Jira password), administrators first, for choosing who
// administers Solstice.
func (w *Setup) importedAccounts(rw http.ResponseWriter, r *http.Request) error {
	w.mu.Lock()
	conn, job := w.db, w.job
	w.mu.Unlock()
	if job == nil || job.view("").State != "done" {
		return fail(http.StatusConflict, i18n.ImportNotFinished)
	}
	like := "%" + escapeLike(strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))) + "%"
	out := []setupAccount{}
	err := sqlq.Each(r.Context(), conn, `SELECT username, display_name, email, is_admin FROM users
		WHERE active AND password_hash IS NOT NULL
		AND (username LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(email) LIKE ?)
		ORDER BY is_admin DESC, display_name, username LIMIT 50`, func(rows *sql.Rows) error {
		var a setupAccount
		if err := rows.Scan(&a.Username, &a.Name, &a.Email, &a.Admin); err != nil {
			return err
		}
		out = append(out, a)
		return nil
	}, like, like, like)
	if err != nil {
		return err
	}
	return writeJSON(rw, http.StatusOK, out)
}

// The administrator -------------------------------------------------------------

// auditSetupComplete is the audit log's entry for the end of setup.
const auditSetupComplete = "setup.complete"

// minAdminPassword is the shortest password the wizard accepts.
const minAdminPassword = 12

// createAdmin creates the first administrator, or makes an imported account
// one, and ends setup.
func (w *Setup) createAdmin(rw http.ResponseWriter, r *http.Request) error {
	var in struct {
		Existing string `json:"existing"` // an imported account
		Username string `json:"username"`
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if err := w.begin(r.Context()); err != nil {
		return err
	}
	locked := true
	defer func() {
		if locked {
			w.mu.Unlock()
		}
	}()
	if w.db == nil {
		return fail(http.StatusConflict, i18n.SetupDatabaseRequired)
	}
	if w.job != nil && w.job.running() {
		return fail(http.StatusConflict, i18n.ImportRunning)
	}
	ctx := r.Context()
	var username string
	if existing := strings.ToLower(strings.TrimSpace(in.Existing)); existing != "" {
		if !w.imported {
			return fail(http.StatusBadRequest, i18n.SetupAccountNotImported)
		}
		res, err := w.db.ExecContext(ctx, `UPDATE users SET is_admin = TRUE
			WHERE username = ? AND active AND password_hash IS NOT NULL`, existing)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			var found int
			if err := w.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username = ? AND active
				AND password_hash IS NOT NULL`, existing).Scan(&found); err != nil {
				return err
			}
			if found == 0 { // otherwise it was an administrator already
				return fail(http.StatusBadRequest, i18n.SetupAccountNotImported)
			}
		}
		username = existing
	} else {
		username = strings.ToLower(strings.TrimSpace(in.Username))
		name, email := strings.TrimSpace(in.Name), strings.TrimSpace(in.Email)
		if !usernamePattern.MatchString(username) {
			return fail(http.StatusBadRequest, i18n.AccountUsernameInvalid)
		}
		if name == "" {
			name = username
		}
		if a, err := mail.ParseAddress(email); err != nil || a.Address != email || a.Name != "" {
			return fail(http.StatusBadRequest, i18n.AccountEmailInvalid)
		}
		if utf8Len(in.Password) < minAdminPassword {
			return failf(http.StatusBadRequest, i18n.AccountPasswordTooShort, minAdminPassword)
		}
		if strings.EqualFold(strings.TrimSpace(in.Password), username) {
			return fail(http.StatusBadRequest, i18n.AccountPasswordIsUsername)
		}
		hash, err := auth.HashPassword(in.Password)
		if err != nil {
			return err
		}
		if _, err := w.db.ExecContext(ctx, `INSERT INTO users (username, display_name, email, password_hash, is_admin)
			VALUES (?, ?, ?, ?, TRUE)`, username, name, email, hash); err != nil {
			if strings.Contains(err.Error(), "Duplicate") {
				return fail(http.StatusConflict, i18n.AccountUsernameTaken)
			}
			return err
		}
	}
	var id int64
	if err := w.db.QueryRowContext(ctx, `SELECT id FROM users WHERE username = ?`, username).Scan(&id); err != nil {
		return err
	}
	how := "created"
	if in.Existing != "" {
		how = "imported account"
	}
	if err := audit.Record(ctx, w.db, audit.Entry{UserID: id, Username: username, IP: clientIP(r, w.proxies),
		Action: auditSetupComplete, Target: audit.Target{Type: "user", ID: strconv.FormatInt(id, 10), Label: username},
		Details: map[string]any{"administrator": how, "imported": w.imported, "version": w.opt.Version}}); err != nil {
		w.log.Error("audit log write failed", "action", auditSetupComplete, "err", err)
	}
	out := map[string]any{"username": username, "baseURL": w.baseURL, "tz": w.tz, "imported": w.imported,
		"server": w.server, "config": w.lastSave}
	locked = false
	w.mu.Unlock()
	w.finish(username)
	return writeJSON(rw, http.StatusOK, out)
}

func utf8Len(s string) int { return len([]rune(s)) }
