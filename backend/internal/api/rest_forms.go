package api

// REST API v2: the forms of issues. createmeta and editmeta tell clients
// which fields a form has before they create or edit an issue. They are built
// from what rest_write.go really accepts (editableFields, settableKind,
// checkCustomContext) and from the permissions applyUpdate and createIssue
// check, so that a field listed here can be set, and one that can be set is
// listed. The issue picker, which clients use for autocompletion, is here too.

import (
	"context"
	"database/sql"
	"errors"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// formProject is the project of a form.
type formProject struct {
	id        int64
	key, name string
	kind      string
	types     []typeRef // its issue types, in order
}

// formParts are the allowed values a project gives its forms: its components,
// versions and security levels, built once for all its issue types.
type formParts struct {
	components, versions, levels []any
}

// formContext is the context of a custom field: the projects and issue types
// it applies to, nil for all of them.
type formContext struct {
	projects map[int64]bool
	types    map[string]bool
}

// formBuilder describes fields for one request, keeping what several forms
// share (a project's components, a field's options and context) once loaded.
type formBuilder struct {
	s           *Server
	ctx         context.Context
	o           *restOut
	sub         *perm.Subject
	cat         *catalog
	customs     []customFieldDef
	options     map[int64][]fieldOption
	values      map[int64][]any // the allowed values of the fields with options
	parts       map[int64]*formParts
	contexts    map[int64]*formContext // loaded with the first form that needs it
	priorities  []any
	resolutions []any
}

func (s *Server) newFormBuilder(r *http.Request, sub *perm.Subject) (*formBuilder, error) {
	cat, err := s.catalogue(r.Context())
	if err != nil {
		return nil, err
	}
	customs, err := s.customFields(r.Context())
	if err != nil {
		return nil, err
	}
	b := &formBuilder{s: s, ctx: r.Context(), o: s.restOut(r), sub: sub, cat: cat, customs: customs,
		options: map[int64][]fieldOption{}, values: map[int64][]any{}, parts: map[int64]*formParts{}}
	b.priorities, b.resolutions = []any{}, []any{}
	for _, v := range cat.Priorities {
		b.priorities = append(b.priorities, b.o.priority(&v))
	}
	for _, v := range cat.Resolutions {
		b.resolutions = append(b.resolutions, b.o.resolution(&v))
	}
	return b, nil
}

// outside tells whether the context of a custom field excludes a project and
// issue type, as fieldsOutside does for the database, from contexts loaded
// once.
func (b *formBuilder) outside(f customFieldDef, projectID int64, typeID string) (bool, error) {
	if b.contexts == nil {
		contexts := map[int64]*formContext{}
		of := func(id int64) *formContext {
			if contexts[id] == nil {
				contexts[id] = &formContext{}
			}
			return contexts[id]
		}
		if err := sqlq.Each(b.ctx, b.s.db, `SELECT field_id, project_id FROM custom_field_projects`, func(rows *sql.Rows) error {
			var field, project int64
			if err := rows.Scan(&field, &project); err != nil {
				return err
			}
			if c := of(field); c.projects == nil {
				c.projects = map[int64]bool{project: true}
			} else {
				c.projects[project] = true
			}
			return nil
		}); err != nil {
			return false, err
		}
		if err := sqlq.Each(b.ctx, b.s.db, `SELECT field_id, issue_type_id FROM custom_field_issue_types`, func(rows *sql.Rows) error {
			var field int64
			var typ string
			if err := rows.Scan(&field, &typ); err != nil {
				return err
			}
			if c := of(field); c.types == nil {
				c.types = map[string]bool{typ: true}
			} else {
				c.types[typ] = true
			}
			return nil
		}); err != nil {
			return false, err
		}
		b.contexts = contexts
	}
	c := b.contexts[f.ID]
	if c == nil {
		return false, nil
	}
	return (c.projects != nil && !c.projects[projectID]) || (c.types != nil && !c.types[typeID]), nil
}

// projectTypes lists the issue types of a project, in order.
func (b *formBuilder) projectTypes(projectID int64) ([]typeRef, error) {
	var out []typeRef
	err := sqlq.Each(b.ctx, b.s.db, `SELECT issue_type_id FROM project_issue_types WHERE project_id = ? ORDER BY position`,
		func(rows *sql.Rows) error {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			out = append(out, b.cat.typeOf(id))
			return nil
		}, projectID)
	return out, err
}

func (b *formBuilder) partsOf(projectID int64) (*formParts, error) {
	if p, ok := b.parts[projectID]; ok {
		return p, nil
	}
	p := &formParts{components: []any{}, versions: []any{}, levels: []any{}}
	if err := sqlq.Each(b.ctx, b.s.db, `SELECT id, name, archived FROM components WHERE project_id = ? ORDER BY name`,
		func(rows *sql.Rows) error {
			var id int64
			var name string
			var archived bool
			if err := rows.Scan(&id, &name, &archived); err != nil {
				return err
			}
			p.components = append(p.components, map[string]any{"self": b.o.api + "/component/" + b.o.id(id), "id": b.o.id(id),
				"name": name, "archived": archived})
			return nil
		}, projectID); err != nil {
		return nil, err
	}
	if err := sqlq.Each(b.ctx, b.s.db, `SELECT id, name, released, archived, release_date FROM versions
		WHERE project_id = ? ORDER BY position, name`, func(rows *sql.Rows) error {
		var v versionData
		if err := rows.Scan(&v.ID, &v.Name, &v.Released, &v.Archived, &v.Release); err != nil {
			return err
		}
		p.versions = append(p.versions, b.o.version(v, projectID))
		return nil
	}, projectID); err != nil {
		return nil, err
	}
	if err := sqlq.Each(b.ctx, b.s.db, `SELECT id, name FROM security_levels WHERE project_id = ? ORDER BY name`,
		func(rows *sql.Rows) error {
			var v idName
			if err := rows.Scan(&v.ID, &v.Name); err != nil {
				return err
			}
			p.levels = append(p.levels, map[string]any{"self": b.o.api + "/securitylevel/" + b.o.id(v.ID),
				"id": b.o.id(v.ID), "name": v.Name, "description": ""})
			return nil
		}, projectID); err != nil {
		return nil, err
	}
	b.parts[projectID] = p
	return p, nil
}

func (b *formBuilder) optionsOf(f customFieldDef) ([]fieldOption, error) {
	if o, ok := b.options[f.ID]; ok {
		return o, nil
	}
	o, err := b.s.fieldOptions(b.ctx, f.ID)
	b.options[f.ID] = o
	return o, err
}

// optionValues lists the options a user may choose in a custom field: the
// enabled ones, a cascading field's children under their parent.
func (b *formBuilder) optionValues(f customFieldDef) ([]any, error) {
	if v, ok := b.values[f.ID]; ok {
		return v, nil
	}
	options, err := b.optionsOf(f)
	if err != nil {
		return nil, err
	}
	value := func(o fieldOption) map[string]any {
		return map[string]any{"self": b.o.api + "/customFieldOption/" + b.o.id(o.id), "value": o.value,
			"id": b.o.id(o.id), "disabled": false}
	}
	out := []any{}
	for _, o := range options {
		if o.parent != 0 || o.disabled {
			continue
		}
		v := value(o)
		if f.Kind == "cascadingselect" {
			children := []any{}
			for _, c := range options {
				if c.parent == o.id && !c.disabled {
					children = append(children, value(c))
				}
			}
			v["cascadingOptions"] = children
		}
		out = append(out, v)
	}
	b.values[f.ID] = out
	return out, nil
}

// fields describes the fields of the form for an issue type of a project: to
// create an issue (cur nil), or to edit cur. Only the fields the user may set
// are listed.
func (b *formBuilder) fields(p formProject, typ typeRef, cur *issueRow) (map[string]any, error) {
	creating := cur == nil
	subtask := typ.Subtask
	var asIssue *perm.Issue // for the permissions that look at the issue
	reporter := &perm.Issue{ProjectID: p.id, ReporterID: b.sub.UserID}
	if !creating {
		v := cur.perm()
		asIssue, reporter, subtask = &v, &v, cur.ParentID.Valid
	}
	can := func(permission string, is *perm.Issue) bool { return b.sub.Can(permission, p.id, is) }
	// Creating takes the permission to create, which the caller checked; the
	// custom fields, written once the issue exists, take that to edit it as
	// its reporter.
	canEdit := creating || can(perm.Edit, reporter)
	canEditCustom := can(perm.Edit, reporter)

	out := map[string]any{}
	system := func(id string, required bool, operations ...string) map[string]any {
		f, _ := systemField(id)
		schema := map[string]any{"type": f.typ, "system": f.id}
		if f.items != "" {
			schema["items"] = f.items
		}
		field := map[string]any{"required": required, "schema": schema, "name": b.o.msg(f.name), "key": id,
			"hasDefaultValue": false, "operations": operations}
		out[id] = field
		return field
	}

	if creating {
		system("project", true, "set")["allowedValues"] = []any{b.o.projectRef(p.id, p.key, p.name, p.kind)}
		system("issuetype", true, "set")["allowedValues"] = []any{b.o.issueType(typ)}
		if subtask {
			system("parent", true, "set")
		}
	} else if canEdit {
		// A sub-task only becomes another sub-task type, and the reverse.
		allowed := []any{}
		for _, t := range p.types {
			if t.Subtask == subtask {
				allowed = append(allowed, b.o.issueType(t))
			}
		}
		system("issuetype", true, "set")["allowedValues"] = allowed
	}

	parts, err := b.partsOf(p.id)
	if err != nil {
		return nil, err
	}
	permitted := func(id string) bool {
		switch id {
		case "assignee":
			return can(perm.Assign, asIssue) || can(perm.Assignable, asIssue)
		case "security":
			// A sub-task has its parent's level.
			return can(perm.Security, asIssue) && !subtask
		case "resolution":
			// Reaching a status that is not "done" clears it, and creating
			// ignores it.
			return !creating && can(perm.Transition, asIssue) && b.cat.statusOf(cur.StatusID).Category == issue.Done
		}
		return canEdit
	}
	for id := range editableFields {
		if id == "issuetype" || !permitted(id) {
			continue
		}
		operations := []string{"set"}
		switch id {
		case "labels", "components", "fixVersions", "versions":
			operations = []string{"add", "set", "remove"}
		}
		f := system(id, id == "summary", operations...)
		switch id {
		case "priority":
			f["hasDefaultValue"] = creating && len(b.cat.Priorities) > 0
			f["allowedValues"] = b.priorities
		case "resolution":
			f["allowedValues"] = b.resolutions
		case "components":
			f["allowedValues"] = parts.components
		case "fixVersions", "versions":
			f["allowedValues"] = parts.versions
		case "security":
			f["allowedValues"] = parts.levels
		case "assignee":
			f["autoCompleteUrl"] = b.o.api + "/user/assignable/search?project=" + p.key + "&username="
			if !creating {
				f["autoCompleteUrl"] = b.o.api + "/user/assignable/search?issueKey=" + cur.Key() + "&username="
			}
		}
	}
	if canEdit {
		system("timetracking", false, "set", "edit")
	}
	if can(perm.Comment, reporter) {
		system("comment", false, "add")
	}

	// Custom fields: those whose kind translateCustom takes and, but for the
	// epic link and the sprint, which checkCustomContext does not look at,
	// whose context holds this project and type.
	for _, f := range b.customs {
		if !settableKind(f.Kind) {
			continue
		}
		switch f.Kind {
		case "gh-epic-link":
			// Sub-tasks follow their parent's epic, and an epic is in none;
			// creating, which has no type to check yet, allows it.
			if subtask || (!creating && isEpicType(typ)) || !canEdit {
				continue
			}
		case "gh-sprint":
			if !canEditCustom {
				continue
			}
		default:
			if !canEditCustom {
				continue
			}
			outside, err := b.outside(f, p.id, typ.ID)
			if err != nil {
				return nil, err
			}
			if outside {
				continue
			}
		}
		stype, items := customSchema(f.Kind)
		schema := map[string]any{"type": stype, "custom": f.TypeKey, "customId": f.ID}
		if items != "" {
			schema["items"] = items
		}
		field := map[string]any{"required": false, "schema": schema, "name": f.Name, "key": customID(f.ID),
			"hasDefaultValue": false, "operations": []string{"set"}}
		switch f.Kind {
		case "select", "radiobuttons", "gh-epic-status", "multiselect", "multicheckboxes", "cascadingselect":
			if field["allowedValues"], err = b.optionValues(f); err != nil {
				return nil, err
			}
		case "userpicker", "multiuserpicker":
			field["autoCompleteUrl"] = b.o.api + "/user/search?username="
		}
		out[customID(f.ID)] = field
	}
	return out, nil
}

// restCreateMeta describes the forms for creating issues: the projects the
// user may create issues in, their issue types and, with
// expand=projects.issuetypes.fields, the fields of each. projectKeys,
// projectIds, issuetypeIds and issuetypeNames narrow the answer.
func (s *Server) restCreateMeta(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	b, err := s.newFormBuilder(r, sub)
	if err != nil {
		return err
	}
	keys, ids := expandSet(q["projectKeys"]...), expandSet(q["projectIds"]...)
	var wanted map[int64]bool
	if len(keys)+len(ids) > 0 {
		wanted = map[int64]bool{}
		for key := range keys {
			id, err := s.projectByKey(ctx, key)
			if errors.Is(err, errNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			wanted[id] = true
		}
		for v := range ids {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				wanted[id] = true
			}
		}
	}
	typeIDs, typeNames := expandSet(q["issuetypeIds"]...), expandSet(q["issuetypeNames"]...)
	typeWanted := func(t typeRef) bool {
		if len(typeIDs)+len(typeNames) == 0 || typeIDs[t.ID] {
			return true
		}
		for name := range typeNames {
			if strings.EqualFold(name, t.Name) {
				return true
			}
		}
		return false
	}
	withFields := expandSet(q["expand"]...)["projects.issuetypes.fields"]

	var projects []formProject
	if err := sqlq.Each(ctx, s.db, `SELECT id, pkey, name, kind FROM projects ORDER BY name, id`, func(rows *sql.Rows) error {
		var p formProject
		if err := rows.Scan(&p.id, &p.key, &p.name, &p.kind); err != nil {
			return err
		}
		// As createIssue asks: to create, and to see the project.
		if (wanted == nil || wanted[p.id]) && sub.Can(perm.Create, p.id, nil) && sub.CanBrowseProject(p.id) {
			projects = append(projects, p)
		}
		return nil
	}); err != nil {
		return err
	}
	out := []any{}
	for _, p := range projects {
		if p.types, err = b.projectTypes(p.id); err != nil {
			return err
		}
		types := []any{}
		for _, t := range p.types {
			if !typeWanted(t) {
				continue
			}
			jt := b.o.issueType(t)
			if withFields {
				jt["expand"] = "fields"
				if jt["fields"], err = b.fields(p, t, nil); err != nil {
					return err
				}
			}
			types = append(types, jt)
		}
		if len(types) == 0 {
			continue
		}
		jp := b.o.projectRef(p.id, p.key, p.name, p.kind)
		jp["expand"] = "issuetypes"
		jp["issuetypes"] = types
		out = append(out, jp)
	}
	return writeJSON(w, http.StatusOK, map[string]any{"expand": "projects", "projects": out})
}

// restEditMeta describes the fields the user may edit on an issue.
func (s *Server) restEditMeta(w http.ResponseWriter, r *http.Request) error {
	sub, iss, err := s.restSubjectIssue(r)
	if err != nil {
		return err
	}
	b, err := s.newFormBuilder(r, sub)
	if err != nil {
		return err
	}
	p := formProject{id: iss.ProjectID, key: iss.ProjectKey, name: iss.ProjectName}
	if p.types, err = b.projectTypes(p.id); err != nil {
		return err
	}
	fields, err := b.fields(p, b.cat.typeOf(iss.TypeID), iss)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{"fields": fields})
}

