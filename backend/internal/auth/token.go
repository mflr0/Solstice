package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// TokenPrefix starts every personal access token, so that a leaked token is
// easy to recognise (secret scanners, logs).
const TokenPrefix = "sol_"

// tokenTouchEvery limits how often a token's last use is written: scripts
// may call the API many times a second.
const tokenTouchEvery = time.Minute

// NewToken creates a personal access token for a user and returns it. This is
// the only time the token exists in clear: the database keeps its hash.
// expires may be nil for a token that never expires.
func NewToken(ctx context.Context, db *sql.DB, userID int64, name string, expires *time.Time) (token string, id int64, err error) {
	raw, err := RandomToken()
	if err != nil {
		return "", 0, err
	}
	token = TokenPrefix + raw
	var exp any
	if expires != nil {
		exp = expires.UTC()
	}
	res, err := db.ExecContext(ctx, `INSERT INTO api_tokens (user_id, name, token_hash, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)`, userID, name, TokenHash(token), time.Now().UTC(), exp)
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
// access token, and records that the token was used.
func LookupToken(ctx context.Context, db *sql.DB, token string) (*User, error) {
	if !LooksLikeToken(token) {
		return nil, ErrNoSession
	}
	var u User
	var lastUsed sql.NullTime
	now := time.Now().UTC()
	err := db.QueryRowContext(ctx, `SELECT u.id, u.username, u.display_name, u.email, u.is_admin, COALESCE(u.locale, ''),
		u.directory, t.id, t.last_used_at
		FROM api_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = ? AND (t.expires_at IS NULL OR t.expires_at > ?) AND u.active`, TokenHash(token), now).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Admin, &u.Locale, &u.Directory, &u.TokenID, &lastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}
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
