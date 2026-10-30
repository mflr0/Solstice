package tests

// Tests of the OAuth authorization server of the MCP endpoint. They need
// SOLSTICE_TEST_DB and are skipped without it.

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"solstice/internal/auth"
	"solstice/internal/jirafixture"
)

// oauthEnv is a server of its own whose client metadata documents come from
// a local HTTPS server, which the real fetcher would refuse as private.
type oauthEnv struct {
	t        *testing.T
	base     string
	clientID string
	redirect string
	resource string
}

func newOAuthEnv(t *testing.T) *oauthEnv {
	t.Helper()
	srv, base := newServer(t)
	e := &oauthEnv{t: t, base: base, redirect: "http://127.0.0.1:9/callback", resource: base + "/mcp"}
	docs := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/client.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"client_id": e.clientID, "client_name": "Agent de test",
			"redirect_uris": []string{e.redirect}, "token_endpoint_auth_method": "none"})
	}))
	t.Cleanup(docs.Close)
	e.clientID = docs.URL + "/client.json"
	srv.TestHooks().SetOAuthFetcher(docs.Client())
	return e
}

// pkce returns a code verifier and its S256 challenge.
func pkce(seed string) (verifier, challenge string) {
	verifier = strings.Repeat(seed, 50)[:64]
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

// request is the query of an authorization request; set changes it.
func (e *oauthEnv) request(challenge string, set map[string]string) string {
	q := url.Values{"response_type": {"code"}, "client_id": {e.clientID}, "redirect_uri": {e.redirect},
		"code_challenge": {challenge}, "code_challenge_method": {"S256"}, "resource": {e.resource}, "state": {"xyz"}}
	for k, v := range set {
		if v == "" {
			q.Del(k)
		} else {
			q.Set(k, v)
		}
	}
	return q.Encode()
}

// authorize ticks scopes and allows; it returns the code.
func (e *oauthEnv) authorize(c *client, query string, scopes ...string) string {
	e.t.Helper()
	var out struct{ Redirect string }
	c.call("POST", "/api/oauth/consent", map[string]any{"request": query, "allow": true, "scopes": scopes}, 200, &out)
	u, err := url.Parse(out.Redirect)
	if err != nil || !strings.HasPrefix(out.Redirect, e.redirect+"?") {
		e.t.Fatalf("redirect %q", out.Redirect)
	}
	q := u.Query()
	if q.Get("state") != "xyz" || q.Get("iss") != e.base || q.Get("code") == "" {
		e.t.Fatalf("redirect %q", out.Redirect)
	}
	return q.Get("code")
}

type tokenAnswer struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string
	Error        string
}

// token posts to the token endpoint.
func (e *oauthEnv) token(form url.Values) (int, tokenAnswer) {
	e.t.Helper()
	resp, err := http.PostForm(e.base+"/oauth/token", form)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out tokenAnswer
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (e *oauthEnv) exchange(code, verifier string, set map[string]string) (int, tokenAnswer) {
	e.t.Helper()
	f := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {e.clientID},
		"redirect_uri": {e.redirect}, "code_verifier": {verifier}, "resource": {e.resource}}
	for k, v := range set {
		if v == "" {
			f.Del(k)
		} else {
			f.Set(k, v)
		}
	}
	return e.token(f)
}

func (e *oauthEnv) refresh(token string) (int, tokenAnswer) {
	e.t.Helper()
	return e.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {token}, "client_id": {e.clientID},
		"resource": {e.resource}})
}

// mcpSignedIn tells whether /mcp takes a bearer token: it answers 401
// otherwise.
func (e *oauthEnv) mcpSignedIn(token string) bool {
	e.t.Helper()
	status, _, _ := call(e.t, nil, e.base, "POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"},
		func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("Accept", "application/json, text/event-stream")
		})
	return status != 401
}

type grantRow struct {
	ID         int64
	ClientID   string
	ClientName string
	Scopes     []string
}

