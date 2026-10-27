package tests

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"maps"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/config"
	"solstice/internal/directory"
)

// fakeProvider is an OpenID Connect provider that signs in whoever claims
// says, without asking: the browser goes straight back with a code.
type fakeProvider struct {
	t   *testing.T
	url string
	key *rsa.PrivateKey

	mu     sync.Mutex
	claims map[string]any            // the ID token's, over the standard ones; nil drops a claim
	signer *rsa.PrivateKey           // signs the ID token; nil for key
	deny   bool                      // the person refuses: error=access_denied
	codes  map[string]map[string]any // the authorization request of each code
}

const oidcClient, oidcSecret = "solstice-test", "test-secret"

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &fakeProvider{t: t, key: key, codes: map[string]map[string]any{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{"issuer": p.url, "authorization_endpoint": p.url + "/authorize",
			"token_endpoint": p.url + "/token", "jwks_uri": p.url + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"}})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "k1", "use": "sig",
			"alg": "RS256", "n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes())}}})
	})
	mux.HandleFunc("GET /authorize", p.authorize)
	mux.HandleFunc("POST /token", p.token)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	p.url = srv.URL
	return p
}

// sso is the provider as Solstice's configuration would set it up.
func (p *fakeProvider) sso(require, admin string) *directory.OIDC {
	cfg := config.Defaults().OIDC
	cfg.Issuer, cfg.ClientID, cfg.ClientSecret, cfg.Name = p.url, oidcClient, oidcSecret, "Test ID"
	cfg.RequireGroup, cfg.AdminGroup = require, admin
	return directory.NewOIDC(cfg)
}

func (p *fakeProvider) set(claims map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claims = claims
}

func (p *fakeProvider) setDeny(deny bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deny = deny
}

func (p *fakeProvider) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	back, err := url.Parse(q.Get("redirect_uri"))
	if err != nil || q.Get("client_id") != oidcClient || q.Get("response_type") != "code" ||
		q.Get("code_challenge_method") != "S256" || q.Get("nonce") == "" {
		p.t.Errorf("authorization request: %s", r.URL.RawQuery)
		http.Error(w, "bad request", 400)
		return
	}
	p.mu.Lock()
	deny := p.deny
	code := rand.Text()
	p.codes[code] = map[string]any{"nonce": q.Get("nonce"), "challenge": q.Get("code_challenge"),
		"redirect": q.Get("redirect_uri")}
	p.mu.Unlock()
	v := url.Values{"state": {q.Get("state")}}
	if deny {
		v.Set("error", "access_denied")
	} else {
		v.Set("code", code)
	}
	back.RawQuery = v.Encode()
	http.Redirect(w, r, back.String(), http.StatusFound)
}

func (p *fakeProvider) token(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.FormValue("client_id"), r.FormValue("client_secret")
	}
	p.mu.Lock()
	req, known := p.codes[r.FormValue("code")]
	delete(p.codes, r.FormValue("code"))
	claims, signer := p.claims, p.signer
	p.mu.Unlock()
	sum := sha256.Sum256([]byte(r.FormValue("code_verifier")))
	if id != oidcClient || secret != oidcSecret || !known || req["challenge"] != b64(sum[:]) ||
		req["redirect"] != r.FormValue("redirect_uri") {
		w.WriteHeader(400)
		writeTestJSON(w, map[string]string{"error": "invalid_grant"})
		return
	}
	now := time.Now()
	all := map[string]any{"iss": p.url, "aud": oidcClient, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix(),
		"nonce": req["nonce"]}
	maps.Copy(all, claims)
	maps.DeleteFunc(all, func(_ string, v any) bool { return v == nil })
	if signer == nil {
		signer = p.key
	}
	writeTestJSON(w, map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 60,
		"id_token": signJWT(p.t, signer, all)})
}

