package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

// CookieName is the session cookie.
const CookieName = "solstice_session"

// SessionLifetime is how long a sign-in lasts by default (administrators
// may choose another lifetime: NewSessionFor).
const SessionLifetime = 30 * 24 * time.Hour

// User is the signed-in user attached to a request.
type User struct {
	ID          int64
	Username    string
	DisplayName string
	Email       string
	Admin       bool
	Locale      string // interface language chosen by the user, "" to follow the browser
	Directory   string // the directory that made the account its own ("ldap", "oidc"), "" for a local account
	TokenID     int64  // the personal access token the request signed in with, 0 for none
}

// ErrNoSession means the token matches no valid session.
var ErrNoSession = errors.New("no valid session")

// TokenHash is what the database stores of a token (hex SHA-256): a leaked
// table cannot be replayed.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// tokenBytes is the size of a token before encoding: 256 bits.
const tokenBytes = 32

// RandomToken returns a fresh token: 32 bytes from the system's random
// source, URL-safe.
func RandomToken() (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// NewSession opens a session of the default lifetime and returns its token.
func NewSession(ctx context.Context, db *sql.DB, userID int64) (string, time.Time, error) {
	return NewSessionFor(ctx, db, userID, SessionLifetime)
}

// NewSessionFor opens a session lasting lifetime and returns its token.
func NewSessionFor(ctx context.Context, db *sql.DB, userID int64, lifetime time.Duration) (string, time.Time, error) {
	token, err := RandomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	now := time.Now().UTC()
	expires := now.Add(lifetime)
	_, err = db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)`, TokenHash(token), userID, now, expires)
	return token, expires, err
}

// Lookup returns the active user behind a session token.
func Lookup(ctx context.Context, db *sql.DB, token string) (*User, error) {
	var u User
	err := db.QueryRowContext(ctx, `SELECT u.id, u.username, u.display_name, u.email, u.is_admin, COALESCE(u.locale, ''),
		u.directory
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ? AND u.active`, TokenHash(token), time.Now().UTC()).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Admin, &u.Locale, &u.Directory)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// EndSession deletes a session.
func EndSession(ctx context.Context, db *sql.DB, token string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, TokenHash(token))
	return err
}

// CanonicalUsername is the one form of a typed user name used to look the
// account up and to count failed sign-ins: surrounding spaces removed, lower
// case. The database compares names with a collation that ignores trailing
// spaces, so "admin" and "admin " are the same account and must share their
// counters.
func CanonicalUsername(typed string) string {
	return strings.ToLower(strings.TrimSpace(typed))
}

// Authenticate checks a user name and password. Jira hashes are upgraded to
// Argon2id on the first successful sign-in. With a directory (nil for none),
// the accounts of any directory (LDAP or OpenID Connect) are checked there,
// and so is a name no account has, or one without a password: see
// directorySignIn.
func Authenticate(ctx context.Context, db *sql.DB, dir Directory, username, password string) (*User, error) {
	username = CanonicalUsername(username)
	var u User
	var hash sql.NullString
	var active bool
	err := db.QueryRowContext(ctx, `SELECT id, username, display_name, email, is_admin, COALESCE(locale, ''), directory,
		password_hash, active
		FROM users WHERE username = LOWER(?)`, username).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Admin, &u.Locale, &u.Directory, &hash, &active)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if dir != nil {
			return directorySignIn(ctx, db, dir, 0, username, password)
		}
		// Spend the same time as a real check so user names cannot be probed.
		CheckPassword(dummyHash(), password)
		return nil, ErrNoSession
	case err != nil:
		return nil, err
	case active && dir != nil && (u.Directory != "" || !hash.Valid):
		return directorySignIn(ctx, db, dir, u.ID, username, password)
	case !active || u.Directory != "" || !hash.Valid:
		CheckPassword(dummyHash(), password)
		return nil, ErrNoSession
	}
	ok, rehash := CheckPassword(hash.String, password)
	if !ok {
		if !strings.HasPrefix(hash.String, "$argon2id$") {
			// An imported Jira hash is refused quicker than Argon2id: spend
			// that time too, so such accounts cannot be told from the others.
			// Only failures are timed by an attacker.
			CheckPassword(dummyHash(), password)
		}
		return nil, ErrNoSession
	}
	if rehash {
		if fresh, err := HashPassword(password); err == nil {
			_, _ = db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, fresh, u.ID)
		}
	}
	return &u, nil
}

// SetPasswordHash stores a new password hash for a user and ends every other
// way into the account: EndCredentials. An account of a directory has no
// password here: ErrDirectoryAccount. Call it in the transaction that makes
// the change.
func SetPasswordHash(ctx context.Context, ex execer, userID int64, hash, keep string) (revoked int64, err error) {
	if err := refuseDirectory(ctx, ex, userID); err != nil {
		return 0, err
	}
	if _, err := ex.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID); err != nil {
		return 0, err
	}
	return EndCredentials(ctx, ex, userID, keep)
}

// EndCredentials ends the sessions of an account, except the one whose token
// hash is keep ("" for none), deletes its personal access tokens and revokes
// the links mailed to it, after a password change, a deactivation or a
// reactivation. It returns how many access tokens it deleted.
func EndCredentials(ctx context.Context, ex execer, userID int64, keep string) (revoked int64, err error) {
	if _, err := ex.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash <> ?`, userID, keep); err != nil {
		return 0, err
	}
	res, err := ex.ExecContext(ctx, `DELETE FROM api_tokens WHERE user_id = ?`, userID)
	if err != nil {
		return 0, err
	}
	if revoked, err = res.RowsAffected(); err != nil {
		return 0, err
	}
	return revoked, RevokeAccountTokens(ctx, ex, userID, time.Now())
}

// dummyHash is built on first use, not at init: every binary linking auth,
// such as `solstice health`, would otherwise pay one Argon2id at start-up.
var dummyHash = sync.OnceValue(func() string {
	h, _ := HashPassword("solstice-timing-equaliser")
	return h
})

// PrepareSignIn builds the hash that unknown accounts are checked against,
// so that the first such sign-in takes no longer than the next ones.
func PrepareSignIn() { dummyHash() }