// grant is the user's grant to the test client, as the account lists it.
func (e *oauthEnv) grant(c *client) grantRow {
	e.t.Helper()
	var grants []grantRow
	c.call("GET", "/api/me/oauth-grants", nil, 200, &grants)
	i := slices.IndexFunc(grants, func(g grantRow) bool { return g.ClientID == e.clientID })
	if i < 0 {
		e.t.Fatalf("no grant to %s: %+v", e.clientID, grants)
	}
	return grants[i]
}

func TestOAuthMetadata(t *testing.T) {
	e := newOAuthEnv(t)
	var meta map[string]any
	anonymousAt(t, e.base).call("GET", "/.well-known/oauth-authorization-server", nil, 200, &meta)
	if meta["issuer"] != e.base || meta["token_endpoint"] != e.base+"/oauth/token" ||
		meta["authorization_endpoint"] != e.base+"/oauth/authorize" ||
		fmt.Sprint(meta["code_challenge_methods_supported"]) != "[S256]" ||
		meta["client_id_metadata_document_supported"] != true ||
		meta["authorization_response_iss_parameter_supported"] != true {
		t.Errorf("metadata: %v", meta)
	}
	if strings.Contains(fmt.Sprint(meta["scopes_supported"]), "admin") {
		t.Errorf("admin offered: %v", meta["scopes_supported"])
	}
	var prm struct {
		AuthorizationServers []string `json:"authorization_servers"`
	}
	anonymousAt(t, e.base).call("GET", "/.well-known/oauth-protected-resource", nil, 200, &prm)
	if !slices.Equal(prm.AuthorizationServers, []string{e.base}) {
		t.Errorf("protected resource metadata names %v", prm.AuthorizationServers)
	}
}

