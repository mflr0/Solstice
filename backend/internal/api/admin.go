package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	netmail "net/mail"
	"regexp"

	"github.com/go-sql-driver/mysql"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/mail"
	"solstice/internal/sqlq"
	"strings"
	"time"
)

type adminUser struct {
	ID          int64      `json:"id"`
	Username    string     `json:"username"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	Active      bool       `json:"active"`
	Admin       bool       `json:"admin"`
	HasPassword bool       `json:"hasPassword"`
	Directory   string     `json:"directory"` // "ldap", "" for a local account
	Created     time.Time  `json:"created"`
	LastLogin   *time.Time `json:"lastLogin"`
	Groups      []idName   `json:"groups"`
	Tokens      *int       `json:"tokens,omitempty"` // detail only
}

func requireAdmin(r *http.Request) error {
	if !currentUser(r).Admin {
		return errForbidden
	}
	return nil
}

// validEmail accepts a bare address: marie@example.org, not "Marie" <…>.
func validEmail(s string) bool {
	a, err := netmail.ParseAddress(s)
	return err == nil && a.Address == s && a.Name == ""
}

const adminUserColumns = `id, username, display_name, email, active, is_admin, password_hash IS NOT NULL, directory,
	created_at, last_login_at`

func scanAdminUser(sc interface{ Scan(...any) error }) (adminUser, error) {
	var u adminUser
	var last sql.NullTime
	err := sc.Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.Active, &u.Admin, &u.HasPassword, &u.Directory, &u.Created, &last)
	u.LastLogin = sqlq.Time(last)
	u.Groups = []idName{}
	return u, err
}

func (s *Server) withGroups(ctx context.Context, users []adminUser) error {
	if len(users) == 0 {
		return nil
	}
	index := map[int64]int{}
	args := make([]any, 0, len(users))
	for i, u := range users {
		index[u.ID] = i
		args = append(args, u.ID)
	}
	return sqlq.Each(ctx, s.db, `SELECT m.user_id, g.id, g.name FROM group_members m JOIN user_groups g ON g.id = m.group_id
		WHERE m.user_id IN (`+sqlq.Marks(len(args))+`) ORDER BY g.name`, func(rows *sql.Rows) error {
		var uid int64
		var g idName
		if err := rows.Scan(&uid, &g.ID, &g.Name); err != nil {
			return err
		}
		users[index[uid]].Groups = append(users[index[uid]].Groups, g)
		return nil
	}, args...)
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	match, args := sqlq.LikeWords(q, "username", "LOWER(display_name)", "LOWER(email)")
	out := []adminUser{}
	err := sqlq.Each(r.Context(), s.db, `SELECT `+adminUserColumns+` FROM users
		WHERE `+match+`
		ORDER BY active DESC, display_name LIMIT 200`, func(rows *sql.Rows) error {
		u, err := scanAdminUser(rows)
		if err != nil {
			return err
		}
		out = append(out, u)
		return nil
	}, args...)
	if err != nil {
		return err
	}
	if err := s.withGroups(r.Context(), out); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// adminUserByID loads one account with its groups and token count.
func (s *Server) adminUserByID(ctx context.Context, id int64) (*adminUser, error) {
	u, err := scanAdminUser(s.db.QueryRowContext(ctx, `SELECT `+adminUserColumns+` FROM users WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	list := []adminUser{u}
	if err := s.withGroups(ctx, list); err != nil {
		return nil, err
	}
	var tokens int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_tokens WHERE user_id = ?`, id).Scan(&tokens); err != nil {
		return nil, err
	}
	list[0].Tokens = &tokens
	return &list[0], nil
}

func (s *Server) adminGetUser(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	u, err := s.adminUserByID(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, u)
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._@-]{0,99}$`)

// The account rules, for the command line, which creates accounts without the API.

// ValidUsername tells whether a lowercased login may name a new account.
func ValidUsername(login string) bool { return usernamePattern.MatchString(login) }

// ValidEmail tells whether s is a bare address an account may have.
func ValidEmail(s string) bool { return validEmail(s) }

// PasswordRules applies the password policy, minLen being the instance's minimum.
func PasswordRules(password, username string, minLen int) error {
	return passwordRules(password, username, minLen)
}

// StoredPasswordMin is the instance's minimum password length, the default
// when the setting is absent or unreadable.
func StoredPasswordMin(ctx context.Context, db *sql.DB) int { return storedPasswordMin(ctx, db) }

