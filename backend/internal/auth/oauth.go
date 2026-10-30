package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"time"
)

// Prefixes of OAuth tokens. They differ from TokenPrefix, so that the
// interface's API and the REST API, which only take personal access tokens,
// never accept one.
const (
	OAuthAccessPrefix  = "solo_"
	OAuthRefreshPrefix = "solor_"
)

// Lifetimes of OAuth codes and access tokens. A refresh token's lifetime is
// the instance's setting, renewed at each refresh.
const (
	OAuthCodeLifetime   = 5 * time.Minute
	OAuthAccessLifetime = time.Hour
)

const (
	oauthAccess  = "access"
	oauthRefresh = "refresh"
)

// ErrOAuthGrant is an authorization code or refresh token that is unknown,
// expired, already used, or presented by another client or with a wrong
// PKCE verifier: the token endpoint's invalid_grant.
var ErrOAuthGrant = errors.New("auth: invalid OAuth grant")

// ErrOAuthReplay comes with ErrOAuthGrant for a code or refresh token
// presented again: the tokens it gave are revoked.
var ErrOAuthReplay = errors.New("auth: OAuth code or refresh token replayed")

// OAuthClient is an OAuth client as its metadata document or its
// registration describes it.
type OAuthClient struct {
	ID   string // the document's URL, or the client_id given at registration
	Name string
}

// OAuthCode is what an authorization code was issued for.
type OAuthCode struct {
	Client      OAuthClient
	RedirectURI string
	Challenge   string // PKCE S256 code challenge
	Scopes      []Scope
}

// OAuthTokens is the answer of the token endpoint.
type OAuthTokens struct {
	Access  string
	Refresh string
	Scopes  []Scope
	GrantID int64
}

