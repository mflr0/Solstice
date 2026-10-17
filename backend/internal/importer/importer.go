// Package importer copies a Jira Data Center database into Solstice.
//
// The Jira database is read directly (PostgreSQL or MySQL). Jira's own
// identifiers are kept, so links between records survive the trip and an
// import can be run again with Replace. Jira "schemes" (permission, issue
// security, issue type schemes) are converted into per-project settings.
//
// Rows that point at something that no longer exists in Jira (orphans) are
// skipped and counted in the report rather than failing the import.
package importer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"solstice/internal/jiradb"
	"solstice/internal/sqlq"
)

// Options configures an import.
type Options struct {
	// SourceURL is the Jira database: postgres://… or mysql://….
	SourceURL string
	// SourceTZ is the time zone of the Jira server. MySQL stores wall-clock
	// times in it; on both engines it gives the day of date-only fields.
	SourceTZ *time.Location
	// JiraHome, when set, is the Jira home directory attachments are copied from.
	JiraHome string
	// DataDir is Solstice's data directory (attachments go to DataDir/attachments).
	DataDir string
	// Replace wipes Solstice's existing data first, keeping its own
	// configuration (see keep.go). Without it, a non-empty Solstice database
	// is refused.
	Replace bool
	// Operator is the user name of the administrator who started the import
	// from Solstice: they keep their account, password and rights.
	Operator string
	Log      *slog.Logger
	// Progress, when set, is told how the import advances: as each phase
	// starts, now and then while it copies rows, and once at the end. It runs
	// on the importing goroutine and must return quickly.
	Progress func(Progress)
}

// Progress is how far an import has gone.
type Progress struct {
	Phase string `json:"phase"` // the phase under way ("users", "issues"…), "" once finished
	Done  int    `json:"done"`  // phases finished
	Total int    `json:"total"` // phases in all
	Rows  int    `json:"rows"`  // rows written so far, all tables together
	Steps []Step `json:"steps"` // the report's lines so far
}

// Report summarises what an import did.
type Report struct {
	Steps    []Step   `json:"steps"`
	Warnings []string `json:"warnings"`
}

// Step is one kind of record and how many were copied or left out.
type Step struct {
	Name     string `json:"name"`
	Imported int    `json:"imported"`
	Skipped  int    `json:"skipped"`
}

// Count returns the number imported for a step name, or -1 if absent.
func (r *Report) Count(name string) int {
	for _, s := range r.Steps {
		if s.Name == name {
			return s.Imported
		}
	}
	return -1
}

func (r *Report) step(name string, imported, skipped int) {
	r.Steps = append(r.Steps, Step{Name: name, Imported: imported, Skipped: skipped})
}

func (r *Report) warn(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}

// ErrBusy means another import into the same database is running.
var ErrBusy = errors.New("another import into this database is running")

