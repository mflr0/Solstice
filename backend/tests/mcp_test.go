package tests

// Tests of the MCP endpoint, through the SDK's client. They need
// SOLSTICE_TEST_DB and are skipped without it.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"solstice/internal/auth"
	"solstice/internal/jirafixture"
)

func TestMCP(t *testing.T) {
	needDB(t)
	paul := login(t, "paul.martin", jirafixture.Password)

	// The metadata names the resource and every scope.
	var meta struct {
		Resource        string   `json:"resource"`
		ScopesSupported []string `json:"scopes_supported"`
	}
	anonymous(t).call("GET", "/.well-known/oauth-protected-resource", nil, 200, &meta)
	if !strings.HasSuffix(meta.Resource, "/mcp") || len(meta.ScopesSupported) != len(auth.Scopes) {
		t.Errorf("metadata: %+v", meta)
	}
	for _, sc := range auth.Scopes {
		if !slices.Contains(meta.ScopesSupported, string(sc)) {
			t.Errorf("metadata lacks scope %s: %v", sc, meta.ScopesSupported)
		}
	}

	// No token, a session cookie or an unknown token: 401, pointing to the
	// metadata.
	for name, setup := range map[string]func(*http.Request){
		"no token": nil,
		"cookie":   func(r *http.Request) { r.Header.Set("Accept", "application/json, text/event-stream") },
		"unknown":  bearer("sol_unknown"),
	} {
		c := paul
		if name != "cookie" {
			c = nil
		}
		status, h, body := call(t, c, testURL, "POST", "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}, setup)
		want := `Bearer resource_metadata="` + strings.TrimSuffix(meta.Resource, "/mcp") + `/.well-known/oauth-protected-resource"`
		if status != 401 || h.Get("WWW-Authenticate") != want {
			t.Errorf("%s: %d %q %s, want 401 %q", name, status, h.Get("WWW-Authenticate"), body, want)
		}
	}

	// A read token lists and calls the read tools, and sees only what its
	// owner may see.
	session := mcpSession(t, mcpToken(t, "paul.martin", auth.ScopeRead))
	tools := mcpToolNames(t, session)
	for _, name := range []string{"search_issues", "get_issue"} {
		if !slices.Contains(tools, name) {
			t.Errorf("read token's tools lack %s: %v", name, tools)
		}
	}
	visible := createIssue(t, paul, "DEMO", "Lu par un agent", "")
	restricted := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN security_levels l ON l.id = i.security_level_id JOIN users u ON u.id = i.reporter_id
		WHERE l.name = 'Restreint' AND u.username NOT IN ('paul.martin', 'marie.durand') AND u.active
		ORDER BY i.num LIMIT 1`)

	var found struct {
		Total  int
		Issues []struct{ Key string }
	}
	mcpCall(t, session, "search_issues", map[string]any{"jql": "key in (" + visible + ", " + restricted + ")"}, &found)
	if found.Total != 1 || len(found.Issues) != 1 || found.Issues[0].Key != visible {
		t.Errorf("search_issues: %+v, want only %s", found, visible)
	}
	var got struct {
		Key    string
		Fields struct{ Summary string }
	}
	mcpCall(t, session, "get_issue", map[string]any{"issueKey": visible, "fields": []string{"summary"}}, &got)
	if got.Key != visible || got.Fields.Summary != "Lu par un agent" {
		t.Errorf("get_issue %s: %+v", visible, got)
	}
	if res := mcpRawCall(t, session, "get_issue", map[string]any{"issueKey": restricted}); !res.IsError {
		t.Errorf("get_issue %s (secured): %s", restricted, mcpResultText(res))
	}
	if res := mcpRawCall(t, session, "get_issue", map[string]any{"key": visible}); !res.IsError {
		t.Errorf("get_issue with a wrong argument: %s", mcpResultText(res))
	}

	// A token without read lists no read tool, and calling one names the scope.
	bare := mcpSession(t, mcpToken(t, "paul.martin"))
	if tools := mcpToolNames(t, bare); len(tools) != 0 {
		t.Errorf("token without read lists %v", tools)
	}
	res := mcpRawCall(t, bare, "get_issue", map[string]any{"issueKey": visible})
	if !res.IsError || !strings.Contains(mcpResultText(res), "read") {
		t.Errorf("token without read calls get_issue: %s", mcpResultText(res))
	}
}

// mcpToken creates a token with these scopes straight in the database. With
// none, its stored scopes are cleared: every scope implies read and the API
// creates no token without one, so this is the only token without read.
func mcpToken(t *testing.T, username string, scopes ...auth.Scope) string {
	t.Helper()
	token, id, err := auth.NewToken(context.Background(), testDB, userID(t, username), "mcp "+username,
		append([]auth.Scope{auth.ScopeRead}, scopes...), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testDB.Exec(`DELETE FROM api_tokens WHERE id = ?`, id) })
	if len(scopes) == 0 {
		if _, err := testDB.Exec(`UPDATE api_tokens SET scopes = '' WHERE id = ?`, id); err != nil {
			t.Fatal(err)
		}
	}
	return token
}

// bearerTransport signs every request in with a token.
type bearerTransport string

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+string(b))
	return http.DefaultTransport.RoundTrip(r)
}

func mcpSession(t *testing.T, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "solstice-tests", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: testURL + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport(token)}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func mcpToolNames(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func mcpRawCall(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

// mcpCall calls a tool that must succeed, and decodes its JSON answer.
func mcpCall(t *testing.T, session *mcp.ClientSession, name string, args map[string]any, out any) {
	t.Helper()
	res := mcpRawCall(t, session, name, args)
	text := mcpResultText(res)
	if res.IsError {
		t.Fatalf("%s: %s", name, text)
	}
	if err := json.Unmarshal([]byte(text), out); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, text)
	}
}

func mcpResultText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func TestMCPFilters(t *testing.T) {
	needDB(t)
	paul := login(t, "paul.martin", jirafixture.Password)
	marie := login(t, "marie.durand", jirafixture.Password)
	visible := createIssue(t, paul, "DEMO", "Trouvé par un filtre favori", "")

	// A read token runs a favourite filter: lists it, then searches its JQL.
	var fav struct{ ID int64 }
	paul.call("POST", "/api/filters", map[string]any{"name": unique("Favori MCP "), "query": "key = " + visible}, 201, &fav)
	t.Cleanup(func() { paul.do("DELETE", fmt.Sprintf("/api/filters/%d", fav.ID), nil) })
	paul.call("PUT", fmt.Sprintf("/api/filters/%d/favourite", fav.ID), nil, 204, nil)
	read := mcpSession(t, mcpToken(t, "paul.martin", auth.ScopeRead))
	var favourites []struct{ ID, JQL string }
	mcpCall(t, read, "list_favourite_filters", nil, &favourites)
	i := slices.IndexFunc(favourites, func(f struct{ ID, JQL string }) bool { return f.ID == strconv.FormatInt(fav.ID, 10) })
	if i < 0 {
		t.Fatalf("list_favourite_filters lacks %d: %+v", fav.ID, favourites)
	}
	var found struct{ Issues []struct{ Key string } }
	mcpCall(t, read, "search_issues", map[string]any{"jql": favourites[i].JQL}, &found)
	if len(found.Issues) != 1 || found.Issues[0].Key != visible {
		t.Errorf("search_issues %q: %+v, want %s", favourites[i].JQL, found, visible)
	}

	// The read token lists no write tool, and calling one names the scope.
	if tools := mcpToolNames(t, read); slices.Contains(tools, "create_filter") {
		t.Errorf("read token lists create_filter: %v", tools)
	}
	res := mcpRawCall(t, read, "delete_filter", map[string]any{"filterId": strconv.FormatInt(fav.ID, 10)})
	if !res.IsError || !strings.Contains(mcpResultText(res), string(auth.ScopeFilters)) {
		t.Errorf("read token calls delete_filter: %s", mcpResultText(res))
	}

	// Another user's private filter stays hidden.
	var private struct{ ID int64 }
	marie.call("POST", "/api/filters", map[string]any{"name": unique("Privé MCP "), "query": "project = DEMO"}, 201, &private)
	t.Cleanup(func() { marie.do("DELETE", fmt.Sprintf("/api/filters/%d", private.ID), nil) })
	if res := mcpRawCall(t, read, "get_filter", map[string]any{"filterId": strconv.FormatInt(private.ID, 10)}); !res.IsError {
		t.Errorf("get_filter of marie.durand's private filter: %s", mcpResultText(res))
	}

	// A filters:write token creates a filter and deletes it.
	write := mcpSession(t, mcpToken(t, "paul.martin", auth.ScopeFilters))
	name := unique("Créé par MCP ")
	var created struct{ ID, Name, JQL string }
	mcpCall(t, write, "create_filter", map[string]any{"name": name, "jql": "project = DEMO", "favourite": true}, &created)
	if created.Name != name || created.JQL != "project = DEMO" {
		t.Errorf("create_filter: %+v", created)
	}
	t.Cleanup(func() { paul.do("DELETE", "/api/filters/"+created.ID, nil) })
	var got struct {
		Name      string
		Favourite bool
	}
	mcpCall(t, write, "get_filter", map[string]any{"filterId": created.ID}, &got)
	if got.Name != name || !got.Favourite {
		t.Errorf("get_filter after create_filter: %+v", got)
	}
	if res := mcpRawCall(t, write, "delete_filter", map[string]any{"filterId": created.ID}); res.IsError {
		t.Fatalf("delete_filter: %s", mcpResultText(res))
	}
	if res := mcpRawCall(t, write, "get_filter", map[string]any{"filterId": created.ID}); !res.IsError {
		t.Errorf("get_filter after delete_filter: %s", mcpResultText(res))
	}
	if res := mcpRawCall(t, write, "delete_filter", map[string]any{"filterId": strconv.FormatInt(private.ID, 10)}); !res.IsError {
		t.Errorf("delete_filter of marie.durand's private filter: %s", mcpResultText(res))
	}
}
