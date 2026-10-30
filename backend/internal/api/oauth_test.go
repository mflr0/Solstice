package api

import (
	"net/netip"
	"testing"

	"solstice/internal/auth"
)

func TestValidClientID(t *testing.T) {
	for id, want := range map[string]bool{
		"https://app.example.com/oauth/client.json": true,
		"https://app.example.com/c?v=1":             true,
		"http://app.example.com/client.json":        false,
		"https://app.example.com":                   false,
		"https://app.example.com/":                  false,
		"https://user@app.example.com/client.json":  false,
		"https://app.example.com/client.json#x":     false,
		"https://app.example.com/a/../client.json":  false,
		"https:///client.json":                      false,
		"app.example.com/client.json":               false,
	} {
		if got := validClientID(id); got != want {
			t.Errorf("validClientID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestAllowsRedirect(t *testing.T) {
	doc := &oauthClientDoc{RedirectURIs: []string{"https://app.example.com/cb", "http://127.0.0.1:3000/callback",
		"http://localhost/callback"}}
	for uri, want := range map[string]bool{
		"https://app.example.com/cb":       true,
		"https://app.example.com/cb2":      false,
		"https://app.example.com:8443/cb":  false,
		"http://127.0.0.1:3000/callback":   true,
		"http://127.0.0.1:54321/callback":  true,
		"http://localhost:6000/callback":   true,
		"http://127.0.0.1:3000/other":      false,
		"http://localhost:6000/callback?x": false,
		"http://evil.example/callback":     false,
		"https://127.0.0.1:3000/callback":  false,
	} {
		if got := doc.allowsRedirect(uri); got != want {
			t.Errorf("allowsRedirect(%q) = %v, want %v", uri, got, want)
		}
	}
}

func TestSameResource(t *testing.T) {
	want := "https://solstice.example.com/mcp"
	for got, ok := range map[string]bool{
		"https://solstice.example.com/mcp":   true,
		"HTTPS://Solstice.Example.com/mcp":   true,
		"https://solstice.example.com/mcp/":  true,
		"https://solstice.example.com":       false,
		"https://solstice.example.com/api":   false,
		"https://other.example.com/mcp":      false,
		"https://solstice.example.com/mcp#":  false,
		"https://solstice.example.com/mcp#x": false,
		"":                                   false,
	} {
		if sameResource(got, want) != ok {
			t.Errorf("sameResource(%q) = %v, want %v", got, !ok, ok)
		}
	}
}

func TestPublicAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"93.184.216.34":   true,
		"2606:2800::1":    true,
		"127.0.0.1":       false,
		"::1":             false,
		"10.1.2.3":        false,
		"192.168.43.160":  false,
		"172.16.0.1":      false,
		"100.64.0.1":      false,
		"169.254.169.254": false,
		"fd12::1":         false,
		"0.0.0.0":         false,
		"::ffff:10.0.0.1": false,
	} {
		if got := publicAddr(netip.MustParseAddr(addr)); got != want {
			t.Errorf("publicAddr(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestPKCEMatches(t *testing.T) {
	// RFC 7636 Appendix B.
	verifier, challenge := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if !auth.PKCEMatches(verifier, challenge) {
		t.Error("RFC 7636 example refused")
	}
	if auth.PKCEMatches(verifier[:42]+"x", challenge) || auth.PKCEMatches("short", challenge) {
		t.Error("wrong verifier accepted")
	}
}

func TestGrantableScopes(t *testing.T) {
	for _, sc := range grantableScopes() {
		if sc == auth.ScopeAdmin {
			t.Fatal("admin is grantable")
		}
	}
	if len(grantableScopes()) != len(auth.Scopes)-1 {
		t.Errorf("grantable: %v", grantableScopes())
	}
}
