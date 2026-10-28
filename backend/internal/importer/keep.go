package importer

// What an import keeps of Solstice's own data. An import replaces what
// comes from Jira (users, groups, projects, issues, catalogues…), but not
// Solstice's configuration:
//
//   - settings, webhooks and the audit log stay as they are (the audit log's
//     and webhooks' links to accounts are restored by user name);
//   - personal access tokens, notification preferences, open sessions,
//     interface language, last sign-in and administrator rights are kept for
//     the accounts that still exist afterwards, matched by user name;
//   - accounts created in Solstice (no Jira key) whose user name Jira does
//     not have are kept, without groups or roles;
//   - the operator (Options.Operator), who starts the import from the
//     administration, keeps their account, password, rights and session.
//
// Everything else that hangs off the replaced data goes: webhook
// deliveries, e-mails waiting to be sent, notification batches, filter
// subscriptions, project notification settings, custom field contexts.

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"solstice/internal/i18n"
	"solstice/internal/sqlq"
)

// keptTables are never emptied by an import.
var keptTables = []string{"schema_migrations", "settings", "webhooks", "audit_log"}

// userRows are tables whose rows belong to a user: saved before the import
// and put back for the users that still exist, by user name.
var userRows = []struct{ table, column string }{
	{"api_tokens", "user_id"},
	{"notification_prefs", "user_id"},
	{"sessions", "user_id"},
}

// savedRows are rows of a table, all columns, with the user name of their
// user column.
type savedRows struct {
	table, column string
	columns       []string
	rows          [][]any
	users         []string
}

// kept is what prepareTarget saved, for restore.
type kept struct {
	local    *savedRows        // accounts created in Solstice
	operator *savedRows        // the operator's account, as it was
	admins   []string          // administrators' user names
	extras   map[string][2]any // user name → locale, last sign-in
	rows     []*savedRows
	hooks    map[int64]string // webhook → its creator's user name
}

// save reads rows of a table joined with the user name of userCol.
func (r *run) save(table, userCol, where string, args ...any) (*savedRows, error) {
	q := "SELECT u.username, t.* FROM `" + table + "` t JOIN users u ON u.id = t." + userCol
	if where != "" {
		q += " WHERE " + where
	}
	rows, err := r.conn.QueryContext(r.ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := &savedRows{table: table, column: userCol, columns: cols[1:]}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		name, _ := vals[0].([]byte)
		out.users = append(out.users, string(name))
		out.rows = append(out.rows, vals[1:])
	}
	return out, rows.Err()
}

// restore inserts saved rows back, their user column set to the id that
// bears their user name now; rows of users gone are dropped. Columns in
// skip are left to their defaults (the id of a re-created account).
func (r *run) restore(s *savedRows, skip ...string) (int, error) {
	var cols []string
	var idx []int
	for i, c := range s.columns {
		if !slices.Contains(skip, c) {
			cols = append(cols, "`"+c+"`")
			idx = append(idx, i)
		}
	}
	q := "INSERT IGNORE INTO `" + s.table + "` (" + strings.Join(cols, ", ") + ") VALUES (" + sqlq.Marks(len(cols)) + ")"
	n := 0
	for i, row := range s.rows {
		var uid int64
		if s.column != "id" {
			err := r.conn.QueryRowContext(r.ctx, `SELECT id FROM users WHERE username = ?`, s.users[i]).Scan(&uid)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return n, err
			}
		}
		args := make([]any, 0, len(idx))
		for _, j := range idx {
			v := row[j]
			if s.columns[j] == s.column && s.column != "id" {
				v = uid
			}
			args = append(args, v)
		}
		if _, err := r.dst.ExecContext(r.ctx, q, args...); err != nil {
			return n, fmt.Errorf("restoring %s: %w", s.table, err)
		}
		n++
	}
	return n, nil
}