// AuthorizeOAuth records that a user allows a client these scopes, replacing
// what an earlier consent gave it, and returns an authorization code. The
// tokens of an earlier consent keep their own scopes.
func AuthorizeOAuth(ctx context.Context, db *sql.DB, userID int64, c OAuthCode, now time.Time) (code string, grantID int64, err error) {
	code, err = RandomToken()
	if err != nil {
		return "", 0, err
	}
	now = now.UTC()
	scopes := joinScopes(NormalizeScopes(c.Scopes))
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO oauth_grants (user_id, client_id, client_key, client_name, scopes, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE client_name = VALUES(client_name), scopes = VALUES(scopes)`,
		userID, c.Client.ID, TokenHash(c.Client.ID), c.Client.Name, scopes, now); err != nil {
		return "", 0, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM oauth_grants WHERE user_id = ? AND client_key = ?`,
		userID, TokenHash(c.Client.ID)).Scan(&grantID); err != nil {
		return "", 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO oauth_codes (grant_id, code_hash, redirect_uri, code_challenge, scopes, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`, grantID, TokenHash(code), c.RedirectURI, c.Challenge, scopes,
		now.Add(OAuthCodeLifetime)); err != nil {
		return "", 0, err
	}
	return code, grantID, tx.Commit()
}

// ExchangeOAuthCode turns an authorization code into tokens, once. A code
// presented again revokes the tokens it gave: whoever replays it may have
// stolen it.
func ExchangeOAuthCode(ctx context.Context, db *sql.DB, code, clientID, redirectURI, verifier string,
	refreshLife time.Duration, now time.Time) (*OAuthTokens, error) {
	now = now.UTC()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var id, grantID int64
	var gotClient, gotRedirect, challenge, scopes string
	var expires time.Time
	var used sql.NullTime
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT c.id, c.grant_id, g.client_id, c.redirect_uri, c.code_challenge, c.scopes,
		c.expires_at, c.used_at, u.active
		FROM oauth_codes c JOIN oauth_grants g ON g.id = c.grant_id JOIN users u ON u.id = g.user_id
		WHERE c.code_hash = ? FOR UPDATE`, TokenHash(code)).
		Scan(&id, &grantID, &gotClient, &gotRedirect, &challenge, &scopes, &expires, &used, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrOAuthGrant
	}
	if err != nil {
		return nil, err
	}
	if used.Valid {
		if _, err := tx.ExecContext(ctx, `DELETE FROM oauth_tokens WHERE family = ?`, id); err != nil {
			return nil, err
		}
		return nil, errors.Join(ErrOAuthGrant, ErrOAuthReplay, tx.Commit())
	}
	// Any attempt uses the code up, so a wrong verifier cannot be retried.
	if _, err := tx.ExecContext(ctx, `UPDATE oauth_codes SET used_at = ? WHERE id = ?`, now, id); err != nil {
		return nil, err
	}
	if !active || !expires.After(now) || gotClient != clientID || gotRedirect != redirectURI || !PKCEMatches(verifier, challenge) {
		return nil, errors.Join(ErrOAuthGrant, tx.Commit())
	}
	t, err := issueOAuthTokens(ctx, tx, grantID, id, ParseScopes(scopes), refreshLife, now)
	if err != nil {
		return nil, err
	}
	return t, tx.Commit()
}

// RefreshOAuth turns a refresh token into new tokens, and the refresh token
// into a used one. scopes, when not empty, narrows what the new tokens may
// do: it cannot widen it. A used refresh token presented again revokes
// every token of its family.
func RefreshOAuth(ctx context.Context, db *sql.DB, refresh, clientID string, scopes []Scope,
	refreshLife time.Duration, now time.Time) (*OAuthTokens, error) {
	if !strings.HasPrefix(refresh, OAuthRefreshPrefix) {
		return nil, ErrOAuthGrant
	}
	now = now.UTC()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var id, grantID, family int64
	var gotClient, stored string
	var expires time.Time
	var used sql.NullTime
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT t.id, t.grant_id, t.family, g.client_id, t.scopes, t.expires_at, t.used_at, u.active
		FROM oauth_tokens t JOIN oauth_grants g ON g.id = t.grant_id JOIN users u ON u.id = g.user_id
		WHERE t.token_hash = ? AND t.kind = ? FOR UPDATE`, TokenHash(refresh), oauthRefresh).
		Scan(&id, &grantID, &family, &gotClient, &stored, &expires, &used, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrOAuthGrant
	}
	if err != nil {
		return nil, err
	}
	if used.Valid {
		if _, err := tx.ExecContext(ctx, `DELETE FROM oauth_tokens WHERE family = ?`, family); err != nil {
			return nil, err
		}
		return nil, errors.Join(ErrOAuthGrant, ErrOAuthReplay, tx.Commit())
	}
	if !active || !expires.After(now) || gotClient != clientID {
		return nil, ErrOAuthGrant
	}
	granted := ParseScopes(stored)
	if len(scopes) > 0 {
		for _, sc := range scopes {
			if !slices.Contains(granted, sc) {
				return nil, ErrOAuthScope
			}
		}
		granted = NormalizeScopes(append(scopes, ScopeRead))
	}
	if _, err := tx.ExecContext(ctx, `UPDATE oauth_tokens SET used_at = ? WHERE id = ?`, now, id); err != nil {
		return nil, err
	}
	t, err := issueOAuthTokens(ctx, tx, grantID, family, granted, refreshLife, now)
	if err != nil {
		return nil, err
	}
	return t, tx.Commit()
}

// ErrOAuthScope is a refresh asking for a scope the grant does not have.
var ErrOAuthScope = errors.New("auth: OAuth scope not granted")

func issueOAuthTokens(ctx context.Context, tx *sql.Tx, grantID, family int64, scopes []Scope,
	refreshLife time.Duration, now time.Time) (*OAuthTokens, error) {
	t := &OAuthTokens{Scopes: scopes, GrantID: grantID}
	for _, k := range []struct {
		kind, prefix string
		life         time.Duration
		out          *string
	}{{oauthAccess, OAuthAccessPrefix, OAuthAccessLifetime, &t.Access}, {oauthRefresh, OAuthRefreshPrefix, refreshLife, &t.Refresh}} {
		raw, err := RandomToken()
		if err != nil {
			return nil, err
		}
		*k.out = k.prefix + raw
		if _, err := tx.ExecContext(ctx, `INSERT INTO oauth_tokens (grant_id, family, kind, token_hash, scopes, created_at, expires_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, grantID, family, k.kind, TokenHash(*k.out), joinScopes(scopes), now,
			now.Add(k.life)); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE oauth_grants SET last_used_at = ? WHERE id = ?`, now, grantID); err != nil {
		return nil, err
	}
	return t, nil
}

// PKCEMatches tells whether a code verifier hashes to the S256 challenge
// (RFC 7636 §4.6).
func PKCEMatches(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	return subtle.ConstantTimeCompare([]byte(base64.RawURLEncoding.EncodeToString(sum[:])), []byte(challenge)) == 1
}

// RevokeOAuthToken revokes a token a client holds (RFC 7009): an access
// token alone, a refresh token with its whole family. A token of another
// client, or an unknown one, is left alone.
func RevokeOAuthToken(ctx context.Context, db *sql.DB, token, clientID string) error {
	var id, family int64
	var kind string
	err := db.QueryRowContext(ctx, `SELECT t.id, t.family, t.kind FROM oauth_tokens t JOIN oauth_grants g ON g.id = t.grant_id
		WHERE t.token_hash = ? AND g.client_id = ?`, TokenHash(token), clientID).Scan(&id, &family, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if kind == oauthRefresh {
		_, err = db.ExecContext(ctx, `DELETE FROM oauth_tokens WHERE family = ?`, family)
	} else {
		_, err = db.ExecContext(ctx, `DELETE FROM oauth_tokens WHERE id = ?`, id)
	}
	return err
}

// LooksLikeOAuthToken tells whether a secret has the shape of an OAuth
// access token.
func LooksLikeOAuthToken(s string) bool {
	return strings.HasPrefix(s, OAuthAccessPrefix) && len(s) > len(OAuthAccessPrefix)+20
}

// LookupOAuthToken returns the active user behind a valid, unexpired OAuth
// access token, with the token's scopes, and records that the grant was used.
func LookupOAuthToken(ctx context.Context, db *sql.DB, token string) (*User, error) {
	if !LooksLikeOAuthToken(token) {
		return nil, ErrNoSession
	}
	var u User
	var lastUsed sql.NullTime
	var scopes string
	now := time.Now().UTC()
	err := db.QueryRowContext(ctx, `SELECT u.id, u.username, u.display_name, u.email, u.is_admin, COALESCE(u.locale, ''),
		u.directory, g.id, t.scopes, g.last_used_at
		FROM oauth_tokens t JOIN oauth_grants g ON g.id = t.grant_id JOIN users u ON u.id = g.user_id
		WHERE t.token_hash = ? AND t.kind = ? AND t.expires_at > ? AND u.active`, TokenHash(token), oauthAccess, now).
		Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.Admin, &u.Locale, &u.Directory, &u.GrantID, &scopes, &lastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, err
	}
	u.Scopes = ParseScopes(scopes)
	if !lastUsed.Valid || now.Sub(lastUsed.Time) > tokenTouchEvery {
		if _, err := db.ExecContext(ctx, `UPDATE oauth_grants SET last_used_at = ? WHERE id = ?`, now, u.GrantID); err != nil {
			return nil, err
		}
	}
	return &u, nil
}

// OAuthUnusedClientLifetime is how long a registered client that no user
// authorized is kept.
const OAuthUnusedClientLifetime = 24 * time.Hour

// PurgeOAuth deletes the expired authorization codes and OAuth tokens, and
// the registered clients nobody authorized in time.
func PurgeOAuth(ctx context.Context, db *sql.DB, now time.Time) error {
	now = now.UTC()
	if _, err := db.ExecContext(ctx, `DELETE FROM oauth_codes WHERE expires_at < ?`, now); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM oauth_tokens WHERE expires_at < ?`, now); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `DELETE FROM oauth_clients WHERE authorized_at IS NULL AND created_at < ?`,
		now.Add(-OAuthUnusedClientLifetime))
	return err
}
