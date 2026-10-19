package api

import (
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"time"

	"solstice/internal/i18n"
)

// What an administrator's invitation or reset link refuses, and in which
// order.
func TestAdminLinkRefusal(t *testing.T) {
	signedIn := sql.NullTime{Time: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC), Valid: true}
	for _, c := range []struct {
		name   string
		link   adminLink
		to     linkTarget
		status int
		key    i18n.Key
	}{
		{"invitation", inviteLink, linkTarget{Active: true, Email: "ana@example.org"}, 0, ""},
		{"reset", resetLink, linkTarget{Active: true, Email: "ana@example.org", LastLogin: signedIn}, 0, ""},
		{"inactive", resetLink, linkTarget{Email: "ana@example.org"}, http.StatusConflict, i18n.AccountInactive},
		{"inactive before signed in", inviteLink, linkTarget{Email: "ana@example.org", LastLogin: signedIn},
			http.StatusConflict, i18n.AccountInactive},
		{"invitation after signing in", inviteLink, linkTarget{Active: true, Email: "ana@example.org", LastLogin: signedIn},
			http.StatusConflict, i18n.AccountAlreadySignedIn},
		{"no address", resetLink, linkTarget{Active: true}, http.StatusBadRequest, i18n.AccountEmailMissing},
		{"not an address", inviteLink, linkTarget{Active: true, Email: "Ana"}, http.StatusBadRequest, i18n.AccountEmailMissing},
		{"address with spaces", resetLink, linkTarget{Active: true, Email: " ana@example.org "}, 0, ""},
	} {
		err := c.link.refusal(&c.to)
		var he *httpError
		switch {
		case c.status == 0 && err != nil:
			t.Errorf("%s: refused with %v", c.name, err)
		case c.status != 0 && (!errors.As(err, &he) || he.status != c.status || he.msg.Key != c.key):
			t.Errorf("%s: refusal = %v, want %d %s", c.name, err, c.status, c.key)
		}
	}
}
