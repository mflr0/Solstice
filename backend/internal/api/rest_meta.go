package api

// REST API v2: the server, users, projects, catalogues, fields, saved
// filters and permissions.

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"html"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// The version of Jira's REST API that Solstice answers like: clients test it
// to know what they may call.
var jiraVersionNumbers = []int{9, 12, 0}

func (s *Server) restServerInfo(w http.ResponseWriter, r *http.Request) error {
	o := s.restOut(r)
	version := fmt.Sprintf("%d.%d.%d", jiraVersionNumbers[0], jiraVersionNumbers[1], jiraVersionNumbers[2])
	return writeJSON(w, http.StatusOK, map[string]any{
		"baseUrl": o.base, "version": version, "versionNumbers": jiraVersionNumbers, "deploymentType": "Server",
		"buildNumber": 912000, "buildDate": "2026-01-01T00:00:00.000+0000", "serverTime": o.time(time.Now()),
		"scmInfo": "", "serverTitle": "Solstice",
	})
}

// fullUser is a user as /myself and /user show one. Like people's pages in
// the interface, it lists groups only to the user themselves and to
// administrators.
func (s *Server) fullUser(ctx context.Context, o *restOut, id int64, withGroups bool) (map[string]any, error) {
	users, err := s.jiraUsers(ctx, o, id)
	if err != nil {
		return nil, err
	}
	u, ok := users[id]
	if !ok {
		return nil, errNotFound
	}
	var locale sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT locale FROM users WHERE id = ?`, id).Scan(&locale); err != nil {
		return nil, err
	}
	groups := []any{}
	if err := sqlq.Each(ctx, s.db, `SELECT g.name FROM group_members m JOIN user_groups g ON g.id = m.group_id
		WHERE m.user_id = ? AND ? ORDER BY g.name`, func(rows *sql.Rows) error {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		groups = append(groups, map[string]string{"name": name, "self": o.api + "/group?groupname=" + url.QueryEscape(name)})
		return nil
	}, id, withGroups); err != nil {
		return nil, err
	}
	out := map[string]any{"self": u.Self, "key": u.Key, "name": u.Name, "emailAddress": u.EmailAddress,
		"avatarUrls": u.AvatarURLs, "displayName": u.DisplayName, "active": u.Active, "timeZone": u.TimeZone,
		"groups":           map[string]any{"size": len(groups), "items": groups},
		"applicationRoles": map[string]any{"size": 0, "items": []any{}}, "expand": "groups,applicationRoles"}
	if locale.Valid {
		out["locale"] = locale.String
	}
	return out, nil
}

func (s *Server) restMyself(w http.ResponseWriter, r *http.Request) error {
	out, err := s.fullUser(r.Context(), s.restOut(r), currentUser(r).ID, true)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// restUser shows a user by name (?username=) or Jira key (?key=).
func (s *Server) restUser(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	name := q.Get("username")
	if name == "" {
		name = q.Get("key")
	}
	id, ok := s.userByName(r.Context(), strings.TrimSpace(name))
	if !ok || strings.TrimSpace(name) == "" {
		return errNotFound
	}
	me := currentUser(r)
	out, err := s.fullUser(r.Context(), s.restOut(r), id, me.Admin || me.ID == id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// userList answers a list of users selected by a query on users (alias u).
func (s *Server) userList(w http.ResponseWriter, r *http.Request, where string, args []any) error {
	startAt, maxResults := paging(r, 50, 1000)
	var ids []int64
	if err := sqlq.Each(r.Context(), s.db, `SELECT u.id FROM users u WHERE `+where+`
		ORDER BY u.display_name, u.username LIMIT ? OFFSET ?`, func(rows *sql.Rows) error {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
		return nil
	}, append(args, maxResults, startAt)...); err != nil {
		return err
	}
	users, err := s.jiraUsers(r.Context(), s.restOut(r), ids...)
	if err != nil {
		return err
	}
	out := []any{}
	for _, id := range ids {
		out = append(out, users[id])
	}
	return writeJSON(w, http.StatusOK, out)
}

// userMatch selects users whose name, display name or e-mail contains q ("."
// matches everyone), active or not as asked.
func userMatch(r *http.Request, q string) (string, []any) {
	params := r.URL.Query()
	active := params.Get("includeActive") != "false"
	inactive := params.Get("includeInactive") == "true"
	where := "((u.active AND ?) OR (NOT u.active AND ?))"
	args := []any{active, inactive}
	if q = strings.ToLower(strings.TrimSpace(q)); q != "" && q != "." {
		like := "%" + sqlq.EscapeLike(q) + "%"
		where += " AND (u.username LIKE ? OR LOWER(u.display_name) LIKE ? OR LOWER(u.email) LIKE ?)"
		args = append(args, like, like, like)
	}
	return where, args
}

func (s *Server) restUserSearch(w http.ResponseWriter, r *http.Request) error {
	where, args := userMatch(r, r.URL.Query().Get("username"))
	return s.userList(w, r, where, args)
}

// restAssignable lists who can be assigned issues of a project (?project=)
// or of an issue (?issueKey=), as the interface's assignee picker.
func (s *Server) restAssignable(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	var projectID int64
	if key := q.Get("issueKey"); key != "" {
		iss, err := s.findIssue(ctx, key)
		if err != nil || !sub.CanSee(iss.perm()) {
			return errNotFound
		}
		projectID = iss.ProjectID
	} else if projectID, err = s.restProjectID(ctx, q.Get("project")); err != nil {
		return err
	}
	if !sub.CanBrowseProject(projectID) {
		return errNotFound
	}
	holders, hargs, err := s.perms.HoldersSQL(ctx, projectID, perm.Assignable)
	if err != nil {
		return err
	}
	where, args := userMatch(r, q.Get("username"))
	return s.userList(w, r, where+" AND u.active AND u.id IN ("+holders+")", append(args, hargs...))
}

// restProjectID finds a project by key, former key, or id.
func (s *Server) restProjectID(ctx context.Context, v string) (int64, error) {
	v = strings.TrimSpace(v)
	if id, err := strconv.ParseInt(v, 10, 64); err == nil {
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE id = ?`, id).Scan(&n); err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, errNotFound
		}
		return id, nil
	}
	return s.projectByKey(ctx, v)
}

