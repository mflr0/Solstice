package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"solstice/internal/auth"
	"solstice/internal/i18n"
)

type meResponse struct {
	ID       int64   `json:"id"`
	Username string  `json:"username"`
	Name     string  `json:"name"`
	Email    string  `json:"email"`
	Admin    bool    `json:"admin"`
	Locale   *string `json:"locale"` // null: none chosen
}

func meOf(u *auth.User) meResponse {
	me := meResponse{ID: u.ID, Username: u.Username, Name: u.DisplayName, Email: u.Email, Admin: u.Admin}
	if u.Locale != "" {
		me.Locale = &u.Locale
	}
	return me
}

// userByID loads an account for its id, with the fields meOf answers with.
func (s *Server) userByID(ctx context.Context, id int64) (auth.User, error) {
	var u auth.User
	err := s.db.QueryRowContext(ctx, `SELECT id, username, display_name, email, is_admin, COALESCE(locale, '')
		FROM users WHERE id = ?`, id).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Admin, &u.Locale)
	return u, err
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
	return writeJSON(w, http.StatusOK, meOf(u))
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
	return writeJSON(w, http.StatusOK, meOf(currentUser(r)))
}

// updateMe changes the user's own profile: name, e-mail address,
// interface language (one of i18n.Locales, or null to have none) and
// password, which takes the current one. Fields left out keep their value.
func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name            *string         `json:"name"`
		Email           *string         `json:"email"`
		Locale          json.RawMessage `json:"locale"`
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
		name, err := cleanName(*in.Name, 255)
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
	if in.NewPassword != nil || emailChanged {
		if u.TokenID != 0 {
			return fail(http.StatusForbidden, i18n.AccountTokenCannotChangePassword)
		}
		if in.CurrentPassword == nil || *in.CurrentPassword == "" {
			return fail(http.StatusBadRequest, i18n.AccountCurrentPasswordRequired)
		}
		if in.NewPassword != nil {
			if err := s.checkNewPassword(ctx, *in.NewPassword); err != nil {
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
	for column, v := range sets {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET `+column+` = ? WHERE id = ?`, v, u.ID); err != nil {
			return err
		}
	}
	// Links mailed to the old address must not work once it is not theirs.
	if email, ok := sets["email"]; ok && email != before.Email {
		if err := auth.RevokeAccountTokens(ctx, s.db, u.ID, time.Now()); err != nil {
			return err
		}
	}
	if in.NewPassword != nil {
		if err := auth.SetPassword(ctx, s.db, u.ID, *in.NewPassword); err != nil {
			return err
		}
		// The other sign-ins end; this one goes on.
		current := ""
		if c, err := r.Cookie(auth.CookieName); err == nil {
			current = auth.TokenHash(c.Value)
		}
		if err := s.endSessions(ctx, u.ID, current); err != nil {
			return err
		}
	}
	s.auditUserChange(r, u.ID, before, in.NewPassword != nil)
	me, err := s.userByID(ctx, u.ID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, meOf(&me))
}