// Run performs the import into target.
func Run(ctx context.Context, target *sql.DB, opt Options) (*Report, error) {
	if opt.SourceTZ == nil {
		return nil, errors.New("the Jira server time zone is required")
	}
	if opt.Log == nil {
		opt.Log = slog.Default()
	}
	src, engine, err := jiradb.Open(opt.SourceURL, opt.SourceTZ)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	if err := src.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("cannot reach the Jira database: %w", err)
	}

	// One connection for the whole import: foreign key checks are switched off
	// for this session so tables can be filled in any order. Orphans are
	// filtered out in code instead.
	dst, err := target.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	// One import at a time per database, whichever process runs it.
	var locked sql.NullInt64
	if err := dst.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT('solstice-import:', DATABASE()), 0)").Scan(&locked); err != nil {
		return nil, err
	}
	if locked.Int64 != 1 {
		return nil, ErrBusy
	}
	// In a closure: a plain defer would run the query now and keep its row
	// open on dst.
	defer func() {
		dst.QueryRowContext(context.WithoutCancel(ctx), "SELECT RELEASE_LOCK(CONCAT('solstice-import:', DATABASE()))").
			Scan(&locked)
	}()
	if _, err := dst.ExecContext(ctx, "SET SESSION foreign_key_checks = 0"); err != nil {
		return nil, err
	}
	defer dst.ExecContext(context.WithoutCancel(ctx), "SET SESSION foreign_key_checks = 1")

	r := &run{
		ctx:    ctx,
		src:    src,
		engine: engine,
		tz:     opt.SourceTZ,
		dst:    dst,
		opt:    opt,
		rep:    &Report{},
	}
	k, err := r.prepareTarget()
	if err != nil {
		return nil, err
	}
	steps := []struct {
		name string
		fn   func() error
	}{
		{"users", r.importUsers},
		{"groups", r.importGroups},
		{"catalogues", r.importCatalogues},
		{"projects", r.importProjects},
		{"roles", r.importRoles},
		{"project issue types", r.importProjectIssueTypes},
		{"workflows", r.importWorkflows},
		{"permissions", r.importPermissions},
		{"security levels", r.importSecurityLevels},
		{"components and versions", r.importComponentsAndVersions},
		{"custom fields", r.importCustomFields},
		{"issues", r.importIssues},
		{"issue details", r.importIssueDetails},
		{"comments and work logs", r.importCommentsAndWorklogs},
		{"attachments", r.importAttachments},
		{"custom field values", r.importCustomFieldValues},
		{"history", r.importHistory},
		{"filters", r.importFilters},
		{"boards and sprints", r.importBoards},
	}
	r.phases = len(steps)
	var runErr error
	for i, s := range steps {
		r.phase, r.done = s.name, i
		r.progress(true)
		start := time.Now()
		if err := s.fn(); err != nil {
			runErr = fmt.Errorf("%s: %w", s.name, err)
			break
		}
		opt.Log.Info("import step done", "step", s.name, "duration", time.Since(start).Round(time.Millisecond))
	}
	if r.strangers > 0 {
		r.rep.warn("%s named by issues, comments or history but unknown to Jira, kept as inactive accounts",
			plural(r.strangers, "user", "users"))
	}
	// Solstice's own data goes back even after a failure: the operator must
	// still be able to sign in and try again.
	r.ctx = context.WithoutCancel(ctx)
	if err := r.putBack(k); err != nil {
		if runErr == nil {
			runErr = fmt.Errorf("restoring Solstice's own data: %w", err)
		} else {
			opt.Log.Error("restoring Solstice's own data failed", "err", err)
		}
	}
	if runErr != nil {
		return r.rep, runErr
	}
	r.phase, r.done = "", len(steps)
	r.progress(true)
	return r.rep, nil
}

// progress tells Options.Progress where the import stands; unless forced,
// at most a few times a second.
func (r *run) progress(force bool) {
	if r.opt.Progress == nil || (!force && time.Since(r.told) < 250*time.Millisecond) {
		return
	}
	r.told = time.Now()
	r.opt.Progress(Progress{Phase: r.phase, Done: r.done, Total: r.phases, Rows: r.rows,
		Steps: slices.Clone(r.rep.Steps)})
}

