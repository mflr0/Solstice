package api

// Solstice is the OAuth 2.1 authorization server of its MCP endpoint, as
// the MCP authorization spec describes: authorization code flow with PKCE
// (S256), clients known by their metadata document (oauth_client.go),
// tokens bound to /mcp (RFC 8707), the issuer in the response (RFC 9207).
//
// The interface's consent page (/oauth/authorize) checks the request with
// GET /api/oauth/authorize and answers it with POST /api/oauth/consent,
// which gives the address to send the browser back to. Who may authorize
// follows the token policy (token_policy.go).

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/sqlq"
)

// Paths of the authorization server.
const (
	oauthMeta      = "/.well-known/oauth-authorization-server"
	oauthAuthorize = "/oauth/authorize"
	oauthToken     = "/oauth/token"
	oauthRevoke    = "/oauth/revoke"
)

// The refresh token lifetime setting, in days: a refresh token unused for
// that long expires, and the user authorizes the client again.
const (
	oauthRefreshSetting = "oauth.refresh_days"
	defaultRefreshDays  = 30
	maxRefreshDays      = 365
)

// maxTokenForm caps the body of the token and revocation endpoints.
const maxTokenForm = 64 << 10

func (s *Server) oauthRoutes(mux *http.ServeMux, route routeFunc) {
	s.handle(mux, "GET "+oauthMeta, scopePublic, http.HandlerFunc(s.oauthMetadata))
	s.handle(mux, "POST "+oauthToken, scopePublic, http.HandlerFunc(s.oauthTokenEndpoint))
	s.handle(mux, "POST "+oauthRevoke, scopePublic, http.HandlerFunc(s.oauthRevokeEndpoint))
	route("GET /api/oauth/authorize", s.oauthCheck, auth.ScopeRead)
	route("POST /api/oauth/consent", s.oauthConsent, auth.ScopeProfile)
	route("GET /api/me/oauth-grants", s.myGrants, auth.ScopeRead)
	route("DELETE /api/me/oauth-grants/{id}", s.revokeMyGrant, auth.ScopeProfile)
}

// grantableScopes are the scopes a client may be given: all but admin.
func grantableScopes() []auth.Scope {
	return slices.DeleteFunc(slices.Clone(auth.Scopes), func(sc auth.Scope) bool { return sc == auth.ScopeAdmin })
}

func scopeNames(scopes []auth.Scope) []string {
	out := make([]string, len(scopes))
	for i, sc := range scopes {
		out[i] = string(sc)
	}
	return out
}

// oauthMetadata serves the authorization server metadata (RFC 8414).
func (s *Server) oauthMetadata(w http.ResponseWriter, r *http.Request) {
	base := s.restOut(r).base
	w.Header().Set("Cache-Control", "max-age=3600")
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         base,
		"authorization_endpoint":                         base + oauthAuthorize,
		"token_endpoint":                                 base + oauthToken,
		"revocation_endpoint":                            base + oauthRevoke,
		"scopes_supported":                               scopeNames(grantableScopes()),
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"revocation_endpoint_auth_methods_supported":     []string{"none"},
		"authorization_response_iss_parameter_supported": true,
		"client_id_metadata_document_supported":          true,
	})
}

// mcpResource is the canonical URI of the MCP endpoint, the only resource
// tokens are issued for.
func (s *Server) mcpResource(r *http.Request) string { return s.restOut(r).base + mcpPath }

// sameResource compares a resource parameter with the endpoint's URI,
// ignoring the case of scheme and host and a trailing slash.
func sameResource(got, want string) bool {
	norm := func(v string) string {
		u, err := url.Parse(v)
		if err != nil || strings.Contains(v, "#") {
			return ""
		}
		return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + strings.TrimSuffix(u.EscapedPath(), "/")
	}
	g := norm(got)
	return g != "" && g == norm(want)
}

// authzRequest is an authorization request that passed its checks.
type authzRequest struct {
	doc         *oauthClientDoc
	redirectURI string
	state       string
	challenge   string
	offered     []auth.Scope // what the consent page offers, read first
}

// back is the address that sends the browser back to the client with these
// answer parameters, the state and the issuer.
func (s *Server) back(r *http.Request, redirectURI, state string, params url.Values) string {
	u, _ := url.Parse(redirectURI)
	q := u.Query()
	for k, v := range params {
		q[k] = v
	}
	if state != "" {
		q.Set("state", state)
	}
	q.Set("iss", s.restOut(r).base)
	u.RawQuery = q.Encode()
	return u.String()
}

