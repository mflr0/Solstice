package api

// The audit log, for administrators: who signed in, and who changed
// accounts, access rights and configuration.

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"slices"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/sqlq"
	"strconv"
	"strings"
	"time"
)

// auditLog records an action of the current user. Handlers call it once the
// action succeeded; failing to write the entry is logged but does not undo
// or fail the action.
func (s *Server) auditLog(r *http.Request, action string, target audit.Target, details map[string]any) {
	e := audit.Entry{IP: clientIP(r, s.proxies), Action: action, Target: target, Details: details}
	if u := currentUser(r); u != nil {
		e.UserID, e.Username = u.ID, u.Username
		if u.TokenID != 0 {
			// Which personal access token did it, should it leak.
			if e.Details == nil {
				e.Details = map[string]any{}
			}
			e.Details["pat"] = u.TokenID
		}
	}
	if err := audit.Record(r.Context(), s.db, e); err != nil {
		s.log.Error("audit log write failed", "action", action, "err", err)
	}
}

// auditLogin records a sign-in attempt, without its password: a success when
// u is the account signed in, a failure when u is nil. The user name is the
// one typed: it may match no account.
func (s *Server) auditLogin(r *http.Request, username string, u *auth.User, method string) {
	e := audit.Entry{Username: strings.ToLower(strings.TrimSpace(username)), IP: clientIP(r, s.proxies),
		Action: audit.LoginFailure, Details: map[string]any{"method": method}}
	if u != nil {
		e.Action, e.UserID, e.Username = audit.LoginSuccess, u.ID, u.Username
	} else if e.Username != "" {
		s.db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE username = ?`, e.Username).Scan(&e.UserID)
	}
	if err := audit.Record(r.Context(), s.db, e); err != nil {
		s.log.Error("audit log write failed", "action", e.Action, "err", err)
	}
}

// auditLockout records that an address was locked after repeated failed
// sign-ins, once when the lock starts (not for each attempt while it lasts).
// The entry names the address and, if the user name that was typed matches an
// account, that account; the log line the address and the id of the account.
// What was typed is kept nowhere, as it may be a password.
func (s *Server) auditLockout(r *http.Request, username string, d time.Duration) {
	name, ip, seconds := strings.ToLower(strings.TrimSpace(username)), clientIP(r, s.proxies), int(d.Seconds())+1
	e := audit.Entry{IP: ip, Action: audit.LoginLockout, Details: map[string]any{"seconds": seconds}}
	logged := []any{"address", ip}
	if s.db != nil && name != "" {
		s.db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE username = ?`, name).Scan(&e.UserID)
		if e.UserID != 0 {
			e.Username = name
			logged = append(logged, "user", e.UserID)
		}
	}
	s.log.Warn("sign-in address locked out", logged...)
	if s.db == nil {
		return
	}
	if err := audit.Record(r.Context(), s.db, e); err != nil {
		s.log.Error("audit log write failed", "action", e.Action, "err", err)
	}
}

type auditEntryOut struct {
	ID       int64          `json:"id"`
	Created  time.Time      `json:"created"`
	User     *userRef       `json:"user"`     // the account, if it still exists
	Username *string        `json:"username"` // as recorded
	IP       *string        `json:"ip"`
	Action   string         `json:"action"`
	Target   *auditTarget   `json:"target"`
	Details  map[string]any `json:"details"`
}

type auditTarget struct {
	Type  string  `json:"type"`
	ID    *string `json:"id"`
	Label *string `json:"label"`
}

const auditPageSize = 50

