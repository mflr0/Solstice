package api

// Creating and editing issues through the REST API: Jira's "fields" and
// "update" are translated into the interface API's fields, which its
// functions then check and apply. Custom field values go through
// custom_values.go, which the interface API shares.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// jiraEditInput is the body of Jira's create and edit requests.
type jiraEditInput struct {
	Fields map[string]json.RawMessage              `json:"fields"`
	Update map[string][]map[string]json.RawMessage `json:"update"`
}

// jiraEdit is such a request, translated.
type jiraEdit struct {
	ui       fields         // for createIssue (as JSON) or applyUpdate
	custom   []customWrite  // custom field values to store
	sprint   *int64         // the sprint field: a sprint, or 0 for none
	comments []commentInput // comments to add (update.comment)
}

// ref is a reference to something in a Jira request: {"id": …}, {"key": …},
// {"name": …}, {"value": …}, or a plain string.
type ref struct {
	id, key, name, value string
	null                 bool
}

func (r ref) text() string {
	for _, v := range []string{r.name, r.key, r.value, r.id} {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseRef(raw json.RawMessage) (ref, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return ref{null: true}, true
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return ref{name: s, value: s}, true
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return ref{id: n.String()}, true
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ref{}, false
	}
	var out ref
	for k, dst := range map[string]*string{"id": &out.id, "key": &out.key, "name": &out.name, "value": &out.value} {
		switch v := m[k].(type) {
		case string:
			*dst = v
		case float64:
			*dst = strconv.FormatFloat(v, 'f', -1, 64)
		}
	}
	// {"name": null} names nothing, like null.
	out.null = out.text() == ""
	return out, true
}

func parseRefs(raw json.RawMessage) ([]ref, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, true
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return nil, false
	}
	var out []ref
	for _, item := range list {
		r, ok := parseRef(item)
		if !ok {
			return nil, false
		}
		out = append(out, r)
	}
	return out, true
}

func unknownValue(field, value string) error {
	return fieldError(field, failf(http.StatusBadRequest, i18n.FieldValueUnknown, value))
}

func invalidValue(field string) error {
	return fieldError(field, failf(http.StatusBadRequest, i18n.FieldValueInvalid, field))
}

// named finds an id among (id, name) pairs: by id, or by name ignoring case.
func named[T comparable](pairs map[T]string, r ref, parseID func(string) (T, bool)) (T, bool) {
	if r.id != "" {
		if id, ok := parseID(r.id); ok {
			if _, found := pairs[id]; found {
				return id, true
			}
		}
	}
	for id, name := range pairs {
		if t := r.text(); t != "" && strings.EqualFold(name, t) {
			return id, true
		}
	}
	var zero T
	return zero, false
}

func stringID(s string) (string, bool) { return s, s != "" }