// authzError is an error of a request whose client and redirect URI are
// known good: the page shows it and offers to go back to the client with
// it (RFC 6749 §4.1.2.1).
func (s *Server) authzError(r *http.Request, a *authzRequest, code string, status int, key i18n.Key, args ...any) error {
	return &httpError{status: status, msg: i18n.M(key, args...), extra: map[string]any{
		"redirect": s.back(r, a.redirectURI, a.state, url.Values{"error": {code}}),
		"client":   a.doc.ClientName,
	}}
}

// readAuthorization checks an authorization request for the signed-in user.
// Until the client and the redirect URI are known good, an error must not
// send the browser to it.
func (s *Server) readAuthorization(r *http.Request, q url.Values) (*authzRequest, error) {
	u := currentUser(r)
	if u.ByToken() {
		return nil, fail(http.StatusForbidden, i18n.OAuthTokenCannotAuthorize)
	}
	clientID := q.Get("client_id")
	if !validClientID(clientID) {
		return nil, fail(http.StatusBadRequest, i18n.OAuthClientInvalid)
	}
	doc, err := s.fetchClient(r.Context(), clientID)
	if err != nil {
		s.log.Warn("OAuth client document refused", "client", clientID, "err", err)
		return nil, failf(http.StatusBadRequest, i18n.OAuthClientDocument, clientID)
	}
	a := &authzRequest{doc: doc, redirectURI: q.Get("redirect_uri"), state: q.Get("state")}
	if !doc.allowsRedirect(a.redirectURI) {
		return nil, fail(http.StatusBadRequest, i18n.OAuthRedirectInvalid)
	}
	if q.Get("response_type") != "code" {
		return nil, s.authzError(r, a, "unsupported_response_type", http.StatusBadRequest, i18n.OAuthResponseType)
	}
	a.challenge = q.Get("code_challenge")
	if q.Get("code_challenge_method") != "S256" || len(a.challenge) != 43 {
		return nil, s.authzError(r, a, "invalid_request", http.StatusBadRequest, i18n.OAuthPKCERequired)
	}
	if want := s.mcpResource(r); !sameResource(q.Get("resource"), want) {
		return nil, s.authzError(r, a, "invalid_target", http.StatusBadRequest, i18n.OAuthResourceInvalid, want)
	}
	var asked []auth.Scope
	for _, f := range strings.Fields(q.Get("scope")) {
		switch sc := auth.Scope(f); {
		case f == "offline_access" || sc == auth.ScopeAdmin:
			// Refresh tokens come anyway; admin is never given.
		case !sc.Known():
			return nil, s.authzError(r, a, "invalid_scope", http.StatusBadRequest, i18n.OAuthScopeUnknown, f)
		default:
			asked = append(asked, sc)
		}
	}
	if len(asked) == 0 && strings.TrimSpace(q.Get("scope")) == "" {
		asked = grantableScopes()
	}
	a.offered = auth.NormalizeScopes(append(asked, auth.ScopeRead))
	if ok, err := s.mayCreateTokens(r.Context(), u); err != nil {
		return nil, err
	} else if !ok {
		return nil, s.authzError(r, a, "access_denied", http.StatusForbidden, i18n.OAuthNotAllowed)
	}
	return a, nil
}

// oauthCheck tells the consent page what it asks the user to allow.
func (s *Server) oauthCheck(w http.ResponseWriter, r *http.Request) error {
	a, err := s.readAuthorization(r, r.URL.Query())
	if err != nil {
		return err
	}
	cid, _ := url.Parse(a.doc.ClientID)
	redirect, _ := url.Parse(a.redirectURI)
	return writeJSON(w, http.StatusOK, map[string]any{
		"client":       map[string]string{"id": a.doc.ClientID, "name": a.doc.ClientName, "host": cid.Host},
		"redirectHost": redirect.Host,
		"scopes":       a.offered,
	})
}

