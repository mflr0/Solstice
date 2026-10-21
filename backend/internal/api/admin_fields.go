package api

// Custom fields, for global administrators. New fields are stored with
// Jira's type keys, so that the issue view, the REST API and JQL treat them
// exactly as the fields an import brought. A field applies to every
// project and issue type, or to those its context lists.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/sqlq"
)

const jiraFieldPrefix = "com.atlassian.jira.plugin.system.customfieldtypes:"

// fieldTypes are the types a field can be created with: Solstice's name,
// Jira's kind (the end of its type key) and searcher.
var fieldTypes = []struct{ name, kind, searcher string }{
	{"text", "textfield", "textsearcher"},
	{"textarea", "textarea", "textsearcher"},
	{"number", "float", "exactnumber"},
	{"date", "datepicker", "daterange"},
	{"datetime", "datetime", "datetimerange"},
	{"select", "select", "multiselectsearcher"},
	{"multiselect", "multiselect", "multiselectsearcher"},
	{"checkboxes", "multicheckboxes", "checkboxsearcher"},
	{"radio", "radiobuttons", "radiosearcher"},
	{"user", "userpicker", "userpickergroupsearcher"},
	{"multiuser", "multiuserpicker", "userpickergroupsearcher"},
	{"labels", "labels", "labelsearcher"},
	{"url", "url", "exacttextsearcher"},
}

// fieldTypeName gives Solstice's name of a Jira type key, "" for a type
// Solstice only displays.
func fieldTypeName(typeKey string) string {
	kind := issue.FieldKind(typeKey)
	for _, t := range fieldTypes {
		if t.kind == kind {
			return t.name
		}
	}
	return ""
}

// protectedKinds are Jira Software's fields that boards rely on.
func protectedKind(kind string) bool { return hiddenKinds[kind] || kind == "gh-epic-link" }

type optionOut struct {
	ID       int64  `json:"id"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
	Position int    `json:"position"`
	Issues   int    `json:"issues"`   // issues with this option (or one under it) chosen
	Children int    `json:"children"` // options under it (cascading fields)
}

type fieldContext struct {
	Projects   []projectBrief `json:"projects"`
	IssueTypes []typeRef      `json:"issueTypes"`
}

type fieldAdminOut struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name"`
	Description *string      `json:"description"`
	Type        *string      `json:"type"`
	TypeKey     string       `json:"typeKey"`
	Options     int          `json:"options"` // top-level options
	Issues      int          `json:"issues"`
	Context     fieldContext `json:"context"`
}

// fieldDetailOut is a field with its top-level options in place of their
// count.
type fieldDetailOut struct {
	fieldAdminOut
	Options []optionOut `json:"options"`
}