func intID(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

// pairs loads (id, name) pairs of a project's components, versions or
// security levels.
func (s *Server) pairs(ctx context.Context, table string, projectID int64) (map[int64]string, error) {
	out := map[int64]string{}
	err := sqlq.Each(ctx, s.db, `SELECT id, name FROM `+table+` WHERE project_id = ?`, func(rows *sql.Rows) error {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		out[id] = name
		return nil
	}, projectID)
	return out, err
}

// editableFields are the system fields the REST API sets, with the
// interface API's name for each.
var editableFields = map[string]string{
	"summary": "summary", "description": "description", "environment": "environment", "issuetype": "type",
	"priority": "priority", "assignee": "assignee", "labels": "labels", "components": "components",
	"fixVersions": "fixVersions", "versions": "affectsVersions", "duedate": "due", "security": "securityLevel",
	"resolution": "resolution",
}

// translateEdit turns Jira's fields and update operations into a jiraEdit.
// cur is the issue being edited (with its labels, components, versions),
// nil for a creation.
func (s *Server) translateEdit(ctx context.Context, sub *perm.Subject, cat *catalog, p *editProject, cur *issueData,
	in jiraEditInput) (*jiraEdit, error) {
	edit := &jiraEdit{ui: fields{}}
	values := map[string]json.RawMessage{}
	for k, v := range in.Fields {
		values[k] = v
	}
	// Update operations: set replaces, add and remove change a list, and
	// comment.add adds a comment.
	for _, field := range slices.Sorted(maps.Keys(in.Update)) {
		for _, op := range in.Update[field] {
			for _, verb := range slices.Sorted(maps.Keys(op)) {
				arg := op[verb]
				switch {
				case field == "comment" && verb == "add":
					var c struct {
						Body       string          `json:"body"`
						Visibility json.RawMessage `json:"visibility"`
					}
					if json.Unmarshal(arg, &c) != nil {
						return nil, invalidValue("comment")
					}
					ci, err := s.commentVisibility(ctx, p.id, c.Body, c.Visibility)
					if err != nil {
						return nil, err
					}
					edit.comments = append(edit.comments, ci)
				case verb == "set" || verb == "edit":
					values[field] = arg
				case verb == "add" || verb == "remove":
					merged, err := s.mergeList(ctx, p, cur, field, verb, arg, values[field])
					if err != nil {
						return nil, err
					}
					values[field] = merged
				default:
					return nil, fieldError(field, failf(http.StatusBadRequest, i18n.FieldOperationUnsupported, verb))
				}
			}
		}
	}

	for _, field := range slices.Sorted(maps.Keys(values)) {
		if err := s.translateField(ctx, sub, cat, p, cur, edit, field, values[field]); err != nil {
			return nil, err
		}
	}
	return edit, nil
}

// mergeList applies an add or remove operation to a list field (labels,
// components, versions), starting from the value set in the same request or
// the issue's current one. It returns the whole list.
func (s *Server) mergeList(ctx context.Context, p *editProject, cur *issueData, field, verb string, arg,
	pending json.RawMessage) (json.RawMessage, error) {
	var items []json.RawMessage
	switch {
	case pending != nil:
		if json.Unmarshal(pending, &items) != nil {
			return nil, invalidValue(field)
		}
	case cur != nil:
		switch field {
		case "labels":
			for _, l := range cur.labels {
				raw, _ := json.Marshal(l)
				items = append(items, raw)
			}
		case "components":
			for _, c := range cur.components {
				raw, _ := json.Marshal(map[string]string{"id": strconv.FormatInt(c.ID, 10)})
				items = append(items, raw)
			}
		case "fixVersions", "versions":
			list := cur.fixVersions
			if field == "versions" {
				list = cur.versions
			}
			for _, v := range list {
				raw, _ := json.Marshal(map[string]string{"id": strconv.FormatInt(v.ID, 10)})
				items = append(items, raw)
			}
		default:
			return nil, fieldError(field, failf(http.StatusBadRequest, i18n.FieldOperationUnsupported, verb))
		}
	}
	// Compare items by what they name, resolved to ids where possible.
	key := func(raw json.RawMessage) string {
		r, _ := parseRef(raw)
		if field == "labels" {
			return r.text()
		}
		table := map[string]string{"components": "components", "fixVersions": "versions", "versions": "versions"}[field]
		if table != "" {
			if names, err := s.pairs(ctx, table, p.id); err == nil {
				if id, ok := named(names, r, intID); ok {
					return strconv.FormatInt(id, 10)
				}
			}
		}
		return strings.ToLower(r.text())
	}
	k := key(arg)
	items = slices.DeleteFunc(items, func(raw json.RawMessage) bool { return key(raw) == k })
	if verb == "add" {
		items = append(items, arg)
	}
	if items == nil {
		items = []json.RawMessage{}
	}
	return json.Marshal(items)
}

func (s *Server) translateField(ctx context.Context, sub *perm.Subject, cat *catalog, p *editProject, cur *issueData,
	edit *jiraEdit, field string, raw json.RawMessage) error {
	set := func(name string, v any) error {
		b, err := json.Marshal(v)
		edit.ui[name] = b
		return err
	}
	if strings.HasPrefix(field, "customfield_") {
		return s.translateCustom(ctx, p, edit, field, raw)
	}
	switch field {
	case "project":
		r, ok := parseRef(raw)
		if !ok || r.null {
			return invalidValue(field)
		}
		if cur != nil {
			if r.key != "" && !strings.EqualFold(r.key, p.key) || r.id != "" && r.id != strconv.FormatInt(p.id, 10) {
				return fieldError(field, fail(http.StatusBadRequest, i18n.IssueMoveUseAction))
			}
			return nil
		}
		return set("project", p.key)
	case "parent":
		r, ok := parseRef(raw)
		if !ok {
			return invalidValue(field)
		}
		parent, err := s.refIssue(ctx, sub, r)
		if cur != nil {
			if r.null && !cur.row.ParentID.Valid || err == nil && cur.row.ParentID.Int64 == parent.ID {
				return nil
			}
			return fieldError(field, fail(http.StatusBadRequest, i18n.IssueParentUnsupported))
		}
		if r.null {
			return nil
		}
		if err != nil {
			return fieldError(field, fail(http.StatusBadRequest, i18n.IssueParentNotFound))
		}
		return set("parent", parent.Key())
	case "reporter":
		// Solstice records who really created an issue.
		r, ok := parseRef(raw)
		if !ok {
			return invalidValue(field)
		}
		id, found := s.userByName(ctx, r.text())
		want := sub.UserID
		if cur != nil {
			want = cur.row.ReporterID.Int64
		}
		if !found || id != want {
			return fieldError(field, fail(http.StatusBadRequest, i18n.IssueReporterFixed))
		}
		return nil
	case "summary", "description", "environment":
		var v *string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		return set(field, v)
	case "duedate":
		var v *string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		return set("due", v)
	case "timetracking":
		// {"originalEstimate": "1w 2d", "remainingEstimate": "3h"}; what is
		// not given stays as it is, "" or null clears.
		var v map[string]*string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		for _, key := range slices.Sorted(maps.Keys(v)) {
			text := v[key]
			if key != "originalEstimate" && key != "remainingEstimate" {
				return invalidValue(field)
			}
			if text != nil {
				if _, ok := parseEstimate(*text); !ok && strings.TrimSpace(*text) != "" {
					return fieldError(field, fail(http.StatusBadRequest, i18n.IssueEstimateInvalid))
				}
			}
			if err := set(key, text); err != nil {
				return err
			}
		}
		return nil
	case "labels":
		var v []string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		if v == nil {
			v = []string{}
		}
		return set("labels", v)
	case "issuetype":
		r, ok := parseRef(raw)
		if !ok || r.null {
			return invalidValue(field)
		}
		names := map[string]string{}
		for _, t := range cat.IssueTypes {
			ok, err := s.projectHasType(ctx, p.id, t.ID)
			if err != nil {
				return err
			}
			if ok {
				names[t.ID] = t.Name
			}
		}
		id, found := named(names, r, stringID)
		if !found {
			return unknownValue(field, r.text())
		}
		p.issueType = id
		return set("type", id)
	case "priority", "resolution":
		r, ok := parseRef(raw)
		if !ok {
			return invalidValue(field)
		}
		if r.null {
			return set(field, nil)
		}
		names := map[string]string{}
		if field == "priority" {
			for _, v := range cat.Priorities {
				names[v.ID] = v.Name
			}
		} else {
			for _, v := range cat.Resolutions {
				names[v.ID] = v.Name
			}
		}
		id, found := named(names, r, stringID)
		if !found {
			return unknownValue(field, r.text())
		}
		return set(field, id)
	case "assignee":
		r, ok := parseRef(raw)
		if !ok {
			return invalidValue(field)
		}
		id, err := s.assigneeRef(ctx, p, r)
		if err != nil {
			return err
		}
		if cur == nil && id == nil {
			return nil // the project's default assignee
		}
		return set("assignee", id)
	case "security":
		r, ok := parseRef(raw)
		if !ok {
			return invalidValue(field)
		}
		if r.null {
			return set("securityLevel", nil)
		}
		levels, err := s.pairs(ctx, "security_levels", p.id)
		if err != nil {
			return err
		}
		id, found := named(levels, r, intID)
		if !found {
			return unknownValue(field, r.text())
		}
		return set("securityLevel", id)
	case "components", "fixVersions", "versions":
		refs, ok := parseRefs(raw)
		if !ok {
			return invalidValue(field)
		}
		table := "components"
		if field != "components" {
			table = "versions"
		}
		names, err := s.pairs(ctx, table, p.id)
		if err != nil {
			return err
		}
		ids := []int64{}
		for _, r := range refs {
			id, found := named(names, r, intID)
			if !found {
				return unknownValue(field, r.text())
			}
			ids = append(ids, id)
		}
		return set(editableFields[field], ids)
	}
	return fieldError(field, failf(http.StatusBadRequest, i18n.FieldNotSettable, field))
}

// assigneeRef reads an assignee: a user (by name or key), null for nobody,
// or "-1" for the project's default assignee. It returns a user id or nil.
func (s *Server) assigneeRef(ctx context.Context, p *editProject, r ref) (any, error) {
	if r.null {
		return nil, nil
	}
	if r.text() == "-1" {
		if p.assignTo == "lead" && p.lead.Valid {
			return p.lead.Int64, nil
		}
		return nil, nil
	}
	id, found := s.userByName(ctx, r.text())
	if !found {
		return nil, unknownValue("assignee", r.text())
	}
	return id, nil
}

// refIssue finds an issue the user sees by key or id.
func (s *Server) refIssue(ctx context.Context, sub *perm.Subject, r ref) (*issueRow, error) {
	var iss *issueRow
	var err error
	switch {
	case r.key != "" || r.name != "":
		iss, err = s.findIssue(ctx, r.text())
	case r.id != "":
		id, perr := strconv.ParseInt(r.id, 10, 64)
		if perr != nil {
			return nil, errNotFound
		}
		iss, err = s.findIssueByID(ctx, id)
	default:
		return nil, errNotFound
	}
	if err != nil || !sub.CanSee(iss.perm()) {
		return nil, errNotFound
	}
	return iss, nil
}

// sprintBoard returns the board of an open sprint, if the user sees it.
func (s *Server) sprintBoard(ctx context.Context, sub *perm.Subject, sprint int64) (*boardRow, error) {
	var board sql.NullInt64
	var state issue.SprintState
	err := s.db.QueryRowContext(ctx, `SELECT board_id, state FROM sprints WHERE id = ?`, sprint).Scan(&board, &state)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (!board.Valid || state == issue.SprintClosed)) {
		return nil, fail(http.StatusBadRequest, i18n.SprintUnknown)
	}
	if err != nil {
		return nil, err
	}
	b, err := s.boardByID(ctx, sub, board.Int64)
	if err != nil {
		return nil, fail(http.StatusBadRequest, i18n.SprintUnknown)
	}
	return b, nil
}

