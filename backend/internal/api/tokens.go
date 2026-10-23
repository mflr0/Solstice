package api

// Personal access tokens: scripts, CI jobs and IDE plugins sign in with them
// instead of a password, as with Jira Data Center ("Authorization: Bearer").
// Each user manages their own; administrators can list and revoke anyone's.

import (
	"database/sql"
	"errors"
	"net/http"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/sqlq"
	"strings"
	"time"
	"unicode/utf8"
)

// credentials reads what scripts and API clients send: "Authorization:
// Bearer <personal access token>", or HTTP basic with a user name and either
// the password (throttled like the sign-in form) or a personal access token.
// method is "token" for a bearer token, "basic" otherwise.
func (s *Server) credentials(r *http.Request) (u *auth.User, method string, err error) {
	ctx := r.Context()
	if scheme, value, ok := strings.Cut(r.Header.Get("Authorization"), " "); ok && strings.EqualFold(scheme, "Bearer") {
		u, err := auth.LookupToken(ctx, s.db, strings.TrimSpace(value))
		if errors.Is(err, auth.ErrNoSession) {
			s.rejectToken(r, strings.TrimSpace(value))
		}
		return u, "token", err
	}
	name, pass, ok := r.BasicAuth()
	if !ok {
		return nil, "", auth.ErrNoSession
	}
	// A token is checked before the throttle: it cannot be guessed, and
	// someone guessing the account's password must not block its scripts.
	if auth.LooksLikeToken(pass) {
		u, err := auth.LookupToken(ctx, s.db, pass)
		switch {
		case err == nil && strings.EqualFold(u.Username, strings.TrimSpace(name)):
			return u, "basic", nil
		case err != nil && !errors.Is(err, auth.ErrNoSession):
			return nil, "", err
		}
	}
	u, err = s.checkPassword(r, name, pass)
	if errors.Is(err, auth.ErrNoSession) {
		s.auditLogin(r, name, nil, "basic")
	}
	return u, "basic", err
}

// rejectToken audits a refused token at most once an hour, so that anyone
// sending a string cannot fill the log; the token itself is never logged.
func (s *Server) rejectToken(r *http.Request, token string) {
	ref := auth.LookupRefused(r.Context(), s.db, token)
	if ref == nil {
		s.log.Debug("API token rejected", "reason", "unknown")
		return
	}
	s.log.Warn("API token rejected", "token", ref.ID, "name", ref.Name, "owner", ref.Username, "reason", ref.Reason)
	if !s.noteRefusedToken(ref.ID) {
		return
	}
	record(r.Context(), s.db, s.log, audit.Entry{UserID: ref.UserID, Username: ref.Username, IP: clientIP(r, s.proxies),
		Action: audit.TokenReject, Target: target("token", ref.ID, ref.Name)})
}

func (s *Server) noteRefusedToken(id int64) bool {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	now := time.Now()
	if at, ok := s.tokenAud[id]; ok && now.Sub(at) < time.Hour {
		return false
	}
	s.tokenAud[id] = now
	if len(s.tokenAud) > 1000 {
		for k, at := range s.tokenAud {
			if now.Sub(at) >= time.Hour {
				delete(s.tokenAud, k)
			}
		}
	}
	return true
}

type tokenOut struct {
	ID       int64      `json:"id"`
	Name     string     `json:"name"`
	Created  time.Time  `json:"created"`
	Expires  *time.Time `json:"expires"`
	LastUsed *time.Time `json:"lastUsed"`
	Expired  bool       `json:"expired"`
	User     *userRef   `json:"user,omitempty"`  // in the administrators' list
	Token    string     `json:"token,omitempty"` // only when just created
}