func (s *Server) fieldList(ctx context.Context, where string, args ...any) ([]fieldAdminOut, error) {
	out := []fieldAdminOut{}
	index := map[int64]int{}
	err := sqlq.Each(ctx, s.db, `SELECT f.id, f.name, f.description, f.type_key,
		(SELECT COUNT(*) FROM custom_field_options o WHERE o.field_id = f.id AND o.parent_id IS NULL),
		(SELECT COUNT(*) FROM (SELECT field_id, issue_id FROM custom_field_values
			UNION SELECT field_id, issue_id FROM labels WHERE field_id IS NOT NULL) x WHERE x.field_id = f.id)
		FROM custom_fields f WHERE `+where+` ORDER BY f.name, f.id`, func(rows *sql.Rows) error {
		var f fieldAdminOut
		var desc sql.NullString
		if err := rows.Scan(&f.ID, &f.Name, &desc, &f.TypeKey, &f.Options, &f.Issues); err != nil {
			return err
		}
		f.Description = sqlq.String(desc)
		if t := fieldTypeName(f.TypeKey); t != "" {
			f.Type = &t
		}
		f.Context = fieldContext{Projects: []projectBrief{}, IssueTypes: []typeRef{}}
		index[f.ID] = len(out)
		out = append(out, f)
		return nil
	}, args...)
	if err != nil || len(out) == 0 {
		return out, err
	}
	if err := sqlq.Each(ctx, s.db, `SELECT c.field_id, p.id, p.pkey, p.name FROM custom_field_projects c
		JOIN projects p ON p.id = c.project_id ORDER BY p.pkey`, func(rows *sql.Rows) error {
		var field int64
		var p projectBrief
		if err := rows.Scan(&field, &p.ID, &p.Key, &p.Name); err != nil {
			return err
		}
		if i, ok := index[field]; ok {
			out[i].Context.Projects = append(out[i].Context.Projects, p)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return nil, err
	}
	err = sqlq.Each(ctx, s.db, `SELECT field_id, issue_type_id FROM custom_field_issue_types`, func(rows *sql.Rows) error {
		var field int64
		var t string
		if err := rows.Scan(&field, &t); err != nil {
			return err
		}
		if i, ok := index[field]; ok {
			out[i].Context.IssueTypes = append(out[i].Context.IssueTypes, cat.typeOf(t))
		}
		return nil
	})
	return out, err
}

func (s *Server) fieldDetail(ctx context.Context, id int64) (*fieldDetailOut, error) {
	list, err := s.fieldList(ctx, "f.id = ?", id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, errNotFound
	}
	f := &fieldDetailOut{fieldAdminOut: list[0], Options: []optionOut{}}
	if err := sqlq.Each(ctx, s.db, `SELECT o.id, o.value, o.disabled, o.position,
		(SELECT COUNT(DISTINCT v.issue_id) FROM custom_field_values v WHERE v.field_id = o.field_id
			AND v.string_value IN (SELECT CAST(c.id AS CHAR) FROM custom_field_options c
				WHERE c.id = o.id OR c.parent_id = o.id)),
		(SELECT COUNT(*) FROM custom_field_options c WHERE c.parent_id = o.id)
		FROM custom_field_options o WHERE o.field_id = ? AND o.parent_id IS NULL ORDER BY o.position, o.id`,
		func(rows *sql.Rows) error {
			var o optionOut
			if err := rows.Scan(&o.ID, &o.Value, &o.Disabled, &o.Position, &o.Issues, &o.Children); err != nil {
				return err
			}
			f.Options = append(f.Options, o)
			return nil
		}, id); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Server) adminFields(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	out, err := s.fieldList(r.Context(), "TRUE")
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) fieldTypes(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	out := make([]map[string]any, 0, len(fieldTypes))
	for _, t := range fieldTypes {
		out = append(out, map[string]any{"type": t.name, "typeKey": jiraFieldPrefix + t.kind,
			"options": optionKind(t.kind)})
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) adminField(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	f, err := s.fieldDetail(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, f)
}

type contextInput struct {
	Projects   []string `json:"projects"`   // project keys
	IssueTypes []string `json:"issueTypes"` // issue type ids
}

// checkContext resolves a context's projects and issue types.
func (s *Server) checkContext(ctx context.Context, in *contextInput) ([]int64, []string, error) {
	var projects []int64
	for _, key := range in.Projects {
		id, err := s.projectByKey(ctx, key)
		if errors.Is(err, errNotFound) {
			return nil, nil, failf(http.StatusBadRequest, i18n.ProjectUnknown, key)
		}
		if err != nil {
			return nil, nil, err
		}
		if !slices.Contains(projects, id) {
			projects = append(projects, id)
		}
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return nil, nil, err
	}
	var types []string
	for _, t := range in.IssueTypes {
		if _, ok := cat.issueType[t]; !ok {
			return nil, nil, failf(http.StatusBadRequest, i18n.IssueTypeUnknown, t)
		}
		if !slices.Contains(types, t) {
			types = append(types, t)
		}
	}
	return projects, types, nil
}

func (s *Server) fieldNameTaken(ctx context.Context, name string, except int64) error {
	return s.nameTaken(ctx, "custom_fields", name, except, i18n.CommonAlreadyExists)
}

func writeContext(ctx context.Context, tx *sql.Tx, field int64, projects []int64, types []string) error {
	for _, table := range []string{"custom_field_projects", "custom_field_issue_types"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE field_id = ?`, field); err != nil {
			return err
		}
	}
	for _, p := range projects {
		if _, err := tx.ExecContext(ctx, `INSERT INTO custom_field_projects (field_id, project_id) VALUES (?, ?)`,
			field, p); err != nil {
			return err
		}
	}
	for _, t := range types {
		if _, err := tx.ExecContext(ctx, `INSERT INTO custom_field_issue_types (field_id, issue_type_id) VALUES (?, ?)`,
			field, t); err != nil {
			return err
		}
	}
	return nil
}

// cleanOption checks an option's text.
func cleanOption(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", fail(http.StatusBadRequest, i18n.CustomFieldOptionEmpty)
	}
	if len([]rune(v)) > 255 {
		return "", fail(http.StatusBadRequest, i18n.CustomFieldOptionTooLong)
	}
	return v, nil
}

func (s *Server) createField(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		Name        string        `json:"name"`
		Description *string       `json:"description"`
		Type        string        `json:"type"`
		Options     []string      `json:"options"`
		Context     *contextInput `json:"context"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	name, err := cleanName(in.Name, 255)
	if err != nil {
		return err
	}
	if err := s.fieldNameTaken(ctx, name, 0); err != nil {
		return err
	}
	i := slices.IndexFunc(fieldTypes, func(t struct{ name, kind, searcher string }) bool { return t.name == in.Type })
	if i < 0 {
		names := make([]string, 0, len(fieldTypes))
		for _, t := range fieldTypes {
			names = append(names, t.name)
		}
		return failf(http.StatusBadRequest, i18n.CustomFieldTypeUnknown, strings.Join(names, ", "))
	}
	ft := fieldTypes[i]
	if len(in.Options) > 0 && !optionKind(ft.kind) {
		return fail(http.StatusBadRequest, i18n.CustomFieldNoOptions)
	}
	var options []string
	for _, o := range in.Options {
		v, err := cleanOption(o)
		if err != nil {
			return err
		}
		if slices.ContainsFunc(options, func(x string) bool { return strings.EqualFold(x, v) }) {
			return failf(http.StatusConflict, i18n.CommonAlreadyExists, v)
		}
		options = append(options, v)
	}
	var projects []int64
	var types []string
	if in.Context != nil {
		if projects, types, err = s.checkContext(ctx, in.Context); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO custom_fields (type_key, searcher_key, name, description)
		VALUES (?, ?, ?, ?)`, jiraFieldPrefix+ft.kind, jiraFieldPrefix+ft.searcher, name, optDescription(in.Description))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	for pos, o := range options {
		if _, err := tx.ExecContext(ctx, `INSERT INTO custom_field_options (field_id, position, value) VALUES (?, ?, ?)`,
			id, pos+1, o); err != nil {
			return err
		}
	}
	if err := writeContext(ctx, tx, id, projects, types); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	f, err := s.fieldDetail(ctx, id)
	if err != nil {
		return err
	}
	s.auditLog(r, audit.FieldCreate, target("field", id, name),
		map[string]any{"type": ft.name, "options": options, "context": contextNames(f.Context)})
	return writeJSON(w, http.StatusCreated, f)
}

func contextNames(c fieldContext) map[string][]string {
	out := map[string][]string{"projects": {}, "issueTypes": {}}
	for _, p := range c.Projects {
		out["projects"] = append(out["projects"], p.Key)
	}
	for _, t := range c.IssueTypes {
		out["issueTypes"] = append(out["issueTypes"], t.Name)
	}
	return out
}

func (s *Server) updateField(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Name        *string       `json:"name"`
		Description *string       `json:"description"`
		Context     *contextInput `json:"context"`
	}
	raw, err := readJSONFields(r, &in)
	if err != nil {
		return err
	}
	ctx := r.Context()
	before, err := s.fieldDetail(ctx, id)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if in.Name != nil {
		name, err := cleanName(*in.Name, 255)
		if err != nil {
			return err
		}
		if err := s.fieldNameTaken(ctx, name, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE custom_fields SET name = ? WHERE id = ?`, name, id); err != nil {
			return err
		}
	}
	if _, ok := raw["description"]; ok {
		if _, err := tx.ExecContext(ctx, `UPDATE custom_fields SET description = ? WHERE id = ?`,
			optDescription(in.Description), id); err != nil {
			return err
		}
	}
	if _, ok := raw["context"]; ok {
		c := in.Context
		if c == nil {
			c = &contextInput{}
		}
		projects, types, err := s.checkContext(ctx, c)
		if err != nil {
			return err
		}
		if err := writeContext(ctx, tx, id, projects, types); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	after, err := s.fieldDetail(ctx, id)
	if err != nil {
		return err
	}
	changes := map[string]any{}
	diff(changes, "name", before.Name, after.Name)
	diff(changes, "description", derefStr(before.Description), derefStr(after.Description))
	diff(changes, "context", contextNames(before.Context), contextNames(after.Context))
	if len(changes) > 0 {
		s.auditLog(r, audit.FieldUpdate, target("field", id, after.Name), changes)
	}
	return writeJSON(w, http.StatusOK, after)
}

func (s *Server) deleteField(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Confirm string `json:"confirm"`
	}
	if err := readOptionalJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	f, err := s.fieldDetail(ctx, id)
	if err != nil {
		return err
	}
	if protectedKind(issue.FieldKind(f.TypeKey)) {
		return fail(http.StatusConflict, i18n.CustomFieldUsedByBoards)
	}
	estimate := "customfield_" + strconv.FormatInt(id, 10)
	var boards int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM boards WHERE estimate = ?`, estimate).Scan(&boards); err != nil {
		return err
	}
	if f.Issues > 0 && strings.TrimSpace(in.Confirm) != f.Name {
		return &httpError{status: http.StatusConflict,
			extra: map[string]any{"usage": map[string]int{"issues": f.Issues, "boards": boards}},
			msg:   i18n.M(i18n.CustomFieldDeleteConfirm)}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE boards SET estimate = NULL, updated_at = ? WHERE estimate = ?`,
		time.Now().UTC(), estimate); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM custom_fields WHERE id = ?`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auditLog(r, audit.FieldDelete, target("field", id, f.Name),
		map[string]any{"issues": f.Issues, "boards": boards})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// optionField loads a field whose options can be edited.
func (s *Server) optionField(r *http.Request) (*fieldDetailOut, error) {
	if err := requireAdmin(r); err != nil {
		return nil, err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return nil, err
	}
	f, err := s.fieldDetail(r.Context(), id)
	if err != nil {
		return nil, err
	}
	if !optionKind(issue.FieldKind(f.TypeKey)) {
		return nil, fail(http.StatusBadRequest, i18n.CustomFieldNoOptions)
	}
	return f, nil
}

func (s *Server) optionTaken(ctx context.Context, field int64, value string, except int64) error {
	return s.valueTaken(ctx, "custom_field_options", "value", "field_id = ? AND parent_id IS NULL", []any{field}, value,
		except, i18n.CommonAlreadyExists)
}

func (s *Server) writeField(w http.ResponseWriter, r *http.Request, status int, id int64) error {
	f, err := s.fieldDetail(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, status, f)
}

func (s *Server) addFieldOption(w http.ResponseWriter, r *http.Request) error {
	f, err := s.optionField(r)
	if err != nil {
		return err
	}
	var in struct {
		Value string `json:"value"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	v, err := cleanOption(in.Value)
	if err != nil {
		return err
	}
	if err := s.optionTaken(ctx, f.ID, v, 0); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO custom_field_options (field_id, position, value)
		SELECT ?, COALESCE(MAX(position), 0) + 1, ? FROM custom_field_options WHERE field_id = ?`, f.ID, v, f.ID); err != nil {
		return err
	}
	s.auditLog(r, audit.FieldOptions, target("field", f.ID, f.Name),
		map[string]any{"added": v})
	return s.writeField(w, r, http.StatusCreated, f.ID)
}

func (s *Server) updateFieldOption(w http.ResponseWriter, r *http.Request) error {
	f, err := s.optionField(r)
	if err != nil {
		return err
	}
	option, err := pathID(r, "option")
	if err != nil {
		return err
	}
	var in struct {
		Value    *string `json:"value"`
		Disabled *bool   `json:"disabled"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	var old string
	var disabled bool
	err = s.db.QueryRowContext(ctx, `SELECT value, disabled FROM custom_field_options WHERE id = ? AND field_id = ?`,
		option, f.ID).Scan(&old, &disabled)
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	details := map[string]any{"option": old}
	if in.Value != nil {
		v, err := cleanOption(*in.Value)
		if err != nil {
			return err
		}
		if err := s.optionTaken(ctx, f.ID, v, option); err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE custom_field_options SET value = ? WHERE id = ?`, v, option); err != nil {
			return err
		}
		diff(details, "value", old, v)
	}
	if in.Disabled != nil {
		if _, err := s.db.ExecContext(ctx, `UPDATE custom_field_options SET disabled = ? WHERE id = ?`,
			*in.Disabled, option); err != nil {
			return err
		}
		if *in.Disabled != disabled {
			details["disabled"] = *in.Disabled
		}
	}
	if len(details) > 1 {
		s.auditLog(r, audit.FieldOptions, target("field", f.ID, f.Name),
			details)
	}
	return s.writeField(w, r, http.StatusOK, f.ID)
}

// deleteFieldOption removes an option, and the options under it (a cascading
// field's children). An option some issues have chosen is refused (409 with
// the number of issues) unless the body names where their value goes
// (`moveTo`, another option of the field at the same level) or asks to
// clear it (`clear`).
func (s *Server) deleteFieldOption(w http.ResponseWriter, r *http.Request) error {
	f, err := s.optionField(r)
	if err != nil {
		return err
	}
	option, err := pathID(r, "option")
	if err != nil {
		return err
	}
	var in struct {
		MoveTo *int64 `json:"moveTo"`
		Clear  bool   `json:"clear"`
	}
	if err := readOptionalJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	var value string
	var parent sql.NullInt64
	err = s.db.QueryRowContext(ctx, `SELECT value, parent_id FROM custom_field_options WHERE id = ? AND field_id = ?`,
		option, f.ID).Scan(&value, &parent)
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	// The option and those under it: their ids are what values hold.
	gone := []int64{option}
	if err := sqlq.Each(ctx, s.db, `SELECT id FROM custom_field_options WHERE parent_id = ? AND field_id = ?`,
		func(rows *sql.Rows) error {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			gone = append(gone, id)
			return nil
		}, option, f.ID); err != nil {
		return err
	}
	marks := sqlq.Marks(len(gone))
	args := []any{f.ID}
	for _, id := range gone {
		args = append(args, strconv.FormatInt(id, 10))
	}
	var issues int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT issue_id) FROM custom_field_values
		WHERE field_id = ? AND string_value IN (`+marks+`)`, args...).Scan(&issues); err != nil {
		return err
	}
	var moveTo string
	if in.MoveTo != nil {
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM custom_field_options WHERE id = ? AND field_id = ?
			AND parent_id <=> ? AND id <> ?`, *in.MoveTo, f.ID, parent, option).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fail(http.StatusBadRequest, i18n.CustomFieldOptionMoveTo)
		}
		moveTo = strconv.FormatInt(*in.MoveTo, 10)
	}
	if issues > 0 && moveTo == "" && !in.Clear {
		return &httpError{status: http.StatusConflict, extra: map[string]any{"usage": map[string]int{"issues": issues}},
			msg: i18n.M(i18n.CustomFieldOptionInUse)}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if moveTo != "" && issues > 0 {
		// An issue already holding the target (a multiple choice) keeps one value.
		if _, err := tx.ExecContext(ctx, `DELETE FROM custom_field_values WHERE field_id = ? AND string_value = ?
			AND issue_id IN (SELECT issue_id FROM (SELECT issue_id FROM custom_field_values
				WHERE field_id = ? AND string_value = ?) t)`, f.ID, strconv.FormatInt(option, 10), f.ID, moveTo); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE custom_field_values SET string_value = ?
			WHERE field_id = ? AND string_value = ?`, moveTo, f.ID, strconv.FormatInt(option, 10)); err != nil {
			return err
		}
	}
	// What is left (cleared values, children's values) goes with the option.
	if _, err := tx.ExecContext(ctx, `DELETE FROM custom_field_values WHERE field_id = ? AND string_value IN (`+marks+`)`,
		args...); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM custom_field_options WHERE id = ? AND field_id = ?`, option, f.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	details := map[string]any{"removed": value, "issues": issues}
	if moveTo != "" {
		details["movedTo"] = moveTo
	}
	s.auditLog(r, audit.FieldOptions, target("field", f.ID, f.Name), details)
	return s.writeField(w, r, http.StatusOK, f.ID)
}

func (s *Server) orderFieldOptions(w http.ResponseWriter, r *http.Request) error {
	f, err := s.optionField(r)
	if err != nil {
		return err
	}
	ids, err := readOrder(r)
	if err != nil {
		return err
	}
	if err := sameIDs(f.Options, ids, func(o optionOut) string { return strconv.FormatInt(o.ID, 10) }); err != nil {
		return err
	}
	ctx := r.Context()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE custom_field_options SET position = ? WHERE id = ? AND field_id = ?`,
			i+1, id, f.ID); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auditLog(r, audit.FieldOptions, target("field", f.ID, f.Name),
		map[string]any{"order": ids})
	return s.writeField(w, r, http.StatusOK, f.ID)
}

