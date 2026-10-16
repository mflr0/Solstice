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

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/sqlq"
	"strconv"
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

// endSessions signs a user out everywhere, except the session whose token
// hash is keep ("" for none).
func (s *Server) endSessions(ctx context.Context, userID int64, keep string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash <> ?`, userID, keep)
	return err
}

const adminUserColumns = `id, username, display_name, email, active, is_admin, password_hash IS NOT NULL, created_at,
	last_login_at`

func scanAdminUser(sc interface{ Scan(...any) error }) (adminUser, error) {
	var u adminUser
	var last sql.NullTime
	err := sc.Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.Active, &u.Admin, &u.HasPassword, &u.Created, &last)
	if last.Valid {
		u.LastLogin = &last.Time
	}
	u.Groups = []idName{}
	return u, err
}

// withGroups fills the groups of the users listed.
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
	like := "%" + escapeLike(q) + "%"
	out := []adminUser{}
	err := sqlq.Each(r.Context(), s.db, `SELECT `+adminUserColumns+` FROM users
		WHERE username LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(email) LIKE ?
		ORDER BY active DESC, display_name LIMIT 200`, func(rows *sql.Rows) error {
		u, err := scanAdminUser(rows)
		if err != nil {
			return err
		}
		out = append(out, u)
		return nil
	}, like, like, like)
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
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return errNotFound
	}
	u, err := s.adminUserByID(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, u)
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._@-]{0,99}$`)

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
	if err := s.checkNewPassword(ctx, in.Password); err != nil {
		return err
	}
	if err := s.checkGroups(ctx, in.Groups); err != nil {
		return err
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
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
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auditUserCreated(r, id)
	if len(in.Groups) > 0 {
		s.auditLog(r, audit.GroupMembers, audit.Target{Type: "user", ID: strconv.FormatInt(id, 10), Label: in.Username},
			map[string]any{"groups": s.groupNames(ctx, in.Groups)})
	}
	return writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) adminUpdateUser(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return errNotFound
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
	if id == currentUser(r).ID && ((in.Active != nil && !*in.Active) || (in.Admin != nil && !*in.Admin)) {
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
	if in.Email != nil {
		if email := strings.TrimSpace(*in.Email); email != "" && !validEmail(email) {
			return fail(http.StatusBadRequest, i18n.AccountEmailInvalid)
		}
	}
	if in.Password != nil {
		if err := s.checkNewPassword(ctx, *in.Password); err != nil {
			return err
		}
	}
	before, err := s.userSnapshot(ctx, id)
	if err != nil {
		return err
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET display_name = ? WHERE id = ?`, strings.TrimSpace(*in.Name), id); err != nil {
			return err
		}
	}
	if in.Email != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET email = ? WHERE id = ?`, strings.TrimSpace(*in.Email), id); err != nil {
			return err
		}
	}
	if in.Active != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET active = ? WHERE id = ?`, *in.Active, id); err != nil {
			return err
		}
		if !*in.Active {
			if err := s.endSessions(ctx, id, ""); err != nil {
				return err
			}
		}
	}
	if in.Admin != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET is_admin = ? WHERE id = ?`, *in.Admin, id); err != nil {
			return err
		}
	}
	if in.Password != nil {
		if err := auth.SetPassword(ctx, s.db, id, *in.Password); err != nil {
			return err
		}
		// A new password signs out every session opened with the old one.
		if id != currentUser(r).ID {
			if err := s.endSessions(ctx, id, ""); err != nil {
				return err
			}
		}
	}
	s.perms.Invalidate()
	s.auditUserChange(r, id, before, in.Password != nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// adminResetPassword gives an account a generated password, shown once to
// the administrator, and signs it out everywhere.
func (s *Server) adminResetPassword(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return errNotFound
	}
	ctx := r.Context()
	u, err := s.adminUserByID(ctx, id)
	if err != nil {
		return err
	}
	if id == currentUser(r).ID {
		return fail(http.StatusBadRequest, i18n.AccountOwnPasswordInProfile)
	}
	n := max(s.passwordMin(ctx), 16)
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	password := base64.RawURLEncoding.EncodeToString(raw)[:n]
	if err := auth.SetPassword(ctx, s.db, id, password); err != nil {
		return err
	}
	if err := s.endSessions(ctx, id, ""); err != nil {
		return err
	}
	s.auditLog(r, audit.PasswordReset, audit.Target{Type: "user", ID: strconv.FormatInt(id, 10), Label: u.Username}, nil)
	return writeJSON(w, http.StatusOK, map[string]string{"password": password})
}

// adminUserGroups replaces the groups of an account.
func (s *Server) adminUserGroups(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return errNotFound
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
	if strings.Join(from, ",") != strings.Join(to, ",") {
		s.auditLog(r, audit.GroupMembers, audit.Target{Type: "user", ID: strconv.FormatInt(id, 10), Label: u.Username},
			map[string]any{"from": from, "to": to})
	}
	u, err = s.adminUserByID(ctx, id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, u)
}

// isDuplicate tells a unique key violation.
func isDuplicate(err error) bool { return err != nil && strings.Contains(err.Error(), "Duplicate") }
