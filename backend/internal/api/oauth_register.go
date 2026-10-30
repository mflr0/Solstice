package api

// Dynamic Client Registration (RFC 7591), which the MCP authorization spec
// keeps for clients that have no metadata document: anyone may register a
// public client, unless the administration turned it off. A registered
// client's name is its own claim: the consent page says so.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"solstice/internal/audit"
	"solstice/internal/auth"
	"solstice/internal/i18n"
	"solstice/internal/sqlq"
)

const (
	oauthRegister = "/register"

	// registeredPrefix starts the client_id of a registered client, which
	// tells it from a metadata document URL.
	registeredPrefix = "solc_"

	oauthRegistrationSetting = "oauth.registration"

	maxRedirectURIs  = 10
	maxRedirectURI   = 1024
	registrationsMax = 10 // per client address and registrationTime
	registrationTime = time.Hour
)

// registrationOn tells whether clients may register; a missing or
// unreadable setting means yes.
func (s *Server) registrationOn(ctx context.Context) bool {
	on := true
	if ok, err := s.out.settings.GetJSON(ctx, oauthRegistrationSetting, &on); err != nil || !ok {
		return true
	}
	return on
}

// validRedirectURI tells whether a client may register a redirect URI:
// HTTPS, or HTTP on a loopback address (RFC 8252 §7.3), with no user or
// fragment.
func validRedirectURI(uri string) bool {
	if len(uri) > maxRedirectURI || strings.Contains(uri, "#") {
		return false
	}
	u, err := url.Parse(uri)
	if err != nil || u.User != nil || u.Host == "" {
		return false
	}
	return u.Scheme == "https" || (u.Scheme == "http" && loopbackHost(u.Hostname()))
}

// registerFail is an error answer of the registration endpoint (RFC 7591
// §3.2.2).
func registerFail(w http.ResponseWriter, code, description string) {
	oauthFail(w, http.StatusBadRequest, code, description)
}

// oauthRegisterEndpoint registers a public client.
func (s *Server) oauthRegisterEndpoint(w http.ResponseWriter, r *http.Request) {
	setBodyReadDeadline(w, time.Now().Add(bodyReadTimeout))
	defer setBodyReadDeadline(w, time.Time{})
	ctx := r.Context()
	if !s.registrationOn(ctx) {
		oauthFail(w, http.StatusForbidden, "access_denied", "client registration is turned off")
		return
	}
	if wait := s.registrations.take(clientIP(r, s.proxies)); wait > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		oauthFail(w, http.StatusTooManyRequests, "temporarily_unavailable", "too many registrations from this address")
		return
	}
	var in struct {
		RedirectURIs  []string `json:"redirect_uris"`
		ClientName    string   `json:"client_name"`
		AuthMethod    string   `json:"token_endpoint_auth_method"`
		GrantTypes    []string `json:"grant_types"`
		ResponseTypes []string `json:"response_types"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxTokenForm)).Decode(&in); err != nil {
		registerFail(w, "invalid_client_metadata", "the body must be a JSON object")
		return
	}
	in.ClientName = strings.TrimSpace(in.ClientName)
	switch {
	case len(in.RedirectURIs) == 0 || len(in.RedirectURIs) > maxRedirectURIs:
		registerFail(w, "invalid_redirect_uri", "redirect_uris must list 1 to "+strconv.Itoa(maxRedirectURIs)+" addresses")
		return
	case slices.ContainsFunc(in.RedirectURIs, func(u string) bool { return !validRedirectURI(u) }):
		registerFail(w, "invalid_redirect_uri", "a redirect URI must be https, or http on a loopback address")
		return
	case in.ClientName == "":
		registerFail(w, "invalid_client_metadata", "client_name is required")
		return
	case in.AuthMethod != "" && in.AuthMethod != "none":
		registerFail(w, "invalid_client_metadata", "token_endpoint_auth_method must be none: only public clients are supported")
		return
	case slices.ContainsFunc(in.GrantTypes, func(g string) bool { return g != "authorization_code" && g != "refresh_token" }):
		registerFail(w, "invalid_client_metadata", "grant_types may only hold authorization_code and refresh_token")
		return
	case slices.ContainsFunc(in.ResponseTypes, func(t string) bool { return t != "code" }):
		registerFail(w, "invalid_client_metadata", "response_types may only hold code")
		return
	}
	if utf8.RuneCountInString(in.ClientName) > maxClientName {
		in.ClientName = string([]rune(in.ClientName)[:maxClientName])
	}
	raw, err := auth.RandomToken()
	if err != nil {
		s.registerError(w, r, err)
		return
	}
	id, now := registeredPrefix+raw, time.Now().UTC()
	uris, _ := json.Marshal(in.RedirectURIs)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO oauth_clients (client_id, client_name, redirect_uris, created_at)
		VALUES (?, ?, ?, ?)`, id, in.ClientName, string(uris), now); err != nil {
		s.registerError(w, r, err)
		return
	}
	s.auditLog(r, audit.OAuthRegister, audit.Target{Type: audit.TargetOAuthClient, ID: id, Label: in.ClientName},
		map[string]any{"redirectUris": in.RedirectURIs})
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]any{
		"client_id":                  id,
		"client_id_issued_at":        now.Unix(),
		"client_name":                in.ClientName,
		"redirect_uris":              in.RedirectURIs,
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	})
}

