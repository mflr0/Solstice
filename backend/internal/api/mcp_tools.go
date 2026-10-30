package api

// The tools of the MCP endpoint. Each one runs a handler of the REST API on
// a request rewritten from its arguments, so that what an agent sees and may
// do stays exactly what the REST API allows.

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
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
	method     string // GET when empty
	path       string // under /rest/api/2; {v} stands for the path value v
	h          func(*Server, http.ResponseWriter, *http.Request) error
	params     []mcpParam
	// encode builds the request body from the arguments; by default, the
	// JSON object of the body arguments.
	encode   func(args map[string]any) (body io.ReadCloser, contentType string, err error)
	schema   *jsonschema.Schema
	resolved *jsonschema.Resolved
}

// mcpParam is an argument of a tool: a query parameter of the REST
// operation, its path value pathValue, or a field of its JSON body.
type mcpParam struct {
	name, kind, desc string // kind: string, integer, boolean, object or list (of strings)
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
)

func mcpRead(name, path string, h func(*Server, http.ResponseWriter, *http.Request) error, desc string, params ...mcpParam) mcpTool {
	return mcpTool{name: name, desc: desc, scope: auth.ScopeRead, path: path, h: h, params: params}
}

func mcpWrite(name, method, path string, h func(*Server, http.ResponseWriter, *http.Request) error, desc string,
	params ...mcpParam) mcpTool {
	return mcpTool{name: name, desc: desc, scope: auth.ScopeIssues, method: method, path: path, h: h, params: params}
}

// mcpOn is the handler of a REST operation that adds (on) or removes.
func mcpOn(h func(*Server, bool) handler, on bool) func(*Server, http.ResponseWriter, *http.Request) error {
	return func(s *Server, w http.ResponseWriter, r *http.Request) error { return h(s, on)(w, r) }
}

// mcpBody is an argument sent in the JSON body.
func mcpBody(name, kind, desc string) mcpParam {
	return mcpParam{name: name, kind: kind, desc: desc, body: true}
}

