package tests

import (
	"os"
	"slices"
	"testing"
	"time"

	"solstice/internal/config"
	"solstice/internal/directory"
)

// ldapServer returns the directory of the OpenLDAP server SOLSTICE_TEST_LDAP
// names (compose.yaml's ldap service, loaded with testdata/ldap), or skips.
func ldapServer(t *testing.T) *directory.LDAP {
	t.Helper()
	addr := os.Getenv("SOLSTICE_TEST_LDAP")
	if addr == "" {
		if os.Getenv("SOLSTICE_TEST_REQUIRE_DB") != "" {
			t.Fatal("SOLSTICE_TEST_REQUIRE_DB is set: SOLSTICE_TEST_LDAP must be set too")
		}
		t.Skip("SOLSTICE_TEST_LDAP not set: LDAP test skipped")
	}
	cfg := config.Defaults().LDAP
	cfg.URL, cfg.Insecure = addr, true
	cfg.BindDN, cfg.BindPassword = "cn=admin,dc=example,dc=org", "admin"
	cfg.UserBase, cfg.GroupBase = "ou=people,dc=example,dc=org", "ou=groups,dc=example,dc=org"
	cfg.RequireGroup, cfg.AdminGroup = "solstice-users", "Solstice-Admins"
	cfg.Timeout = 5 * time.Second
	d, err := directory.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestLDAPSignIn(t *testing.T) {
	dir := ldapServer(t)
	srv, base := newServer(t)
	srv.SetDirectory(dir)
	directoryUser(t, "ldap.ada", "solstice-users", "solstice-admins", "ldap-developers")
	directoryUser(t, "ldap.bob")

	for _, bad := range [][2]string{
		{"ldap.ada", "wrong"},
		{"ldap.ada", ""},
		{"ldap.*", "ada-ldap-password"},          // no wildcard in the filter
		{"ldap.bob", "bob-ldap-password"},        // not in solstice-users
		{"ldap.nobody", "ada-ldap-password"},     // not in the directory
		{"ldap.ada)(uid=*", "ada-ldap-password"}, // no filter injection
	} {
		// A server each: more than five failures would lock this address out.
		bs, bbase := newServer(t)
		bs.SetDirectory(dir)
		if status, _ := anonymousAt(t, bbase).do("POST", "/api/login",
			map[string]string{"username": bad[0], "password": bad[1]}); status != 401 {
			t.Errorf("%q/%q: %d, want 401", bad[0], bad[1], status)
		}
	}
	var n int
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM users WHERE username IN ('ldap.bob', 'ldap.nobody')`).Scan(&n); err != nil || n != 0 {
		t.Errorf("accounts created for refused sign-ins: %d %v", n, err)
	}

	c := loginAt(t, base, "LDAP.Ada", "ada-ldap-password")
	var me struct {
		Username, Name, Email, Directory string
		Admin                            bool
	}
	c.get("/api/me", &me)
	if me.Username != "ldap.ada" || me.Name != "Ada Lovelace" || me.Email != "ada@ldap.example" ||
		me.Directory != "ldap" || !me.Admin {
		t.Errorf("account: %+v", me)
	}
	want := []string{"ldap-developers*", "solstice-admins*", "solstice-users*"}
	if got := memberships(t, "ldap.ada"); !slices.Equal(got, want) {
		t.Errorf("groups: %v, want %v", got, want)
	}
}
