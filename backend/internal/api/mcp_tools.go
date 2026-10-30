package api

// The tools of the MCP endpoint. Each one runs a handler of the REST API on
// a request rewritten from its arguments, so that what an agent sees and may
// do stays exactly what the REST API allows.

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"solstice/internal/auth"
	"solstice/internal/issue"
)

// mcpTool is a tool and the REST operation it runs.
type mcpTool struct {
	name, desc string
	scope      auth.Scope
	method     string // GET if empty
	api        string // issue.RESTAPIPath if empty
	path       string // under api; {v} stands for the path value v
	h          func(*Server, http.ResponseWriter, *http.Request) error
	params     []mcpParam
	schema     *jsonschema.Schema
	resolved   *jsonschema.Resolved
}

// mcpParam is an argument of a tool: a query parameter of the REST
// operation, its path value pathValue, or a field of its JSON body.
type mcpParam struct {
	name, kind, desc string // kind: string, integer, boolean or list (of strings)
	required         bool
	pathValue        string
	body             bool
}

// Arguments shared by several tools.
var (
	mcpIssueKey   = mcpParam{name: "issueKey", kind: "string", desc: "Issue key (DEMO-12) or id.", required: true, pathValue: "key"}
	mcpProjectKey = mcpParam{name: "projectKey", kind: "string", desc: "Project key or id.", required: true, pathValue: "key"}
	mcpStartAt    = mcpParam{name: "startAt", kind: "integer", desc: "Index of the first result, from 0."}
	mcpMaxResults = mcpParam{name: "maxResults", kind: "integer", desc: "Most results to return."}
	mcpFields     = mcpParam{name: "fields", kind: "list", desc: `Fields to return: ids or names, "*all", "*navigable", "-field" to leave one out.`}
	mcpExpand     = mcpParam{name: "expand", kind: "list", desc: "Expansions, such as renderedFields, names, changelog, transitions."}
	mcpBoardID    = mcpParam{name: "boardId", kind: "integer", desc: "Board id.", required: true, pathValue: "id"}
	mcpSprintID   = mcpParam{name: "sprintId", kind: "integer", desc: "Sprint id.", required: true, pathValue: "id"}
	mcpEpicKey    = mcpParam{name: "epicKey", kind: "string", desc: "Epic key (DEMO-3) or id.", required: true, pathValue: "key"}
	mcpJQL        = mcpParam{name: "jql", kind: "string", desc: "JQL the issues must also match."}
	mcpIssueList  = mcpParam{name: "issues", kind: "list", desc: "Issue keys or ids, at most 50.", required: true, body: true}
	mcpRankBefore = mcpParam{name: "rankBeforeIssue", kind: "string", desc: "Put the issues just before this one in the board order.", body: true}
	mcpRankAfter  = mcpParam{name: "rankAfterIssue", kind: "string", desc: "Put the issues just after this one in the board order.", body: true}
)

func mcpRead(name, path string, h func(*Server, http.ResponseWriter, *http.Request) error, desc string, params ...mcpParam) mcpTool {
	return mcpTool{name: name, desc: desc, scope: auth.ScopeRead, path: path, h: h, params: params}
}

// mcpAgile is a tool of the Agile REST API: a read, or a change that needs
// agile:write.
func mcpAgile(name, method, path string, h func(*Server, http.ResponseWriter, *http.Request) error, desc string, params ...mcpParam) mcpTool {
	scope := auth.ScopeRead
	if method != http.MethodGet {
		scope = auth.ScopeAgile
	}
	return mcpTool{name: name, desc: desc, scope: scope, method: method, api: issue.AgileAPIPath, path: path, h: h,
		params: params}
}