// Projects ---------------------------------------------------------------------

func (s *Server) restProjects(w http.ResponseWriter, r *http.Request) error {
	var list []projectOut
	if err := s.uiCall(r, s.listProjects, http.MethodGet, "/api/projects", nil, nil, &list); err != nil {
		return err
	}
	o := s.restOut(r)
	out := []any{}
	for _, p := range list {
		jp := o.projectRef(p.ID, p.Key, p.Name, p.Kind)
		jp["expand"] = "description,lead,url,projectKeys"
		if p.Category != nil {
			jp["projectCategory"] = map[string]any{"name": *p.Category}
		}
		out = append(out, jp)
	}
	return writeJSON(w, http.StatusOK, out)
}

// projectDetail is what the interface API says of a project.
type projectDetail struct {
	Project         projectOut     `json:"project"`
	IssueTypes      []typeRef      `json:"issueTypes"`
	Components      []componentOut `json:"components"`
	Versions        []versionOut   `json:"versions"`
	DefaultAssignee string         `json:"defaultAssignee"`
}

// pathProject loads the project of the path (key or id) through the
// interface API, which checks that the user sees it.
func (s *Server) pathProject(r *http.Request) (*projectDetail, error) {
	id, err := s.restProjectID(r.Context(), r.PathValue("key"))
	if err != nil {
		return nil, err
	}
	var key string
	if err := s.db.QueryRowContext(r.Context(), `SELECT pkey FROM projects WHERE id = ?`, id).Scan(&key); err != nil {
		return nil, err
	}
	var p projectDetail
	err = s.uiCall(r, s.getProject, http.MethodGet, "/api/projects/"+key, map[string]string{"key": key}, nil, &p)
	return &p, err
}

