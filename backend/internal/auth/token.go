package auth

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"
)

// TokenPrefix starts every personal access token, so that a leaked token is
// easy to recognise (secret scanners, logs).
const TokenPrefix = "sol_"

// tokenTouchEvery limits how often a token's last use is written: scripts
// may call the API many times a second.
const tokenTouchEvery = time.Minute

// Scope is what a personal access token may do. Every route of the API
// requires one; a session cookie or a password has them all.
type Scope string

// The scopes of personal access tokens. Every scope implies ScopeRead.
const (
	ScopeRead     Scope = "read"           // every GET outside /api/admin, and the POSTs that only read
	ScopeIssues   Scope = "issues:write"   // issues, links, attachments, votes, watches
	ScopeComments Scope = "comments:write" // comments
	ScopeWorklogs Scope = "worklogs:write" // work logs
	ScopeAgile    Scope = "agile:write"    // boards, sprints, rank
	ScopeFilters  Scope = "filters:write"  // filters, favourites, subscriptions
	ScopeProjects Scope = "projects:write" // project settings, versions, components, workflows, statuses
	ScopeProfile  Scope = "profile:write"  // the user's own account, dashboard and notification settings
	ScopeAdmin    Scope = "admin"          // /api/admin; only for administrators
)

// Scopes lists every scope, in the order answers list them.
var Scopes = []Scope{ScopeRead, ScopeIssues, ScopeComments, ScopeWorklogs, ScopeAgile, ScopeFilters,
	ScopeProjects, ScopeProfile, ScopeAdmin}

// Known tells whether s is the name of a scope.
func (s Scope) Known() bool { return slices.Contains(Scopes, s) }

// Allows tells whether the user may use a route that requires scope: always
// when signed in otherwise than with a token.
func (u *User) Allows(scope Scope) bool {
	if !u.ByToken() {
		return true
	}
	return slices.Contains(u.Scopes, scope) || (scope == ScopeRead && len(u.Scopes) > 0)
}

// NormalizeScopes returns the scopes in the order of Scopes, without
// duplicates. They must all be known.
func NormalizeScopes(in []Scope) []Scope {
	out := []Scope{}
	for _, s := range Scopes {
		if slices.Contains(in, s) {
			out = append(out, s)
		}
	}
	return out
}

// ParseScopes reads the scopes column of api_tokens, skipping unknown names.
func ParseScopes(stored string) []Scope {
	var out []Scope
	for _, f := range strings.Fields(stored) {
		if s := Scope(f); s.Known() {
			out = append(out, s)
		}
	}
	return out
}

func joinScopes(scopes []Scope) string {
	parts := make([]string, len(scopes))
	for i, s := range scopes {
		parts[i] = string(s)
	}
	return strings.Join(parts, " ")
}

// NewToken creates a personal access token for a user and returns it. This is
// the only time the token exists in clear: the database keeps its hash.
// expires may be nil for a token that never expires. scopes must be known
// and not empty.
func NewToken(ctx context.Context, db *sql.DB, userID int64, name string, scopes []Scope, expires *time.Time) (token string, id int64, err error) {
	scopes = NormalizeScopes(scopes)
	if len(scopes) == 0 {
		return "", 0, errors.New("auth: a token needs a scope")
	}
	raw, err := RandomToken()
	if err != nil {
		return "", 0, err
	}
	token = TokenPrefix + raw
	var exp any
	if expires != nil {
		exp = expires.UTC()
	}
	res, err := db.ExecContext(ctx, `INSERT INTO api_tokens (user_id, name, token_hash, scopes, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`, userID, name, TokenHash(token), joinScopes(scopes), time.Now().UTC(), exp)
	if err != nil {
		return "", 0, err
	}
	id, err = res.LastInsertId()
	return token, id, err
}

// LooksLikeToken tells whether a secret has the shape of a personal access
// token (as opposed to a password).
func LooksLikeToken(s string) bool {
	return strings.HasPrefix(s, TokenPrefix) && len(s) > len(TokenPrefix)+20
}

// LookupToken returns the active user behind a valid, unexpired personal
// access token, with the token's scopes, and records that the token was used.
func LookupToken(ctx context.Context, db *sql.DB, token string) (*User, error) {
	if !LooksLikeToken(token) {
		return nil, ErrNoSession
	}
	var u User
	var lastUsed sql.NullTime
	var scopes string
	now := time.Now().UTC()
	err := db.QueryRowContext(ctx, `SELECT u.id, u.username, u.display_name, u.email, u.is_admin, COALESCE(u.locale, ''),
		u.directory, t.id, t.scopes, t.last_used_at
		FROM api_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = ? AND (t.expires_at IS NULL OR t.expires_at > ?) AND u.active`, TokenHash(token), now).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Admin, &u.Locale, &u.Directory, &u.TokenID, &scopes, &lastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}
	u.Scopes = ParseScopes(scopes)
	if !lastUsed.Valid || now.Sub(lastUsed.Time) > tokenTouchEvery {
		if _, err := db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, now, u.TokenID); err != nil {
			return nil, err
		}
	}
	return &u, nil
}

// RefusedToken is a personal access token that exists but cannot sign anyone
// in: it has expired, or its owner's account is inactive. It never carries
// the token itself, only its id and name.
type RefusedToken struct {
	ID       int64
	Name     string
	UserID   int64
	Username string
	Reason   string // "expired" or "inactive"
}

// LookupRefused returns what is known of a token that was presented but
// cannot be used, or nil when no row has that hash. A revoked token is
// deleted, so it is indistinguishable from an unknown one; only its log line
// reports the rejection. It never reads back the token.
func LookupRefused(ctx context.Context, db *sql.DB, token string) *RefusedToken {
	if !LooksLikeToken(token) {
		return nil
	}
	var t RefusedToken
	var expires sql.NullTime
	var active bool
	now := time.Now().UTC()
	err := db.QueryRowContext(ctx, `SELECT t.id, t.name, t.user_id, u.username, u.active, t.expires_at
		FROM api_tokens t JOIN users u ON u.id = t.user_id WHERE t.token_hash = ?`, TokenHash(token)).
		Scan(&t.ID, &t.Name, &t.UserID, &t.Username, &active, &expires)
	if err != nil {
		return nil
	}
	switch {
	case !active:
		t.Reason = "inactive"
	case expires.Valid && !expires.Time.After(now):
		t.Reason = "expired"
	default:
		// Valid and active: LookupToken would have signed it in.
		return nil
	}
	return &t
}