// Contexts at work ------------------------------------------------------------------

// fieldsOutside lists, of the fields given, those whose context excludes a
// project and issue type. Without a type, only the project counts.
func (s *Server) fieldsOutside(ctx context.Context, projectID int64, typeID string, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(ids)+2)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, projectID)
	types := ""
	if typeID != "" {
		types = `
		AND (NOT EXISTS (SELECT 1 FROM custom_field_issue_types c WHERE c.field_id = f.id)
			OR EXISTS (SELECT 1 FROM custom_field_issue_types c WHERE c.field_id = f.id AND c.issue_type_id = ?))`
		args = append(args, typeID)
	}
	var out []int64
	err := sqlq.Each(ctx, s.db, `SELECT f.id FROM custom_fields f WHERE f.id IN (`+sqlq.Marks(len(ids))+`) AND NOT (
		(NOT EXISTS (SELECT 1 FROM custom_field_projects c WHERE c.field_id = f.id)
			OR EXISTS (SELECT 1 FROM custom_field_projects c WHERE c.field_id = f.id AND c.project_id = ?))`+types+`)`,
		func(rows *sql.Rows) error {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			out = append(out, id)
			return nil
		}, args...)
	return out, err
}

// checkCustomContext refuses values for fields that do not apply to the
// issue's project and type.
func (s *Server) checkCustomContext(ctx context.Context, projectID int64, typeID string, writes []customWrite) error {
	ids := make([]int64, 0, len(writes))
	for _, w := range writes {
		ids = append(ids, w.field.ID)
	}
	outside, err := s.fieldsOutside(ctx, projectID, typeID, ids)
	if err != nil || len(outside) == 0 {
		return err
	}
	for _, w := range writes {
		if w.field.ID == outside[0] {
			return fieldError(customID(w.field.ID), failf(http.StatusBadRequest, i18n.FieldNotOffered, w.field.Name))
		}
	}
	return nil
}

