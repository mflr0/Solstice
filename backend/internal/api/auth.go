package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/sqlq"
)

type meResponse struct {
	ID       int64   `json:"id"`
	Username string  `json:"username"`
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Admin    bool    `json:"admin"`
	Locale   *string `json:"locale"` // null: none chosen
	// Directory is "ldap" or "oidc" for an account that signs in through a
	// directory or a provider, with no password of its own.
	Directory string `json:"directory"`
	// Project scopes Work, Boards and Search; null covers every project.
	Project *projectBrief `json:"project"`
}

func meOf(u *auth.User) meResponse {
	me := meResponse{ID: u.ID, Username: u.Username, Name: u.DisplayName, Email: u.Email, Admin: u.Admin,
		Directory: u.Directory}
	if u.Locale != "" {
		me.Locale = &u.Locale
	}
	return me
}

// meFor is meOf with the project the user scoped their pages to. A project
// they can no longer browse is forgotten.
func (s *Server) meFor(ctx context.Context, u *auth.User) (meResponse, error) {
	me := meOf(u)
	var p projectBrief
	err := s.db.QueryRowContext(ctx, `SELECT p.id, p.pkey, p.name FROM users u JOIN projects p ON p.id = u.scope_project_id
		WHERE u.id = ?`, u.ID).Scan(&p.ID, &p.Key, &p.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return me, nil
	}
	if err != nil {
		return me, err
	}
	sub, err := s.perms.Subject(ctx, u.ID, u.Admin)
	if err != nil {
		return me, err
	}
	if !sub.Admin && !sub.CanBrowseProject(p.ID) {
		_, err := s.db.ExecContext(ctx, `UPDATE users SET scope_project_id = NULL WHERE id = ? AND scope_project_id = ?`, u.ID, p.ID)
		return me, err
	}
	me.Project = &p
	return me, nil
}

// writeMe answers with the user's own account.
func (s *Server) writeMe(w http.ResponseWriter, r *http.Request, u *auth.User) error {
	me, err := s.meFor(r.Context(), u)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, me)
}

// userByID loads an account for its id, with the fields meOf answers with.
func (s *Server) userByID(ctx context.Context, id int64) (auth.User, error) {
	var u auth.User
	err := s.db.QueryRowContext(ctx, `SELECT id, username, display_name, email, is_admin, COALESCE(locale, ''), directory
		FROM users WHERE id = ?`, id).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Admin, &u.Locale, &u.Directory)
	return u, err
}

// scopeProject is the id of a project the user may scope their pages to:
// one they browse. Another is unknown, so keys cannot be probed.
func (s *Server) scopeProject(r *http.Request, key string) (int64, error) {
	id, err := s.projectByKey(r.Context(), key)
	if errors.Is(err, errNotFound) {
		return 0, failf(http.StatusBadRequest, i18n.ProjectUnknown, key)
	}
	if err != nil {
		return 0, err
	}
	sub, err := s.subject(r)
	if err != nil {
		return 0, err
	}
	if !sub.Admin && !sub.CanBrowseProject(id) {
		return 0, failf(http.StatusBadRequest, i18n.ProjectUnknown, key)
	}
	return id, nil
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	u, err := s.signIn(w, r, in.Username, in.Password, "form")
	if err != nil {
		return err
	}
	return s.writeMe(w, r, u)
}

// signIn throttles and audits every way of signing in with a password;
// method says how the user came in, for the audit log.
func (s *Server) signIn(w http.ResponseWriter, r *http.Request, username, password, method string) (*auth.User, error) {
	u, err := s.checkPassword(r, username, password)
	if errors.Is(err, auth.ErrNoSession) {
		s.auditLogin(r, username, nil, method)
		return nil, fail(http.StatusUnauthorized, i18n.AccountSignInFailed)
	}
	if err != nil {
		return nil, err
	}
	if err := s.startSession(w, r, u); err != nil {
		return nil, err
	}
	s.auditLogin(r, username, u, method)
	return u, nil
}

// startSession signs u in: it opens a session, notes the sign-in and sets
// the session cookie.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, u *auth.User) error {
	token, expires, err := auth.NewSessionFor(r.Context(), s.db, u.ID, s.sessionLifetime(r.Context()))
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(r.Context(), `UPDATE users SET last_login_at = ? WHERE id = ?`,
		time.Now().UTC(), u.ID); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secureRequest(r, s.base(), s.proxies, s.log),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if err := s.signOut(w, r); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) signOut(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(auth.CookieName); err == nil {
		if err := auth.EndSession(r.Context(), s.db, c.Value); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{Name: auth.CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		Secure: secureRequest(r, s.base(), s.proxies, s.log), SameSite: http.SameSiteLaxMode})
	return nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	return s.writeMe(w, r, currentUser(r))
}

