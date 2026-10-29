package api

// The tools of the MCP endpoint. Each one runs a handler of the REST API on
// a GET rewritten from its arguments, so that what an agent sees and may do
// stays exactly what the REST API allows.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	path       string // under /rest/api/2; {v} stands for the path value v
	h          func(*Server, http.ResponseWriter, *http.Request) error
	params     []mcpParam
	schema     *jsonschema.Schema
	resolved   *jsonschema.Resolved
}

// mcpParam is an argument of a tool: a query parameter of the REST
// operation, or its path value pathValue.
type mcpParam struct {
	name, kind, desc string // kind: string, integer, boolean or list (of strings)
	required         bool
	pathValue        string
}

// Arguments shared by several tools.
var (
	mcpIssueKey   = mcpParam{name: "issueKey", kind: "string", desc: "Issue key (DEMO-12) or id.", required: true, pathValue: "key"}
	mcpProjectKey = mcpParam{name: "projectKey", kind: "string", desc: "Project key or id.", required: true, pathValue: "key"}
	mcpStartAt    = mcpParam{name: "startAt", kind: "integer", desc: "Index of the first result, from 0."}
	mcpMaxResults = mcpParam{name: "maxResults", kind: "integer", desc: "Most results to return."}
	mcpFields     = mcpParam{name: "fields", kind: "list", desc: `Fields to return: ids or names, "*all", "*navigable", "-field" to leave one out.`}
	mcpExpand     = mcpParam{name: "expand", kind: "list", desc: "Expansions, such as renderedFields, names, changelog, transitions."}
)

func mcpRead(name, path string, h func(*Server, http.ResponseWriter, *http.Request) error, desc string, params ...mcpParam) mcpTool {
	return mcpTool{name: name, desc: desc, scope: auth.ScopeRead, path: path, h: h, params: params}
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
		values := map[string]string{}
		for _, p := range t.params {
			v, ok := args[p.name]
			if !ok {
				continue
			}
			if p.pathValue != "" {
				s := v.(string)
				values[p.pathValue] = s
				path = strings.Replace(path, "{"+p.pathValue+"}", url.PathEscape(s), 1)
				continue
			}
			switch v := v.(type) {
			case string:
				query.Set(p.name, v)
			case bool:
				query.Set(p.name, strconv.FormatBool(v))
			case float64:
				query.Set(p.name, strconv.FormatFloat(v, 'f', -1, 64))
			case []any:
				for _, e := range v {
					query.Add(p.name, e.(string))
				}
			}
		}
		// r carries the user and lasts the whole call: a stateless POST is
		// answered once its tools are done.
		req := r.Clone(r.Context())
		req.Method, req.URL = http.MethodGet, &url.URL{Path: issue.RESTAPIPath + path, RawQuery: query.Encode()}
		req.RequestURI, req.Pattern = req.URL.RequestURI(), "GET "+issue.RESTAPIPath+t.path
		req.Body, req.ContentLength = http.NoBody, 0
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
		return mcpText(false, rec.body.String()), nil
	}
}

func mcpText(isError bool, text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: isError, Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}