func (s *Server) registerError(w http.ResponseWriter, r *http.Request, err error) {
	if err := boundary(s.log, "OAuth client registration failed", r, err); err != nil {
		oauthFail(w, http.StatusInternalServerError, "server_error", "")
	}
}

// errClientUnknown is a client_id of the registered kind that is not, or no
// longer, registered.
var errClientUnknown = errors.New("client not registered")

// registeredClient reads a registered client.
func (s *Server) registeredClient(ctx context.Context, id string) (*oauthClientDoc, error) {
	doc := &oauthClientDoc{ClientID: id, Registered: true}
	var uris string
	err := s.db.QueryRowContext(ctx, `SELECT client_name, redirect_uris FROM oauth_clients WHERE client_id = ?`, id).
		Scan(&doc.ClientName, &uris)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errClientUnknown
	}
	if err != nil {
		return nil, err
	}
	return doc, json.Unmarshal([]byte(uris), &doc.RedirectURIs)
}

// oauthClient reads the client of an authorization request: a registered
// one, or one known by its metadata document.
func (s *Server) oauthClient(ctx context.Context, id string) (*oauthClientDoc, error) {
	if strings.HasPrefix(id, registeredPrefix) {
		doc, err := s.registeredClient(ctx, id)
		if errors.Is(err, errClientUnknown) {
			return nil, fail(http.StatusBadRequest, i18n.OAuthClientUnknown)
		}
		return doc, err
	}
	if !validClientID(id) {
		return nil, fail(http.StatusBadRequest, i18n.OAuthClientInvalid)
	}
	doc, err := s.fetchClient(ctx, id)
	if err != nil {
		s.log.Warn("OAuth client document refused", "client", id, "err", err)
		return nil, failf(http.StatusBadRequest, i18n.OAuthClientDocument, id)
	}
	return doc, nil
}

// registrationLimit counts registrations per client address in fixed
// windows.
type registrationLimit struct {
	mu   sync.Mutex
	seen map[string]*registrationCount
	now  func() time.Time
}

type registrationCount struct {
	start time.Time
	n     int
}

func newRegistrationLimit() *registrationLimit {
	return &registrationLimit{seen: map[string]*registrationCount{}, now: time.Now}
}

// take counts a registration from an address, and returns how long it
// must wait instead when it is over the limit.
func (l *registrationLimit) take(addr string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.seen) > 10000 {
		for k, c := range l.seen {
			if now.Sub(c.start) >= registrationTime {
				delete(l.seen, k)
			}
		}
	}
	c := l.seen[addr]
	if c == nil || now.Sub(c.start) >= registrationTime {
		c = &registrationCount{start: now}
		l.seen[addr] = c
	}
	if c.n >= registrationsMax {
		return c.start.Add(registrationTime).Sub(now)
	}
	c.n++
	return 0
}

type adminGrantOut struct {
	oauthGrantOut
	User       userRef `json:"user"`
	Registered bool    `json:"registered"`
}

// adminGrants lists every user's OAuth grants, most recently used first.
func (s *Server) adminGrants(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	out := []adminGrantOut{}
	err := sqlq.Each(r.Context(), s.db, `SELECT g.id, g.client_id, g.client_name, g.scopes, g.created_at, g.last_used_at,
		u.id, u.username, u.display_name, u.active
		FROM oauth_grants g JOIN users u ON u.id = g.user_id
		ORDER BY COALESCE(g.last_used_at, g.created_at) DESC, g.id DESC`,
		func(rows *sql.Rows) error {
			var g adminGrantOut
			var scopes string
			var used sql.NullTime
			if err := rows.Scan(&g.ID, &g.ClientID, &g.ClientName, &scopes, &g.Created, &used,
				&g.User.ID, &g.User.Username, &g.User.Name, &g.User.Active); err != nil {
				return err
			}
			g.oauthGrantOut = oauthGrantRow(g.oauthGrantOut, scopes, used)
			g.Registered = strings.HasPrefix(g.ClientID, registeredPrefix)
			out = append(out, g)
			return nil
		})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// adminRevokeGrant revokes anyone's grant, with every token it holds.
func (s *Server) adminRevokeGrant(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	return s.revokeGrant(w, r, 0)
}
