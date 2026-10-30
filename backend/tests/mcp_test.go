package tests

// Tests of the MCP endpoint, through the SDK's client. They need
// SOLSTICE_TEST_DB and are skipped without it.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
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

	// The metadata names the resource and every scope but admin, which no
	// tool needs.
	var meta struct {
		Resource        string   `json:"resource"`
		ScopesSupported []string `json:"scopes_supported"`
	}
	anonymous(t).call("GET", "/.well-known/oauth-protected-resource", nil, 200, &meta)
	if !strings.HasSuffix(meta.Resource, "/mcp") || len(meta.ScopesSupported) != len(auth.Scopes)-1 {
		t.Errorf("metadata: %+v", meta)
	}
	for _, sc := range auth.Scopes {
		if slices.Contains(meta.ScopesSupported, string(sc)) == (sc == auth.ScopeAdmin) {
			t.Errorf("metadata scopes: %v", meta.ScopesSupported)
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

// A comments:write token comments but can't log work; a worklogs:write token
// logs work, and the issue's time tracking follows.
func TestMCPCommentsWorklogs(t *testing.T) {
	needDB(t)
	key := createIssue(t, login(t, "paul.martin", jirafixture.Password), "DEMO", unique("mcp comments"), "")

	commenter := mcpSession(t, mcpToken(t, "paul.martin", auth.ScopeComments))
	if tools := mcpToolNames(t, commenter); !slices.Contains(tools, "add_comment") || slices.Contains(tools, "add_worklog") {
		t.Errorf("comments:write token's tools: %v", tools)
	}
	var c struct {
		ID, Body string
		Author   struct{ Name string }
	}
	mcpCall(t, commenter, "add_comment", map[string]any{"issueKey": key, "body": "Vu par un agent."}, &c)
	if c.ID == "" || c.Body != "Vu par un agent." || c.Author.Name != "paul.martin" {
		t.Fatalf("add_comment: %+v", c)
	}
	mcpCall(t, commenter, "edit_comment", map[string]any{"issueKey": key, "commentId": c.ID, "body": "Corrigé par un agent."}, &c)
	if c.Body != "Corrigé par un agent." {
		t.Errorf("edit_comment: %+v", c)
	}
	if res := mcpRawCall(t, commenter, "delete_comment", map[string]any{"issueKey": key, "commentId": c.ID}); res.IsError {
		t.Errorf("delete_comment: %s", mcpResultText(res))
	}
	var comments struct{ Total int }
	mcpCall(t, commenter, "list_comments", map[string]any{"issueKey": key}, &comments)
	if comments.Total != 0 {
		t.Errorf("%d comments left after delete_comment", comments.Total)
	}
	res := mcpRawCall(t, commenter, "add_worklog", map[string]any{"issueKey": key, "timeSpent": "1h"})
	if !res.IsError || !strings.Contains(mcpResultText(res), string(auth.ScopeWorklogs)) {
		t.Errorf("comments:write token calls add_worklog: %s", mcpResultText(res))
	}

	if status, e := restBasicAs(t, "paul.martin")("PUT", "/rest/api/2/issue/"+key,
		map[string]any{"fields": map[string]any{"timetracking": map[string]string{"originalEstimate": "4h"}}}); status != 204 {
		t.Fatalf("set the estimate of %s: %d %v", key, status, e)
	}
	logger := mcpSession(t, mcpToken(t, "paul.martin", auth.ScopeWorklogs))
	tracking := func() (spent, remaining int) {
		t.Helper()
		var got struct {
			Fields struct{ Timespent, Timeestimate int }
		}
		mcpCall(t, logger, "get_issue", map[string]any{"issueKey": key, "fields": []string{"timespent", "timeestimate"}}, &got)
		return got.Fields.Timespent, got.Fields.Timeestimate
	}
	var wl struct {
		ID               string
		TimeSpentSeconds int
		Comment          string
	}
	mcpCall(t, logger, "add_worklog", map[string]any{"issueKey": key, "timeSpent": "1h 30m", "comment": "Analyse"}, &wl)
	if wl.ID == "" || wl.TimeSpentSeconds != 5400 || wl.Comment != "Analyse" {
		t.Fatalf("add_worklog: %+v", wl)
	}
	if spent, remaining := tracking(); spent != 5400 || remaining != 9000 {
		t.Errorf("after add_worklog: spent %d, remaining %d; want 5400, 9000", spent, remaining)
	}
	mcpCall(t, logger, "edit_worklog", map[string]any{"issueKey": key, "worklogId": wl.ID, "timeSpentSeconds": 7200}, &wl)
	if wl.TimeSpentSeconds != 7200 || wl.Comment != "Analyse" {
		t.Errorf("edit_worklog: %+v", wl)
	}
	if spent, remaining := tracking(); spent != 7200 || remaining != 7200 {
		t.Errorf("after edit_worklog: spent %d, remaining %d; want 7200, 7200", spent, remaining)
	}
	if res := mcpRawCall(t, logger, "delete_worklog", map[string]any{"issueKey": key, "worklogId": wl.ID}); res.IsError {
		t.Errorf("delete_worklog: %s", mcpResultText(res))
	}
	if spent, _ := tracking(); spent != 0 {
		t.Errorf("after delete_worklog: spent %d, want 0", spent)
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

// A projects:write token manages the versions and components of the projects
// its owner administers, and only those.
func TestMCPVersionsComponents(t *testing.T) {
	needDB(t)
	// paul.martin leads WEB, which makes him its administrator, but not DEMO's.
	reader := mcpSession(t, mcpToken(t, "paul.martin", auth.ScopeRead))
	if tools := mcpToolNames(t, reader); slices.Contains(tools, "create_version") || !slices.Contains(tools, "list_versions") {
		t.Errorf("read token's tools: %v", tools)
	}
	if res := mcpRawCall(t, reader, "create_version", map[string]any{"project": "WEB", "name": "9.2-mcp-read"}); !res.IsError ||
		!strings.Contains(mcpResultText(res), string(auth.ScopeProjects)) {
		t.Errorf("read token calls create_version: %s", mcpResultText(res))
	}

	session := mcpSession(t, mcpToken(t, "paul.martin", auth.ScopeProjects))
	var v struct{ ID, Name, ReleaseDate string }
	mcpCall(t, session, "create_version", map[string]any{"project": "WEB", "name": "9.2-mcp", "releaseDate": "2026-12-01"}, &v)
	if v.ID == "" || v.Name != "9.2-mcp" || v.ReleaseDate != "2026-12-01" {
		t.Fatalf("create_version: %+v", v)
	}
	var released struct{ Released bool }
	mcpCall(t, session, "update_version", map[string]any{"versionId": v.ID, "released": true}, &released)
	if !released.Released {
		t.Errorf("update_version: not released")
	}
	var list []struct{ ID string }
	mcpCall(t, reader, "list_versions", map[string]any{"projectKey": "WEB"}, &list)
	if !slices.ContainsFunc(list, func(e struct{ ID string }) bool { return e.ID == v.ID }) {
		t.Errorf("list_versions of WEB: %+v, want %s", list, v.ID)
	}
	if res := mcpRawCall(t, session, "delete_version", map[string]any{"versionId": v.ID}); res.IsError {
		t.Errorf("delete_version: %s", mcpResultText(res))
	}
	if res := mcpRawCall(t, reader, "get_version", map[string]any{"versionId": v.ID}); !res.IsError {
		t.Errorf("get_version after delete_version: %s", mcpResultText(res))
	}

	var c struct {
		ID, Name, AssigneeType string
		Lead                   struct{ Name string }
	}
	mcpCall(t, session, "create_component", map[string]any{"project": "WEB", "name": "mcp-composant",
		"leadUserName": "paul.martin", "assigneeType": "COMPONENT_LEAD"}, &c)
	if c.ID == "" || c.Lead.Name != "paul.martin" || c.AssigneeType != "COMPONENT_LEAD" {
		t.Fatalf("create_component: %+v", c)
	}
	if res := mcpRawCall(t, session, "delete_component", map[string]any{"componentId": c.ID}); res.IsError {
		t.Errorf("delete_component: %s", mcpResultText(res))
	}

	// The same scope does not reach a project its owner does not administer.
	status, _ := restBasicAs(t, "paul.martin")("POST", "/rest/api/2/version", map[string]any{"project": "DEMO", "name": "9.2-mcp-paul"})
	res := mcpRawCall(t, session, "create_version", map[string]any{"project": "DEMO", "name": "9.2-mcp-paul"})
	if status != 403 || !res.IsError {
		t.Errorf("create_version in DEMO: MCP %s, REST %d", mcpResultText(res), status)
	}
	var demo []struct{ Name string }
	mcpCall(t, reader, "list_versions", map[string]any{"projectKey": "DEMO"}, &demo)
	if slices.ContainsFunc(demo, func(e struct{ Name string }) bool { return e.Name == "9.2-mcp-paul" }) {
		t.Errorf("refused create_version still added a version to DEMO")
	}
}