func (s *Server) restProject(w http.ResponseWriter, r *http.Request) error {
	p, err := s.pathProject(r)
	if err != nil {
		return err
	}
	o := s.restOut(r)
	out := o.projectRef(p.Project.ID, p.Project.Key, p.Project.Name, p.Project.Kind)
	out["expand"] = "description,lead,url,projectKeys"
	out["description"] = ""
	if p.Project.Description != nil {
		out["description"] = *p.Project.Description
	}
	out["lead"] = nil
	if p.Project.Lead != nil {
		users, err := s.jiraUsers(r.Context(), o, p.Project.Lead.ID)
		if err != nil {
			return err
		}
		out["lead"] = userOf(users, p.Project.Lead.ID)
	}
	out["assigneeType"] = map[string]string{"lead": "PROJECT_LEAD"}[p.DefaultAssignee]
	if out["assigneeType"] == "" {
		out["assigneeType"] = "UNASSIGNED"
	}
	types := []any{}
	for _, t := range p.IssueTypes {
		types = append(types, o.issueType(t))
	}
	out["issueTypes"] = types
	out["components"] = o.components(p)
	out["versions"] = o.versions(p)
	out["projectKeys"] = []string{p.Project.Key}
	out["roles"] = map[string]any{}
	out["archived"] = false
	if p.Project.Category != nil {
		out["projectCategory"] = map[string]any{"name": *p.Project.Category}
	}
	return writeJSON(w, http.StatusOK, out)
}

func (o *restOut) components(p *projectDetail) []any {
	out := []any{}
	for _, c := range p.Components {
		out = append(out, map[string]any{"self": o.api + "/component/" + o.id(c.ID), "id": o.id(c.ID), "name": c.Name,
			"assigneeType": "PROJECT_DEFAULT", "isAssigneeTypeValid": false, "project": p.Project.Key,
			"projectId": p.Project.ID, "archived": c.Archived})
	}
	return out
}

func (o *restOut) versions(p *projectDetail) []any {
	out := []any{}
	for _, v := range p.Versions {
		jv := map[string]any{"self": o.api + "/version/" + o.id(v.ID), "id": o.id(v.ID), "name": v.Name,
			"archived": v.Archived, "released": v.Released, "projectId": p.Project.ID}
		if v.ReleaseDate != nil {
			jv["releaseDate"] = *v.ReleaseDate
		}
		out = append(out, jv)
	}
	return out
}

func (s *Server) restProjectParts(which string) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		p, err := s.pathProject(r)
		if err != nil {
			return err
		}
		o := s.restOut(r)
		if which == "versions" {
			return writeJSON(w, http.StatusOK, o.versions(p))
		}
		return writeJSON(w, http.StatusOK, o.components(p))
	}
}

// restProjectStatuses lists, for each issue type of a project, the statuses
// its workflow uses.
func (s *Server) restProjectStatuses(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	p, err := s.pathProject(r)
	if err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	o := s.restOut(r)
	out := []any{}
	for _, t := range p.IssueTypes {
		statuses, err := s.typeStatuses(ctx, cat, p.Project.ID, t.ID)
		if err != nil {
			return err
		}
		list := []any{}
		for _, st := range statuses {
			if st.ID != "" {
				list = append(list, o.status(st))
			}
		}
		jt := o.issueType(t)
		jt["statuses"] = list
		out = append(out, jt)
	}
	return writeJSON(w, http.StatusOK, out)
}

// Catalogues -------------------------------------------------------------------

func (s *Server) restPriorities(w http.ResponseWriter, r *http.Request) error {
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return err
	}
	o := s.restOut(r)
	out := []any{}
	for _, p := range cat.Priorities {
		out = append(out, o.priority(&p))
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) restStatuses(w http.ResponseWriter, r *http.Request) error {
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return err
	}
	o := s.restOut(r)
	out := []any{}
	for _, st := range cat.Statuses {
		out = append(out, o.status(st))
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) restIssueTypes(w http.ResponseWriter, r *http.Request) error {
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return err
	}
	o := s.restOut(r)
	out := []any{}
	for _, t := range cat.IssueTypes {
		out = append(out, o.issueType(t))
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) restResolutions(w http.ResponseWriter, r *http.Request) error {
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return err
	}
	o := s.restOut(r)
	out := []any{}
	for _, v := range cat.Resolutions {
		out = append(out, o.resolution(&v))
	}
	return writeJSON(w, http.StatusOK, out)
}