// Issue picker sizes, as Jira's: the history and the current search show a
// few issues each.
const (
	pickerHistory = 5
	pickerCurrent = 10
)

// highlight escapes text for HTML and bolds where it contains the query.
func highlight(text string, query *regexp.Regexp) string {
	if query == nil {
		return html.EscapeString(text)
	}
	var b strings.Builder
	last := 0
	for _, m := range query.FindAllStringIndex(text, -1) {
		b.WriteString(html.EscapeString(text[last:m[0]]))
		b.WriteString("<b>" + html.EscapeString(text[m[0]:m[1]]) + "</b>")
		last = m[1]
	}
	b.WriteString(html.EscapeString(text[last:]))
	return b.String()
}

// pickerKey is an issue key typed whole.
var pickerKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*-[0-9]+$`)

// restIssuePicker suggests issues to pick, in Jira's two sections: the
// history (hs) and the current search (cs). Solstice keeps no record of the
// issues people viewed: the history is the issues the user last changed or
// commented. Both only hold issues the user sees. The sections' label and sub
// texts are in the caller's language.
func (s *Server) restIssuePicker(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	q := r.URL.Query()
	query := strings.TrimSpace(q.Get("query"))

	// The condition every section shares: what the query matches, and what the
	// current issue and the sub-task options leave out.
	vis, args := sub.VisibleSQL("i")
	where := vis
	// The issue the query names whole comes first: a key, or the number of an
	// issue of the current project. Only characters and numbers that cannot
	// change the query are written into it.
	exact := ""
	if pickerKey.MatchString(query) {
		// Safe as text, as pickerKey allows only letters, digits, "_" and "-"; searchRows binds no argument for its ORDER BY.
		exact = "(CONCAT(p.pkey, '-', i.num) = '" + strings.ToUpper(query) + "') DESC, "
	}
	if query != "" {
		like := "%" + escapeLike(strings.ToLower(query)) + "%"
		cond := "LOWER(i.summary) LIKE ? OR CONCAT(p.pkey, '-', i.num) LIKE ?"
		more := []any{like, strings.ToUpper(like)}
		// A number alone is an issue of the current project.
		if num, err := strconv.ParseInt(query, 10, 64); err == nil {
			if project, err := strconv.ParseInt(q.Get("currentProjectId"), 10, 64); err == nil {
				cond += " OR (i.project_id = ? AND i.num = ?)"
				more = append(more, project, num)
				exact = "(i.project_id = " + strconv.FormatInt(project, 10) + " AND i.num = " + strconv.FormatInt(num, 10) + ") DESC, "
			}
		}
		where += " AND (" + cond + ")"
		args = append(args, more...)
	}
	if key := q.Get("currentIssueKey"); key != "" {
		if cur, err := s.findIssue(ctx, key); err == nil && sub.CanSee(cur.perm()) {
			where += " AND i.id <> ?"
			args = append(args, cur.ID)
			if cur.ParentID.Valid && q.Get("showSubTaskParent") == "false" {
				where += " AND i.id <> ?"
				args = append(args, cur.ParentID.Int64)
			}
		}
	}
	if q.Get("showSubTasks") == "false" {
		where += " AND i.parent_id IS NULL"
	}

	history, historyTotal, err := s.searchRows(ctx, where+` AND i.id IN (SELECT issue_id FROM change_groups WHERE author_id = ?
		UNION SELECT issue_id FROM comments WHERE author_id = ?)`, append(slices.Clone(args), sub.UserID, sub.UserID),
		exact+"i.updated_at DESC", 0, pickerHistory)
	if err != nil {
		return err
	}

	// The current search: the issues of the JQL the client is showing, if any.
	current, order := where, "i.updated_at DESC"
	currentArgs := slices.Clone(args)
	if jqlText := strings.TrimSpace(q.Get("currentJQL")); jqlText != "" {
		jqlWhere, jqlArgs, jqlOrder, _, err := s.searchCondition(ctx, sub, url.Values{"jql": {jqlText}})
		if err != nil {
			return err
		}
		current, order = where+" AND "+jqlWhere, jqlOrder
		currentArgs = append(currentArgs, jqlArgs...)
	}
	found, foundTotal, err := s.searchRows(ctx, current, currentArgs, exact+order, 0, pickerCurrent)
	if err != nil {
		return err
	}

	var re *regexp.Regexp
	if query != "" {
		re = regexp.MustCompile("(?i)" + regexp.QuoteMeta(query))
	}
	o := s.restOut(r)
	section := func(id string, label i18n.Message, rows []*issueRow, total int) map[string]any {
		issues := []any{}
		for _, iss := range rows {
			issues = append(issues, map[string]any{"id": iss.ID, "key": iss.Key(), "keyHtml": highlight(iss.Key(), re),
				"summary": highlight(iss.Summary, re), "summaryText": iss.Summary})
		}
		return map[string]any{"id": id, "label": o.msg(label),
			"sub": o.msg(i18n.M(i18n.RESTPickerShowing, len(rows), total)), "issues": issues}
	}
	return writeJSON(w, http.StatusOK, map[string]any{"sections": []any{
		section("hs", i18n.M(i18n.RESTPickerHistory), history, historyTotal),
		section("cs", i18n.M(i18n.RESTPickerCurrent), found, foundTotal)}})
}