func TestOAuthFlow(t *testing.T) {
	e := newOAuthEnv(t)
	admin := loginAt(t, e.base, "admin", "admin")
	t.Cleanup(func() { testDB.Exec(`DELETE FROM oauth_grants WHERE client_id = ?`, e.clientID) })
	verifier, challenge := pkce("abcdefgh")

	// The consent page learns what is asked: admin is never offered, read
	// always is.
	query := e.request(challenge, map[string]string{"scope": "issues:write comments:write admin"})
	var check struct {
		Client       struct{ Name, Host string }
		RedirectHost string
		Scopes       []string
	}
	admin.call("GET", "/api/oauth/authorize?"+query, nil, 200, &check)
	if check.Client.Name != "Agent de test" || check.RedirectHost != "127.0.0.1:9" ||
		!slices.Equal(check.Scopes, []string{"read", "issues:write", "comments:write"}) {
		t.Fatalf("check: %+v", check)
	}

	// Only the ticked scopes are granted: comments:write is unticked.
	status, tok := e.exchange(e.authorize(admin, query, "issues:write"), verifier, nil)
	if status != 200 || tok.TokenType != "Bearer" || tok.ExpiresIn != 3600 || tok.Scope != "read issues:write" ||
		!strings.HasPrefix(tok.AccessToken, auth.OAuthAccessPrefix) || tok.RefreshToken == "" {
		t.Fatalf("token: %d %+v", status, tok)
	}
	tools := mcpToolNames(t, mcpSessionAt(t, e.base, tok.AccessToken))
	if !slices.Contains(tools, "create_issue") || slices.Contains(tools, "add_comment") || !slices.Contains(tools, "get_issue") {
		t.Errorf("tools of read issues:write: %v", tools)
	}

	// The token works on /mcp only.
	if status, _, body := call(t, nil, e.base, "GET", "/api/me", nil, bearer(tok.AccessToken)); status != 401 {
		t.Errorf("/api/me with an OAuth token: %d %s", status, body)
	}
	if status, _, body := call(t, nil, e.base, "GET", "/rest/api/2/myself", nil, bearer(tok.AccessToken)); status != 401 {
		t.Errorf("/rest with an OAuth token: %d %s", status, body)
	}
	if status, _, body := call(t, nil, e.base, "GET", "/rest/api/2/myself", nil, basic("admin", tok.AccessToken)); status != 401 {
		t.Errorf("/rest with an OAuth token as password: %d %s", status, body)
	}

	// The account lists the grant with its scopes; the audit log has it.
	if g := e.grant(admin); g.ClientName != "Agent de test" || !slices.Equal(g.Scopes, []string{"read", "issues:write"}) {
		t.Fatalf("grant: %+v", g)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'oauth.grant' AND target_id = ?`, e.clientID); n == "0" {
		t.Error("no oauth.grant audit entry")
	}

	// A refresh token works once; used again, it revokes what it gave.
	status, fresh := e.refresh(tok.RefreshToken)
	if status != 200 || fresh.AccessToken == "" || fresh.RefreshToken == tok.RefreshToken || fresh.Scope != "read issues:write" {
		t.Fatalf("refresh: %d %+v", status, fresh)
	}
	if !e.mcpSignedIn(fresh.AccessToken) {
		t.Error("refreshed token refused on /mcp")
	}
	if status, again := e.refresh(tok.RefreshToken); status != 400 || again.Error != "invalid_grant" {
		t.Errorf("refresh token used twice: %d %+v", status, again)
	}
	if e.mcpSignedIn(fresh.AccessToken) {
		t.Error("token of a replayed refresh still works")
	}
	if status, _ := e.refresh(fresh.RefreshToken); status != 400 {
		t.Errorf("refresh token of a replayed family still works: %d", status)
	}

	// Revoking the grant stops its tokens.
	_, tok = e.exchange(e.authorize(admin, query, "issues:write"), verifier, nil)
	if !e.mcpSignedIn(tok.AccessToken) {
		t.Fatal("new token refused")
	}
	admin.call("DELETE", "/api/me/oauth-grants/"+strconv.FormatInt(e.grant(admin).ID, 10), nil, 204, nil)
	if e.mcpSignedIn(tok.AccessToken) {
		t.Error("token of a revoked grant still works")
	}
	if status, _ := e.refresh(tok.RefreshToken); status != 400 {
		t.Errorf("refresh of a revoked grant: %d", status)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'oauth.revoke' AND target_id = ?`, e.clientID); n == "0" {
		t.Error("no oauth.revoke audit entry")
	}

	// With no scope asked, every scope but admin is offered.
	admin.call("GET", "/api/oauth/authorize?"+e.request(challenge, nil), nil, 200, &check)
	if want := []string{"read", "issues:write", "comments:write", "worklogs:write", "agile:write", "filters:write",
		"projects:write", "profile:write"}; !slices.Equal(check.Scopes, want) {
		t.Errorf("offered without scope: %v, want %v", check.Scopes, want)
	}
}

