package devseed

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"solstice/internal/audit"
	"solstice/internal/auth"
)

// User is a mock account.
type User struct {
	Username, Name string
	Locale         string // "" follows the browser
}

// Users are the mock accounts, one per interface language and one without.
var Users = []User{
	{"alice", "Alice Martin", "en"},
	{"marie", "Marie Dubois", "fr"},
	{"hans", "Hans Müller", "de"},
	{"giulia", "Giulia Rossi", "it"},
	{"priya", "Priya Sharma", "hi"},
	{"tom", "Tom Baker", ""},
}

// createAdmin writes the administrator directly, as "solstice user add"
// does: the API needs an account to sign in with.
func createAdmin(ctx context.Context, conn *sql.DB) error {
	hash, err := auth.HashPassword(AdminPassword)
	if err != nil {
		return err
	}
	res, err := conn.ExecContext(ctx, `INSERT INTO users (username, display_name, email, password_hash, is_admin)
		VALUES (?, ?, ?, ?, TRUE)`, Admin, "Seed Admin", Admin+"@example.org", hash)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for _, action := range []string{audit.UserCreate, audit.AdminGrant} {
		if err := audit.Record(ctx, conn, audit.Entry{Action: action,
			Target:  audit.Target{Type: audit.TargetUser, ID: strconv.FormatInt(id, 10), Label: Admin},
			Details: map[string]any{"via": "dev seed"}}); err != nil {
			return err
		}
	}
	return nil
}

// ensureUser creates u through the administration and sets its language as
// the person would, unless the account exists.
func ensureUser(ctx context.Context, conn *sql.DB, admin *client, u User) (id int64, created bool, err error) {
	err = conn.QueryRowContext(ctx, `SELECT id FROM users WHERE username = ?`, u.Username).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	var out struct{ ID int64 }
	if err := admin.call("POST", "/api/admin/users", map[string]any{"username": u.Username, "name": u.Name,
		"email": u.Username + "@example.org", "password": UserPassword, "admin": false}, &out); err != nil {
		return 0, false, err
	}
	if u.Locale != "" {
		c, err := login(ctx, admin.base, u.Username, UserPassword)
		if err != nil {
			return 0, false, err
		}
		if err := c.call("PATCH", "/api/me", map[string]any{"locale": u.Locale}, nil); err != nil {
			return 0, false, err
		}
	}
	return out.ID, true, nil
}