// IsDuplicate tells a unique key violation.
func IsDuplicate(err error) bool { return isDuplicate(err) }

func (s *Server) adminCreateUser(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		Username string  `json:"username"`
		Name     string  `json:"name"`
		Email    string  `json:"email"`
		Password string  `json:"password"`
		Admin    bool    `json:"admin"`
		Groups   []int64 `json:"groups"`
		// Invite creates the account without a password and mails its
		// person a link to choose one.
		Invite bool `json:"invite"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	if !usernamePattern.MatchString(in.Username) {
		return fail(http.StatusBadRequest, i18n.AccountUsernameInvalid)
	}
	if strings.TrimSpace(in.Name) == "" {
		in.Name = in.Username
	}
	if email := strings.TrimSpace(in.Email); email != "" && !validEmail(email) {
		return fail(http.StatusBadRequest, i18n.AccountEmailInvalid)
	}
	var cfg mail.Config
	var hash any // NULL for an invited account: its person chooses
	if in.Invite {
		if in.Password != "" {
			return fail(http.StatusBadRequest, i18n.AccountInviteNoPassword)
		}
		if !mail.ValidAddress(strings.TrimSpace(in.Email)) {
			return fail(http.StatusBadRequest, i18n.AccountInviteEmailRequired)
		}
		var ok bool
		if cfg, ok = s.linkConfig(ctx); !ok {
			return fail(http.StatusConflict, i18n.AccountMailUnavailable)
		}
	} else {
		if err := s.checkNewPassword(ctx, in.Password, in.Username); err != nil {
			return err
		}
		h, err := auth.HashPassword(in.Password)
		if err != nil {
			return err
		}
		hash = h
	}
	if err := s.checkGroups(ctx, in.Groups); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO users (username, display_name, email, password_hash, is_admin)
		VALUES (?, ?, ?, ?, ?)`, in.Username, strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), hash, in.Admin)
	if err != nil {
		if isDuplicate(err) {
			return fail(http.StatusConflict, i18n.AccountUsernameTaken)
		}
		return err
	}
	id, _ := res.LastInsertId()
	for _, g := range in.Groups {
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO group_members (group_id, user_id) VALUES (?, ?)`, g, id); err != nil {
			return err
		}
	}
	if in.Invite {
		to := &linkTarget{ID: id, Username: in.Username, Name: strings.TrimSpace(in.Name), Email: strings.TrimSpace(in.Email)}
		if err := s.queueAccountLink(ctx, tx, cfg, to, auth.PurposeInvite); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auditUserCreated(r, id)
	if in.Invite {
		s.out.outbox.Wake()
		s.auditLog(r, audit.UserInvite, target(audit.TargetUser, id, in.Username), nil)
	}
	if len(in.Groups) > 0 {
		s.auditLog(r, audit.GroupMembers, target(audit.TargetUser, id, in.Username),
			map[string]any{"groups": s.groupNames(ctx, in.Groups)})
	}
	return writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) adminUpdateUser(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Name     *string `json:"name"`
		Email    *string `json:"email"`
		Active   *bool   `json:"active"`
		Admin    *bool   `json:"admin"`
		Password *string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	me := currentUser(r)
	self := id == me.ID
	if self && ((in.Active != nil && !*in.Active) || (in.Admin != nil && !*in.Admin)) {
		return fail(http.StatusBadRequest, i18n.AccountOwnAdminRights)
	}
	ctx := r.Context()
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = ?`, id).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return errNotFound
	}
	before, err := s.userSnapshot(ctx, id)
	if err != nil {
		return err
	}
	emailChanged := in.Email != nil && strings.TrimSpace(*in.Email) != before.Email
	// The profile asks for the current password before changing either, and
	// refuses a token: this path must not get around that.
	switch {
	case self && in.Password != nil:
		return fail(http.StatusBadRequest, i18n.AccountOwnPasswordInProfile)
	case self && emailChanged:
		return fail(http.StatusBadRequest, i18n.AccountOwnEmailInProfile)
	case me.ByToken() && in.Password != nil:
		return fail(http.StatusForbidden, i18n.AccountTokenCannotChangePassword)
	case me.ByToken() && emailChanged:
		return fail(http.StatusForbidden, i18n.AccountTokenCannotChangeEmail)
	}
	if in.Email != nil {
		if email := strings.TrimSpace(*in.Email); email != "" && !validEmail(email) {
			return fail(http.StatusBadRequest, i18n.AccountEmailInvalid)
		}
	}
	if in.Password != nil {
		if err := s.checkNewPassword(ctx, *in.Password, before.Username); err != nil {
			return err
		}
	}
	var hash string
	if in.Password != nil {
		if hash, err = auth.HashPassword(*in.Password); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET display_name = ? WHERE id = ?`, strings.TrimSpace(*in.Name), id); err != nil {
			return err
		}
	}
	if in.Email != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET email = ? WHERE id = ?`, strings.TrimSpace(*in.Email), id); err != nil {
			return err
		}
		// Links mailed to the old address must not work once it is not theirs.
		if emailChanged {
			if err := auth.RevokeAccountTokens(ctx, tx, id, time.Now()); err != nil {
				return err
			}
		}
	}
	var revoked int64
	if in.Active != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET active = ? WHERE id = ?`, *in.Active, id); err != nil {
			return err
		}
		// A reactivated account starts clean too: an import deactivates
		// accounts without ending anything.
		if *in.Active != before.Active {
			if revoked, err = auth.EndCredentials(ctx, tx, id, ""); err != nil {
				return err
			}
		}
	}
	if in.Admin != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET is_admin = ? WHERE id = ?`, *in.Admin, id); err != nil {
			return err
		}
	}
	if in.Password != nil {
		n, err := auth.SetPasswordHash(ctx, tx, id, hash, "")
		if err != nil {
			return err
		}
		revoked += n
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	s.auditUserChange(r, id, before, in.Password != nil, revoked)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// adminResetPassword gives an account a generated password, shown once to
// the administrator, and signs it out everywhere. With {"method": "link"} it
// mails the account a link to choose its own password instead.
func (s *Server) adminResetPassword(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Method string `json:"method"`
	}
	if err := readOptionalJSON(r, &in); err != nil {
		return err
	}
	if in.Method != "" && in.Method != "generate" && in.Method != "link" {
		return failf(http.StatusBadRequest, i18n.FieldValueInvalid, "method")
	}
	ctx := r.Context()
	u, err := s.adminUserByID(ctx, id)
	if err != nil {
		return err
	}
	if id == currentUser(r).ID {
		return fail(http.StatusBadRequest, i18n.AccountOwnPasswordInProfile)
	}
	if in.Method == "link" {
		return s.adminResetLink(w, r, id)
	}
	n := max(s.passwordMin(ctx), 16)
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	password := base64.RawURLEncoding.EncodeToString(raw)[:n]
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	revoked, err := auth.SetPasswordHash(ctx, tx, id, hash, "")
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auditLog(r, audit.PasswordReset, target(audit.TargetUser, id, u.Username),
		map[string]any{"revokedAccessTokens": revoked})
	return writeJSON(w, http.StatusOK, map[string]string{"password": password})
}

