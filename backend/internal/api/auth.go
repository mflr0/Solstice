package api

import (
	"errors"
	"net/http"

	"solstice/internal/auth"
)

type meResponse struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Admin    bool   `json:"admin"`
}

func meOf(u *auth.User) meResponse {
	return meResponse{ID: u.ID, Username: u.Username, Name: u.DisplayName, Email: u.Email, Admin: u.Admin}
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
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
	})
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
