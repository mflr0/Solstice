package directory

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"solstice/internal/auth"
	"solstice/internal/config"
)

func TestFiltersEscapeWhatIsTyped(t *testing.T) {
	for _, c := range []struct{ typed, want string }{
		{"marie", "(uid=marie)"},
		{"*", `(uid=\2a)`},
		{"a)(uid=*", `(uid=a\29\28uid=\2a)`},
		{`back\slash`, `(uid=back\5cslash)`},
	} {
		if got := UserFilter("(uid={username})", c.typed); got != c.want {
			t.Errorf("UserFilter(%q) = %s, want %s", c.typed, got, c.want)
		}
	}
	got := GroupFilter("(|(member={dn})(memberUid={username}))", "cn=a*b,dc=example", "a*b")
	if want := `(|(member=cn=a\2ab,dc=example)(memberUid=a\2ab))`; got != want {
		t.Errorf("GroupFilter = %s, want %s", got, want)
	}
}

func TestNew(t *testing.T) {
	if d, err := New(config.LDAPSettings{}); d != nil || err != nil {
		t.Errorf("off: %v, %v", d, err)
	}
	bad := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(bad, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, ca := range []string{bad, filepath.Join(t.TempDir(), "absent.pem")} {
		if _, err := New(config.LDAPSettings{URL: "ldaps://ldap.example", CAFile: ca}); err == nil {
			t.Errorf("CA file %s: no error", ca)
		}
	}
	d, err := New(config.LDAPSettings{URL: "ldaps://ldap.example:636"})
	if err != nil || d.tls.ServerName != "ldap.example" {
		t.Errorf("ldaps: %v, %+v", err, d)
	}
}

// An empty password is refused before any connection: the server would take
// it as an unauthenticated bind and accept it.
func TestEmptyPasswordNeverReachesTheServer(t *testing.T) {
	d, err := New(config.LDAPSettings{URL: "ldaps://192.0.2.1", Timeout: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range [][2]string{{"marie", ""}, {"", "pw"}} {
		if _, err := d.Authenticate(context.Background(), c[0], c[1]); !errors.Is(err, auth.ErrNoSession) {
			t.Errorf("%q/%q: %v, want ErrNoSession without dialling", c[0], c[1], err)
		}
	}
	if d.Name() != "ldap" {
		t.Errorf("Name = %s", d.Name())
	}
}