// restFields lists the system fields Solstice keeps and every custom field.
func (s *Server) restFields(w http.ResponseWriter, r *http.Request) error {
	fields, err := s.customFields(r.Context())
	if err != nil {
		return err
	}
	o := s.restOut(r)
	out := []any{}
	for _, f := range systemFields {
		jf := map[string]any{"id": f.id, "name": o.msg(f.name), "custom": false, "orderable": f.orderable(),
			"navigable": f.navigable, "searchable": len(f.clauses) > 0, "clauseNames": f.clauses}
		if f.typ != "" {
			jf["schema"] = systemSchema(f)
		}
		out = append(out, jf)
	}
	for _, f := range fields {
		out = append(out, map[string]any{"id": customID(f.ID), "name": f.Name, "custom": true, "orderable": true,
			"navigable": true, "searchable": true, "schema": customFieldSchema(f),
			"clauseNames": []string{"cf[" + strconv.FormatInt(f.ID, 10) + "]", f.Name}})
	}
	return writeJSON(w, http.StatusOK, out)
}

func fieldSchema(typ, items string, who map[string]any) map[string]any {
	who["type"] = typ
	if items != "" {
		who["items"] = items
	}
	return who
}

func systemSchema(f sysField) map[string]any {
	return fieldSchema(f.typ, f.items, map[string]any{"system": f.id})
}

func customFieldSchema(f customFieldDef) map[string]any {
	typ, items := customSchema(f.Kind)
	return fieldSchema(typ, items, map[string]any{"custom": f.TypeKey, "customId": f.ID})
}

// customSchema gives the schema type of a custom field kind.
func customSchema(kind string) (typ, items string) {
	switch kind {
	case "float":
		return "number", ""
	case "datepicker":
		return "date", ""
	case "datetime":
		return "datetime", ""
	case "select", "radiobuttons", "gh-epic-status":
		return "option", ""
	case "cascadingselect":
		return "option-with-child", ""
	case "multiselect", "multicheckboxes":
		return "array", "option"
	case "userpicker":
		return "user", ""
	case "multiuserpicker":
		return "array", "user"
	case "labels":
		return "array", "string"
	case "gh-sprint":
		return "array", "string"
	case "gh-epic-link":
		return "any", ""
	}
	return "string", ""
}

// Saved filters ----------------------------------------------------------------

func (s *Server) jiraFilter(ctx context.Context, o *restOut, f filterOut) (map[string]any, error) {
	var owner any
	if f.Owner != nil {
		users, err := s.jiraUsers(ctx, o, f.Owner.ID)
		if err != nil {
			return nil, err
		}
		owner = userOf(users, f.Owner.ID)
	}
	shares := []any{}
	for _, sh := range f.Shares {
		var p map[string]any
		switch sh.Kind {
		case "everyone":
			p = map[string]any{"type": "global"}
		case "group":
			p = map[string]any{"type": "group", "group": map[string]any{"name": sh.Name}}
		case "project":
			var key, name string
			s.db.QueryRowContext(ctx, `SELECT pkey, name FROM projects WHERE id = ?`, sh.ID).Scan(&key, &name)
			proj := map[string]any{"id": o.id(sh.ID), "key": key, "name": name, "self": o.api + "/project/" + o.id(sh.ID)}
			p = map[string]any{"type": "project", "project": proj}
			if sh.Role != 0 {
				var role string
				s.db.QueryRowContext(ctx, `SELECT name FROM project_roles WHERE id = ?`, sh.Role).Scan(&role)
				p = map[string]any{"type": "projectRole", "project": proj, "role": map[string]any{"id": sh.Role, "name": role}}
			}
		case "user":
			users, err := s.jiraUsers(ctx, o, sh.ID)
			if err != nil {
				return nil, err
			}
			p = map[string]any{"type": "user", "user": userOf(users, sh.ID)}
		default:
			continue
		}
		shares = append(shares, p)
	}
	out := map[string]any{"self": o.api + "/filter/" + o.id(f.ID), "id": o.id(f.ID), "name": f.Name,
		"owner": owner, "jql": f.Query, "viewUrl": o.base + "/issues/?filter=" + o.id(f.ID),
		"searchUrl": o.api + "/search?jql=" + url.QueryEscape(f.Query), "favourite": f.Favourite,
		"sharePermissions": shares, "subscriptions": map[string]any{"size": 0, "items": []any{}}}
	if f.Description != nil {
		out["description"] = *f.Description
	}
	return out, nil
}