// run holds the state shared by the import steps: how Jira identifiers map to
// Solstice rows, and which records exist (to filter orphans).
type run struct {
	ctx    context.Context
	src    *sql.DB
	engine jiradb.Engine
	tz     *time.Location
	dst    *sql.Conn
	opt    Options
	rep    *Report

	// Where the import stands, for Options.Progress.
	phase        string
	done, phases int
	rows         int
	told         time.Time

	users       map[string]int64 // Jira user key → Solstice user id
	usersByName map[string]int64 // lower-case user name → Solstice user id
	cwd         map[string]cwdUser
	activeUsers []int64
	licences    map[string][]int64 // Jira application (e.g. jira-software) → licensed groups
	nextUserID  int64
	groups      map[string]int64 // lower-case group name → Solstice group id
	userGroups  map[int64][]int64
	projects    map[int64]*project
	roles       map[int64]bool
	issueTypes  map[string]bool
	statuses    map[string]bool
	priorities  map[string]bool
	resolutions map[string]bool
	components  map[int64]bool
	versions    map[int64]bool
	fields      map[int64]string // custom field id → type key
	options     map[int64]bool
	linkTypes   map[int64]bool
	subtaskLink map[int64]bool   // Jira's hidden link types that encode sub-tasks
	epicLink    map[int64]bool   // Jira Software's link type from an epic to its issues
	epicOf      map[int64]int64  // issue → epic, applied once all issues exist
	filters     map[int64]string // saved filter id → its query
	issues      map[int64]issueRef
	// Security levels are copied per project, so one Jira level can become
	// several Solstice levels: (Jira level id, project id) → Solstice level id.
	levels      map[[2]int64]int64
	nextLevelID int64
	strangers   int // users named by rows but unknown to Jira, given an inactive account
}

type project struct {
	key, originalKey string
	leadID           int64
	maxNum           int64
}

type issueRef struct {
	project int64
	num     int64
}

// prepareTarget refuses to import over existing data unless Replace is set.
// It saves what Solstice keeps (keep.go), then empties every other table,
// default catalogues of a fresh instance included.
func (r *run) prepareTarget() (*kept, error) {
	var n int
	if err := r.dst.QueryRowContext(r.ctx,
		"SELECT (SELECT COUNT(*) FROM users) + (SELECT COUNT(*) FROM projects)").Scan(&n); err != nil {
		return nil, fmt.Errorf("the Solstice database is not initialised (run solstice migrate): %w", err)
	}
	if n > 0 && !r.opt.Replace {
		return nil, errors.New("the Solstice database already holds data; use -replace to overwrite it")
	}
	k, err := r.keep()
	if err != nil {
		return nil, err
	}
	rows, err := r.dst.QueryContext(r.ctx, `SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'`)
	if err != nil {
		return nil, err
	}
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			rows.Close()
			return nil, err
		}
		if !slices.Contains(keptTables, t) {
			tables = append(tables, t)
		}
	}
	rows.Close()
	for _, t := range tables {
		if _, err := r.dst.ExecContext(r.ctx, "TRUNCATE TABLE `"+t+"`"); err != nil {
			return nil, err
		}
	}
	return k, nil
}

// leftOut warns about rows that were not imported because they point to
// something Jira no longer has.
func (r *run) leftOut(n int, one, many, why string) {
	if n > 0 {
		r.rep.warn("%s %s, not imported", plural(n, one, many), why)
	}
}