func TestOAuthRefusals(t *testing.T) {
	e := newOAuthEnv(t)
	admin := loginAt(t, e.base, "admin", "admin")
	t.Cleanup(func() { testDB.Exec(`DELETE FROM oauth_grants WHERE client_id = ?`, e.clientID) })
	verifier, challenge := pkce("ijklmnop")
	query := e.request(challenge, nil)

	// A wrong verifier fails, and uses the code up.
	code := e.authorize(admin, query)
	wrong, _ := pkce("qrstuvwx")
	if status, tok := e.exchange(code, wrong, nil); status != 400 || tok.Error != "invalid_grant" {
		t.Errorf("wrong verifier: %d %+v", status, tok)
	}
	if status, _ := e.exchange(code, verifier, nil); status != 400 {
		t.Errorf("code after a wrong verifier: %d", status)
	}

	// A code works once; used again, it revokes the tokens it gave.
	code = e.authorize(admin, query)
	status, tok := e.exchange(code, verifier, nil)
	if status != 200 {
		t.Fatalf("exchange: %d %+v", status, tok)
	}
	if status, again := e.exchange(code, verifier, nil); status != 400 || again.Error != "invalid_grant" {
		t.Errorf("reused code: %d %+v", status, again)
	}
	if e.mcpSignedIn(tok.AccessToken) {
		t.Error("token of a reused code still works")
	}

	// A token request without resource, or for another one, fails.
	for _, resource := range []string{"", e.base + "/api"} {
		code = e.authorize(admin, query)
		if status, tok := e.exchange(code, verifier, map[string]string{"resource": resource}); status != 400 || tok.Error != "invalid_target" {
			t.Errorf("token request with resource %q: %d %+v", resource, status, tok)
		}
	}
	// So does a code presented with another redirect URI or client.
	code = e.authorize(admin, query)
	if status, _ := e.exchange(code, verifier, map[string]string{"redirect_uri": "http://127.0.0.1:9/other"}); status != 400 {
		t.Errorf("token request with another redirect URI: %d", status)
	}
	code = e.authorize(admin, query)
	if status, _ := e.exchange(code, verifier, map[string]string{"client_id": "https://other.example/client.json"}); status != 400 {
		t.Errorf("token request by another client: %d", status)
	}

	// Authorization requests: without resource, PKCE or with an unknown
	// scope, the page offers to go back with the error; with a redirect URI
	// the document does not list, or a client_id that is no HTTPS URL, it
	// must not.
	for name, tc := range map[string]struct {
		set      map[string]string
		error    string
		redirect bool
	}{
		"no resource":      {map[string]string{"resource": ""}, "invalid_target", true},
		"other resource":   {map[string]string{"resource": e.base + "/api"}, "invalid_target", true},
		"no PKCE":          {map[string]string{"code_challenge": "", "code_challenge_method": ""}, "invalid_request", true},
		"plain PKCE":       {map[string]string{"code_challenge_method": "plain"}, "invalid_request", true},
		"unknown scope":    {map[string]string{"scope": "read everything"}, "invalid_scope", true},
		"token response":   {map[string]string{"response_type": "token"}, "unsupported_response_type", true},
		"other redirect":   {map[string]string{"redirect_uri": "http://127.0.0.1:9/other"}, "", false},
		"foreign redirect": {map[string]string{"redirect_uri": "https://evil.example/callback"}, "", false},
		"http client":      {map[string]string{"client_id": "http://example.com/client.json"}, "", false},
	} {
		q := e.request(challenge, tc.set)
		status, raw := admin.do("GET", "/api/oauth/authorize?"+q, nil)
		out := obj(t, raw)
		redirect, _ := out["redirect"].(string)
		switch {
		case status != 400 || out["error"] == "":
			t.Errorf("%s: %d %s", name, status, raw)
		case tc.redirect && !strings.Contains(redirect, "error="+tc.error):
			t.Errorf("%s: redirect %q, want error=%s", name, redirect, tc.error)
		case !tc.redirect && redirect != "":
			t.Errorf("%s: offers to redirect to %q", name, redirect)
		}
		// Consent re-checks the request.
		if status, _ := admin.do("POST", "/api/oauth/consent", map[string]any{"request": q, "allow": true}); status != 400 {
			t.Errorf("%s: consent answered %d", name, status)
		}
	}

	// A scope that was not offered cannot be ticked.
	q := e.request(challenge, map[string]string{"scope": "read"})
	if status, _ := admin.do("POST", "/api/oauth/consent", map[string]any{"request": q, "allow": true,
		"scopes": []string{"issues:write"}}); status != 400 {
		t.Errorf("ticked a scope not offered: %d", status)
	}

	// Refusing sends access_denied back, and is audited.
	var out struct{ Redirect string }
	admin.call("POST", "/api/oauth/consent", map[string]any{"request": query, "allow": false}, 200, &out)
	if !strings.Contains(out.Redirect, "error=access_denied") || !strings.Contains(out.Redirect, "state=xyz") {
		t.Errorf("deny redirect: %q", out.Redirect)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'oauth.deny' AND target_id = ?`, e.clientID); n == "0" {
		t.Error("no oauth.deny audit entry")
	}

	// Who may authorize follows the token policy (administrators only by
	// default), and a token cannot authorize.
	paul := loginAt(t, e.base, "paul.martin", jirafixture.Password)
	status, raw := paul.do("GET", "/api/oauth/authorize?"+query, nil)
	if status != 403 || !strings.Contains(str(obj(t, raw), "redirect"), "error=access_denied") {
		t.Errorf("paul under admins-only: %d %s", status, raw)
	}
	pat := newToken(t, "admin", nil)
	if status, _, body := call(t, nil, e.base, "GET", "/api/oauth/authorize?"+query, nil, bearer(pat)); status != 403 {
		t.Errorf("a token authorizes: %d %s", status, body)
	}
}

// The administration sets how long an unused refresh token lasts.
func TestOAuthRefreshSetting(t *testing.T) {
	e := newOAuthEnv(t)
	admin := loginAt(t, e.base, "admin", "admin")
	t.Cleanup(func() {
		admin.do("PUT", "/api/admin/tokens/settings", map[string]any{"who": "admins", "refreshDays": 30})
		testDB.Exec(`DELETE FROM oauth_grants WHERE client_id = ?`, e.clientID)
	})
	var policy struct {
		Who         string
		RefreshDays int
	}
	admin.call("GET", "/api/admin/tokens/settings", nil, 200, &policy)
	if policy.RefreshDays != 30 {
		t.Errorf("default refresh days: %+v", policy)
	}
	for _, days := range []int{0, 366} {
		if status, _ := admin.do("PUT", "/api/admin/tokens/settings", map[string]any{"who": "admins", "refreshDays": days}); status != 400 {
			t.Errorf("refresh days %d: %d", days, status)
		}
	}
	admin.call("PUT", "/api/admin/tokens/settings", map[string]any{"who": "admins", "refreshDays": 7}, 200, &policy)
	if policy.RefreshDays != 7 {
		t.Errorf("set refresh days: %+v", policy)
	}
	// Leaving it out keeps it.
	admin.call("PUT", "/api/admin/tokens/settings", map[string]any{"who": "admins"}, 200, &policy)
	if policy.RefreshDays != 7 {
		t.Errorf("refresh days after a PUT without them: %+v", policy)
	}

	verifier, challenge := pkce("yzabcdef")
	_, tok := e.exchange(e.authorize(admin, e.request(challenge, nil)), verifier, nil)
	days := pick(t, `SELECT DATEDIFF(expires_at, created_at) FROM oauth_tokens WHERE token_hash = ?`, auth.TokenHash(tok.RefreshToken))
	if days != "7" {
		t.Errorf("refresh token lasts %s days, want 7", days)
	}
}

// register registers a client at /register and returns the status and the
// answer.
func (e *oauthEnv) register(body map[string]any) (int, map[string]any) {
	e.t.Helper()
	status, _, raw := call(e.t, nil, e.base, "POST", "/register", body, nil)
	return status, obj(e.t, raw)
}

type adminGrantRow struct {
	ID         int64
	ClientID   string
	Registered bool
	User       struct{ Username string }
}

// A client registered at /register goes through the flow like one known by
// its metadata document, and an administrator can revoke what it was given.
func TestOAuthRegisteredClient(t *testing.T) {
	e := newOAuthEnv(t)
	admin := loginAt(t, e.base, "admin", "admin")
	status, reg := e.register(map[string]any{"client_name": "Agent enregistré", "redirect_uris": []string{e.redirect},
		"token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"},
		"response_types": []string{"code"}, "logo_uri": "https://example.com/logo.png"})
	id := str(reg, "client_id")
	if status != 201 || !strings.HasPrefix(id, "solc_") || reg["client_name"] != "Agent enregistré" ||
		reg["token_endpoint_auth_method"] != "none" {
		t.Fatalf("register: %d %v", status, reg)
	}
	e.clientID = id
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM oauth_grants WHERE client_id = ?`, id)
		testDB.Exec(`DELETE FROM oauth_clients WHERE client_id = ?`, id)
	})
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'oauth.register' AND target_id = ?`, id); n == "0" {
		t.Error("no oauth.register audit entry")
	}

	verifier, challenge := pkce("regclien")
	query := e.request(challenge, map[string]string{"scope": "issues:write"})
	var check struct {
		Client struct {
			Name       string
			Registered bool
		}
	}
	admin.call("GET", "/api/oauth/authorize?"+query, nil, 200, &check)
	if check.Client.Name != "Agent enregistré" || !check.Client.Registered {
		t.Errorf("check: %+v", check)
	}
	// A redirect URI it did not register is refused, without going back.
	status, raw := admin.do("GET", "/api/oauth/authorize?"+e.request(challenge,
		map[string]string{"redirect_uri": "http://127.0.0.1:9/other"}), nil)
	if status != 400 || str(obj(t, raw), "redirect") != "" {
		t.Errorf("unregistered redirect: %d %s", status, raw)
	}

	status, tok := e.exchange(e.authorize(admin, query, "issues:write"), verifier, nil)
	if status != 200 || tok.Scope != "read issues:write" || !e.mcpSignedIn(tok.AccessToken) {
		t.Fatalf("token: %d %+v", status, tok)
	}
	if pick(t, `SELECT authorized_at IS NOT NULL FROM oauth_clients WHERE client_id = ?`, id) != "1" {
		t.Error("authorized_at not set at consent")
	}

	// The administration lists every grant and revokes any: its tokens stop.
	var grants []adminGrantRow
	admin.call("GET", "/api/admin/oauth-grants", nil, 200, &grants)
	i := slices.IndexFunc(grants, func(g adminGrantRow) bool { return g.ClientID == id })
	if i < 0 || !grants[i].Registered || grants[i].User.Username != "admin" {
		t.Fatalf("admin grants: %+v", grants)
	}
	paul := loginAt(t, e.base, "paul.martin", jirafixture.Password)
	if status, _ := paul.do("GET", "/api/admin/oauth-grants", nil); status != 403 {
		t.Errorf("a non-admin lists grants: %d", status)
	}
	if status, _ := paul.do("DELETE", "/api/admin/oauth-grants/"+strconv.FormatInt(grants[i].ID, 10), nil); status != 403 {
		t.Errorf("a non-admin revokes a grant: %d", status)
	}
	if status, _ := paul.do("DELETE", "/api/me/oauth-grants/"+strconv.FormatInt(grants[i].ID, 10), nil); status != 404 {
		t.Errorf("a user revokes another's grant: %d", status)
	}
	admin.call("DELETE", "/api/admin/oauth-grants/"+strconv.FormatInt(grants[i].ID, 10), nil, 204, nil)
	if e.mcpSignedIn(tok.AccessToken) {
		t.Error("token of a grant revoked by an administrator still works")
	}
	if status, _ := e.refresh(tok.RefreshToken); status != 400 {
		t.Errorf("refresh of a revoked grant: %d", status)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'oauth.revoke' AND target_id = ?`, id); n == "0" {
		t.Error("no oauth.revoke audit entry")
	}

	// An unknown registered client is refused, without going back.
	status, raw = admin.do("GET", "/api/oauth/authorize?"+e.request(challenge,
		map[string]string{"client_id": "solc_unknown"}), nil)
	if status != 400 || str(obj(t, raw), "redirect") != "" {
		t.Errorf("unknown registered client: %d %s", status, raw)
	}
}

