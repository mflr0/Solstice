package auth

import (
	"context"
	"database/sql"
	"errors"
)

// OIDCName is what users.directory holds for an account an OpenID Connect
// sign-in created.
const OIDCName = "oidc"

// SingleSignOn signs people in through an OpenID Connect provider: the
// browser is sent to AuthURL, and comes back with a code for Exchange.
type SingleSignOn interface {
	// Name is the provider as the sign-in button names it.
	Name() string
	// AuthURL is where the browser signs in. redirectURI is where it comes
	// back; state, nonce and verifier (PKCE) are fresh random values.
	AuthURL(ctx context.Context, redirectURI, state, nonce, verifier string) (string, error)
	// Exchange trades the code the browser came back with for the person's
	// identity, checking the ID token and its nonce. ErrNoSession, with the
	// identity: the person is not allowed in (not in the required group).
	Exchange(ctx context.Context, redirectURI, code, verifier, nonce string) (*Identity, error)
}

// Identity is a person as an OpenID Connect provider vouches for them.
type Identity struct {
	Issuer  string
	Subject string // the provider's stable identifier of the person
	// Username is the user name the provider gives, canonical
	// (CanonicalUsername); it finds the account at the first sign-in only.
	Username string
	DirectoryEntry
}

// ErrAccountTaken means the user name of an identity belongs to an account
// that is not to be linked to it: one with a local password, or one linked
// to another identity of the same provider.
var ErrAccountTaken = errors.New("the account of this user name cannot sign in with OpenID Connect")

// IdentitySignIn signs in the account linked to id's issuer and subject. At
// the first sign-in, that is the account of id's user name, when it belongs
// to a directory or has no password, or a new one; it is linked then. Name,
// address, administrator right and groups are copied as for a directory
// (saveEntry). A deactivated account: ErrNoSession.
func IdentitySignIn(ctx context.Context, db *sql.DB, id *Identity) (*User, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var userID int64
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT id, active FROM users WHERE oidc_issuer = ? AND oidc_subject = ? FOR UPDATE`,
		id.Issuer, id.Subject).Scan(&userID, &active)
	switch {
	case err == nil:
		if !active {
			return nil, ErrNoSession
		}
	case errors.Is(err, sql.ErrNoRows):
		if userID, err = linkIdentity(ctx, tx, id); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	if userID, err = saveEntry(ctx, tx, userID, OIDCName, id.Username, &id.DirectoryEntry); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET oidc_issuer = ?, oidc_subject = ? WHERE id = ?`,
		id.Issuer, id.Subject, userID); err != nil {
		return nil, err
	}
	return commitUser(ctx, tx, userID)
}

// linkIdentity returns the account an identity signs in with for the first
// time: the one of its user name, 0 when there is none (saveEntry creates
// it).
func linkIdentity(ctx context.Context, tx *sql.Tx, id *Identity) (int64, error) {
	if id.Username == "" {
		return 0, ErrNoSession
	}
	var userID int64
	var active, password, linked bool
	var directory string
	// An account linked at another issuer (the provider changed) is linked
	// again.
	err := tx.QueryRowContext(ctx, `SELECT id, active, directory, password_hash IS NOT NULL,
		oidc_subject IS NOT NULL AND oidc_issuer = ?
		FROM users WHERE username = ? FOR UPDATE`, id.Issuer, id.Username).Scan(&userID, &active, &directory, &password, &linked)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, nil
	case err != nil:
		return 0, err
	case !active:
		return 0, ErrNoSession
	case linked || (directory == "" && password):
		return 0, ErrAccountTaken
	}
	return userID, nil
}
