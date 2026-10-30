package api

// The Model Context Protocol endpoint, /mcp: AI agents reach Solstice
// through tools, one per operation of the Jira-compatible REST API (see
// mcp_tools.go).
//
// Sign-in is "Authorization: Bearer <token>" only, a personal access token
// or an OAuth access token (oauth.go): no cookie, so no request can be
// forged cross-site, and no password. A
// request without a valid token is answered 401, pointing to the protected
// resource metadata (RFC 9728) as the MCP authorization spec asks.
//
// The transport is stateless and answers in JSON: each POST carries a whole
// exchange, so the endpoint keeps no session and no stream open, and every
// request is signed in again.

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	solauth "solstice/internal/auth"
	"solstice/internal/i18n"
)

// Paths of the endpoint and of its protected resource metadata.
const (
	mcpPath          = "/mcp"
	mcpResourceMeta  = "/.well-known/oauth-protected-resource"
	mcpServerVersion = "1.0"
)

// mcpRoutes registers the MCP endpoint and its metadata.
func (s *Server) mcpRoutes(mux *http.ServeMux) {
	s.handle(mux, "GET "+mcpResourceMeta, scopePublic, http.HandlerFunc(s.mcpMetadata))
	// The endpoint needs a token but no scope: each tool requires its own.
	mux.Handle(mcpPath, s.mcpEndpoint())
}

// mcpMetadata serves the protected resource metadata of the endpoint.
func (s *Server) mcpMetadata(w http.ResponseWriter, r *http.Request) {
	base := s.restOut(r).base
	// No admin tool exists, and OAuth never grants admin.
	scopes := scopeNames(grantableScopes())
	auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               base + mcpPath,
		ResourceName:           "Solstice",
		AuthorizationServers:   []string{base},
		ScopesSupported:        scopes,
		BearerMethodsSupported: []string{"header"},
	}).ServeHTTP(w, r)
}

func (s *Server) mcpEndpoint() http.Handler {
	h := mcp.NewStreamableHTTPHandler(s.mcpServer, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		// The protection refuses a loopback connection whose Host is not
		// loopback: every request behind a reverse proxy on the same machine.
		// It guards servers that trust whoever reaches them; this one needs a
		// token, which a page rebound to the address cannot have.
		DisableLocalhostProtection: true,
		// Limited below, once the token's scopes are known.
		MaxRequestBodyBytes:          -1,
		PropagateRequestCancellation: true,
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength != 0 {
			setBodyReadDeadline(w, time.Now().Add(bodyReadTimeout))
			defer setBodyReadDeadline(w, time.Time{})
		}
		u, err := s.mcpUser(r)
		if err != nil {
			if err := boundary(s.log, "MCP sign-in failed", r, err); err != nil {
				w.Header().Set("WWW-Authenticate",
					fmt.Sprintf(`Bearer resource_metadata=%q`, s.restOut(r).base+mcpResourceMeta))
				s.writeError(w, r, err)
			}
			return
		}
		limit := int64(maxJSONBody)
		if u.Allows(solauth.ScopeIssues) { // add_attachment takes a file of up to maxUpload, base64-encoded
			limit += int64(base64.StdEncoding.EncodedLen(int(s.maxUpload)))
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userCtxKey, u)))
	})
}

// mcpUser signs an MCP request in with its bearer token.
func (s *Server) mcpUser(r *http.Request) (*solauth.User, error) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return nil, fail(http.StatusUnauthorized, i18n.RequestSignIn)
	}
	token = strings.TrimSpace(token)
	lookup := solauth.LookupToken
	if solauth.LooksLikeOAuthToken(token) {
		lookup = solauth.LookupOAuthToken
	}
	u, err := lookup(r.Context(), s.db, token)
	if errors.Is(err, solauth.ErrNoSession) {
		s.rejectToken(r, token)
		return nil, fail(http.StatusUnauthorized, i18n.RequestCredentialsRefused)
	}
	return u, err
}

// mcpServer is the MCP server of one request: the tools its token's scopes
// allow, run as the token's owner.
func (s *Server) mcpServer(r *http.Request) *mcp.Server {
	u := currentUser(r)
	srv := mcp.NewServer(&mcp.Implementation{Name: "solstice", Title: "Solstice", Version: mcpServerVersion},
		&mcp.ServerOptions{
			Instructions: "Solstice is an issue tracker. Tools answer in the JSON of the Jira Server REST API, " +
				"and only show what the token's owner may see.",
			Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
		})
	allowed := map[string]bool{}
	for i := range mcpTools {
		if t := &mcpTools[i]; u.Allows(t.scope) {
			allowed[t.name] = true
			srv.AddTool(t.tool(), s.mcpCall(r, t))
		}
	}
	// A call to a tool the token may not use names the scope it lacks,
	// rather than the tool being unknown.
	srv.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if call, ok := req.(*mcp.CallToolRequest); ok && !allowed[call.Params.Name] {
				if i := slices.IndexFunc(mcpTools, func(t mcpTool) bool { return t.name == call.Params.Name }); i >= 0 {
					return mcpError(r, failf(http.StatusForbidden, i18n.TokenScopeMissing, mcpTools[i].scope)), nil
				}
			}
			return next(ctx, method, req)
		}
	})
	return srv
}

// mcpError is the result of a tool call that failed.
func mcpError(r *http.Request, err error) *mcp.CallToolResult {
	var he *httpError
	errors.As(err, &he)
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: he.msg.In(locale(r))}}}
}