// mcpTools are the tools of the endpoint.
var mcpTools = []mcpTool{
	mcpRead("search_issues", "/search", (*Server).restSearch,
		"Issues matching a JQL query, a page at a time (50 by default, at most 1000), with the total count. "+
			"Returns the navigable fields unless fields says otherwise.",
		mcpParam{name: "jql", kind: "string", desc: `JQL query, e.g. project = DEMO AND status = "In Progress" ORDER BY updated DESC.`},
		mcpStartAt, mcpMaxResults, mcpFields, mcpExpand),
	mcpRead("get_issue", "/issue/{key}", (*Server).restGetIssue,
		"One issue with all its fields (or those of fields), its comments, work log, links and attachments.",
		mcpIssueKey, mcpFields, mcpExpand),
	mcpRead("pick_issues", "/issue/picker", (*Server).restIssuePicker,
		"Issues whose key or summary matches a text, as the issue picker suggests them.",
		mcpParam{name: "query", kind: "string", desc: "Text to match."},
		mcpParam{name: "currentJQL", kind: "string", desc: "JQL the suggestions must also match."},
		mcpParam{name: "currentIssueKey", kind: "string", desc: "Issue to leave out."},
		mcpParam{name: "currentProjectId", kind: "string", desc: "Project to search first."},
		mcpParam{name: "showSubTasks", kind: "boolean", desc: "Include sub-tasks (default true)."},
		mcpParam{name: "showSubTaskParent", kind: "boolean", desc: "Include the parent of currentIssueKey (default true)."}),
	mcpRead("get_transitions", "/issue/{key}/transitions", (*Server).restTransitions,
		"Workflow transitions the user can take on an issue now, with the status each leads to.",
		mcpIssueKey,
		mcpParam{name: "transitionId", kind: "string", desc: "Only this transition."},
		mcpParam{name: "expand", kind: "list", desc: "transitions.fields for the fields of each transition's screen."}),
	mcpRead("list_comments", "/issue/{key}/comment", (*Server).restComments,
		"Comments of an issue, oldest first, a page at a time.",
		mcpIssueKey, mcpStartAt, mcpMaxResults,
		mcpParam{name: "orderBy", kind: "string", desc: "created, or -created for newest first."}),
	mcpRead("get_comment", "/issue/{key}/comment/{id}", (*Server).restComment,
		"One comment of an issue.",
		mcpIssueKey, mcpParam{name: "commentId", kind: "string", desc: "Comment id.", required: true, pathValue: "id"}),
	mcpRead("list_worklogs", "/issue/{key}/worklog", (*Server).restWorklogs,
		"Work logged on an issue: author, time spent, start, comment.",
		mcpIssueKey, mcpStartAt, mcpMaxResults),
	mcpRead("get_worklog", "/issue/{key}/worklog/{id}", (*Server).restWorklog,
		"One work log entry of an issue.",
		mcpIssueKey, mcpParam{name: "worklogId", kind: "string", desc: "Work log id.", required: true, pathValue: "id"}),
	mcpRead("get_watchers", "/issue/{key}/watchers", (*Server).restWatchers,
		"Who watches an issue, and whether the user does.",
		mcpIssueKey),
	mcpRead("list_remote_links", "/issue/{key}/remotelink", (*Server).restRemoteLinks,
		"Links from an issue to outside pages: title, address, relationship.",
		mcpIssueKey, mcpParam{name: "globalId", kind: "string", desc: "Only the link with this global id."}),
	mcpRead("get_remote_link", "/issue/{key}/remotelink/{linkId}", (*Server).restRemoteLink,
		"One remote link of an issue.",
		mcpIssueKey, mcpParam{name: "linkId", kind: "string", desc: "Remote link id.", required: true, pathValue: "linkId"}),
	mcpRead("get_attachment", "/attachment/{id}", (*Server).restAttachment,
		"An attachment's name, size, type, author and download address.",
		mcpParam{name: "attachmentId", kind: "string", desc: "Attachment id.", required: true, pathValue: "id"}),
	mcpRead("get_create_meta", "/issue/createmeta", (*Server).restCreateMeta,
		"Projects and issue types the user can create issues in; with expand projects.issuetypes.fields, their fields and allowed values.",
		mcpParam{name: "projectKeys", kind: "list", desc: "Only these projects, by key."},
		mcpParam{name: "projectIds", kind: "list", desc: "Only these projects, by id."},
		mcpParam{name: "issuetypeIds", kind: "list", desc: "Only these issue types, by id."},
		mcpParam{name: "issuetypeNames", kind: "list", desc: "Only these issue types, by name."},
		mcpParam{name: "expand", kind: "list", desc: "projects.issuetypes.fields for the fields."}),
	mcpRead("get_edit_meta", "/issue/{key}/editmeta", (*Server).restEditMeta,
		"Fields of an issue the user can edit, with their allowed values.",
		mcpIssueKey),
	mcpRead("get_my_permissions", "/mypermissions", (*Server).restMyPermissions,
		"Which permissions the user has, globally or in a project or an issue.",
		mcpParam{name: "projectKey", kind: "string", desc: "In this project."},
		mcpParam{name: "projectId", kind: "string", desc: "In this project, by id."},
		mcpParam{name: "issueKey", kind: "string", desc: "On this issue."},
		mcpParam{name: "issueId", kind: "string", desc: "On this issue, by id."}),

	mcpRead("list_projects", "/project", (*Server).restProjects,
		"Projects the user can see: id, key, name, type.",
	),
	mcpRead("get_project", "/project/{key}", (*Server).restProject,
		"One project: key, name, description, lead, issue types.",
		mcpProjectKey),
	mcpRead("get_project_statuses", "/project/{key}/statuses", (*Server).restProjectStatuses,
		"The statuses each issue type of a project can have.",
		mcpProjectKey),

	mcpRead("get_myself", "/myself", (*Server).restMyself,
		"The signed-in user: user name, display name, e-mail, time zone, language.",
	),
	mcpRead("get_user", "/user", (*Server).restUser,
		"One user by user name or Jira user key: display name, e-mail, active.",
		mcpParam{name: "username", kind: "string", desc: "User name."},
		mcpParam{name: "key", kind: "string", desc: "Jira user key."}),
	mcpRead("search_users", "/user/search", (*Server).restUserSearch,
		"Active users whose user name, display name or e-mail matches a text.",
		mcpParam{name: "username", kind: "string", desc: "Text to match.", required: true},
		mcpStartAt, mcpMaxResults),
	mcpRead("search_assignable_users", "/user/assignable/search", (*Server).restAssignable,
		"Users who may be assigned the issues of a project, or one issue.",
		mcpParam{name: "project", kind: "string", desc: "Project key."},
		mcpParam{name: "issueKey", kind: "string", desc: "Issue key."},
		mcpParam{name: "username", kind: "string", desc: "Text to match."},
		mcpStartAt, mcpMaxResults),
	mcpRead("search_groups", "/groups/picker", (*Server).restGroupPicker,
		"Groups whose name contains a text.",
		mcpParam{name: "query", kind: "string", desc: "Text to match."},
		mcpParam{name: "exclude", kind: "list", desc: "Group names to leave out."},
		mcpMaxResults),
	mcpRead("list_group_members", "/group/member", (*Server).restGroupMembers,
		"Members of a group, a page at a time (at most 50).",
		mcpParam{name: "groupname", kind: "string", desc: "Group name.", required: true},
		mcpParam{name: "includeInactiveUsers", kind: "boolean", desc: "Include deactivated users."},
		mcpStartAt, mcpMaxResults),

	mcpRead("list_fields", "/field", (*Server).restFields,
		"Every field, system and custom: id, name, type, whether it can be searched and sorted on.",
	),
	mcpRead("list_issue_types", "/issuetype", (*Server).restIssueTypes, "Every issue type: id, name, whether a sub-task."),
	mcpRead("list_statuses", "/status", (*Server).restStatuses, "Every status: id, name, category."),
	mcpRead("list_priorities", "/priority", (*Server).restPriorities, "Every priority: id, name, colour."),
	mcpRead("list_resolutions", "/resolution", (*Server).restResolutions, "Every resolution: id, name."),
	mcpRead("list_issue_link_types", "/issueLinkType", (*Server).restLinkTypes,
		"Kinds of issue links, with their inward and outward names."),
	mcpRead("get_server_info", "/serverInfo", (*Server).restServerInfo, "This server's version, base URL and time."),

	mcpAgile("list_boards", "GET", "/board", (*Server).agileBoards,
		"Boards the user can see: id, name, type (scrum or kanban), project.",
		mcpParam{name: "projectKeyOrId", kind: "string", desc: "Only the boards of this project."},
		mcpParam{name: "type", kind: "string", desc: "scrum or kanban."},
		mcpParam{name: "name", kind: "string", desc: "Only boards whose name contains this text."},
		mcpStartAt, mcpMaxResults),
	mcpAgile("get_board", "GET", "/board/{id}", (*Server).agileBoard,
		"One board: name, type, project.", mcpBoardID),
	mcpAgile("get_board_configuration", "GET", "/board/{id}/configuration", (*Server).agileBoardConfiguration,
		"A board's columns and their statuses, its filter and its estimation field.", mcpBoardID),
	mcpAgile("list_sprints", "GET", "/board/{id}/sprint", (*Server).agileBoardSprints,
		"Sprints of a Scrum board, closed first, then active, then future.",
		mcpBoardID, mcpParam{name: "state", kind: "string", desc: "Only these states, comma-separated: active, future, closed."},
		mcpStartAt, mcpMaxResults),
	mcpAgile("list_board_issues", "GET", "/board/{id}/issue", (*Server).agileBoardIssues,
		"Issues of a board, in board order, a page at a time (50 by default, at most 1000).",
		mcpBoardID, mcpJQL, mcpStartAt, mcpMaxResults, mcpFields, mcpExpand),
	mcpAgile("get_backlog", "GET", "/board/{id}/backlog", (*Server).agileBacklog,
		"Issues in a board's backlog, in board order: not done and in no open sprint of the board.",
		mcpBoardID, mcpJQL, mcpStartAt, mcpMaxResults, mcpFields, mcpExpand),
	mcpAgile("get_sprint", "GET", "/sprint/{id}", (*Server).agileSprint,
		"One sprint: name, state, goal, dates, board.", mcpSprintID),
	mcpAgile("list_sprint_issues", "GET", "/sprint/{id}/issue", (*Server).agileSprintIssues,
		"Issues in a sprint, in board order.",
		mcpSprintID, mcpJQL, mcpStartAt, mcpMaxResults, mcpFields, mcpExpand),
	mcpAgile("get_epic", "GET", "/epic/{key}", (*Server).agileEpic,
		"One epic: key, name, summary, whether done.", mcpEpicKey),
	mcpAgile("list_epic_issues", "GET", "/epic/{key}/issue", (*Server).agileEpicIssues,
		"Issues of an epic, in board order.",
		mcpEpicKey, mcpJQL, mcpStartAt, mcpMaxResults, mcpFields, mcpExpand),
	mcpAgile("update_sprint", "POST", "/sprint/{id}", (*Server).agileEditSprint,
		"Changes a sprint; what is not given stays. state active starts a future sprint, closed completes "+
			"the active one and sends its open issues back to the backlog. Returns the sprint.",
		mcpSprintID,
		mcpParam{name: "name", kind: "string", desc: "New name.", body: true},
		mcpParam{name: "goal", kind: "string", desc: "New goal.", body: true},
		mcpParam{name: "startDate", kind: "string", desc: "Planned start, ISO 8601; empty clears it.", body: true},
		mcpParam{name: "endDate", kind: "string", desc: "Planned end, ISO 8601; empty clears it.", body: true},
		mcpParam{name: "state", kind: "string", desc: "active or closed.", body: true}),
	mcpAgile("delete_sprint", "DELETE", "/sprint/{id}", (*Server).agileDeleteSprint,
		"Deletes a sprint that has not started; its issues go back to the backlog.", mcpSprintID),
	mcpAgile("move_issues_to_sprint", "POST", "/sprint/{id}/issue", (*Server).agileMoveToSprint,
		"Puts issues in an open sprint, and ranks them if rankBeforeIssue or rankAfterIssue is given.",
		mcpSprintID, mcpIssueList, mcpRankBefore, mcpRankAfter),
	mcpAgile("move_issues_to_backlog", "POST", "/backlog/issue", (*Server).agileMoveToBacklog,
		"Takes issues out of their open sprint.", mcpIssueList),
	mcpAgile("rank_issues", "PUT", "/issue/rank", (*Server).agileRank,
		"Moves issues, in the given order, just before or just after another one in the board order. "+
			"Give exactly one of rankBeforeIssue and rankAfterIssue.",
		mcpIssueList, mcpRankBefore, mcpRankAfter),
}