// updateMe changes the user's own profile: name, e-mail address,
// interface language (one of i18n.Locales, or null to have none), the
// project that scopes their pages (a key they may browse, or null for every
// project) and password, which takes the current one. Fields left out keep
// their value.
func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name            *string         `json:"name"`
		Email           *string         `json:"email"`
		Locale          json.RawMessage `json:"locale"`
		Project         json.RawMessage `json:"project"`
		CurrentPassword *string         `json:"currentPassword"`
		NewPassword     *string         `json:"newPassword"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	u := currentUser(r)
	sets := map[string]any{}
	if in.Name != nil {
		name, err := cleanName(*in.Name, sqlq.VarcharWidth)
		if err != nil {
			return err
		}
		sets["display_name"] = name
	}
	emailChanged := false
	if in.Email != nil {
		email := strings.TrimSpace(*in.Email)
		if email != "" && !validEmail(email) {
			return fail(http.StatusBadRequest, i18n.AccountEmailInvalid)
		}
		if email != strings.TrimSpace(u.Email) {
			// The address receives the links that reset the account: as for
			// the password, a token cannot change it and the current password
			// is asked for.
			if u.TokenID != 0 {
				return fail(http.StatusForbidden, i18n.AccountTokenCannotChangeEmail)
			}
			emailChanged = true
		}
		sets["email"] = email
	}
	if in.Locale != nil {
		var l *string
		if err := json.Unmarshal(in.Locale, &l); err != nil || (l != nil && !i18n.Supported(*l)) {
			return failf(http.StatusBadRequest, i18n.AccountLanguageUnsupported, strings.Join(i18n.Locales, ", "))
		}
		sets["locale"] = l
	}
	if in.Project != nil {
		var key *string
		if err := json.Unmarshal(in.Project, &key); err != nil {
			return failf(http.StatusBadRequest, i18n.ProjectUnknown, string(in.Project))
		}
		if key == nil {
			sets["scope_project_id"] = nil
		} else {
			id, err := s.scopeProject(r, *key)
			if err != nil {
				return err
			}
			sets["scope_project_id"] = id
		}
	}
	if in.NewPassword != nil || emailChanged {
		if u.TokenID != 0 {
			return fail(http.StatusForbidden, i18n.AccountTokenCannotChangePassword)
		}
		if in.CurrentPassword == nil || *in.CurrentPassword == "" {
			return fail(http.StatusBadRequest, i18n.AccountCurrentPasswordRequired)
		}
		if in.NewPassword != nil {
			if err := s.checkNewPassword(ctx, *in.NewPassword, u.Username); err != nil {
				return err
			}
		}
		if _, err := s.checkPassword(r, u.Username, *in.CurrentPassword); errors.Is(err, auth.ErrNoSession) {
			return fail(http.StatusBadRequest, i18n.AccountCurrentPasswordIncorrect)
		} else if err != nil {
			return err
		}
	}
	before, err := s.userSnapshot(ctx, u.ID)
	if err != nil {
		return err
	}
	var hash string
	if in.NewPassword != nil {
		if hash, err = auth.HashPassword(*in.NewPassword); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for column, v := range sets {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET `+column+` = ? WHERE id = ?`, v, u.ID); err != nil {
			return err
		}
	}
	// Links mailed to the old address must not work once it is not theirs.
	if email, ok := sets["email"]; ok && email != before.Email {
		if err := auth.RevokeAccountTokens(ctx, tx, u.ID, time.Now()); err != nil {
			return err
		}
	}
	var revoked int64
	if in.NewPassword != nil {
		// The other sign-ins end; this one goes on.
		current := ""
		if c, err := r.Cookie(auth.CookieName); err == nil {
			current = auth.TokenHash(c.Value)
		}
		if revoked, err = auth.SetPasswordHash(ctx, tx, u.ID, hash, current); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auditUserChange(r, u.ID, before, in.NewPassword != nil, revoked)
	me, err := s.userByID(ctx, u.ID)
	if err != nil {
		return err
	}
	return s.writeMe(w, r, &me)
}