// oauthConsent answers an authorization request: {"request": the query of
// /oauth/authorize, "allow": bool, "scopes": the ticked scopes}. The answer
// is {"redirect": address}, with a code when allowed.
func (s *Server) oauthConsent(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Request string       `json:"request"`
		Allow   bool         `json:"allow"`
		Scopes  []auth.Scope `json:"scopes"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	q, err := url.ParseQuery(strings.TrimPrefix(in.Request, "?"))
	if err != nil {
		return failf(http.StatusBadRequest, i18n.RequestInvalid, err.Error())
	}
	a, err := s.readAuthorization(r, q)
	if err != nil {
		return err
	}
	tgt := audit.Target{Type: audit.TargetOAuthClient, ID: a.doc.ClientID, Label: a.doc.ClientName}
	if !in.Allow {
		s.auditLog(r, audit.OAuthDeny, tgt, nil)
		return writeJSON(w, http.StatusOK, map[string]string{
			"redirect": s.back(r, a.redirectURI, a.state, url.Values{"error": {"access_denied"}})})
	}
	for _, sc := range in.Scopes {
		if !slices.Contains(a.offered, sc) {
			return failf(http.StatusBadRequest, i18n.OAuthScopeUnknown, sc)
		}
	}
	scopes := auth.NormalizeScopes(append(in.Scopes, auth.ScopeRead))
	code, grantID, err := auth.AuthorizeOAuth(r.Context(), s.db, currentUser(r).ID, auth.OAuthCode{
		Client:      auth.OAuthClient{ID: a.doc.ClientID, Name: a.doc.ClientName},
		RedirectURI: a.redirectURI, Challenge: a.challenge, Scopes: scopes,
	}, time.Now())
	if err != nil {
		return err
	}
	s.auditLog(r, audit.OAuthGrant, tgt, map[string]any{"grant": grantID, "scopes": scopes})
	return writeJSON(w, http.StatusOK, map[string]string{
		"redirect": s.back(r, a.redirectURI, a.state, url.Values{"code": {code}})})
}

// refreshLifetime is how long an unused refresh token lasts.
func (s *Server) refreshLifetime(ctx context.Context) time.Duration {
	return time.Duration(s.refreshDays(ctx)) * 24 * time.Hour
}

func (s *Server) refreshDays(ctx context.Context) int {
	days := defaultRefreshDays
	if ok, err := s.out.settings.GetJSON(ctx, oauthRefreshSetting, &days); err != nil || !ok || days < 1 || days > maxRefreshDays {
		return defaultRefreshDays
	}
	return days
}

// oauthFail is an error answer of the token endpoint (RFC 6749 §5.2).
func oauthFail(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]string{"error": code, "error_description": description})
}

// readTokenForm reads the form of the token and revocation endpoints. Only
// public clients exist: one presenting credentials is refused.
func readTokenForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxTokenForm)
	if err := r.ParseForm(); err != nil {
		oauthFail(w, http.StatusBadRequest, "invalid_request", "the body must be application/x-www-form-urlencoded")
		return false
	}
	if r.Header.Get("Authorization") != "" || r.PostForm.Get("client_secret") != "" {
		oauthFail(w, http.StatusUnauthorized, "invalid_client", "only public clients are supported")
		return false
	}
	if r.PostForm.Get("client_id") == "" {
		oauthFail(w, http.StatusBadRequest, "invalid_request", "client_id is required")
		return false
	}
	return true
}

// oauthTokenEndpoint gives tokens for an authorization code or a refresh
// token.
func (s *Server) oauthTokenEndpoint(w http.ResponseWriter, r *http.Request) {
	setBodyReadDeadline(w, time.Now().Add(bodyReadTimeout))
	defer setBodyReadDeadline(w, time.Time{})
	if !readTokenForm(w, r) {
		return
	}
	f := r.PostForm
	ctx := r.Context()
	resource := f.Get("resource")
	var t *auth.OAuthTokens
	var err error
	switch f.Get("grant_type") {
	case "authorization_code":
		if !sameResource(resource, s.mcpResource(r)) {
			oauthFail(w, http.StatusBadRequest, "invalid_target", "resource must be "+s.mcpResource(r))
			return
		}
		t, err = auth.ExchangeOAuthCode(ctx, s.db, f.Get("code"), f.Get("client_id"), f.Get("redirect_uri"),
			f.Get("code_verifier"), s.refreshLifetime(ctx), time.Now())
	case "refresh_token":
		if resource != "" && !sameResource(resource, s.mcpResource(r)) {
			oauthFail(w, http.StatusBadRequest, "invalid_target", "resource must be "+s.mcpResource(r))
			return
		}
		var scopes []auth.Scope
		for _, name := range strings.Fields(f.Get("scope")) {
			scopes = append(scopes, auth.Scope(name))
		}
		t, err = auth.RefreshOAuth(ctx, s.db, f.Get("refresh_token"), f.Get("client_id"), scopes,
			s.refreshLifetime(ctx), time.Now())
	default:
		oauthFail(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be authorization_code or refresh_token")
		return
	}
	switch {
	case errors.Is(err, auth.ErrOAuthReplay):
		s.log.Warn("OAuth code or refresh token replayed: its tokens are revoked", "client", f.Get("client_id"))
		oauthFail(w, http.StatusBadRequest, "invalid_grant", "the code or refresh token was already used")
		return
	case errors.Is(err, auth.ErrOAuthGrant):
		oauthFail(w, http.StatusBadRequest, "invalid_grant", "the code or refresh token is invalid or expired, "+
			"or does not match the client, redirect URI or code verifier")
		return
	case errors.Is(err, auth.ErrOAuthScope):
		oauthFail(w, http.StatusBadRequest, "invalid_scope", "a scope asked for was not granted")
		return
	case err != nil:
		if err := boundary(s.log, "OAuth token request failed", r, err); err != nil {
			oauthFail(w, http.StatusInternalServerError, "server_error", "")
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  t.Access,
		"token_type":    "Bearer",
		"expires_in":    int(auth.OAuthAccessLifetime.Seconds()),
		"refresh_token": t.Refresh,
		"scope":         strings.Join(scopeNames(t.Scopes), " "),
	})
}

// oauthRevokeEndpoint revokes a token (RFC 7009): it answers 200 whether or
// not the token existed.
func (s *Server) oauthRevokeEndpoint(w http.ResponseWriter, r *http.Request) {
	setBodyReadDeadline(w, time.Now().Add(bodyReadTimeout))
	defer setBodyReadDeadline(w, time.Time{})
	if !readTokenForm(w, r) {
		return
	}
	if err := auth.RevokeOAuthToken(r.Context(), s.db, r.PostForm.Get("token"), r.PostForm.Get("client_id")); err != nil {
		if err := boundary(s.log, "OAuth revocation failed", r, err); err != nil {
			oauthFail(w, http.StatusServiceUnavailable, "temporarily_unavailable", "")
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

type oauthGrantOut struct {
	ID         int64        `json:"id"`
	ClientID   string       `json:"clientId"`
	ClientName string       `json:"clientName"`
	ClientHost string       `json:"clientHost"`
	Scopes     []auth.Scope `json:"scopes"`
	Created    time.Time    `json:"created"`
	LastUsed   *time.Time   `json:"lastUsed"`
}

// myGrants lists the clients the user authorized, most recently used first.
func (s *Server) myGrants(w http.ResponseWriter, r *http.Request) error {
	out := []oauthGrantOut{}
	err := sqlq.Each(r.Context(), s.db, `SELECT id, client_id, client_name, scopes, created_at, last_used_at
		FROM oauth_grants WHERE user_id = ? ORDER BY COALESCE(last_used_at, created_at) DESC, id DESC`,
		func(rows *sql.Rows) error {
			var g oauthGrantOut
			var scopes string
			var used sql.NullTime
			if err := rows.Scan(&g.ID, &g.ClientID, &g.ClientName, &scopes, &g.Created, &used); err != nil {
				return err
			}
			g.Scopes, g.LastUsed = auth.ParseScopes(scopes), sqlq.Time(used)
			if u, err := url.Parse(g.ClientID); err == nil {
				g.ClientHost = u.Host
			}
			out = append(out, g)
			return nil
		}, currentUser(r).ID)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// revokeMyGrant revokes a client the user authorized, with every token it
// holds.
func (s *Server) revokeMyGrant(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	ctx := r.Context()
	var clientID, name string
	err = s.db.QueryRowContext(ctx, `SELECT client_id, client_name FROM oauth_grants WHERE id = ? AND user_id = ?`,
		id, currentUser(r).ID).Scan(&clientID, &name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errNotFound
		}
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM oauth_grants WHERE id = ?`, id); err != nil {
		return err
	}
	s.auditLog(r, audit.OAuthRevoke, audit.Target{Type: audit.TargetOAuthClient, ID: clientID, Label: name},
		map[string]any{"grant": id})
	w.WriteHeader(http.StatusNoContent)
	return nil
}