// listAudit returns the log, newest first, 50 entries a page. Filters: user
// (a user name), action (exact, or a prefix such as "user" for user.*),
// from and to (dates, inclusive, in the server's time zone, or instants),
// q (text in the target, user name, address or details).
func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ctx := r.Context()
	params := r.URL.Query()
	page := min(10000, max(1, atoi(params.Get("page"), 1)))
	where := []string{"TRUE"}
	var args []any
	if v := strings.ToLower(strings.TrimSpace(params.Get("user"))); v != "" {
		where = append(where, "(a.username = ? OR u.username = ?)")
		args = append(args, v, v)
	}
	if v := strings.TrimSpace(params.Get("action")); v != "" {
		where = append(where, "(a.action = ? OR a.action LIKE ?)")
		args = append(args, v, escapeLike(v)+".%")
	}
	for _, bound := range []struct {
		param, op string
		end       bool
	}{{"from", ">=", false}, {"to", "<", true}} {
		v := strings.TrimSpace(params.Get(bound.param))
		if v == "" {
			continue
		}
		t, err := s.parseInstant(v, bound.end)
		if err != nil {
			return err
		}
		where = append(where, "a.created_at "+bound.op+" ?")
		args = append(args, t)
	}
	if v := strings.TrimSpace(params.Get("q")); v != "" {
		like := "%" + escapeLike(strings.ToLower(v)) + "%"
		where = append(where, `(LOWER(a.target_label) LIKE ? OR LOWER(a.target_id) LIKE ? OR LOWER(a.username) LIKE ?
			OR a.ip LIKE ? OR LOWER(a.details) LIKE ?)`)
		args = append(args, like, like, like, like, like)
	}
	from := ` FROM audit_log a LEFT JOIN users u ON u.id = a.user_id WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return err
	}
	entries := []auditEntryOut{}
	err := sqlq.Each(ctx, s.db, `SELECT a.id, a.created_at, u.id, u.username, u.display_name, u.active, a.username, a.ip,
		a.action, a.target_type, a.target_id, a.target_label, a.details`+from+`
		ORDER BY a.created_at DESC, a.id DESC LIMIT ? OFFSET ?`, func(rows *sql.Rows) error {
		var e auditEntryOut
		var uid sql.NullInt64
		var username, name sql.NullString
		var active sql.NullBool
		var recorded, ip, ttype, tid, tlabel, details sql.NullString
		if err := rows.Scan(&e.ID, &e.Created, &uid, &username, &name, &active, &recorded, &ip, &e.Action, &ttype,
			&tid, &tlabel, &details); err != nil {
			return err
		}
		if uid.Valid {
			e.User = &userRef{ID: uid.Int64, Username: username.String, Name: name.String, Active: active.Bool}
		}
		e.Username, e.IP = sqlq.String(recorded), sqlq.String(ip)
		if ttype.Valid {
			e.Target = &auditTarget{Type: ttype.String, ID: sqlq.String(tid), Label: sqlq.String(tlabel)}
		}
		if details.Valid {
			json.Unmarshal([]byte(details.String), &e.Details)
		}
		entries = append(entries, e)
		return nil
	}, append(args, auditPageSize, (page-1)*auditPageSize)...)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"total": total, "page": page, "size": auditPageSize,
		"entries": entries})
}

// parseInstant reads a date (its start, or with end the start of the next
// day, in the server's time zone) or an RFC 3339 instant.
func (s *Server) parseInstant(v string, end bool) (time.Time, error) {
	if d, err := time.ParseInLocation(time.DateOnly, v, s.zone()); err == nil {
		if end {
			d = d.AddDate(0, 0, 1)
		}
		return d.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return t, failf(http.StatusBadRequest, i18n.CommonDateInvalid, v)
	}
	return t.UTC(), nil
}

func (s *Server) auditSettings(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	days, err := audit.Retention(r.Context(), s.db)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]int{"retentionDays": days})
}

// putAuditSettings changes how long entries are kept: 0 keeps them forever,
// otherwise at least audit.MinRetentionDays. Shortening the period purges at
// once.
func (s *Server) putAuditSettings(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		RetentionDays *int `json:"retentionDays"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	if in.RetentionDays == nil || *in.RetentionDays < 0 || *in.RetentionDays > 36500 {
		return fail(http.StatusBadRequest, i18n.AuditRetentionInvalid)
	}
	if days := *in.RetentionDays; days != 0 && days < audit.MinRetentionDays {
		return failf(http.StatusBadRequest, i18n.AuditRetentionTooShort, audit.MinRetentionDays)
	}
	ctx := r.Context()
	before, err := audit.Retention(ctx, s.db)
	if err != nil {
		return err
	}
	if err := audit.SetRetention(ctx, s.db, *in.RetentionDays); err != nil {
		return err
	}
	if _, err := audit.Purge(ctx, s.db); err != nil {
		return err
	}
	s.auditLog(r, audit.AuditSettings, audit.Target{Type: "settings", ID: audit.RetentionSetting},
		map[string]any{"retentionDays": map[string]int{"from": before, "to": *in.RetentionDays}})
	return writeJSON(w, http.StatusOK, map[string]int{"retentionDays": *in.RetentionDays})
}