func signJWT(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"})
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	signed := b64(header) + "." + b64(payload)
	sum := sha256.Sum256([]byte(signed))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signed + "." + b64(sig)
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func writeTestJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// oidcServer starts a Solstice server signing in through p.
func oidcServer(t *testing.T, p *fakeProvider, require, admin string) (*api.Server, string) {
	t.Helper()
	srv, base := newServer(t)
	srv.SetBaseURL(base)
	srv.SetSingleSignOn(p.sso(require, admin))
	return srv, base
}

// ssoSignIn goes through the sign-in button with c, and returns where the
// browser lands.
func ssoSignIn(t *testing.T, c *client, next string) *url.URL {
	t.Helper()
	resp, err := c.http.Get(c.base + "/api/auth/oidc/start?next=" + url.QueryEscape(next))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.Request.URL
}

// wantRefused checks that a sign-in landed on the sign-in page with that
// error, and signed no one in.
func wantRefused(t *testing.T, c *client, at *url.URL, code string) {
	t.Helper()
	if at.Path != "/login" || at.Query().Get("error") != code {
		t.Errorf("landed on %s, want /login?error=%s", at, code)
	}
	if status := c.get("/api/me", nil); status != 401 {
		t.Errorf("/api/me after a refused sign-in: %d", status)
	}
}

type oidcMe struct {
	Username, Name, Email, Directory string
	Admin                            bool
}

func TestOIDCSignInCreatesAndSyncsTheAccount(t *testing.T) {
	p := newFakeProvider(t)
	_, base := oidcServer(t, p, "", "oidc-admins")
	directoryUser(t, "oidc.newcomer", "oidc-devs", "oidc-admins", "oidc-ops")

	var pub struct{ SSO *struct{ Name string } }
	anonymousAt(t, base).get("/api/settings/public", &pub)
	if pub.SSO == nil || pub.SSO.Name != "Test ID" {
		t.Errorf("public settings: sso %+v", pub.SSO)
	}

	p.set(map[string]any{"sub": "sub-newcomer", "preferred_username": " OIDC.Newcomer ", "name": "New Comer",
		"email": "newcomer@example.org", "email_verified": true, "groups": []string{"OIDC-Devs", "oidc-admins"}})
	c := anonymousAt(t, base)
	if at := ssoSignIn(t, c, "/browse/X-1"); at.Path != "/browse/X-1" {
		t.Errorf("landed on %s", at)
	}
	var me oidcMe
	c.get("/api/me", &me)
	if me != (oidcMe{"oidc.newcomer", "New Comer", "newcomer@example.org", "oidc", true}) {
		t.Errorf("created account: %+v", me)
	}
	if got := memberships(t, "oidc.newcomer"); !slices.Equal(got, []string{"oidc-admins*", "oidc-devs*"}) {
		t.Errorf("groups: %v", got)
	}

	// The subject finds the account, whatever user name the provider now
	// gives; an address the provider has not verified is not taken.
	p.set(map[string]any{"sub": "sub-newcomer", "preferred_username": "someone.else", "email": "new@example.org",
		"email_verified": false, "groups": "oidc-ops"})
	c = anonymousAt(t, base)
	if at := ssoSignIn(t, c, "//evil.example/"); at.String() != base+"/" {
		t.Errorf("next outside the site: landed on %s", at)
	}
	c.get("/api/me", &me)
	if me != (oidcMe{"oidc.newcomer", "New Comer", "newcomer@example.org", "oidc", false}) {
		t.Errorf("second sign-in: %+v", me)
	}
	if got := memberships(t, "oidc.newcomer"); !slices.Equal(got, []string{"oidc-ops*"}) {
		t.Errorf("groups after the second sign-in: %v", got)
	}

	// The account has no password: the form cannot sign it in.
	if status, _ := anonymousAt(t, base).do("POST", "/api/login",
		map[string]string{"username": "oidc.newcomer", "password": ""}); status != 401 {
		t.Errorf("password sign-in: %d", status)
	}
}

func TestOIDCLinksDirectoryAndImportedAccounts(t *testing.T) {
	p := newFakeProvider(t)
	_, base := oidcServer(t, p, "", "")
	directoryUser(t, "oidc.imported")
	directoryUser(t, "oidc.ldap")
	if _, err := testDB.Exec(`INSERT INTO users (username, display_name, email, directory) VALUES
		('oidc.imported', 'Imported', 'imported@example.org', ''), ('oidc.ldap', 'From LDAP', '', 'ldap')`); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ username, sub, directory string }{
		{"oidc.imported", "sub-imported", "oidc"},
		{"oidc.ldap", "sub-ldap", "ldap"}, // an LDAP account stays LDAP's, and also signs in here
	} {
		p.set(map[string]any{"sub": c.sub, "preferred_username": c.username})
		cl := anonymousAt(t, base)
		ssoSignIn(t, cl, "/")
		var me oidcMe
		cl.get("/api/me", &me)
		if me.Username != c.username || me.Directory != c.directory {
			t.Errorf("%s: signed in as %+v", c.username, me)
		}
		var issuer, subject string
		if err := testDB.QueryRow(`SELECT oidc_issuer, oidc_subject FROM users WHERE username = ?`, c.username).
			Scan(&issuer, &subject); err != nil || issuer != p.url || subject != c.sub {
			t.Errorf("%s: linked to %q %q (%v)", c.username, issuer, subject, err)
		}
	}

	// The account is linked to its subject: another one giving its name
	// does not sign in with it.
	p.set(map[string]any{"sub": "sub-impostor", "preferred_username": "oidc.imported"})
	c := anonymousAt(t, base)
	wantRefused(t, c, ssoSignIn(t, c, "/"), "sso-account")

	// Linked at another provider (the issuer changed): linked again by name.
	if _, err := testDB.Exec(`UPDATE users SET oidc_issuer = 'https://old.example' WHERE username = 'oidc.imported'`); err != nil {
		t.Fatal(err)
	}
	c = anonymousAt(t, base)
	ssoSignIn(t, c, "/")
	var me oidcMe
	c.get("/api/me", &me)
	var subject string
	if err := testDB.QueryRow(`SELECT oidc_subject FROM users WHERE username = 'oidc.imported'`).Scan(&subject); err != nil ||
		me.Username != "oidc.imported" || subject != "sub-impostor" {
		t.Errorf("after an issuer change: signed in as %+v, subject %q (%v)", me, subject, err)
	}
}