// keep saves what an import must not lose, and unlinks the audit log and
// webhooks from accounts that are about to be replaced. It runs before the
// tables are emptied.
func (r *run) keep() (*kept, error) {
	k := &kept{extras: map[string][2]any{}, hooks: map[int64]string{}}
	var err error
	if k.local, err = r.save("users", "id", "t.user_key IS NULL"); err != nil {
		return nil, err
	}
	if r.opt.Operator != "" {
		if k.operator, err = r.save("users", "id", "t.username = ?", strings.ToLower(r.opt.Operator)); err != nil {
			return nil, err
		}
	}
	rows, err := r.conn.QueryContext(r.ctx, `SELECT username, is_admin, locale, last_login_at FROM users`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var name string
		var admin bool
		var locale sql.NullString
		var last sql.NullTime
		if err := rows.Scan(&name, &admin, &locale, &last); err != nil {
			rows.Close()
			return nil, err
		}
		if admin {
			k.admins = append(k.admins, name)
		}
		k.extras[name] = [2]any{sqlq.Value(locale), ntime(last)}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, t := range userRows {
		s, err := r.save(t.table, t.column, "")
		if err != nil {
			return nil, err
		}
		k.rows = append(k.rows, s)
	}
	rows, err = r.conn.QueryContext(r.ctx, `SELECT w.id, u.username FROM webhooks w JOIN users u ON u.id = w.created_by`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		k.hooks[id] = name
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Account ids are about to change: the audit log keeps the user name
	// of each entry (in a temporary table, to link entries back at the end)
	// and points at no account meanwhile, as do webhooks.
	for _, q := range []string{
		`DROP TEMPORARY TABLE IF EXISTS import_audit_users`,
		`CREATE TEMPORARY TABLE import_audit_users (id BIGINT NOT NULL PRIMARY KEY, username VARCHAR(255) COLLATE utf8mb4_bin NOT NULL)`,
		`INSERT INTO import_audit_users SELECT a.id, u.username FROM audit_log a JOIN users u ON u.id = a.user_id`,
		`UPDATE audit_log SET user_id = NULL WHERE user_id IS NOT NULL`,
		`UPDATE webhooks SET created_by = NULL`,
	} {
		if _, err := r.dst.ExecContext(r.ctx, q); err != nil {
			return nil, err
		}
	}
	return k, nil
}

// putBack restores what keep saved, once the import has run (or failed:
// the operator must still be able to sign in and try again).
func (r *run) putBack(k *kept) error {
	var errs []error
	// Accounts created in Solstice whose name Jira does not use. Project ids
	// change with the import: a chosen project scope would name another one.
	if n, err := r.restore(k.local, "id", "scope_project_id"); err != nil {
		errs = append(errs, err)
	} else if n > 0 {
		r.rep.warn(i18n.N(n, i18n.ImportWarnLocalAccountsOne, i18n.ImportWarnLocalAccountsMany))
	}
	// The operator keeps their access whatever Jira says.
	if k.operator != nil && len(k.operator.rows) == 1 {
		// Re-created as a Solstice account when Jira has no such user name
		// (its old Jira key may belong to someone else now).
		if _, err := r.restore(k.operator, "id", "user_key", "scope_project_id"); err != nil {
			errs = append(errs, err)
		}
		hash := k.operator.rows[0][slices.Index(k.operator.columns, "password_hash")]
		if _, err := r.dst.ExecContext(r.ctx, `UPDATE users SET password_hash = ?, active = TRUE, is_admin = TRUE
			WHERE username = ?`, hash, k.operator.users[0]); err != nil {
			errs = append(errs, err)
		}
	}
	for chunk := range slices.Chunk(k.admins, 500) {
		args := make([]any, len(chunk))
		for i, a := range chunk {
			args[i] = a
		}
		if _, err := r.dst.ExecContext(r.ctx, `UPDATE users SET is_admin = TRUE WHERE username IN (`+
			sqlq.Marks(len(args))+`)`, args...); err != nil {
			errs = append(errs, err)
		}
	}
	for name, v := range k.extras {
		if v[0] == nil && v[1] == nil {
			continue
		}
		if _, err := r.dst.ExecContext(r.ctx, `UPDATE users SET locale = COALESCE(?, locale),
			last_login_at = COALESCE(?, last_login_at) WHERE username = ?`, v[0], v[1], name); err != nil {
			errs = append(errs, err)
		}
	}
	for _, s := range k.rows {
		if _, err := r.restore(s); err != nil {
			errs = append(errs, err)
		}
	}
	for id, name := range k.hooks {
		if _, err := r.dst.ExecContext(r.ctx, `UPDATE webhooks SET created_by = (SELECT id FROM users WHERE username = ?)
			WHERE id = ?`, name, id); err != nil {
			errs = append(errs, err)
		}
	}
	for _, q := range []string{
		`UPDATE audit_log a JOIN import_audit_users t ON t.id = a.id JOIN users u ON u.username = t.username
			SET a.user_id = u.id`,
		`DROP TEMPORARY TABLE IF EXISTS import_audit_users`,
	} {
		if _, err := r.dst.ExecContext(r.ctx, q); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