func TestOAuthRegistrationRefusals(t *testing.T) {
	e := newOAuthEnv(t)
	valid := func(set map[string]any) map[string]any {
		b := map[string]any{"client_name": "Agent", "redirect_uris": []string{"https://app.example.com/cb"}}
		for k, v := range set {
			if v == nil {
				delete(b, k)
			} else {
				b[k] = v
			}
		}
		return b
	}
	for name, tc := range map[string]struct {
		body  map[string]any
		error string
	}{
		"plain HTTP":       {valid(map[string]any{"redirect_uris": []string{"http://app.example.com/cb"}}), "invalid_redirect_uri"},
		"custom scheme":    {valid(map[string]any{"redirect_uris": []string{"app://cb"}}), "invalid_redirect_uri"},
		"no redirect":      {valid(map[string]any{"redirect_uris": nil}), "invalid_redirect_uri"},
		"no name":          {valid(map[string]any{"client_name": " "}), "invalid_client_metadata"},
		"confidential":     {valid(map[string]any{"token_endpoint_auth_method": "client_secret_basic"}), "invalid_client_metadata"},
		"implicit grant":   {valid(map[string]any{"grant_types": []string{"implicit"}}), "invalid_client_metadata"},
		"token response":   {valid(map[string]any{"response_types": []string{"token"}}), "invalid_client_metadata"},
		"loopback is fine": {valid(map[string]any{"redirect_uris": []string{"http://127.0.0.1/cb"}}), ""},
	} {
		status, out := e.register(tc.body)
		if tc.error == "" {
			if status != 201 {
				t.Errorf("%s: %d %v", name, status, out)
			}
			testDB.Exec(`DELETE FROM oauth_clients WHERE client_id = ?`, str(out, "client_id"))
			continue
		}
		if status != 400 || out["error"] != tc.error {
			t.Errorf("%s: %d %v, want %s", name, status, out, tc.error)
		}
	}

	// Turned off, /register refuses and the metadata no longer names it.
	admin := loginAt(t, e.base, "admin", "admin")
	meta := func() map[string]any {
		var m map[string]any
		anonymousAt(t, e.base).call("GET", "/.well-known/oauth-authorization-server", nil, 200, &m)
		return m
	}
	if meta()["registration_endpoint"] != e.base+"/register" {
		t.Errorf("metadata with registration on: %v", meta())
	}
	t.Cleanup(func() {
		admin.do("PUT", "/api/admin/tokens/settings", map[string]any{"who": "admins", "registration": true})
	})
	var policy struct{ Registration bool }
	admin.call("PUT", "/api/admin/tokens/settings", map[string]any{"who": "admins", "registration": false}, 200, &policy)
	if policy.Registration {
		t.Errorf("registration still on: %+v", policy)
	}
	if status, out := e.register(valid(nil)); status != 403 || out["error"] != "access_denied" {
		t.Errorf("register while off: %d %v", status, out)
	}
	if m := meta(); m["registration_endpoint"] != nil {
		t.Errorf("metadata names registration while off: %v", m)
	}
	// Leaving it out of a PUT keeps it.
	admin.call("PUT", "/api/admin/tokens/settings", map[string]any{"who": "admins"}, 200, &policy)
	if policy.Registration {
		t.Errorf("registration after a PUT without it: %+v", policy)
	}
}

