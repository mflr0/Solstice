package auth

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"solstice/internal/sqlq"
)

// Directory checks passwords against an external directory, such as LDAP.
type Directory interface {
	// Name is what users.directory holds for the directory's accounts.
	Name() string
	// Authenticate checks a canonical user name and a password, and returns
	// what the directory says of the person. ErrNoSession: unknown name,
	// wrong password, or not allowed in.
	Authenticate(ctx context.Context, username, password string) (*DirectoryEntry, error)
}

// DirectoryEntry is a person as a directory describes them.
type DirectoryEntry struct {
	Name   string   // display name, "" to keep the account's
	Email  string   // "" to keep the account's
	Groups []string // names of the groups the person is in
	Admin  *bool    // whether the person is an administrator, nil to leave it to Solstice
}

// ErrDirectoryAccount means the account's password belongs to its directory:
// Solstice neither sets it nor sends links to choose one.
var ErrDirectoryAccount = errors.New("the password of this account is managed by its directory")

// refuseDirectory returns ErrDirectoryAccount for an account of a directory.
func refuseDirectory(ctx context.Context, ex execer, userID int64) error {
	var directory string
	err := ex.QueryRowContext(ctx, `SELECT directory FROM users WHERE id = ?`, userID).Scan(&directory)
	if err == nil && directory != "" {
		return ErrDirectoryAccount
	}
	return err
}

// directorySignIn signs in through dir. id is the account of that name: one
// of a directory, or a local one without a password (an account imported
// from Jira's external directory), which becomes the directory's; 0 when
// there is none, and the account is created. Name, address, administrator
// right and groups are then copied from the directory (saveEntry).
func directorySignIn(ctx context.Context, db *sql.DB, dir Directory, id int64, username, password string) (*User, error) {
	e, err := dir.Authenticate(ctx, username, password)
	if err != nil {
		return nil, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if id, err = saveEntry(ctx, tx, id, dir.Name(), username, e); err != nil {
		return nil, err
	}
	return commitUser(ctx, tx, id)
}

// saveEntry copies what a directory says of a person to the account id, or
// to a new account of that user name when id is 0, and returns the
// account's id. An account takes source as its directory unless it has one
// already: an account may sign in through LDAP and OpenID Connect both.
func saveEntry(ctx context.Context, tx *sql.Tx, id int64, source, username string, e *DirectoryEntry) (int64, error) {
	admin := sql.NullBool{Valid: e.Admin != nil}
	if e.Admin != nil {
		admin.Bool = *e.Admin
	}
	if id == 0 {
		name := strings.TrimSpace(e.Name)
		if name == "" {
			name = username
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO users (username, display_name, email, directory, is_admin)
			VALUES (?, ?, ?, ?, ?)`, username, name, strings.TrimSpace(e.Email), source, admin.Bool)
		if err != nil {
			return 0, err
		}
		if id, err = res.LastInsertId(); err != nil {
			return 0, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET directory = IF(directory = '', ?, directory),
			display_name = COALESCE(NULLIF(?, ''), display_name), email = COALESCE(NULLIF(?, ''), email),
			is_admin = COALESCE(?, is_admin) WHERE id = ?`,
			source, strings.TrimSpace(e.Name), strings.TrimSpace(e.Email), admin, id); err != nil {
			return 0, err
		}
		// An imported account may have been sent an invitation: its link
		// would set a password the account no longer uses.
		if err := RevokeAccountTokens(ctx, tx, id, time.Now()); err != nil {
			return 0, err
		}
	}
	return id, syncGroups(ctx, tx, id, e.Groups)
}

// commitUser reads the account id and commits tx.
func commitUser(ctx context.Context, tx *sql.Tx, id int64) (*User, error) {
	var u User
	err := tx.QueryRowContext(ctx, `SELECT id, username, display_name, email, is_admin, COALESCE(locale, ''), directory
		FROM users WHERE id = ?`, id).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Admin, &u.Locale, &u.Directory)
	if err != nil {
		return nil, err
	}
	return &u, tx.Commit()
}

// ApplyGroups enforces a directory's required group and administrators'
// group on an entry (empty names for none): ErrNoSession when the person is
// not in require; with admin, Admin tells whether they are in it.
func ApplyGroups(e *DirectoryEntry, require, admin string) error {
	in := func(group string) bool {
		for _, g := range e.Groups {
			if strings.EqualFold(strings.TrimSpace(g), strings.TrimSpace(group)) {
				return true
			}
		}
		return false
	}
	if require != "" && !in(require) {
		return ErrNoSession
	}
	if admin != "" {
		yes := in(admin)
		e.Admin = &yes
	}
	return nil
}

// syncGroups makes the account a member of exactly the directory groups
// named, among the groups that come from a directory (from_directory). A
// group missing is created; a local group of the same name becomes the
// directory's. Local groups keep their members.
func syncGroups(ctx context.Context, tx *sql.Tx, userID int64, names []string) error {
	var keep []any
	for _, n := range groupNames(names) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO user_groups (name, from_directory) VALUES (?, TRUE)
			ON DUPLICATE KEY UPDATE from_directory = TRUE`, n); err != nil {
			return err
		}
		var gid int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM user_groups WHERE name = ?`, n).Scan(&gid); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO group_members (group_id, user_id) VALUES (?, ?)`,
			gid, userID); err != nil {
			return err
		}
		keep = append(keep, gid)
	}
	q := `DELETE m FROM group_members m JOIN user_groups g ON g.id = m.group_id
		WHERE m.user_id = ? AND g.from_directory`
	if len(keep) > 0 {
		q += ` AND g.id NOT IN (` + sqlq.Marks(len(keep)) + `)`
	}
	_, err := tx.ExecContext(ctx, q, append([]any{userID}, keep...)...)
	return err
}

// groupNames returns the names a directory gives as Solstice stores them:
// lower case, trimmed, each once, sorted. A name too long for the column is
// left out.
func groupNames(names []string) []string {
	var out []string
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if n != "" && utf8.RuneCountInString(n) <= sqlq.VarcharWidth {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
