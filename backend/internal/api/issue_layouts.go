package api

// Issue layouts: the fields of an issue type's create and view forms. Global
// administrators shape a type's template, project administrators override it
// for their project; everyone who browses the project reads the layout that
// applies (package layout resolves it).

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/layout"
	"solstice/internal/sqlq"
)

// Audit actions of the templates: replaced, or back to what they inherit.
const (
	auditLayoutUpdate = "issuelayout.update"
	auditLayoutReset  = "issuelayout.reset"
)

// Where a layout comes from.
const (
	layoutFromProject = "project"
	layoutFromType    = "type"
	layoutFromDefault = "default"
)

type layoutOut struct {
	Form   layout.Form    `json:"form"`
	Source string         `json:"source"`
	Fields []layout.Field `json:"fields"`
}

// layoutTemplate reads a template's fields in order: the type's when
// projectID is 0, else the project's override. None: not customised.
func (s *Server) layoutTemplate(ctx context.Context, projectID int64, typeID string, form layout.Form) ([]layout.Field, error) {
	var out []layout.Field
	err := sqlq.Each(ctx, s.db, `SELECT system_field, custom_field_id, zone, hidden, required FROM issue_layout_fields
		WHERE issue_type_id = ? AND project_key = ? AND form = ? ORDER BY position`, func(rows *sql.Rows) error {
		var system, zone sql.NullString
		var custom sql.NullInt64
		var f layout.Field
		if err := rows.Scan(&system, &custom, &zone, &f.Hidden, &f.Required); err != nil {
			return err
		}
		f.ID, f.Zone = system.String, layout.Zone(zone.String)
		if custom.Valid {
			f.ID = layout.CustomID(custom.Int64)
		}
		out = append(out, f)
		return nil
	}, typeID, projectID, form)
	return out, err
}

// writeLayoutTemplate replaces a template's fields; none resets it.
func (s *Server) writeLayoutTemplate(ctx context.Context, projectID int64, typeID string, form layout.Form,
	fields []layout.Field) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM issue_layout_fields WHERE issue_type_id = ? AND project_key = ? AND form = ?`,
		typeID, projectID, form); err != nil {
		return err
	}
	project := sql.NullInt64{Int64: projectID, Valid: projectID != 0}
	for i, f := range fields {
		system, custom := sql.NullString{String: f.ID, Valid: true}, sql.NullInt64{}
		if id := f.Custom(); id != 0 {
			system, custom = sql.NullString{}, sql.NullInt64{Int64: id, Valid: true}
		}
		zone := sql.NullString{String: string(f.Zone), Valid: f.Zone != ""}
		if _, err := tx.ExecContext(ctx, `INSERT INTO issue_layout_fields
			(project_id, issue_type_id, form, system_field, custom_field_id, zone, position, hidden, required)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, project, typeID, form, system, custom, zone, i+1, f.Hidden, f.Required); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// placeableCustom lists the custom fields a layout may place, in the order
// of the issue forms: those whose type the forms show.
func (s *Server) placeableCustom(ctx context.Context) ([]int64, error) {
	all, err := s.customFields(ctx)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, f := range all {
		if fieldTypeName(f.TypeKey) != "" {
			ids = append(ids, f.ID)
		}
	}
	return ids, nil
}

// readLayout decodes the fields of a PUT and checks a form may hold them.
func (s *Server) readLayout(r *http.Request, form layout.Form) ([]layout.Field, error) {
	var in struct {
		Fields []layout.Field `json:"fields"`
	}
	if err := readJSON(r, &in); err != nil {
		return nil, err
	}
	if err := layout.Check(form, in.Fields); err != nil {
		var e *layout.Error
		if errors.As(err, &e) {
			return nil, failf(http.StatusBadRequest, e.Key, e.Field)
		}
		return nil, err
	}
	custom, err := s.placeableCustom(r.Context())
	if err != nil {
		return nil, err
	}
	for _, f := range in.Fields {
		if id := f.Custom(); id != 0 && !slices.Contains(custom, id) {
			return nil, failf(http.StatusBadRequest, i18n.FieldUnknown, f.ID)
		}
	}
	return in.Fields, nil
}

func pathForm(r *http.Request) (layout.Form, error) {
	form, ok := layout.ParseForm(r.PathValue("form"))
	if !ok {
		return "", errNotFound
	}
	return form, nil
}

// inheritedLayout is what a template holds, else what it inherits: the
// type's template for a project (projectID not 0), else the default.
func (s *Server) inheritedLayout(ctx context.Context, projectID int64, typeID string, form layout.Form) (layoutOut, error) {
	out := layoutOut{Form: form}
	if projectID != 0 {
		fields, err := s.layoutTemplate(ctx, projectID, typeID, form)
		if err != nil || len(fields) > 0 {
			out.Source, out.Fields = layoutFromProject, fields
			return out, err
		}
	}
	fields, err := s.layoutTemplate(ctx, 0, typeID, form)
	if err != nil || len(fields) > 0 {
		out.Source, out.Fields = layoutFromType, fields
		return out, err
	}
	out.Source, out.Fields = layoutFromDefault, layout.Default(form)
	return out, nil
}