// projectFields lists the custom fields offered for a project and an issue
// type, with their enabled options, for issue forms.
func (s *Server) projectFields(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	ctx := r.Context()
	projectID, err := s.projectByKey(ctx, r.PathValue("key"))
	if err != nil || !sub.CanBrowseProject(projectID) {
		return errNotFound
	}
	typeID := r.URL.Query().Get("issueType")
	type optionBrief struct {
		ID    int64  `json:"id"`
		Value string `json:"value"`
	}
	type fieldBrief struct {
		ID      int64         `json:"id"`
		Name    string        `json:"name"`
		Type    string        `json:"type"`
		TypeKey string        `json:"typeKey"`
		Options []optionBrief `json:"options"`
	}
	all, err := s.customFields(ctx)
	if err != nil {
		return err
	}
	var ids []int64
	for _, f := range all {
		if fieldTypeName(f.TypeKey) != "" {
			ids = append(ids, f.ID)
		}
	}
	outside, err := s.fieldsOutside(ctx, projectID, typeID, ids)
	if err != nil {
		return err
	}
	out := []fieldBrief{}
	index := map[int64]int{}
	for _, f := range all {
		t := fieldTypeName(f.TypeKey)
		if t == "" || slices.Contains(outside, f.ID) {
			continue
		}
		index[f.ID] = len(out)
		out = append(out, fieldBrief{ID: f.ID, Name: f.Name, Type: t, TypeKey: f.TypeKey, Options: []optionBrief{}})
	}
	err = sqlq.Each(ctx, s.db, `SELECT field_id, id, value FROM custom_field_options WHERE parent_id IS NULL AND NOT disabled
		ORDER BY position, id`, func(rows *sql.Rows) error {
		var field int64
		var o optionBrief
		if err := rows.Scan(&field, &o.ID, &o.Value); err != nil {
			return err
		}
		if i, ok := index[field]; ok {
			out[i].Options = append(out[i].Options, o)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// readCustomFields translates the "customFields" of an interface request
// ({"<field id>": value as in Jira's REST API}) into custom field writes.
func (s *Server) readCustomFields(ctx context.Context, f fields, projectID int64, typeID string) ([]customWrite, error) {
	if !f.has("customFields") || f.isNull("customFields") {
		return nil, nil
	}
	var values map[string]json.RawMessage
	if err := f.get("customFields", &values); err != nil {
		return nil, err
	}
	p, err := s.editProjectOf(ctx, projectID)
	if err != nil {
		return nil, err
	}
	p.issueType = typeID
	edit := &jiraEdit{ui: fields{}}
	for id, raw := range values {
		field := "customfield_" + strings.TrimPrefix(id, "customfield_")
		if err := s.translateCustom(ctx, p, edit, field, raw); err != nil {
			return nil, err
		}
		if len(edit.ui) > 0 || edit.sprint != nil {
			return nil, failf(http.StatusBadRequest, i18n.FieldNotSettable, field)
		}
	}
	if err := s.checkCustomContext(ctx, projectID, typeID, edit.custom); err != nil {
		return nil, err
	}
	return edit.custom, nil
}