// each runs a query on the Jira database and calls scan for every row.
func (r *run) each(query string, scan func(*sql.Rows) error) error {
	rows, err := r.src.QueryContext(r.ctx, query)
	if err != nil {
		return fmt.Errorf("reading Jira (%s): %w", firstLine(query), err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// hasColumn tells whether the Jira database has a column; some columns only
// exist in recent Jira versions.
func (r *run) hasColumn(table, column string) bool {
	// Case-insensitive: Jira Software's tables are upper-case on PostgreSQL.
	var q string
	if r.engine == jiradb.MySQL {
		q = `SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = DATABASE() AND LOWER(table_name) = ? AND LOWER(column_name) = ?`
	} else {
		q = `SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = current_schema() AND LOWER(table_name) = $1 AND LOWER(column_name) = $2`
	}
	var n int
	if err := r.src.QueryRowContext(r.ctx, q, strings.ToLower(table), strings.ToLower(column)).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// projectIDs returns the imported project ids in order, so that rows created
// per project get the same ids on every run.
func (r *run) projectIDs() []int64 {
	ids := make([]int64, 0, len(r.projects))
	for id := range r.projects {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// quote protects a Jira column named after a reserved word (e.g. "lead",
// reserved since MySQL 8).
func (r *run) quote(column string) string {
	if r.engine == jiradb.MySQL {
		return "`" + column + "`"
	}
	return `"` + column + `"`
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// user returns the Solstice id for a Jira user key. Keys that match no user
// (deleted long ago) get an inactive placeholder so authorship is kept.
func (r *run) user(key sql.NullString) (any, error) {
	if !key.Valid || key.String == "" {
		return nil, nil
	}
	if id, ok := r.users[key.String]; ok {
		return id, nil
	}
	r.nextUserID++
	id := r.nextUserID
	username := "jira-" + strings.ToLower(key.String)
	if _, taken := r.usersByName[username]; taken {
		username += "-" + strconv.FormatInt(id, 10)
	}
	r.usersByName[username] = id
	if _, err := r.dst.ExecContext(r.ctx,
		`INSERT INTO users (id, username, user_key, display_name, active) VALUES (?, ?, ?, ?, FALSE)`,
		id, username, key.String, key.String); err != nil {
		return nil, err
	}
	r.users[key.String] = id
	r.strangers++
	return id, nil
}

// inserter batches rows into multi-row INSERT statements.
type inserter struct {
	r       *run
	head    string
	ncols   int
	maxRows int
	rows    int
	bytes   int
	args    []any
	total   int
}

func (r *run) inserter(table string, cols ...string) *inserter {
	return &inserter{
		r:       r,
		head:    "INSERT INTO " + table + " (" + strings.Join(cols, ", ") + ") VALUES ",
		ncols:   len(cols),
		maxRows: min(500, 60000/len(cols)),
	}
}

func (in *inserter) add(vals ...any) error {
	if len(vals) != in.ncols {
		return fmt.Errorf("BUG: %d values for %d columns in %s", len(vals), in.ncols, in.head)
	}
	// Only plain strings are counted: unwrap sql.NullString with sqlq.Value.
	for _, v := range vals {
		if s, ok := v.(string); ok {
			in.bytes += len(s)
		}
	}
	in.args = append(in.args, vals...)
	in.rows++
	// Stay well under MariaDB's max_allowed_packet with long descriptions.
	if in.rows >= in.maxRows || in.bytes > 4<<20 {
		return in.flush()
	}
	return nil
}

func (in *inserter) flush() error {
	if in.rows == 0 {
		return nil
	}
	tuple := "(" + sqlq.Marks(in.ncols) + ")"
	var sb strings.Builder
	sb.WriteString(in.head)
	for i := range in.rows {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(tuple)
	}
	if _, err := in.r.dst.ExecContext(in.r.ctx, sb.String(), in.args...); err != nil {
		return fmt.Errorf("%s…: %w", strings.TrimSuffix(in.head, " VALUES "), err)
	}
	in.total += in.rows
	in.r.rows += in.rows
	in.r.progress(false)
	in.rows, in.bytes, in.args = 0, 0, in.args[:0]
	return nil
}

// Value helpers: Jira columns are nullable almost everywhere.

func ntime(t sql.NullTime) any {
	if !t.Valid {
		return nil
	}
	return t.Time.UTC()
}

// ndate gives the calendar day of a date-only Jira field. Jira stores those
// as midnight in the server's time zone.
func (r *run) ndate(t sql.NullTime) any {
	if !t.Valid {
		return nil
	}
	return t.Time.In(r.tz).Format(time.DateOnly)
}

// truthy reads Jira's many spellings of a boolean: 'Y', 'true', '1'.
func truthy(s sql.NullString) bool {
	switch strings.ToLower(strings.TrimSpace(s.String)) {
	case "y", "yes", "true", "1":
		return true
	}
	return false
}

func orDefault(s sql.NullString, def string) string {
	if !s.Valid || s.String == "" {
		return def
	}
	return s.String
}

// truncate cuts s to at most n runes (Solstice's VARCHAR(255) columns).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}