func init() {
	for i := range mcpTools {
		t := &mcpTools[i]
		t.schema = &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{},
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}}}
		for _, p := range t.params {
			ps := &jsonschema.Schema{Type: p.kind, Description: p.desc}
			if p.kind == "list" {
				ps.Type, ps.Items = "array", &jsonschema.Schema{Type: "string"}
			}
			t.schema.Properties[p.name] = ps
			t.schema.PropertyOrder = append(t.schema.PropertyOrder, p.name)
			if p.required {
				t.schema.Required = append(t.schema.Required, p.name)
			}
		}
		var err error
		if t.resolved, err = t.schema.Resolve(nil); err != nil {
			panic(fmt.Sprintf("tool %s: %v", t.name, err))
		}
	}
}

func (t *mcpTool) tool() *mcp.Tool {
	return &mcp.Tool{Name: t.name, Description: t.desc, InputSchema: t.schema,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: t.scope == auth.ScopeRead, OpenWorldHint: new(false)}}
}

// mcpCall runs tool t for the request r of the MCP endpoint.
func (s *Server) mcpCall(r *http.Request, t *mcpTool) mcp.ToolHandler {
	return func(_ context.Context, call *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := map[string]any{}
		if len(call.Params.Arguments) > 0 {
			if err := json.Unmarshal(call.Params.Arguments, &args); err != nil {
				return mcpText(true, "Arguments: "+err.Error()), nil
			}
		}
		if err := t.resolved.Validate(args); err != nil {
			return mcpText(true, "Arguments: "+err.Error()), nil
		}
		path, query := t.path, url.Values{}
		values, body := map[string]string{}, map[string]any{}
		for _, p := range t.params {
			v, ok := args[p.name]
			if !ok {
				continue
			}
			switch list, isList := v.([]any); {
			case p.body:
				body[p.name] = v
			case p.pathValue != "":
				s := mcpScalar(v)
				values[p.pathValue] = s
				path = strings.Replace(path, "{"+p.pathValue+"}", url.PathEscape(s), 1)
			case isList:
				for _, e := range list {
					query.Add(p.name, e.(string))
				}
			default:
				query.Set(p.name, mcpScalar(v))
			}
		}
		method, api := cmp.Or(t.method, http.MethodGet), cmp.Or(t.api, issue.RESTAPIPath)
		// r carries the user and lasts the whole call: a stateless POST is
		// answered once its tools are done.
		req := r.Clone(r.Context())
		req.Method, req.URL = method, &url.URL{Path: api + path, RawQuery: query.Encode()}
		req.RequestURI, req.Pattern = req.URL.RequestURI(), method+" "+api+t.path
		req.Body, req.ContentLength = http.NoBody, 0
		if method != http.MethodGet && method != http.MethodDelete {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			req.Body, req.ContentLength = io.NopCloser(bytes.NewReader(b)), int64(len(b))
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range values {
			req.SetPathValue(k, v)
		}
		rec := &recorder{header: http.Header{}}
		err := t.h(s, rec, req)
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		if err := boundary(s.log, "MCP tool failed", req, err); err != nil {
			return mcpError(req, err), nil
		}
		// Changes Jira answers with 204 No Content.
		return mcpText(false, cmp.Or(rec.body.String(), "Done.")), nil
	}
}

// mcpScalar is a string, boolean or number argument as text.
func mcpScalar(v any) string {
	switch v := v.(type) {
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return v.(string)
}

func mcpText(isError bool, text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: isError, Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}