// applySprint puts an issue in an open sprint of a board the user sees, or
// takes it out of its open sprint (0), as the board does.
func (s *Server) applySprint(ctx context.Context, sub *perm.Subject, iss *issueRow, sprint int64) error {
	if sprint == 0 {
		return s.setSprint(ctx, sub, &boardRow{}, iss, 0, false)
	}
	b, err := s.sprintBoard(ctx, sub, sprint)
	if err != nil {
		return err
	}
	return s.setSprint(ctx, sub, b, iss, sprint, false)
}

// commentVisibility translates a Jira comment (body and visibility) into
// the interface API's.
func (s *Server) commentVisibility(ctx context.Context, projectID int64, body string, raw json.RawMessage) (commentInput, error) {
	in := commentInput{Body: body}
	if len(raw) == 0 || string(raw) == "null" {
		return in, nil
	}
	var v struct {
		Type, Value string
	}
	if json.Unmarshal(raw, &v) != nil {
		return in, invalidValue("visibility")
	}
	in.Visibility = &struct {
		Group string `json:"group"`
		Role  int64  `json:"role"`
	}{}
	switch strings.ToLower(v.Type) {
	case "group":
		in.Visibility.Group = strings.ToLower(v.Value)
	case "role":
		err := s.db.QueryRowContext(ctx, `SELECT id FROM project_roles WHERE LOWER(name) = LOWER(?)
			ORDER BY id LIMIT 1`, v.Value).Scan(&in.Visibility.Role)
		if err != nil {
			return in, unknownValue("visibility", v.Value)
		}
	default:
		return in, invalidValue("visibility")
	}
	return in, nil
}