func (s *Server) restFilter(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	// loadFilters reads id 0 as "every filter".
	if err != nil || id <= 0 {
		return errNotFound
	}
	rows, err := s.loadFilters(r.Context(), sub, id)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return errNotFound
	}
	out, err := s.jiraFilter(r.Context(), s.restOut(r), rows[0].out)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) restFavouriteFilters(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	rows, err := s.loadFilters(r.Context(), sub, 0)
	if err != nil {
		return err
	}
	o := s.restOut(r)
	out := []any{}
	for _, f := range rows {
		if !f.out.Favourite {
			continue
		}
		jf, err := s.jiraFilter(r.Context(), o, f.out)
		if err != nil {
			return err
		}
		out = append(out, jf)
	}
	return writeJSON(w, http.StatusOK, out)
}

// Permissions ------------------------------------------------------------------

// jiraPermissions maps Jira's permission keys to Solstice's permissions
// (Solstice has fewer, broader ones). "global" is the global
// administrator's right; an empty permission is one Solstice never grants.
var jiraPermissions = []struct {
	key  string
	id   int
	perm string
}{
	{"BROWSE_PROJECTS", 10, perm.Browse}, {"BROWSE", 10, perm.Browse}, {"CREATE_ISSUES", 11, perm.Create},
	{"EDIT_ISSUES", 12, perm.Edit}, {"SCHEDULE_ISSUES", 28, perm.Edit}, {"ADD_COMMENTS", 15, perm.Comment},
	{"EDIT_OWN_COMMENTS", 35, perm.Comment}, {"DELETE_OWN_COMMENTS", 37, perm.Comment},
	{"TRANSITION_ISSUES", 46, perm.Transition}, {"RESOLVE_ISSUES", 14, perm.Transition},
	{"CLOSE_ISSUES", 18, perm.Transition}, {"ASSIGN_ISSUES", 13, perm.Assign},
	{"ASSIGNABLE_USER", 17, perm.Assignable}, {"CREATE_ATTACHMENTS", 19, perm.Attach},
	{"DELETE_OWN_ATTACHMENTS", 39, perm.Attach}, {"LINK_ISSUES", 21, perm.Link}, {"WORK_ON_ISSUES", 20, perm.Worklog},
	{"DELETE_ISSUES", 16, perm.Delete}, {"EDIT_ALL_COMMENTS", 34, perm.Moderate},
	{"DELETE_ALL_COMMENTS", 36, perm.Moderate}, {"DELETE_ALL_ATTACHMENTS", 38, perm.Moderate},
	{"MANAGE_WATCHERS", 32, perm.Watchers}, {"VIEW_VOTERS_AND_WATCHERS", 31, perm.Watchers},
	{"SET_ISSUE_SECURITY", 26, perm.Security}, {"ADMINISTER_PROJECTS", 23, perm.Admin},
	{"ADMINISTER", 0, "global"}, {"SYSTEM_ADMIN", 44, "global"}, {"MODIFY_REPORTER", 30, ""}, {"MOVE_ISSUES", 25, ""},
}

// permissionNames name Solstice's permissions, as the project settings do.
var permissionNames = map[string]i18n.Message{
	perm.Browse: i18n.M(i18n.PermissionBrowse), perm.Create: i18n.M(i18n.PermissionCreate), perm.Edit: i18n.M(i18n.PermissionEdit),
	perm.Comment: i18n.M(i18n.PermissionComment), perm.Transition: i18n.M(i18n.PermissionTransition), perm.Assign: i18n.M(i18n.PermissionAssign),
	perm.Assignable: i18n.M(i18n.PermissionAssignable), perm.Attach: i18n.M(i18n.PermissionAttach), perm.Link: i18n.M(i18n.PermissionLink),
	perm.Worklog: i18n.M(i18n.PermissionWorklog), perm.Delete: i18n.M(i18n.PermissionDelete), perm.Moderate: i18n.M(i18n.PermissionModerate),
	perm.Watchers: i18n.M(i18n.PermissionWatchers), perm.Security: i18n.M(i18n.PermissionSecurity),
	perm.Admin: i18n.M(i18n.PermissionAdmin), "global": i18n.M(i18n.PermissionGlobalAdmin),
	"": i18n.M(i18n.PermissionUnavailable),
}

