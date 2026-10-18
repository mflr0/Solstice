package api

// Creating and editing issues through the REST API: Jira's "fields" and
// "update" are translated into the interface API's fields, which its
// functions then check and apply. Custom field values, which the interface
// does not edit yet, are validated here and written alongside.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"slices"

	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
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

// customWrite replaces the value of a custom field.
type customWrite struct {
	field   customFieldDef
	rows    []customRaw
	labels  []string
	display string // the new value, for the history
}

// editProject is what translating a request needs to know of the project.
type editProject struct {
	id        int64
	key       string
	lead      sql.NullInt64
	assignTo  string // default assignee: lead or unassigned
	customs   []customFieldDef
	issueType string
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

// safeURL reports whether a url custom field value may become a link: only
// the http, https and mailto schemes are accepted.
func safeURL(v string) bool {
	u, err := url.Parse(strings.TrimSpace(v))
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto":
		return true
	}
	return false
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
	for field, ops := range in.Update {
		for _, op := range ops {
			for verb, arg := range op {
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

	for field, raw := range values {
		if err := s.translateField(ctx, sub, cat, p, cur, edit, field, raw); err != nil {
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
				return fieldError(field, fail(http.StatusBadRequest, i18n.IssueMoveUnsupported))
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
			return fieldError(field, fail(http.StatusBadRequest, i18n.IssueMoveUnsupported))
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
		for key, text := range v {
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
			if s.projectHasType(ctx, p.id, t.ID) {
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

// The kinds translateCustom takes values for; a test checks the list.
var settableKinds = []string{"gh-epic-link", "gh-sprint", "url", "textfield", "gh-epic-label", "textarea", "float",
	"datepicker", "datetime", "select", "radiobuttons", "gh-epic-status", "multiselect", "multicheckboxes", "cascadingselect",
	"userpicker", "multiuserpicker", "labels"}

func settableKind(kind string) bool { return slices.Contains(settableKinds, kind) }

// translateCustom reads a custom field's new value. The epic link and the
// sprint, which Solstice keeps as properties of the issue, become those.
func (s *Server) translateCustom(ctx context.Context, p *editProject, edit *jiraEdit, field string,
	raw json.RawMessage) error {
	var f customFieldDef
	found := false
	for _, cf := range p.customs {
		if customID(cf.ID) == field {
			f, found = cf, true
		}
	}
	if !found {
		return fieldError(field, failf(http.StatusBadRequest, i18n.FieldNotSettable, field))
	}
	isNull := len(raw) == 0 || string(raw) == "null"
	if !isNull && !settableKind(f.Kind) {
		return fieldError(field, failf(http.StatusBadRequest, i18n.FieldNotSettable, field))
	}
	switch f.Kind {
	case "gh-epic-link":
		var key *string
		if json.Unmarshal(raw, &key) != nil {
			return invalidValue(field)
		}
		b, _ := json.Marshal(key)
		edit.ui["epic"] = b
		return nil
	case "gh-sprint":
		var id int64
		if !isNull {
			var n json.Number
			var list []json.Number
			switch {
			case json.Unmarshal(raw, &n) == nil:
			case json.Unmarshal(raw, &list) == nil && len(list) == 1:
				n = list[0]
			default:
				return invalidValue(field)
			}
			v, err := n.Int64()
			if err != nil {
				return invalidValue(field)
			}
			id = v
		}
		edit.sprint = &id
		return nil
	}
	w := customWrite{field: f}
	if isNull {
		edit.custom = append(edit.custom, w)
		return nil
	}
	var display []string
	addText := func(column string, v string) {
		row := customRaw{}
		switch column {
		case "text":
			row.text = sql.NullString{String: v, Valid: true}
		default:
			row.str = sql.NullString{String: v, Valid: true}
		}
		w.rows = append(w.rows, row)
		display = append(display, v)
	}
	switch f.Kind {
	case "url":
		var v string
		if json.Unmarshal(raw, &v) != nil || utf8.RuneCountInString(v) > 255 {
			return invalidValue(field)
		}
		if strings.TrimSpace(v) != "" {
			if !safeURL(v) {
				return invalidValue(field)
			}
			addText("string", v)
		}
	case "textfield", "gh-epic-label":
		var v string
		if json.Unmarshal(raw, &v) != nil || utf8.RuneCountInString(v) > 255 {
			return invalidValue(field)
		}
		if strings.TrimSpace(v) != "" {
			addText("string", v)
		}
	case "textarea":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		if strings.TrimSpace(v) != "" {
			addText("text", v)
		}
	case "float":
		var n json.Number
		var str string
		if json.Unmarshal(raw, &str) == nil {
			n = json.Number(strings.TrimSpace(str))
		} else if json.Unmarshal(raw, &n) != nil {
			return invalidValue(field)
		}
		v, err := n.Float64()
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return invalidValue(field)
		}
		w.rows = append(w.rows, customRaw{num: sql.NullFloat64{Float64: v, Valid: true}})
		display = append(display, strconv.FormatFloat(v, 'f', -1, 64))
	case "datepicker":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		d, err := time.Parse(time.DateOnly, v)
		if err != nil {
			return fieldError(field, fail(http.StatusBadRequest, i18n.IssueDueDateInvalid))
		}
		w.rows = append(w.rows, customRaw{date: sql.NullTime{Time: d, Valid: true}})
		display = append(display, v)
	case "datetime":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		t, ok := parseJiraTime(v)
		if !ok {
			return invalidValue(field)
		}
		w.rows = append(w.rows, customRaw{date: sql.NullTime{Time: t.UTC(), Valid: true}})
		display = append(display, t.UTC().Format(time.RFC3339))
	case "select", "radiobuttons", "gh-epic-status", "multiselect", "multicheckboxes", "cascadingselect":
		options, err := s.fieldOptions(ctx, f.ID)
		if err != nil {
			return err
		}
		var refs []ref
		if f.Kind == "multiselect" || f.Kind == "multicheckboxes" {
			var ok bool
			if refs, ok = parseRefs(raw); !ok {
				return invalidValue(field)
			}
		} else {
			r, ok := parseRef(raw)
			if !ok {
				return invalidValue(field)
			}
			refs = []ref{r}
		}
		for _, r := range refs {
			opt, ok := pickOption(options, r, 0)
			if !ok {
				return unknownValue(field, r.text())
			}
			w.rows = append(w.rows, customRaw{str: sql.NullString{String: strconv.FormatInt(opt.id, 10), Valid: true}})
			display = append(display, opt.value)
		}
		if f.Kind == "cascadingselect" {
			var m struct {
				Child json.RawMessage `json:"child"`
			}
			json.Unmarshal(raw, &m)
			if r, ok := parseRef(m.Child); ok && !r.null && len(w.rows) == 1 {
				parentID, _ := strconv.ParseInt(w.rows[0].str.String, 10, 64)
				opt, ok := pickOption(options, r, parentID)
				if !ok {
					return unknownValue(field, r.text())
				}
				w.rows = append(w.rows, customRaw{parentKey: sql.NullString{String: "1", Valid: true},
					str: sql.NullString{String: strconv.FormatInt(opt.id, 10), Valid: true}})
				display[0] += " › " + opt.value
			}
		}
	case "userpicker", "multiuserpicker":
		var refs []ref
		if f.Kind == "multiuserpicker" {
			var ok bool
			if refs, ok = parseRefs(raw); !ok {
				return invalidValue(field)
			}
		} else {
			r, ok := parseRef(raw)
			if !ok {
				return invalidValue(field)
			}
			refs = []ref{r}
		}
		for _, r := range refs {
			id, found := s.userByName(ctx, r.text())
			if !found {
				return unknownValue(field, r.text())
			}
			_, name := s.userName(ctx, id)
			w.rows = append(w.rows, customRaw{str: sql.NullString{String: strconv.FormatInt(id, 10), Valid: true}})
			display = append(display, name)
		}
	case "labels":
		var v []string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		seen := map[string]bool{}
		for _, l := range v {
			l = strings.Join(strings.Fields(l), "_")
			if l != "" && utf8.RuneCountInString(l) <= 255 && !seen[l] {
				seen[l] = true
				w.labels = append(w.labels, l)
			}
		}
		slices.Sort(w.labels)
		display = w.labels
	default:
		return fieldError(field, failf(http.StatusBadRequest, i18n.FieldNotSettable, field))
	}
	w.display = strings.Join(display, ", ")
	edit.custom = append(edit.custom, w)
	return nil
}

type fieldOption struct {
	id, parent int64
	value      string
	disabled   bool
}

func (s *Server) fieldOptions(ctx context.Context, fieldID int64) ([]fieldOption, error) {
	var out []fieldOption
	err := sqlq.Each(ctx, s.db, `SELECT id, COALESCE(parent_id, 0), value, disabled FROM custom_field_options
		WHERE field_id = ? ORDER BY position, id`, func(rows *sql.Rows) error {
		var o fieldOption
		if err := rows.Scan(&o.id, &o.parent, &o.value, &o.disabled); err != nil {
			return err
		}
		out = append(out, o)
		return nil
	}, fieldID)
	return out, err
}

// pickOption finds an enabled option under parent (0: top level) by id or
// value (exact, then ignoring case).
func pickOption(options []fieldOption, r ref, parent int64) (fieldOption, bool) {
	for pass := range 3 {
		for _, o := range options {
			if o.parent != parent || o.disabled {
				continue
			}
			switch {
			case pass == 0 && r.id != "" && strconv.FormatInt(o.id, 10) == r.id:
				return o, true
			case pass == 1 && r.text() != "" && o.value == r.text():
				return o, true
			case pass == 2 && r.text() != "" && strings.EqualFold(o.value, r.text()):
				return o, true
			}
		}
	}
	return fieldOption{}, false
}

// applyCustom stores custom field values and records the change in the
// issue's history. Callers check the permission first.
func (s *Server) applyCustom(ctx context.Context, sub *perm.Subject, o *restOut, iss *issueRow, writes []customWrite) error {
	if len(writes) == 0 {
		return nil
	}
	// The values before, as text, for the history.
	set, err := s.loadIssues(ctx, sub, []*issueRow{iss}, func(k string) bool { return k == loadCustom })
	if err != nil {
		return err
	}
	users, err := s.jiraUsers(ctx, o, set.userIDs()...)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	var changes []change
	for _, w := range writes {
		before := o.customText(set, users, w.field, set.list[0])
		if before == w.display {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM custom_field_values WHERE issue_id = ? AND field_id = ?`,
			iss.ID, w.field.ID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM labels WHERE issue_id = ? AND field_id = ?`, iss.ID,
			w.field.ID); err != nil {
			return err
		}
		for _, v := range w.rows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO custom_field_values (issue_id, field_id, parent_key,
				string_value, number_value, text_value, date_value) VALUES (?, ?, ?, ?, ?, ?, ?)`, iss.ID, w.field.ID,
				v.parentKey, v.str, v.num, v.text, v.date); err != nil {
				return err
			}
		}
		for _, l := range w.labels {
			if _, err := tx.ExecContext(ctx, `INSERT INTO labels (issue_id, field_id, label) VALUES (?, ?, ?)`,
				iss.ID, w.field.ID, l); err != nil {
				return err
			}
		}
		changes = append(changes, change{field: w.field.Name, oldString: nullIfEmpty(before),
			newString: nullIfEmpty(w.display)})
	}
	if len(changes) == 0 {
		return nil
	}
	// Jira files these history lines as custom fields.
	res, err := tx.ExecContext(ctx, `INSERT INTO change_groups (issue_id, author_id, created_at) VALUES (?, ?, ?)`,
		iss.ID, sub.UserID, now)
	if err != nil {
		return err
	}
	group, err := res.LastInsertId()
	if err != nil {
		return err
	}
	for _, c := range changes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO change_items (group_id, field_type, field, old_string, new_string)
			VALUES (?, 'custom', ?, ?, ?)`, group, c.field, c.oldString, c.newString); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, now, iss.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.changed(iss, sub.UserID, "issue", changes) // notifications and webhooks too
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// sprintBoard returns the board of an open sprint, if the user sees it.
func (s *Server) sprintBoard(ctx context.Context, sub *perm.Subject, sprint int64) (*boardRow, error) {
	var board sql.NullInt64
	var state string
	err := s.db.QueryRowContext(ctx, `SELECT board_id, state FROM sprints WHERE id = ?`, sprint).Scan(&board, &state)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (!board.Valid || state == "closed")) {
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

// editProjectOf loads what translating a request needs about a project.
func (s *Server) editProjectOf(ctx context.Context, projectID int64) (*editProject, error) {
	p := &editProject{id: projectID}
	if err := s.db.QueryRowContext(ctx, `SELECT pkey, lead_id, default_assignee FROM projects WHERE id = ?`, projectID).
		Scan(&p.key, &p.lead, &p.assignTo); err != nil {
		return nil, err
	}
	var err error
	p.customs, err = s.customFields(ctx)
	return p, err
}