// userSnapshot is what the audit log compares before and after a change to
// an account.
type userSnapshot struct {
	Username, Name, Email string
	Active, Admin         bool
}

func (s *Server) userSnapshot(ctx context.Context, id int64) (userSnapshot, error) {
	var u userSnapshot
	err := s.db.QueryRowContext(ctx, `SELECT username, display_name, email, active, is_admin FROM users WHERE id = ?`, id).
		Scan(&u.Username, &u.Name, &u.Email, &u.Active, &u.Admin)
	return u, err
}

// auditUserCreated records an account an administrator created.
func (s *Server) auditUserCreated(r *http.Request, id int64) {
	u, err := s.userSnapshot(r.Context(), id)
	if err != nil {
		s.log.Error("audit log: account not found", "id", id, "err", err)
		return
	}
	target := audit.Target{Type: "user", ID: strconv.FormatInt(id, 10), Label: u.Username}
	s.auditLog(r, audit.UserCreate, target, map[string]any{"name": u.Name, "email": u.Email, "admin": u.Admin})
	if u.Admin {
		s.auditLog(r, audit.AdminGrant, target, nil)
	}
}

// auditUserChange records what an administrator changed in an account:
// name and e-mail, activation, administrator rights, password (never its
// value).
func (s *Server) auditUserChange(r *http.Request, id int64, before userSnapshot, password bool) {
	after, err := s.userSnapshot(r.Context(), id)
	if err != nil {
		s.log.Error("audit log: account not found", "id", id, "err", err)
		return
	}
	target := audit.Target{Type: "user", ID: strconv.FormatInt(id, 10), Label: after.Username}
	changes := map[string]any{}
	if before.Name != after.Name {
		changes["name"] = map[string]any{"from": before.Name, "to": after.Name}
	}
	if before.Email != after.Email {
		changes["email"] = map[string]any{"from": before.Email, "to": after.Email}
	}
	if len(changes) > 0 {
		s.auditLog(r, audit.UserUpdate, target, changes)
	}
	switch {
	case before.Active && !after.Active:
		s.auditLog(r, audit.UserDeactivate, target, nil)
	case !before.Active && after.Active:
		s.auditLog(r, audit.UserActivate, target, nil)
	}
	switch {
	case !before.Admin && after.Admin:
		s.auditLog(r, audit.AdminGrant, target, nil)
	case before.Admin && !after.Admin:
		s.auditLog(r, audit.AdminRevoke, target, nil)
	}
	if password {
		s.auditLog(r, audit.PasswordChange, target, nil)
	}
}

// projectGrants lists a project's grants as readable lines
// ("edit: role Developers"), for comparing them before and after a change.
func (s *Server) projectGrants(ctx context.Context, projectID int64) (map[string]bool, error) {
	out := map[string]bool{}
	err := sqlq.Each(ctx, s.db, `SELECT g.permission, g.holder, COALESCE(u.username, ug.name, pr.name, '')
		FROM project_permissions g
		LEFT JOIN users u ON u.id = g.user_id
		LEFT JOIN user_groups ug ON ug.id = g.group_id
		LEFT JOIN project_roles pr ON pr.id = g.role_id
		WHERE g.project_id = ?`, func(rows *sql.Rows) error {
		var p, holder, name string
		if err := rows.Scan(&p, &holder, &name); err != nil {
			return err
		}
		out[strings.TrimSpace(p+": "+holder+" "+name)] = true
		return nil
	}, projectID)
	return out, err
}