func TestOIDCLeavesLocalPasswordsAlone(t *testing.T) {
	p := newFakeProvider(t)
	_, base := oidcServer(t, p, "", "")
	p.set(map[string]any{"sub": "sub-admin", "preferred_username": "admin"})
	c := anonymousAt(t, base)
	wantRefused(t, c, ssoSignIn(t, c, "/"), "sso-account")
	var linked int
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM users WHERE oidc_subject = 'sub-admin'`).Scan(&linked); err != nil || linked != 0 {
		t.Errorf("linked accounts: %d %v", linked, err)
	}
}

func TestOIDCRefusesWhoIsNotAllowedIn(t *testing.T) {
	p := newFakeProvider(t)
	_, base := oidcServer(t, p, "staff", "")
	directoryUser(t, "oidc.outsider")
	directoryUser(t, "oidc.leaver", "staff")

	p.set(map[string]any{"sub": "sub-outsider", "preferred_username": "oidc.outsider", "groups": []string{"guests"}})
	c := anonymousAt(t, base)
	wantRefused(t, c, ssoSignIn(t, c, "/"), "sso-denied")
	var n int
	if testDB.QueryRow(`SELECT COUNT(*) FROM users WHERE username = 'oidc.outsider'`).Scan(&n); n != 0 {
		t.Error("an account was created for someone outside the required group")
	}

	// A deactivated account stays out.
	p.set(map[string]any{"sub": "sub-leaver", "preferred_username": "oidc.leaver", "groups": []string{"staff"}})
	c = anonymousAt(t, base)
	ssoSignIn(t, c, "/")
	if status := c.get("/api/me", nil); status != 200 {
		t.Fatalf("first sign-in: %d", status)
	}
	if _, err := testDB.Exec(`UPDATE users SET active = FALSE WHERE username = 'oidc.leaver'`); err != nil {
		t.Fatal(err)
	}
	c = anonymousAt(t, base)
	wantRefused(t, c, ssoSignIn(t, c, "/"), "sso-denied")

	// No user name, no account to create.
	p.set(map[string]any{"sub": "sub-nameless"})
	c = anonymousAt(t, base)
	wantRefused(t, c, ssoSignIn(t, c, "/"), "sso-denied")
}

func TestOIDCRejectsBadTokens(t *testing.T) {
	p := newFakeProvider(t)
	_, base := oidcServer(t, p, "", "")
	directoryUser(t, "oidc.forged")
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	good := map[string]any{"sub": "sub-forged", "preferred_username": "oidc.forged"}
	for name, change := range map[string]func(c map[string]any){
		"another audience": func(c map[string]any) { c["aud"] = "someone-else" },
		"another nonce":    func(c map[string]any) { c["nonce"] = "replayed" },
		"no nonce":         func(c map[string]any) { c["nonce"] = nil },
		"another issuer":   func(c map[string]any) { c["iss"] = "https://evil.example" },
		"expired":          func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() },
		"another key":      func(map[string]any) { p.signer = other },
	} {
		claims := maps.Clone(good)
		p.mu.Lock()
		change(claims)
		p.mu.Unlock()
		p.set(claims)
		c := anonymousAt(t, base)
		at := ssoSignIn(t, c, "/")
		p.mu.Lock()
		p.signer = nil
		p.mu.Unlock()
		if at.Query().Get("error") != "sso-failed" || c.get("/api/me", nil) != 401 {
			t.Errorf("%s: landed on %s", name, at)
		}
	}

	// The person refuses at the provider.
	p.set(good)
	p.setDeny(true)
	c := anonymousAt(t, base)
	wantRefused(t, c, ssoSignIn(t, c, "/"), "sso-failed")
	p.setDeny(false)

	// A callback this browser did not start (login CSRF) is refused, even
	// with a code the provider would take.
	victim, attacker := anonymousAt(t, base), anonymousAt(t, base)
	attacker.http.CheckRedirect = func(r *http.Request, _ []*http.Request) error {
		if r.URL.Path == "/api/auth/oidc/callback" {
			return http.ErrUseLastResponse
		}
		return nil
	}
	resp, err := attacker.http.Get(base + "/api/auth/oidc/start")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	callback, err := resp.Location()
	if err != nil {
		t.Fatal(err)
	}
	resp, err = victim.http.Get(callback.String())
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	wantRefused(t, victim, resp.Request.URL, "sso-failed")

	// Without a provider set up, the routes do not exist.
	if status, _ := anonymousAt(t, ownServer(t)).do("GET", "/api/auth/oidc/start", nil); status != 404 {
		t.Errorf("start without a provider: %d", status)
	}
}
