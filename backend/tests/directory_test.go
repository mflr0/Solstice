package tests

import (
	"context"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	"solstice/internal/auth"
)

// fakeDirectory is a directory of one person, ldap.<suffix> with password pw.
type fakeDirectory struct {
	username string
	entry    auth.DirectoryEntry
	calls    atomic.Int32
}

func (d *fakeDirectory) Name() string { return "ldap" }

func (d *fakeDirectory) Authenticate(_ context.Context, username, password string) (*auth.DirectoryEntry, error) {
	d.calls.Add(1)
	if username != d.username || password != "pw" {
		return nil, auth.ErrNoSession
	}
	e := d.entry
	return &e, nil
}

// directoryUser removes, after the test, the account of that name and the
// groups named.
func directoryUser(t *testing.T, username string, groups ...string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := testDB.ExecContext(ctx, `DELETE FROM users WHERE username = ?`, username); err != nil {
			t.Error(err)
		}
		for _, g := range groups {
			if _, err := testDB.ExecContext(ctx, `DELETE FROM user_groups WHERE name = ?`, g); err != nil {
				t.Error(err)
			}
		}
	})
}

// memberships returns the groups of an account, each "name" or "name*" for a
// group of the directory.
func memberships(t *testing.T, username string) []string {
	t.Helper()
	var out []string
	rows, err := testDB.Query(`SELECT g.name, g.from_directory FROM group_members m
		JOIN user_groups g ON g.id = m.group_id JOIN users u ON u.id = m.user_id
		WHERE u.username = ? ORDER BY g.name`, username)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var dir bool
		if err := rows.Scan(&name, &dir); err != nil {
			t.Fatal(err)
		}
		if dir {
			name += "*"
		}
		out = append(out, name)
	}
	return out
}

func TestDirectorySignInCreatesAndSyncsTheAccount(t *testing.T) {
	srv, base := newServer(t)
	yes := true
	dir := &fakeDirectory{username: "ldap.newcomer", entry: auth.DirectoryEntry{Name: "New Comer",
		Email: "newcomer@example.org", Groups: []string{"LDAP-Devs", "ldap-ops"}, Admin: &yes}}
	srv.SetDirectory(dir)
	directoryUser(t, "ldap.newcomer", "ldap-devs", "ldap-ops", "ldap-local")

	if status, _ := anonymousAt(t, base).do("POST", "/api/login",
		map[string]string{"username": "ldap.newcomer", "password": "wrong"}); status != 401 {
		t.Fatalf("wrong password: %d", status)
	}
	c := loginAt(t, base, " LDAP.Newcomer ", "pw")
	var me struct {
		Username, Name, Email, Directory string
		Admin                            bool
	}
	c.get("/api/me", &me)
	if me.Username != "ldap.newcomer" || me.Name != "New Comer" || me.Email != "newcomer@example.org" ||
		me.Directory != "ldap" || !me.Admin {
		t.Errorf("created account: %+v", me)
	}
	if got := memberships(t, "ldap.newcomer"); !slices.Equal(got, []string{"ldap-devs*", "ldap-ops*"}) {
		t.Errorf("groups after the first sign-in: %v", got)
	}

	// A group set by hand stays; a directory group the person left goes;
	// the administrator right follows the directory.
	if _, err := testDB.Exec(`INSERT INTO user_groups (name) VALUES ('ldap-local')`); err != nil {
		t.Fatal(err)
	}
	if _, err := testDB.Exec(`INSERT INTO group_members (group_id, user_id)
		SELECT g.id, u.id FROM user_groups g, users u WHERE g.name = 'ldap-local' AND u.username = 'ldap.newcomer'`); err != nil {
		t.Fatal(err)
	}
	no := false
	dir.entry = auth.DirectoryEntry{Groups: []string{"ldap-ops"}, Admin: &no}
	c = loginAt(t, base, "ldap.newcomer", "pw")
	c.get("/api/me", &me)
	if me.Admin || me.Name != "New Comer" {
		t.Errorf("second sign-in: %+v (admin removed, name kept when the directory gives none)", me)
	}
	if got := memberships(t, "ldap.newcomer"); !slices.Equal(got, []string{"ldap-local", "ldap-ops*"}) {
		t.Errorf("groups after the second sign-in: %v", got)
	}
}

func TestDirectoryLinksAnImportedAccount(t *testing.T) {
	srv, base := newServer(t)
	dir := &fakeDirectory{username: "ldap.imported", entry: auth.DirectoryEntry{Name: "Imported Person"}}
	srv.SetDirectory(dir)
	directoryUser(t, "ldap.imported")
	if _, err := testDB.Exec(`INSERT INTO users (username, display_name, email) VALUES ('ldap.imported', 'Old', 'old@example.org')`); err != nil {
		t.Fatal(err)
	}
	c := loginAt(t, base, "ldap.imported", "pw")
	var me struct{ Name, Email, Directory string }
	c.get("/api/me", &me)
	if me.Directory != "ldap" || me.Name != "Imported Person" || me.Email != "old@example.org" {
		t.Errorf("linked account: %+v", me)
	}
}

func TestDirectoryLeavesLocalPasswordsAlone(t *testing.T) {
	srv, base := newServer(t)
	dir := &fakeDirectory{username: "admin"}
	srv.SetDirectory(dir)
	loginAt(t, base, "admin", "admin")
	// A local account with a password never asks the directory, even when
	// the directory knows the name: the directory cannot take it over.
	if status, _ := anonymousAt(t, base).do("POST", "/api/login",
		map[string]string{"username": "admin", "password": "pw"}); status != 401 {
		t.Errorf("directory password on a local account: %d", status)
	}
	if n := dir.calls.Load(); n != 0 {
		t.Errorf("the directory was asked %d times", n)
	}
}

func TestDirectoryAccountHasNoPasswordHere(t *testing.T) {
	srv, base := newServer(t)
	srv.SetDirectory(&fakeDirectory{username: "ldap.nopw"})
	directoryUser(t, "ldap.nopw")
	self := loginAt(t, base, "ldap.nopw", "pw")
	var id int64
	if err := testDB.QueryRow(`SELECT id FROM users WHERE username = 'ldap.nopw'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	path := "/api/admin/users/" + strconv.FormatInt(id, 10)
	admin := loginAt(t, base, "admin", "admin")
	var u struct{ Directory string }
	admin.call("GET", path, nil, 200, &u)
	if u.Directory != "ldap" {
		t.Errorf("admin view: directory %q", u.Directory)
	}
	for _, c := range []struct {
		who          *client
		method, path string
		body         any
	}{
		{admin, "POST", path + "/reset-password", nil},
		{admin, "PATCH", path, map[string]string{"password": "a long new password 1"}},
		{self, "PATCH", "/api/me", map[string]string{"currentPassword": "pw", "newPassword": "a long new password 1"}},
	} {
		if status, body := c.who.do(c.method, c.path, c.body); status != 409 {
			t.Errorf("%s %s: %d %s, want 409", c.method, c.path, status, body)
		}
	}
	var hash *string
	if err := testDB.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, id).Scan(&hash); err != nil || hash != nil {
		t.Errorf("password_hash: %v %v", hash, err)
	}

	// Without the directory, the account cannot sign in at all.
	if status, _ := anonymousAt(t, ownServer(t)).do("POST", "/api/login",
		map[string]string{"username": "ldap.nopw", "password": "pw"}); status != 401 {
		t.Errorf("directory off: %d", status)
	}
}
