package directory

import (
	"reflect"
	"testing"

	"solstice/internal/auth"
	"solstice/internal/config"
)

func TestIdentityClaims(t *testing.T) {
	cfg := config.Defaults().OIDC
	cfg.UsernameClaim, cfg.GroupsClaim = "upn", "roles"
	for _, c := range []struct {
		claims map[string]any
		want   auth.Identity
	}{
		{map[string]any{"upn": " Marie@Example.org ", "name": "Marie", "email": "marie@example.org",
			"email_verified": true, "roles": []any{"Dev", "", 3, "ops"}},
			auth.Identity{Username: "marie@example.org", DirectoryEntry: auth.DirectoryEntry{Name: "Marie",
				Email: "marie@example.org", Groups: []string{"Dev", "ops"}}}},
		// Some providers send email_verified as a string, or no such claim.
		{map[string]any{"email": "a@example.org", "email_verified": "TRUE", "roles": "Domain Users"},
			auth.Identity{DirectoryEntry: auth.DirectoryEntry{Email: "a@example.org", Groups: []string{"Domain Users"}}}},
		{map[string]any{"email": "b@example.org"},
			auth.Identity{DirectoryEntry: auth.DirectoryEntry{Email: "b@example.org"}}},
		// An address the provider has not verified is not taken.
		{map[string]any{"email": "c@example.org", "email_verified": false, "preferred_username": "c"},
			auth.Identity{}},
		{map[string]any{"email": "d@example.org", "email_verified": "false", "upn": 42}, auth.Identity{}},
	} {
		if got := identity(c.claims, cfg); !reflect.DeepEqual(*got, c.want) {
			t.Errorf("%v:\n got %+v\nwant %+v", c.claims, *got, c.want)
		}
	}
}

func TestNewOIDCOff(t *testing.T) {
	if NewOIDC(config.Defaults().OIDC) != nil {
		t.Error("OpenID Connect on without an issuer")
	}
}
