package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// The single-use links sent by e-mail: to choose a new password after
// forgetting it, and to choose the first password of an invited account.
// Each link carries a random token; the database keeps only its SHA-256 hash
// (table account_tokens), so a leaked copy of the database cannot be used to
// take an account over.
const (
	PurposeReset  = "reset"
	PurposeInvite = "invite"
)

// How long a link works.
const (
	ResetLifetime  = time.Hour
	InviteLifetime = 7 * 24 * time.Hour
)

// usedKeep is how long used tokens stay in the table before the purge.
const usedKeep = 24 * time.Hour

// ErrBadToken means a link matches no token that can still be used: unknown,
// used, expired, replaced by a newer one, or the account is deactivated. The
// causes are not told apart.
var ErrBadToken = errors.New("link invalid, used or expired")

// AccountToken is a token that can be used.
type AccountToken struct {
	ID      int64
	UserID  int64
	Purpose string
	Expires time.Time
}

// execer is the database or a transaction.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// NewAccountTokenValue returns a fresh token for a link sent by e-mail.
func NewAccountTokenValue() (string, error) { return RandomToken() }

// AccountTokenHash is what the database stores of a token.
func AccountTokenHash(token string) string { return TokenHash(token) }

// Lifetime is how long a token of a purpose works; 0 for an unknown purpose.
func Lifetime(purpose string) time.Duration {
	switch purpose {
	case PurposeReset:
		return ResetLifetime
	case PurposeInvite:
		return InviteLifetime
	}
	return 0
}

// Usable tells whether a token with this expiry and use time works at now.
func Usable(expires time.Time, used sql.NullTime, now time.Time) bool {
	return !used.Valid && expires.After(now)
}

// IssueAccountToken creates a token of a purpose for an account and returns
// it; it exists in clear nowhere else. Every unused token the account had is
// replaced: only the newest link works. The account row is locked meanwhile,
// so two requests at once cannot both leave a working link.
func IssueAccountToken(ctx context.Context, tx *sql.Tx, userID int64, purpose string, now time.Time) (string, error) {
	life := Lifetime(purpose)
	if life == 0 {
		return "", errors.New("unknown account token purpose")
	}
	token, err := NewAccountTokenValue()
	if err != nil {
		return "", err
	}
	now = now.UTC()
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id = ? FOR UPDATE`, userID).Scan(&id); err != nil {
		return "", err
	}
	if err := RevokeAccountTokens(ctx, tx, userID, now); err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO account_tokens (user_id, token_hash, purpose, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)`, userID, TokenHash(token), purpose, now, now.Add(life))
	return token, err
}

// LookupAccountToken returns the token behind a link if it can still be used
// by an active account, without using it.
func LookupAccountToken(ctx context.Context, db *sql.DB, token string, now time.Time) (*AccountToken, error) {
	var t AccountToken
	err := db.QueryRowContext(ctx, `SELECT t.id, t.user_id, t.purpose, t.expires_at
		FROM account_tokens t JOIN users u ON u.id = t.user_id
		WHERE t.token_hash = ? AND t.used_at IS NULL AND t.expires_at > ? AND u.active`,
		TokenHash(token), now.UTC()).Scan(&t.ID, &t.UserID, &t.Purpose, &t.Expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBadToken
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UseAccountToken marks a token used, inside the caller's transaction, which
// goes on to change the password: a token works for one request only, even
// when two arrive together, because the update that marks it succeeds once.
func UseAccountToken(ctx context.Context, tx *sql.Tx, token string, now time.Time) (*AccountToken, error) {
	now = now.UTC()
	hash := TokenHash(token)
	res, err := tx.ExecContext(ctx, `UPDATE account_tokens t JOIN users u ON u.id = t.user_id SET t.used_at = ?
		WHERE t.token_hash = ? AND t.used_at IS NULL AND t.expires_at > ? AND u.active`, now, hash, now)
	if err != nil {
		return nil, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return nil, err
	} else if n != 1 {
		return nil, ErrBadToken
	}
	var t AccountToken
	err = tx.QueryRowContext(ctx, `SELECT id, user_id, purpose, expires_at FROM account_tokens WHERE token_hash = ?`,
		hash).Scan(&t.ID, &t.UserID, &t.Purpose, &t.Expires)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// RevokeAccountTokens invalidates every unused token of an account: after a
// password change, a change of e-mail address, or when a newer one is issued.
func RevokeAccountTokens(ctx context.Context, ex execer, userID int64, now time.Time) error {
	_, err := ex.ExecContext(ctx, `UPDATE account_tokens SET used_at = ? WHERE user_id = ? AND used_at IS NULL`,
		now.UTC(), userID)
	return err
}

// PurgeAccountTokens deletes the tokens that are expired, and those used
// more than a day ago.
func PurgeAccountTokens(ctx context.Context, db *sql.DB, now time.Time) (int64, error) {
	now = now.UTC()
	res, err := db.ExecContext(ctx, `DELETE FROM account_tokens WHERE expires_at < ? OR used_at < ?`, now, now.Add(-usedKeep))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