// resolvedLayout serves the layout of a form for a project and an issue type.
func (s *Server) resolvedLayout(w http.ResponseWriter, r *http.Request) error {
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	ctx := r.Context()
	q := r.URL.Query()
	projectID, err := s.projectByKey(ctx, q.Get("project"))
	if err != nil || !sub.CanBrowseProject(projectID) {
		return errNotFound
	}
	form, ok := layout.ParseForm(q.Get("form"))
	if !ok {
		return failf(http.StatusBadRequest, i18n.LayoutFormUnknown, q.Get("form"))
	}
	typeID := q.Get("issueType")
	if has, err := s.projectHasType(ctx, projectID, typeID); err != nil || !has {
		if err != nil {
			return err
		}
		return failf(http.StatusBadRequest, i18n.IssueTypeNotInProject, typeID)
	}
	out, err := s.resolveLayout(ctx, projectID, typeID, form)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// resolveLayout gives the layout of a form for a project and an issue type,
// with the custom fields their context offers.
func (s *Server) resolveLayout(ctx context.Context, projectID int64, typeID string, form layout.Form) (layoutOut, error) {
	out := layoutOut{Form: form, Source: layoutFromDefault}
	override, err := s.layoutTemplate(ctx, projectID, typeID, form)
	if err != nil {
		return out, err
	}
	typ, err := s.layoutTemplate(ctx, 0, typeID, form)
	if err != nil {
		return out, err
	}
	custom, err := s.placeableCustom(ctx)
	if err != nil {
		return out, err
	}
	outside, err := s.fieldsOutside(ctx, projectID, typeID, custom)
	if err != nil {
		return out, err
	}
	offered := slices.DeleteFunc(custom, func(id int64) bool { return slices.Contains(outside, id) })
	out.Fields = layout.Resolve(form, override, typ, offered)
	switch {
	case len(override) > 0:
		out.Source = layoutFromProject
	case len(typ) > 0:
		out.Source = layoutFromType
	}
	return out, nil
}

// typeLayout serves a type's template to global administrators: GET reads
// it, PUT replaces it, DELETE goes back to the default.
func (s *Server) typeLayout(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ctx := r.Context()
	typ, err := s.catalogueItem(ctx, catalogueKinds["issue-types"], r.PathValue("id"))
	if err != nil {
		return err
	}
	form, err := pathForm(r)
	if err != nil {
		return err
	}
	target := audit.Target{Type: audit.TargetType(catalogueKinds["issue-types"].audit), ID: typ.ID, Label: typ.Name}
	return s.serveLayout(w, r, 0, typ.ID, form, target, map[string]any{})
}

// projectLayout serves a project's override of a type's template to the
// project's administrators: GET reads it, PUT replaces it, DELETE goes back
// to the type's template.
func (s *Server) projectLayout(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	ctx := r.Context()
	typeID := r.PathValue("id")
	if has, err := s.projectHasType(ctx, projectID, typeID); err != nil || !has {
		if err != nil {
			return err
		}
		return errNotFound
	}
	form, err := pathForm(r)
	if err != nil {
		return err
	}
	var key string
	if err := s.db.QueryRowContext(ctx, `SELECT pkey FROM projects WHERE id = ?`, projectID).Scan(&key); err != nil {
		return err
	}
	return s.serveLayout(w, r, projectID, typeID, form, target(audit.TargetProject, projectID, key),
		map[string]any{"issueType": typeID})
}

func (s *Server) serveLayout(w http.ResponseWriter, r *http.Request, projectID int64, typeID string, form layout.Form,
	target audit.Target, details map[string]any) error {
	ctx := r.Context()
	details["form"] = form
	switch r.Method {
	case http.MethodPut:
		fields, err := s.readLayout(r, form)
		if err != nil {
			return err
		}
		if err := s.writeLayoutTemplate(ctx, projectID, typeID, form, fields); err != nil {
			return err
		}
		ids := make([]string, 0, len(fields))
		for _, f := range fields {
			ids = append(ids, f.ID)
		}
		details["fields"] = ids
		s.auditLog(r, auditLayoutUpdate, target, details)
	case http.MethodDelete:
		if err := s.writeLayoutTemplate(ctx, projectID, typeID, form, nil); err != nil {
			return err
		}
		s.auditLog(r, auditLayoutReset, target, details)
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	out, err := s.inheritedLayout(ctx, projectID, typeID, form)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}