// listTokens returns tokens, newest first: a user's (userID ≠ 0), or
// everyone's with their owner.
func (s *Server) listTokens(r *http.Request, userID int64, username string) ([]tokenOut, error) {
	q := `SELECT t.id, t.name, t.created_at, t.expires_at, t.last_used_at, u.id, u.username, u.display_name, u.active
		FROM api_tokens t JOIN users u ON u.id = t.user_id`
	var args []any
	switch {
	case userID != 0:
		q += ` WHERE t.user_id = ?`
		args = append(args, userID)
	case username != "":
		q += ` WHERE u.username = LOWER(?)`
		args = append(args, username)
	}
	now := time.Now()
	out := []tokenOut{}
	err := sqlq.Each(r.Context(), s.db, q+` ORDER BY t.created_at DESC, t.id DESC`, func(rows *sql.Rows) error {
		var t tokenOut
		var expires, used sql.NullTime
		var owner userRef
		if err := rows.Scan(&t.ID, &t.Name, &t.Created, &expires, &used, &owner.ID, &owner.Username, &owner.Name,
			&owner.Active); err != nil {
			return err
		}
		if e := sqlq.Time(expires); e != nil {
			t.Expires = e
			t.Expired = !e.After(now)
		}
		t.LastUsed = sqlq.Time(used)
		if userID == 0 {
			t.User = &owner
		}
		out = append(out, t)
		return nil
	}, args...)
	return out, err
}

func (s *Server) myTokens(w http.ResponseWriter, r *http.Request) error {
	out, err := s.listTokens(r, currentUser(r).ID, "")
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// createToken makes a token for the current user, if the instance's policy
// lets them (token_policy.go). The answer is the only time it can be read. A
// token cannot make other tokens: a leaked one must not be able to outlive
// its revocation.
func (s *Server) createToken(w http.ResponseWriter, r *http.Request) error {
	u := currentUser(r)
	if u.TokenID != 0 {
		return fail(http.StatusForbidden, i18n.TokenCannotCreate)
	}
	if ok, err := s.mayCreateTokens(r.Context(), u); err != nil {
		return err
	} else if !ok {
		return fail(http.StatusForbidden, i18n.TokenCreationRestricted)
	}
	var in struct {
		Name    string  `json:"name"`
		Expires *string `json:"expires"` // YYYY-MM-DD (through that day) or an RFC 3339 instant; null: never
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return fail(http.StatusBadRequest, i18n.TokenNameRequired)
	}
	var expires *time.Time
	if in.Expires != nil && strings.TrimSpace(*in.Expires) != "" {
		t, err := s.parseExpiry(strings.TrimSpace(*in.Expires))
		if err != nil {
			return err
		}
		expires = &t
	}
	token, id, err := auth.NewToken(r.Context(), s.db, u.ID, name, expires)
	if err != nil {
		return err
	}
	details := map[string]any{"expires": nil}
	if expires != nil {
		details["expires"] = expires.UTC().Format(time.RFC3339)
	}
	s.auditLog(r, audit.TokenCreate, target("token", id, name), details)
	out := tokenOut{ID: id, Name: name, Created: time.Now().UTC(), Expires: expires, Token: token}
	return writeJSON(w, http.StatusCreated, out)
}

// parseExpiry reads a token's expiry: a date (the token works through that
// day, in the server's time zone) or an instant. It must be in the future.
func (s *Server) parseExpiry(v string) (time.Time, error) {
	var t time.Time
	if d, err := time.ParseInLocation(time.DateOnly, v, s.zone()); err == nil {
		t = d.AddDate(0, 0, 1)
	} else if t, err = time.Parse(time.RFC3339, v); err != nil {
		return t, fail(http.StatusBadRequest, i18n.TokenExpiryInvalid)
	}
	if !t.After(time.Now()) {
		return t, fail(http.StatusBadRequest, i18n.TokenExpiryPast)
	}
	return t.UTC(), nil
}

func (s *Server) revokeMyToken(w http.ResponseWriter, r *http.Request) error {
	return s.revokeToken(w, r, currentUser(r).ID)
}

func (s *Server) adminTokens(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	out, err := s.listTokens(r, 0, strings.TrimSpace(r.URL.Query().Get("user")))
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) adminRevokeToken(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	return s.revokeToken(w, r, 0)
}

// revokeToken deletes a token: one of the user's own (owner ≠ 0), or anyone's.
func (s *Server) revokeToken(w http.ResponseWriter, r *http.Request, owner int64) error {
	ctx := r.Context()
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var name, username string
	var userID int64
	err = s.db.QueryRowContext(ctx, `SELECT t.name, t.user_id, u.username FROM api_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.id = ?`, id).Scan(&name, &userID, &username)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && owner != 0 && userID != owner) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ?`, id); err != nil {
		return err
	}
	s.auditLog(r, audit.TokenRevoke, target("token", id, name),
		map[string]any{"owner": username})
	w.WriteHeader(http.StatusNoContent)
	return nil
}
