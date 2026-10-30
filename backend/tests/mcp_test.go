package tests

// Tests of the MCP endpoint, through the SDK's client. They need
// SOLSTICE_TEST_DB and are skipped without it.

import (
	"context"
	"encoding/json"
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

func TestMCPAgile(t *testing.T) {
	needDB(t)
	marie := login(t, "marie.durand", jirafixture.Password)
	board, sprint := scopeBoard(t, marie, "WEB")
	boardID, _ := strconv.ParseInt(board, 10, 64)
	writes := []string{"update_sprint", "delete_sprint", "move_issues_to_sprint", "move_issues_to_backlog", "rank_issues"}

	// A read token lists boards and sprints, but no tool that changes them.
	reader := mcpSession(t, mcpToken(t, "marie.durand", auth.ScopeRead))
	if tools := mcpToolNames(t, reader); slices.ContainsFunc(writes, func(w string) bool { return slices.Contains(tools, w) }) {
		t.Errorf("read token lists agile writes: %v", tools)
	}
	var boards struct{ Values []struct{ ID int64 } }
	mcpCall(t, reader, "list_boards", map[string]any{"projectKeyOrId": "WEB", "type": "scrum"}, &boards)
	if !slices.ContainsFunc(boards.Values, func(b struct{ ID int64 }) bool { return b.ID == boardID }) {
		t.Errorf("list_boards: %+v, want board %d", boards, boardID)
	}
	var sprints struct {
		Values []struct {
			ID    int64
			State string
		}
	}
	mcpCall(t, reader, "list_sprints", map[string]any{"boardId": boardID}, &sprints)
	if len(sprints.Values) != 1 || sprints.Values[0].ID != sprint || sprints.Values[0].State != "future" {
		t.Errorf("list_sprints: %+v, want future sprint %d", sprints, sprint)
	}
	res := mcpRawCall(t, reader, "update_sprint", map[string]any{"sprintId": sprint, "state": "active"})
	if !res.IsError || !strings.Contains(mcpResultText(res), "agile:write") {
		t.Errorf("read token calls update_sprint: %s", mcpResultText(res))
	}

	// An agile:write token starts the sprint, fills it and ranks its issues.
	writer := mcpSession(t, mcpToken(t, "marie.durand", auth.ScopeAgile))
	var started struct{ State, StartDate string }
	mcpCall(t, writer, "update_sprint", map[string]any{"sprintId": sprint, "state": "active",
		"endDate": "2999-01-01T00:00:00.000+0000"}, &started)
	if started.State != "active" || started.StartDate == "" {
		t.Errorf("update_sprint: %+v", started)
	}
	first, second := createIssue(t, marie, "WEB", "Planifié par un agent", ""), createIssue(t, marie, "WEB", "Classé par un agent", "")
	if res := mcpRawCall(t, writer, "move_issues_to_sprint", map[string]any{"sprintId": sprint, "issues": []string{first, second}}); res.IsError {
		t.Fatalf("move_issues_to_sprint: %s", mcpResultText(res))
	}
	if res := mcpRawCall(t, writer, "rank_issues", map[string]any{"issues": []string{second}, "rankBeforeIssue": first}); res.IsError {
		t.Fatalf("rank_issues: %s", mcpResultText(res))
	}
	var inSprint struct{ Issues []struct{ Key string } }
	mcpCall(t, reader, "list_sprint_issues", map[string]any{"sprintId": sprint}, &inSprint)
	if len(inSprint.Issues) != 2 || inSprint.Issues[0].Key != second || inSprint.Issues[1].Key != first {
		t.Errorf("list_sprint_issues: %+v, want %s then %s", inSprint, second, first)
	}
	// A rank needs exactly one anchor, as on REST.
	if res := mcpRawCall(t, writer, "rank_issues", map[string]any{"issues": []string{second}}); !res.IsError {
		t.Errorf("rank_issues without an anchor: %s", mcpResultText(res))
	}
}