// adminUserGroups replaces the groups of an account.
func (s *Server) adminUserGroups(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Groups []int64 `json:"groups"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	u, err := s.adminUserByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.checkGroups(ctx, in.Groups); err != nil {
		return err
	}
	before := make([]int64, 0, len(u.Groups))
	for _, g := range u.Groups {
		before = append(before, g.ID)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE user_id = ?`, id); err != nil {
		return err
	}
	for _, g := range in.Groups {
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO group_members (group_id, user_id) VALUES (?, ?)`, g, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	from, to := s.groupNames(ctx, before), s.groupNames(ctx, in.Groups)
	if changed(from, to) {
		s.auditLog(r, audit.GroupMembers, target(audit.TargetUser, id, u.Username),
			map[string]any{"from": from, "to": to})
	}
	u, err = s.adminUserByID(ctx, id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, u)
}

// isDuplicate tells a unique key violation (MySQL error 1062).
func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// nameTaken refuses with 409 and key when a row of table other than except
// has this name in any case. except is the id of the row being renamed, of
// the type of the table's id.
func (s *Server) nameTaken(ctx context.Context, table, name string, except any, key i18n.Key) error {
	return s.valueTaken(ctx, table, "name", "TRUE", nil, name, except, key)
}

// valueTaken is nameTaken on any column, among the rows of table that match
// scope, a condition on scopeArgs.
func (s *Server) valueTaken(ctx context.Context, table, column, scope string, scopeArgs []any, value string, except any,
	key i18n.Key) error {
	var n int
	args := append(append([]any{}, scopeArgs...), value, except)
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE `+scope+` AND LOWER(`+column+`) = LOWER(?)
		AND id <> ?`, args...).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return failf(http.StatusConflict, key, value)
	}
	return nil
}