// A registered client nobody authorized is purged after a day; an
// authorized one stays.
func TestOAuthPurgeRegistered(t *testing.T) {
	e := newOAuthEnv(t)
	var ids []string
	for range 2 {
		status, out := e.register(map[string]any{"client_name": "Purge", "redirect_uris": []string{"https://app.example.com/cb"}})
		if status != 201 {
			t.Fatalf("register: %d %v", status, out)
		}
		ids = append(ids, str(out, "client_id"))
	}
	t.Cleanup(func() { testDB.Exec(`DELETE FROM oauth_clients WHERE client_id IN (?, ?)`, ids[0], ids[1]) })
	testDB.Exec(`UPDATE oauth_clients SET authorized_at = created_at WHERE client_id = ?`, ids[1])
	if err := auth.PurgeOAuth(t.Context(), testDB, time.Now().Add(23*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if n := pick(t, `SELECT COUNT(*) FROM oauth_clients WHERE client_id IN (?, ?)`, ids[0], ids[1]); n != "2" {
		t.Errorf("%s clients left before a day, want 2", n)
	}
	if err := auth.PurgeOAuth(t.Context(), testDB, time.Now().Add(25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if n := pick(t, `SELECT COUNT(*) FROM oauth_clients WHERE client_id = ?`, ids[0]); n != "0" {
		t.Error("unauthorized client kept after a day")
	}
	if n := pick(t, `SELECT COUNT(*) FROM oauth_clients WHERE client_id = ?`, ids[1]); n != "1" {
		t.Error("authorized client purged")
	}
}
