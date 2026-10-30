package tests

// Tests of the MCP endpoint, through the SDK's client. They need
// SOLSTICE_TEST_DB and are skipped without it.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"solstice/internal/api"
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

// A token with issues:write creates and changes issues as its owner, within
// what the REST API allows.
func TestMCPWrite(t *testing.T) {
	needDB(t)
	if tools := mcpToolNames(t, mcpSession(t, mcpToken(t, "paul.martin", auth.ScopeRead))); slices.Contains(tools, "create_issue") {
		t.Errorf("read token lists create_issue: %v", tools)
	}
	session := mcpSession(t, mcpToken(t, "paul.martin", auth.ScopeIssues))
	if tools := mcpToolNames(t, session); !slices.Contains(tools, "create_issue") || !slices.Contains(tools, "get_issue") {
		t.Errorf("issues:write token's tools: %v", tools)
	}

	typeID := standardType(t, login(t, "paul.martin", jirafixture.Password), "DEMO")
	var created struct{ Key string }
	mcpCall(t, session, "create_issue", map[string]any{"fields": map[string]any{
		"project": map[string]string{"key": "DEMO"}, "issuetype": map[string]string{"id": typeID}, "summary": "Créé par un agent"}}, &created)
	if created.Key == "" {
		t.Fatal("create_issue returned no key")
	}

	var before, after struct{ Transitions []struct{ ID string } }
	mcpCall(t, session, "get_transitions", map[string]any{"issueKey": created.Key}, &before)
	if len(before.Transitions) == 0 {
		t.Fatalf("no transition from the initial status of %s", created.Key)
	}
	taken := before.Transitions[0].ID
	if res := mcpRawCall(t, session, "transition_issue", map[string]any{"issueKey": created.Key,
		"transition": map[string]string{"id": taken}}); res.IsError {
		t.Fatalf("transition_issue: %s", mcpResultText(res))
	}
	var got struct {
		Fields struct {
			Status struct{ ID string }
		}
		Changelog struct {
			Histories []struct {
				Author struct{ Name string }
				Items  []struct{ Field string }
			}
		}
	}
	mcpCall(t, session, "get_issue", map[string]any{"issueKey": created.Key, "fields": []string{"status"},
		"expand": []string{"changelog"}}, &got)
	recorded := false
	for _, h := range got.Changelog.Histories {
		for _, it := range h.Items {
			if it.Field == "status" {
				recorded = h.Author.Name == "paul.martin"
			}
		}
	}
	if !recorded {
		t.Errorf("history of %s: %+v, want the status change by paul.martin", created.Key, got.Changelog)
	}

	// A transition the issue's status does not offer fails as on REST, and
	// changes nothing.
	mcpCall(t, session, "get_transitions", map[string]any{"issueKey": created.Key}, &after)
	forbidden := "999999"
	for _, tr := range before.Transitions {
		if !slices.ContainsFunc(after.Transitions, func(a struct{ ID string }) bool { return a.ID == tr.ID }) {
			forbidden = tr.ID
		}
	}
	body := map[string]any{"transition": map[string]string{"id": forbidden}}
	status, rest := restBasicAs(t, "paul.martin")("POST", "/rest/api/2/issue/"+created.Key+"/transitions", body)
	res := mcpRawCall(t, session, "transition_issue", map[string]any{"issueKey": created.Key, "transition": body["transition"]})
	if want := str(rest, "errors", "transition"); status != 400 || want == "" || !res.IsError || mcpResultText(res) != want {
		t.Errorf("forbidden transition %s: MCP %q, REST %d %v", forbidden, mcpResultText(res), status, rest)
	}
	var now struct {
		Fields struct{ Status struct{ ID string } }
	}
	mcpCall(t, session, "get_issue", map[string]any{"issueKey": created.Key, "fields": []string{"status"}}, &now)
	if now.Fields.Status.ID != got.Fields.Status.ID {
		t.Errorf("status after a forbidden transition: %s, want %s", now.Fields.Status.ID, got.Fields.Status.ID)
	}

	// A read token may not call a write tool, and is told the scope.
	reader := mcpSession(t, mcpToken(t, "paul.martin", auth.ScopeRead))
	res = mcpRawCall(t, reader, "delete_issue", map[string]any{"issueKey": created.Key})
	if !res.IsError || !strings.Contains(mcpResultText(res), string(auth.ScopeIssues)) {
		t.Errorf("read token calls delete_issue: %s", mcpResultText(res))
	}
	if res := mcpRawCall(t, session, "edit_issue", map[string]any{"issueKey": created.Key,
		"fields": map[string]string{"summary": "Modifié par un agent"}}); res.IsError {
		t.Errorf("edit_issue: %s", mcpResultText(res))
	}
}

// add_attachment takes base64 content, up to the same limit as an upload.
func TestMCPAttachmentLimit(t *testing.T) {
	needDB(t)
	srv := api.New(testDB, t.TempDir(), time.UTC, quiet())
	srv.SetMaxUpload(64 << 10)
	ts := httptest.NewServer(srv.Handler(http.NotFoundHandler()))
	t.Cleanup(ts.Close)
	paul := loginAt(t, ts.URL, "paul.martin", jirafixture.Password)
	key := createIssue(t, paul, "DEMO", unique("mcp attach"), "")
	session := mcpSessionAt(t, ts.URL, mcpToken(t, "paul.martin", auth.ScopeIssues))

	var added []struct {
		Filename string
		Size     int
	}
	mcpCall(t, session, "add_attachment", map[string]any{"issueKey": key, "filename": "petit.txt",
		"content": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 10<<10)))}, &added)
	if len(added) != 1 || added[0].Filename != "petit.txt" || added[0].Size != 10<<10 {
		t.Errorf("add_attachment: %+v", added)
	}
	res := mcpRawCall(t, session, "add_attachment", map[string]any{"issueKey": key, "filename": "gros.txt",
		"content": base64.StdEncoding.EncodeToString([]byte(strings.Repeat("b", 100<<10)))})
	if !res.IsError || mcpResultText(res) != "File too large (100 MB at most)." {
		t.Errorf("add_attachment over the limit: %s", mcpResultText(res))
	}
	if n := storedFiles(t, key); n != "1" {
		t.Errorf("%s attachments stored, want 1", n)
	}

	// Under the default limit, a file whose base64 is over the 4 MiB of an
	// MCP request without issues:write.
	big := base64.StdEncoding.EncodeToString(make([]byte, 5<<20))
	mcpCall(t, mcpSession(t, mcpToken(t, "paul.martin", auth.ScopeIssues)), "add_attachment",
		map[string]any{"issueKey": key, "filename": "grand.bin", "content": big}, &added)
	if len(added) != 1 || added[0].Size != 5<<20 {
		t.Errorf("add_attachment of 5 MiB: %+v", added)
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
	return mcpSessionAt(t, testURL, token)
}

func mcpSessionAt(t *testing.T, base, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "solstice-tests", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: base + "/mcp",
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
