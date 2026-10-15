package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
		return "", fail(http.StatusBadRequest, "Invalid host.")
	case f.Port < 1 || f.Port > 65535:
		return "", fail(http.StatusBadRequest, "Invalid port (1 to 65535).")
	case !dbNamePattern.MatchString(f.Name):
		return "", fail(http.StatusBadRequest, "Invalid database name: letters, digits, _ and - only.")
	case f.User == "":
		return "", fail(http.StatusBadRequest, "Enter the database user.")
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
func probeDatabase(ctx context.Context, f dbForm, rawURL string) (*sql.DB, *dbProbe, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := db.Open(ctx, rawURL)
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
		return nil, failf(http.StatusBadRequest,
			"Version too old: %s. Solstice needs at least MariaDB 10.6 or MySQL 8.0.", server)
	}
	var charset string
	if err := conn.QueryRowContext(ctx, `SELECT DEFAULT_CHARACTER_SET_NAME FROM information_schema.SCHEMATA
		WHERE SCHEMA_NAME = DATABASE()`).Scan(&charset); err != nil {
		return nil, err
	}
	if charset != "utf8mb4" {
		return nil, failf(http.StatusBadRequest, "The database “%s” uses %s; Solstice needs utf8mb4. While it is empty, convert it: ALTER DATABASE %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;",
			name, charset, name)
	}
	var ours int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(table_name = 'schema_migrations'), 0)
		FROM information_schema.tables WHERE table_schema = DATABASE()`).Scan(&p.Tables, &ours); err != nil {
		return nil, err
	}
	if p.Tables > 0 && ours == 0 {
		return nil, failf(http.StatusBadRequest,
			"The database “%s” already holds tables that are not Solstice's: choose an empty database.", name)
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
			return fail(http.StatusBadRequest, "The database server rejected the user name or password.")
		case 1044:
			return failf(http.StatusBadRequest, "User %s has no access to the database “%s”, or it does not exist.", user, name)
		case 1049:
			return failf(http.StatusBadRequest, "There is no database “%s” on this server: create it first.", name)
		}
		return failf(http.StatusBadRequest, "Cannot connect to the database: %s", me.Message)
	}
	if unreachable(err) {
		return failf(http.StatusBadRequest,
			"Cannot reach the database server at %s: check the host, the port and the firewall.", addr)
	}
	return failf(http.StatusBadRequest, "Cannot connect to the database: %s", err.Error())
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
	conn, p, err := probeDatabase(r.Context(), f, rawURL)
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
	switch {
	case w.opt.DBFixed:
		return fail(http.StatusConflict, "The database is set by the server's environment: it cannot be changed here.")
	case w.imported:
		return fail(http.StatusConflict, "An import has already filled the database: it can no longer be changed.")
	}
	conn, p, err := probeDatabase(r.Context(), f, rawURL)
	if err != nil {
		return err
	}
	if err := db.Migrate(r.Context(), conn); err != nil {
		conn.Close()
		return failf(http.StatusInternalServerError, "Creating the tables failed: %s", err.Error())
	}
	if w.db != nil && w.db != conn {
		w.db.Close()
	}
	w.db, w.server = conn, p.Server
	values := map[string]string{config.KeyDB: rawURL}
	if !w.opt.DataFixed {
		values[config.KeyData] = w.opt.DataDir
	}
	saved := w.save(values)
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
		return "", fail(http.StatusBadRequest,
			"Invalid public address: enter something like https://tickets.example.org, without a path.")
	}
	return strings.ToLower(u.Scheme + "://" + u.Host), nil
}

// checkTZ accepts IANA time zone names (Europe/Paris), not "Local".
func checkTZ(name string) error {
	name = strings.TrimSpace(name)
	if _, err := time.LoadLocation(name); err != nil || name == "" || name == "Local" || config.CheckValue(name) != nil {
		return failf(http.StatusBadRequest, "Unknown time zone: %s.", name)
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
	values := map[string]string{}
	if !w.opt.DataFixed {
		// Kept whatever the working directory of the next start.
		values[config.KeyData] = w.opt.DataDir
	}
	if !w.opt.BaseURLFixed {
		u, err := normalizeBaseURL(in.BaseURL)
		if err != nil {
			return err
		}
		w.baseURL, values[config.KeyBaseURL] = u, u
	}
	if !w.opt.TZFixed {
		if err := checkTZ(in.TZ); err != nil {
			return err
		}
		w.tz = strings.TrimSpace(in.TZ)
		values[config.KeyTZ] = w.tz
	}
	saved := w.save(values)
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
		return nil, fail(http.StatusBadRequest, "Invalid Jira database address: postgres://… or mysql://… expected.")
	}
	src, engine, err := jiradb.Open(f.URL, loc)
	if err != nil {
		return nil, fail(http.StatusBadRequest, "Invalid Jira database address: postgres://… or mysql://… expected.")
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
			return nil, failf(http.StatusBadRequest, "This does not look like a Jira database (no %s table).", table)
		}
	}
	if f.JiraHome != "" {
		if !filepath.IsAbs(f.JiraHome) {
			return nil, fail(http.StatusBadRequest, "Enter the full path of the Jira home folder, from the root.")
		}
		if info, err := os.Stat(f.JiraHome); err != nil || !info.IsDir() {
			return nil, failf(http.StatusBadRequest, "Folder not found on the server: %s", f.JiraHome)
		}
		if info, err := os.Stat(filepath.Join(f.JiraHome, "data", "attachments")); err == nil && info.IsDir() {
			p.Attachments = true
		} else {
			p.Warning = i18n.M("No data/attachments folder in %s: only the attachments' details will be imported, not their files.",
				f.JiraHome).In(lang)
		}
	}
	return p, nil
}

// sourceError explains a failed connection to the Jira database.
func sourceError(err error, u *url.URL) error {
	name := strings.TrimPrefix(u.Path, "/")
	var me *mysql.MySQLError
	var pe *pgconn.PgError
	switch {
	case errors.As(err, &me) && me.Number == 1045,
		errors.As(err, &pe) && (pe.Code == "28P01" || pe.Code == "28000"):
		return fail(http.StatusBadRequest, "Jira database: user name or password rejected.")
	case errors.As(err, &me) && me.Number == 1049, errors.As(err, &pe) && pe.Code == "3D000":
		return failf(http.StatusBadRequest, "Jira database: there is no database “%s” on this server.", name)
	case errors.As(err, &me):
		return failf(http.StatusBadRequest, "Cannot open the Jira database: %s", me.Message)
	case errors.As(err, &pe):
		return failf(http.StatusBadRequest, "Cannot open the Jira database: %s", pe.Message)
	case unreachable(err):
		return failf(http.StatusBadRequest,
			"Cannot reach the Jira database at %s: check the host, the port and the firewall.", u.Host)
	}
	return failf(http.StatusBadRequest, "Cannot open the Jira database: %s", err.Error())
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
		v.Error = i18n.M("The import failed: %s", j.err.Error()).In(lang)
	}
	return v
}

func (j *importJob) running() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.state == "running"
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
		return fail(http.StatusConflict, "Set up the database first.")
	}
	if w.job != nil && w.job.running() {
		return fail(http.StatusConflict, "An import is already running.")
	}
	loc, _ := time.LoadLocation(f.TZ)
	job := &importJob{state: "running", started: time.Now()}
	w.job, w.imported = job, true
	conn := w.db
	w.log.Info("import from Jira started", "jira_home", f.JiraHome)
	go func() {
		rep, err := importer.Run(w.ctx, conn, importer.Options{
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
		return fail(http.StatusNotFound, "No import has been started.")
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
		return fail(http.StatusConflict, "Finish the import first.")
	}
	like := "%" + escapeLike(strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))) + "%"
	out := []setupAccount{}
	err := scanAll(r.Context(), conn, `SELECT username, display_name, email, is_admin FROM users
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
		return fail(http.StatusConflict, "Set up the database first.")
	}
	if w.job != nil && w.job.running() {
		return fail(http.StatusConflict, "An import is already running.")
	}
	ctx := r.Context()
	var username string
	if existing := strings.ToLower(strings.TrimSpace(in.Existing)); existing != "" {
		if !w.imported {
			return fail(http.StatusBadRequest, "No such account among the imported ones.")
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
				return fail(http.StatusBadRequest, "No such account among the imported ones.")
			}
		}
		username = existing
	} else {
		username = strings.ToLower(strings.TrimSpace(in.Username))
		name, email := strings.TrimSpace(in.Name), strings.TrimSpace(in.Email)
		if !usernamePattern.MatchString(username) {
			return fail(http.StatusBadRequest, "Invalid username: use lowercase letters, digits and . _ @ -")
		}
		if name == "" {
			name = username
		}
		if a, err := mail.ParseAddress(email); err != nil || a.Address != email || a.Name != "" {
			return fail(http.StatusBadRequest, "Invalid email address.")
		}
		if utf8Len(in.Password) < minAdminPassword {
			return failf(http.StatusBadRequest, "The password must be at least %d characters long.", minAdminPassword)
		}
		if strings.EqualFold(strings.TrimSpace(in.Password), username) {
			return fail(http.StatusBadRequest, "The password cannot be the username.")
		}
		hash, err := auth.HashPassword(in.Password)
		if err != nil {
			return err
		}
		if _, err := w.db.ExecContext(ctx, `INSERT INTO users (username, display_name, email, password_hash, is_admin)
			VALUES (?, ?, ?, ?, TRUE)`, username, name, email, hash); err != nil {
			if strings.Contains(err.Error(), "Duplicate") {
				return fail(http.StatusConflict, "This username is already taken.")
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
	if err := audit.Record(ctx, w.db, audit.Entry{UserID: id, Username: username, IP: clientIP(r),
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
