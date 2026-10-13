package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

// CookieName is the session cookie.
const CookieName = "solstice_session"

// SessionLifetime is how long a sign-in lasts.
const SessionLifetime = 30 * 24 * time.Hour

// User is the signed-in user attached to a request.
type User struct {
	ID          int64
	Username    string
	DisplayName string
	Email       string
	Admin       bool
}

// ErrNoSession means the token matches no valid session.
var ErrNoSession = errors.New("no valid session")

// The database only stores a hash of the token: a leaked sessions table
// cannot be replayed.
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewSession opens a session and returns its token.
func NewSession(ctx context.Context, db *sql.DB, userID int64) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := time.Now().UTC()
	expires := now.Add(SessionLifetime)
	_, err := db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)`, tokenHash(token), userID, now, expires)
	return token, expires, err
}

// Lookup returns the active user behind a session token.
func Lookup(ctx context.Context, db *sql.DB, token string) (*User, error) {
	var u User
	err := db.QueryRowContext(ctx, `SELECT u.id, u.username, u.display_name, u.email, u.is_admin
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ? AND u.active`, tokenHash(token), time.Now().UTC()).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Admin)
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
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash(token))
	return err
}

// Authenticate checks a user name and password. Jira hashes are upgraded to
// Argon2id on the first successful sign-in.
func Authenticate(ctx context.Context, db *sql.DB, username, password string) (*User, error) {
	var u User
	var hash sql.NullString
	err := db.QueryRowContext(ctx, `SELECT id, username, display_name, email, is_admin, password_hash
		FROM users WHERE username = LOWER(?) AND active`, username).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Admin, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		// Spend the same time as a real check so user names cannot be probed.
		CheckPassword(dummyHash, password)
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}
	if !hash.Valid {
		CheckPassword(dummyHash, password)
		return nil, ErrNoSession
	}
	ok, rehash := CheckPassword(hash.String, password)
	if !ok {
		return nil, ErrNoSession
	}
	if rehash {
		if fresh, err := HashPassword(password); err == nil {
			_, _ = db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, fresh, u.ID)
		}
	}
	return &u, nil
}

// SetPassword stores a new password for a user.
func SetPassword(ctx context.Context, db *sql.DB, userID int64, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
	return err
}

var dummyHash, _ = HashPassword("solstice-timing-equaliser")