// auditGrants records the grants a change to a project's permissions added
// and removed.
func (s *Server) auditGrants(r *http.Request, projectID int64, before map[string]bool) {
	after, err := s.projectGrants(r.Context(), projectID)
	if err != nil {
		s.log.Error("audit log: grants not readable", "project", projectID, "err", err)
		return
	}
	added, removed := []string{}, []string{}
	for g := range after {
		if !before[g] {
			added = append(added, g)
		}
	}
	for g := range before {
		if !after[g] {
			removed = append(removed, g)
		}
	}
	if len(added)+len(removed) == 0 {
		return
	}
	slices.Sort(added)
	slices.Sort(removed)
	var key string
	s.db.QueryRowContext(r.Context(), `SELECT pkey FROM projects WHERE id = ?`, projectID).Scan(&key)
	s.auditLog(r, audit.ProjectPerms, audit.Target{Type: "project", ID: strconv.FormatInt(projectID, 10), Label: key},
		map[string]any{"added": added, "removed": removed})
}

// auditWorkflow records a change to a project's workflow.
func (s *Server) auditWorkflow(r *http.Request, action string, id, projectID int64, name string, details map[string]any) {
	var key string
	s.db.QueryRowContext(r.Context(), `SELECT pkey FROM projects WHERE id = ?`, projectID).Scan(&key)
	if details == nil {
		details = map[string]any{}
	}
	details["project"] = key
	s.auditLog(r, action, audit.Target{Type: "workflow", ID: strconv.FormatInt(id, 10), Label: name}, details)
}

// workflowName returns a workflow's name, for the audit log.
func (s *Server) workflowName(ctx context.Context, id int64) string {
	var name string
	s.db.QueryRowContext(ctx, `SELECT name FROM workflows WHERE id = ?`, id).Scan(&name)
	return name
}

// auditBoard records a change to a board.
func (s *Server) auditBoard(r *http.Request, action string, id int64, name string, details map[string]any) {
	s.auditLog(r, action, audit.Target{Type: "board", ID: strconv.FormatInt(id, 10), Label: name}, details)
}

// auditSprint records a change to a sprint.
func (s *Server) auditSprint(r *http.Request, action string, id int64, name string, details map[string]any) {
	s.auditLog(r, action, audit.Target{Type: "sprint", ID: strconv.FormatInt(id, 10), Label: name}, details)
}

// boardChanges names what a change to a board's settings touched.
func boardChanges(in boardInput) []string {
	var out []string
	for name, set := range map[string]bool{"name": in.Name != nil, "subQuery": in.SubQuery != nil,
		"doneCutoff": in.DoneCutoff != nil, "swimlanes": in.Swimlanes != nil, "estimate": in.Estimate != nil,
		"columns": in.Columns != nil, "quickFilters": in.QuickFilters != nil, "lanes": in.Lanes != nil} {
		if set {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// shareNames describes filter shares for the audit log: "everyone",
// "group developpeurs", "project DEMO", "project DEMO, role Developers",
// "user paul.martin".
func (s *Server) shareNames(ctx context.Context, shares []filterShare) []string {
	out := []string{}
	name := func(q string, id int64) string {
		var n string
		if err := s.db.QueryRowContext(ctx, q, id).Scan(&n); err != nil {
			return strconv.FormatInt(id, 10)
		}
		return n
	}
	for _, sh := range shares {
		switch sh.Kind {
		case "everyone":
			out = append(out, "everyone")
		case "group":
			out = append(out, "group "+name(`SELECT name FROM user_groups WHERE id = ?`, sh.ID))
		case "project":
			v := "project " + name(`SELECT pkey FROM projects WHERE id = ?`, sh.ID)
			if sh.Role != 0 {
				v += ", role " + name(`SELECT name FROM project_roles WHERE id = ?`, sh.Role)
			}
			out = append(out, v)
		case "user":
			out = append(out, "user "+name(`SELECT username FROM users WHERE id = ?`, sh.ID))
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// auditFilterShares records a change to whom a saved filter is shared with.
func (s *Server) auditFilterShares(r *http.Request, id int64, filter string, before, after []filterShare) {
	from, to := s.shareNames(r.Context(), before), s.shareNames(r.Context(), after)
	if slices.Equal(from, to) {
		return
	}
	s.auditLog(r, audit.FilterShare, audit.Target{Type: "filter", ID: strconv.FormatInt(id, 10), Label: filter},
		map[string]any{"from": from, "to": to})
}
