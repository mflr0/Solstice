package auth

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
)

// accounts is a users table: user name to email address, both lower case.
type accounts map[string]string

func (a accounts) Connect(context.Context) (driver.Conn, error) { return accountsConn{a}, nil }
func (a accounts) Driver() driver.Driver                        { return nil }

type accountsConn struct{ users accounts }

func (accountsConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (accountsConn) Close() error                        { return nil }
func (accountsConn) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }

func (c accountsConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	typed := args[0].Value.(string)
	var found []string
	for name, email := range c.users {
		if strings.Contains(query, "WHERE username") && name == typed ||
			strings.Contains(query, "WHERE email_key") && email == typed {
			found = append(found, name)
		}
	}
	return &nameRows{found}, nil
}

type nameRows struct{ names []string }

func (*nameRows) Columns() []string { return []string{"username"} }
func (*nameRows) Close() error      { return nil }
func (r *nameRows) Next(dest []driver.Value) error {
	if len(r.names) == 0 {
		return io.EOF
	}
	dest[0], r.names = r.names[0], r.names[1:]
	return nil
}

// A user name that is another account's address names its own account.
func TestResolveLoginUsernameWins(t *testing.T) {
	db := sql.OpenDB(accounts{"alice": "alice@example.org", "alice@example.org": "", "bob": "bob@example.org"})
	for typed, want := range map[string]string{
		"Alice@Example.org":  "alice@example.org",
		"alice":              "alice",
		" Bob@Example.org ":  "bob",
		"nobody@example.org": "nobody@example.org",
	} {
		got, err := ResolveLogin(context.Background(), db, typed)
		if err != nil || got != want {
			t.Errorf("ResolveLogin(%q) = %q, %v; want %q", typed, got, err, want)
		}
	}
}