// Arguments of the tools that create or change issues.
var (
	mcpEditFields = mcpBody("fields", "object",
		`Field values by id or name, as in Jira: {"summary": "…", "priority": {"name": "High"}, "assignee": {"name": "jdupont"}}. `+
			"get_create_meta and get_edit_meta list the fields and their allowed values.")
	mcpEditUpdate = mcpBody("update", "object",
		`Operations per field, as in Jira: {"labels": [{"add": "urgent"}], "comment": [{"add": {"body": "…"}}]}.`)
	mcpRemoteLinkBody = []mcpParam{
		mcpBody("globalId", "string", "Identifier of the linked page; a second add with the same one updates that link."),
		mcpBody("relationship", "string", `How the issue relates to the page, e.g. "mentioned in".`),
		{name: "object", kind: "object", desc: `The page: {"url": "https://…", "title": "…"}.`, required: true, body: true},
	}
)

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

	mcpWrite("create_issue", http.MethodPost, "/issue", (*Server).restCreateIssue,
		`Creates an issue. fields needs project ({"key": "DEMO"}), issuetype ({"name": "Bug"}) and summary. `+
			"Returns the new issue's id and key.",
		mcpEditFields, mcpEditUpdate),
	mcpWrite("edit_issue", http.MethodPut, "/issue/{key}", (*Server).restUpdateIssue,
		"Changes the fields of an issue: fields sets values, update adds, removes or sets them one by one.",
		mcpIssueKey, mcpEditFields, mcpEditUpdate),
	mcpWrite("delete_issue", http.MethodDelete, "/issue/{key}", (*Server).restDeleteIssue,
		"Deletes an issue for good, with its comments, work log and attachments.",
		mcpIssueKey, mcpParam{name: "deleteSubtasks", kind: "boolean", desc: "Also delete its sub-tasks; without it, an issue that has some is kept."}),
	mcpWrite("assign_issue", http.MethodPut, "/issue/{key}/assignee", (*Server).restAssign,
		"Assigns an issue to a user, to nobody, or to the project's default assignee.",
		mcpIssueKey, mcpBody("name", "string", `User name; "-1" for the project's default assignee; leave it out for nobody.`)),
	mcpWrite("transition_issue", http.MethodPost, "/issue/{key}/transitions", (*Server).restTransition,
		"Moves an issue along its workflow, setting the fields of the transition's screen. "+
			"get_transitions lists the transitions the user can take.",
		mcpIssueKey,
		mcpParam{name: "transition", kind: "object", desc: `The transition: {"id": "21"} or {"name": "Start progress"}.`, required: true, body: true},
		mcpEditFields, mcpEditUpdate),
	mcpWrite("add_watcher", http.MethodPost, "/issue/{key}/watchers", mcpOn((*Server).restWatch, true),
		"Makes the user, or another one, watch an issue.",
		mcpIssueKey, mcpParam{name: "username", kind: "string", desc: "Who; the user by default."}),
	mcpWrite("remove_watcher", http.MethodDelete, "/issue/{key}/watchers", mcpOn((*Server).restWatch, false),
		"Stops the user, or another one, watching an issue.",
		mcpIssueKey, mcpParam{name: "username", kind: "string", desc: "Who; the user by default."}),
	mcpWrite("add_vote", http.MethodPost, "/issue/{key}/votes", mcpOn((*Server).restVote, true),
		"Votes for an issue.", mcpIssueKey),
	mcpWrite("remove_vote", http.MethodDelete, "/issue/{key}/votes", mcpOn((*Server).restVote, false),
		"Takes the user's vote back from an issue.", mcpIssueKey),
	{name: "add_attachment", scope: auth.ScopeIssues, method: http.MethodPost, path: "/issue/{key}/attachments",
		h: (*Server).restAttach, encode: mcpAttachment,
		desc: "Attaches a file to an issue, up to the server's upload limit. Returns the attachment.",
		params: []mcpParam{mcpIssueKey,
			{name: "filename", kind: "string", desc: "File name.", required: true},
			{name: "content", kind: "string", desc: "The file's content, base64-encoded.", required: true},
			{name: "mimeType", kind: "string", desc: "Media type, e.g. image/png; application/octet-stream by default."}}},
	mcpWrite("delete_attachment", http.MethodDelete, "/attachment/{id}", (*Server).deleteAttachment,
		"Deletes an attachment.",
		mcpParam{name: "attachmentId", kind: "string", desc: "Attachment id.", required: true, pathValue: "id"}),
	mcpWrite("link_issues", http.MethodPost, "/issueLink", (*Server).restLink,
		`Links two issues; the inward issue is the source: type Blocks, inwardIssue A, outwardIssue B means "A blocks B". `+
			"list_issue_link_types lists the types.",
		mcpParam{name: "type", kind: "object", desc: `Link type: {"name": "Blocks"} or {"id": "10000"}.`, required: true, body: true},
		mcpParam{name: "inwardIssue", kind: "object", desc: `{"key": "DEMO-1"}`, required: true, body: true},
		mcpParam{name: "outwardIssue", kind: "object", desc: `{"key": "DEMO-2"}`, required: true, body: true},
		mcpBody("comment", "object", `Comment to add to the inward issue: {"body": "…"}.`)),
	mcpWrite("delete_issue_link", http.MethodDelete, "/issueLink/{id}", (*Server).deleteLink,
		"Removes a link between two issues; get_issue gives the ids of its links.",
		mcpParam{name: "linkId", kind: "string", desc: "Issue link id.", required: true, pathValue: "id"}),
	mcpWrite("add_remote_link", http.MethodPost, "/issue/{key}/remotelink", (*Server).restAddRemoteLink,
		"Links an issue to an outside page, or updates the link with the same globalId.",
		append([]mcpParam{mcpIssueKey}, mcpRemoteLinkBody...)...),
	mcpWrite("edit_remote_link", http.MethodPut, "/issue/{key}/remotelink/{linkId}", (*Server).restEditRemoteLink,
		"Changes a link from an issue to an outside page.",
		append([]mcpParam{mcpIssueKey, {name: "linkId", kind: "string", desc: "Remote link id.", required: true, pathValue: "linkId"}},
			mcpRemoteLinkBody...)...),
	mcpWrite("delete_remote_link", http.MethodDelete, "/issue/{key}/remotelink/{linkId}", (*Server).restDeleteRemoteLink,
		"Removes a link from an issue to an outside page.",
		mcpIssueKey, mcpParam{name: "linkId", kind: "string", desc: "Remote link id.", required: true, pathValue: "linkId"}),
	mcpWrite("delete_remote_link_by_global_id", http.MethodDelete, "/issue/{key}/remotelink", (*Server).restDeleteRemoteLink,
		"Removes the link from an issue to an outside page that has this global id.",
		mcpIssueKey, mcpParam{name: "globalId", kind: "string", desc: "Global id of the link.", required: true}),
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
	a := &mcp.ToolAnnotations{ReadOnlyHint: t.method == "", OpenWorldHint: new(false),
		IdempotentHint: t.method == http.MethodPut || t.method == http.MethodDelete}
	// A POST adds something, apart from a transition, which may overwrite fields.
	if t.method == http.MethodPost && t.name != "transition_issue" {
		a.DestructiveHint = new(false)
	}
	return &mcp.Tool{Name: t.name, Description: t.desc, InputSchema: t.schema, Annotations: a}
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
		var body map[string]any
		for _, p := range t.params {
			if p.body && body == nil {
				body = map[string]any{}
			}
			v, ok := args[p.name]
			if !ok {
				continue
			}
			if p.body {
				body[p.name] = v
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
		method := cmp.Or(t.method, http.MethodGet)
		// r carries the user and lasts the whole call: a stateless POST is
		// answered once its tools are done.
		req := r.Clone(r.Context())
		req.Method, req.URL = method, &url.URL{Path: issue.RESTAPIPath + path, RawQuery: query.Encode()}
		req.RequestURI, req.Pattern = req.URL.RequestURI(), method+" "+issue.RESTAPIPath+t.path
		req.Body, req.ContentLength = http.NoBody, 0
		req.Header = req.Header.Clone()
		req.Header.Del("Content-Type")
		switch {
		case t.encode != nil:
			b, contentType, err := t.encode(args)
			if err != nil {
				return mcpText(true, "Arguments: "+err.Error()), nil
			}
			// Closing it ends the writer of a body the handler did not read.
			defer b.Close()
			req.Body, req.ContentLength = b, -1
			req.Header.Set("Content-Type", contentType)
		case body != nil:
			raw, err := json.Marshal(body)
			if err != nil {
				return nil, err
			}
			req.Body, req.ContentLength = io.NopCloser(bytes.NewReader(raw)), int64(len(raw))
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
		if rec.body.Len() == 0 {
			return mcpText(false, "Done."), nil
		}
		return mcpText(false, rec.body.String()), nil
	}
}

// mcpAttachment encodes the arguments of add_attachment as the multipart
// body of an upload.
func mcpAttachment(args map[string]any) (io.ReadCloser, string, error) {
	content, err := base64.StdEncoding.DecodeString(args["content"].(string))
	if err != nil {
		return nil, "", fmt.Errorf("content: %w", err)
	}
	mimeType, _ := args["mimeType"].(string)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", mime.FormatMediaType("form-data",
		map[string]string{"name": "file", "filename": args["filename"].(string)}))
	h.Set("Content-Type", cmp.Or(mimeType, "application/octet-stream"))
	// Streamed: the file may weigh as much as the upload limit.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		part, err := mw.CreatePart(h)
		if err == nil {
			_, err = part.Write(content)
		}
		if err == nil {
			err = mw.Close()
		}
		pw.CloseWithError(err)
	}()
	return pr, mw.FormDataContentType(), nil
}

func mcpText(isError bool, text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: isError, Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}
