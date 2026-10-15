package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	u, err := s.checkPassword(r, in.Username, in.Password)
	if errors.Is(err, auth.ErrNoSession) {
		s.auditLogin(r, in.Username, nil, "form")
		return fail(http.StatusUnauthorized, "Identifiant ou mot de passe incorrect.")
	}
	if err != nil {
		return err
	}
	token, expires, err := auth.NewSession(r.Context(), s.db, u.ID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   secureRequest(r, s.baseURL),
		SameSite: http.SameSiteLaxMode,
	})
	s.auditLogin(r, in.Username, u, "form")
	return writeJSON(w, http.StatusOK, meOf(u))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(auth.CookieName); err == nil {
		if err := auth.EndSession(r.Context(), s.db, c.Value); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{Name: auth.CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	return writeJSON(w, http.StatusOK, meOf(currentUser(r)))
}

// updateMe changes the user's own settings: the interface language, one of
// i18n.Locales, or null to have none.
func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Locale json.RawMessage `json:"locale"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	u := currentUser(r)
	if in.Locale != nil {
		var l *string
		if err := json.Unmarshal(in.Locale, &l); err != nil || (l != nil && !i18n.Supported(*l)) {
			return failf(http.StatusBadRequest, "Langue non prise en charge (au choix : %s).", strings.Join(i18n.Locales, ", "))
		}
		if _, err := s.db.ExecContext(r.Context(), `UPDATE users SET locale = ? WHERE id = ?`, l, u.ID); err != nil {
			return err
		}
		u.Locale = ""
		if l != nil {
			u.Locale = *l
		}
	}
	return writeJSON(w, http.StatusOK, meOf(u))
}