// restMyPermissions tells which permissions the user has: in a project
// (projectKey, projectId), on an issue (issueKey, issueId), or in at least
// one project.
func (s *Server) restMyPermissions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	var has func(p string) bool
	switch {
	case q.Get("issueKey") != "" || q.Get("issueId") != "":
		r.SetPathValue("key", q.Get("issueKey")+q.Get("issueId"))
		iss, err := s.restIssue(r, sub)
		if err != nil {
			return err
		}
		pi := iss.perm()
		has = func(p string) bool { return sub.Can(p, iss.ProjectID, &pi) }
	case q.Get("projectKey") != "" || q.Get("projectId") != "":
		id, err := s.restProjectID(ctx, q.Get("projectKey")+q.Get("projectId"))
		if err != nil || !sub.CanBrowseProject(id) {
			return errNotFound
		}
		has = func(p string) bool { return sub.Can(p, id, nil) }
	default:
		var projects []int64
		if err := sqlq.Each(ctx, s.db, `SELECT id FROM projects`, func(rows *sql.Rows) error {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			projects = append(projects, id)
			return nil
		}); err != nil {
			return err
		}
		has = func(p string) bool {
			return slices.ContainsFunc(projects, func(id int64) bool { return sub.Can(p, id, nil) })
		}
	}
	o := s.restOut(r)
	out := map[string]any{}
	for _, jp := range jiraPermissions {
		granted := false
		switch jp.perm {
		case "global":
			granted = sub.Admin
		case "":
		default:
			granted = has(jp.perm)
		}
		typ := "PROJECT"
		if jp.perm == "global" {
			typ = "GLOBAL"
		}
		out[jp.key] = map[string]any{"id": strconv.Itoa(jp.id), "key": jp.key, "name": o.msg(permissionNames[jp.perm]),
			"type": typ, "description": "", "havePermission": granted}
	}
	return writeJSON(w, http.StatusOK, map[string]any{"permissions": out})
}

// Avatars ----------------------------------------------------------------------

// People's pictures are in avatars.go. A project has none: its avatar is its
// initials on a colour drawn from the key.
var avatarColours = []string{"#8a5a44", "#5f7a61", "#4f6d7a", "#7a6a4f", "#6d5a7a", "#7a4f5a", "#4f7a72", "#6b6b4f"}

// avatarSize is the side, in pixels, a client asks for with ?size=.
func avatarSize(r *http.Request) int {
	if size := map[string]int{"xsmall": 16, "small": 24, "medium": 32}[r.URL.Query().Get("size")]; size != 0 {
		return size
	}
	return 48
}

// avatarSVG draws the initials of label on a colour drawn from seed.
func avatarSVG(r *http.Request, seed, label string) []byte {
	size := avatarSize(r)
	var initials []rune
	for word := range strings.FieldsSeq(label) {
		if rs := []rune(word); len(rs) > 0 && len(initials) < 2 && (unicode.IsLetter(rs[0]) || unicode.IsDigit(rs[0])) {
			initials = append(initials, unicode.ToUpper(rs[0]))
		}
	}
	h := fnv.New32a()
	h.Write([]byte(seed))
	colour := avatarColours[int(h.Sum32()%uint32(len(avatarColours)))]
	return fmt.Appendf(nil, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 48 48">`+
		`<rect width="48" height="48" rx="6" fill="%s"/><text x="24" y="24" dy=".35em" text-anchor="middle" `+
		`font-family="sans-serif" font-size="19" fill="#fff">%s</text></svg>`, size, size, colour,
		html.EscapeString(string(initials)))
}

func writeAvatar(w http.ResponseWriter, r *http.Request, seed, label string) error {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	_, err := w.Write(avatarSVG(r, seed, label))
	return err
}

// userAvatar answers Jira's address for a user's picture, with the same
// picture or initials as /api/avatars; a name that is no account gets initials.
func (s *Server) userAvatar(w http.ResponseWriter, r *http.Request) error {
	name := strings.TrimSpace(r.URL.Query().Get("ownerId"))
	if id, ok := s.userByName(r.Context(), name); ok {
		return s.serveUserAvatar(w, r, id)
	}
	serveInitials(w, r, name)
	return nil
}

func (s *Server) projectAvatar(w http.ResponseWriter, r *http.Request) error {
	key := strings.TrimSpace(r.URL.Query().Get("ownerId"))
	return writeAvatar(w, r, key, key)
}
