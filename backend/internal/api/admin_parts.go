package api

// A project's components and versions. Anyone who sees the project reads
// them; project administrators change them. Moving issues from a version
// to another (release, merge, delete) is recorded in their history.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/sqlq"
	"strconv"
	"strings"
	"time"
)

// browsableProject loads a project the user sees (or, for a global
// administrator, any project).
func (s *Server) browsableProject(r *http.Request) (int64, error) {
	sub, err := s.subject(r)
	if err != nil {
		return 0, err
	}
	id, err := s.projectByKey(r.Context(), r.PathValue("key"))
	if err != nil {
		return 0, err
	}
	if !sub.Admin && !sub.CanBrowseProject(id) {
		return 0, errNotFound
	}
	return id, nil
}

func (s *Server) projectKeyOf(ctx context.Context, id int64) string {
	var key string
	s.db.QueryRowContext(ctx, `SELECT pkey FROM projects WHERE id = ?`, id).Scan(&key)
	return key
}

// Components --------------------------------------------------------------------

var assigneeTypes = []string{"project_default", "component_lead", "project_lead", "unassigned"}

type componentAdminOut struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Description  *string  `json:"description"`
	Lead         *userRef `json:"lead"`
	AssigneeType string   `json:"assigneeType"`
	Archived     bool     `json:"archived"`
	Issues       int      `json:"issues"`
}

func (s *Server) components(ctx context.Context, projectID int64, where string, args ...any) ([]componentAdminOut, error) {
	out := []componentAdminOut{}
	var leads []sql.NullInt64
	var ids []int64
	err := sqlq.Each(ctx, s.db, `SELECT c.id, c.name, c.description, c.lead_id, c.assignee_type, c.archived,
		(SELECT COUNT(*) FROM issue_components ic WHERE ic.component_id = c.id)
		FROM components c WHERE c.project_id = ? AND `+where+` ORDER BY c.name, c.id`, func(rows *sql.Rows) error {
		var c componentAdminOut
		var desc sql.NullString
		var lead sql.NullInt64
		if err := rows.Scan(&c.ID, &c.Name, &desc, &lead, &c.AssigneeType, &c.Archived, &c.Issues); err != nil {
			return err
		}
		c.Description = sqlq.String(desc)
		leads = append(leads, lead)
		ids = append(ids, lead.Int64)
		out = append(out, c)
		return nil
	}, append([]any{projectID}, args...)...)
	if err != nil {
		return nil, err
	}
	users, err := s.users(ctx, ids...)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Lead = refOf(users, leads[i])
	}
	return out, nil
}

func (s *Server) listComponents(w http.ResponseWriter, r *http.Request) error {
	id, err := s.browsableProject(r)
	if err != nil {
		return err
	}
	out, err := s.components(r.Context(), id, "TRUE")
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) componentNameTaken(ctx context.Context, projectID int64, name string, except int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM components WHERE project_id = ? AND LOWER(name) = LOWER(?)
		AND id <> ?`, projectID, name, except).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return failf(http.StatusConflict, i18n.CommonAlreadyExists, name)
	}
	return nil
}

// saveComponent creates a component (no id in the path) or changes one.
func (s *Server) saveComponent(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	var in struct {
		Name         *string `json:"name"`
		Description  *string `json:"description"`
		Lead         *int64  `json:"lead"`
		AssigneeType *string `json:"assigneeType"`
		Archived     *bool   `json:"archived"`
	}
	raw, err := readJSONFields(r, &in)
	if err != nil {
		return err
	}
	ctx := r.Context()
	creating := r.PathValue("id") == ""
	var id int64
	var before componentAdminOut
	if !creating {
		if id, err = pathID(r, "id"); err != nil {
			return err
		}
		list, err := s.components(ctx, projectID, "c.id = ?", id)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return errNotFound
		}
		before = list[0]
	} else if in.Name == nil {
		return failf(http.StatusBadRequest, i18n.CommonNameRequired, 255)
	}
	sets := map[string]any{}
	if in.Name != nil {
		name, err := cleanName(*in.Name, 255)
		if err != nil {
			return err
		}
		if err := s.componentNameTaken(ctx, projectID, name, id); err != nil {
			return err
		}
		sets["name"] = name
	}
	if _, ok := raw["description"]; ok {
		sets["description"] = optDescription(in.Description)
	}
	if _, ok := raw["lead"]; ok {
		if in.Lead != nil {
			if err := s.activeUser(ctx, *in.Lead); err != nil {
				return err
			}
		}
		sets["lead_id"] = in.Lead
	}
	if in.AssigneeType != nil {
		if !slices.Contains(assigneeTypes, *in.AssigneeType) {
			return failf(http.StatusBadRequest, i18n.ComponentAssigneeTypeUnknown,
				strings.Join(assigneeTypes, ", "))
		}
		sets["assignee_type"] = *in.AssigneeType
	}
	if in.Archived != nil {
		sets["archived"] = *in.Archived
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if creating {
		res, err := tx.ExecContext(ctx, `INSERT INTO components (project_id, name) VALUES (?, ?)`, projectID, sets["name"])
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
	}
	for column, v := range sets {
		if _, err := tx.ExecContext(ctx, `UPDATE components SET `+column+` = ? WHERE id = ?`, v, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	list, err := s.components(ctx, projectID, "c.id = ?", id)
	if err != nil || len(list) == 0 {
		return err
	}
	c := list[0]
	target := audit.Target{Type: "component", ID: strconv.FormatInt(id, 10), Label: c.Name}
	project := s.projectKeyOf(ctx, projectID)
	if creating {
		s.auditLog(r, audit.ComponentCreate, target, map[string]any{"project": project})
		return writeJSON(w, http.StatusCreated, c)
	}
	changes := map[string]any{}
	for k, v := range map[string][2]any{
		"name": {before.Name, c.Name}, "assigneeType": {before.AssigneeType, c.AssigneeType},
		"archived": {before.Archived, c.Archived}, "lead": {leadName(before.Lead), leadName(c.Lead)},
		"description": {derefStr(before.Description), derefStr(c.Description)},
	} {
		if v[0] != v[1] {
			changes[k] = map[string]any{"from": v[0], "to": v[1]}
		}
	}
	if len(changes) > 0 {
		changes["project"] = project
		s.auditLog(r, audit.ComponentUpdate, target, changes)
	}
	return writeJSON(w, http.StatusOK, c)
}

func leadName(u *userRef) string {
	if u == nil {
		return ""
	}
	return u.Username
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// deleteComponent removes a component; its issues lose it, or get moveTo.
func (s *Server) deleteComponent(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		MoveTo *int64 `json:"moveTo"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &in); err != nil {
			return err
		}
	}
	ctx := r.Context()
	list, err := s.components(ctx, projectID, "c.id = ?", id)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return errNotFound
	}
	c := list[0]
	var target *componentAdminOut
	if in.MoveTo != nil {
		other, err := s.components(ctx, projectID, "c.id = ?", *in.MoveTo)
		if err != nil {
			return err
		}
		if len(other) == 0 || *in.MoveTo == id {
			return fail(http.StatusBadRequest, i18n.ComponentReplacementNotFound)
		}
		target = &other[0]
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	issues, err := idsOf(ctx, tx, `SELECT issue_id FROM issue_components WHERE component_id = ?`, id)
	if err != nil {
		return err
	}
	for _, issue := range issues {
		changes := []change{{field: "Component", oldValue: strconv.FormatInt(id, 10), oldString: c.Name}}
		if target != nil {
			res, err := tx.ExecContext(ctx, `INSERT IGNORE INTO issue_components (issue_id, component_id) VALUES (?, ?)`,
				issue, target.ID)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				changes = append(changes, change{field: "Component", newValue: strconv.FormatInt(target.ID, 10),
					newString: target.Name})
			}
		}
		if err := touchIssue(ctx, tx, issue, now); err != nil {
			return err
		}
		if err := recordChanges(ctx, tx, issue, currentUser(r).ID, now, changes); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM components WHERE id = ?`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	details := map[string]any{"project": s.projectKeyOf(ctx, projectID), "issues": len(issues)}
	if target != nil {
		details["movedTo"] = target.Name
	}
	s.auditLog(r, audit.ComponentDelete, audit.Target{Type: "component", ID: strconv.FormatInt(id, 10), Label: c.Name},
		details)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func touchIssue(ctx context.Context, tx *sql.Tx, issue int64, now time.Time) error {
	_, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, now, issue)
	return err
}

func idsOf(ctx context.Context, tx *sql.Tx, q string, args ...any) ([]int64, error) {
	var out []int64
	err := sqlq.Each(ctx, tx, q, func(rows *sql.Rows) error {
		var id int64
		err := rows.Scan(&id)
		out = append(out, id)
		return err
	}, args...)
	return out, err
}

// componentAssignee applies the default assignee of a new issue's
// components (by name, the first that decides): ok is false when none
// decides and the project's default applies.
func (s *Server) componentAssignee(ctx context.Context, f fields, projectID int64, projectLead int64) (any, bool, error) {
	if !f.has("components") || f.isNull("components") {
		return nil, false, nil
	}
	var ids []int64
	if err := f.get("components", &ids); err != nil || len(ids) == 0 {
		return nil, false, err
	}
	args := []any{projectID}
	for _, id := range ids {
		args = append(args, id)
	}
	var who any
	decided := false
	err := sqlq.Each(ctx, s.db, `SELECT assignee_type, lead_id FROM components WHERE project_id = ? AND id IN (`+
		sqlq.Marks(len(ids))+`) ORDER BY name, id`, func(rows *sql.Rows) error {
		var kind string
		var lead sql.NullInt64
		if err := rows.Scan(&kind, &lead); err != nil {
			return err
		}
		if decided {
			return nil
		}
		switch kind {
		case "component_lead":
			if lead.Valid {
				who, decided = lead.Int64, true
			}
		case "project_lead":
			if projectLead != 0 {
				who, decided = projectLead, true
			}
		case "unassigned":
			who, decided = nil, true
		}
		return nil
	}, args...)
	if err != nil || !decided || who == nil {
		return who, decided, err
	}
	// Someone who may not be assigned issues here is not imposed.
	ok, err := s.isAssignable(ctx, projectID, who.(int64))
	if err != nil || !ok {
		return nil, false, err
	}
	return who, true, nil
}

// Versions ----------------------------------------------------------------------

type versionAdminOut struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	StartDate   *string `json:"startDate"`
	ReleaseDate *string `json:"releaseDate"`
	Released    bool    `json:"released"`
	Archived    bool    `json:"archived"`
	Position    int     `json:"position"`
	Issues      struct {
		Total      int `json:"total"`
		Unresolved int `json:"unresolved"`
	} `json:"issues"`
}

func (s *Server) versions(ctx context.Context, projectID int64, where string, args ...any) ([]versionAdminOut, error) {
	out := []versionAdminOut{}
	err := sqlq.Each(ctx, s.db, `SELECT v.id, v.name, v.description, v.start_date, v.release_date, v.released, v.archived,
		v.position,
		(SELECT COUNT(DISTINCT iv.issue_id) FROM issue_versions iv WHERE iv.version_id = v.id),
		(SELECT COUNT(DISTINCT iv.issue_id) FROM issue_versions iv JOIN issues i ON i.id = iv.issue_id
			WHERE iv.version_id = v.id AND iv.kind = 'fix' AND i.resolution_id IS NULL)
		FROM versions v WHERE v.project_id = ? AND `+where+` ORDER BY v.position, v.id`, func(rows *sql.Rows) error {
		var v versionAdminOut
		var desc sql.NullString
		var start, release sql.NullTime
		if err := rows.Scan(&v.ID, &v.Name, &desc, &start, &release, &v.Released, &v.Archived, &v.Position,
			&v.Issues.Total, &v.Issues.Unresolved); err != nil {
			return err
		}
		v.Description, v.StartDate, v.ReleaseDate = sqlq.String(desc), dateOnly(start), dateOnly(release)
		out = append(out, v)
		return nil
	}, append([]any{projectID}, args...)...)
	return out, err
}

func (s *Server) version(ctx context.Context, projectID, id int64) (versionAdminOut, error) {
	list, err := s.versions(ctx, projectID, "v.id = ?", id)
	if err != nil {
		return versionAdminOut{}, err
	}
	if len(list) == 0 {
		return versionAdminOut{}, errNotFound
	}
	return list[0], nil
}

func (s *Server) listVersions(w http.ResponseWriter, r *http.Request) error {
	id, err := s.browsableProject(r)
	if err != nil {
		return err
	}
	out, err := s.versions(r.Context(), id, "TRUE")
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) versionNameTaken(ctx context.Context, projectID int64, name string, except int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM versions WHERE project_id = ? AND LOWER(name) = LOWER(?)
		AND id <> ?`, projectID, name, except).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return failf(http.StatusConflict, i18n.CommonAlreadyExists, name)
	}
	return nil
}

// optDate reads an optional day: null or "" clears it.
func optDate(raw map[string]json.RawMessage, key string) (any, bool, error) {
	v, ok := raw[key]
	if !ok {
		return nil, false, nil
	}
	var s *string
	if json.Unmarshal(v, &s) != nil {
		return nil, true, failf(http.StatusBadRequest, i18n.FieldValueInvalid, key)
	}
	if s == nil || *s == "" {
		return nil, true, nil
	}
	if _, err := time.Parse(time.DateOnly, *s); err != nil {
		return nil, true, failf(http.StatusBadRequest, i18n.CommonDateInvalid, *s)
	}
	return *s, true, nil
}

// saveVersion creates a version (no id in the path) or changes one.
func (s *Server) saveVersion(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	var in struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		StartDate   *string `json:"startDate"`
		ReleaseDate *string `json:"releaseDate"`
		Released    *bool   `json:"released"`
		Archived    *bool   `json:"archived"`
	}
	raw, err := readJSONFields(r, &in)
	if err != nil {
		return err
	}
	ctx := r.Context()
	creating := r.PathValue("id") == ""
	var id int64
	var before versionAdminOut
	if !creating {
		if id, err = pathID(r, "id"); err != nil {
			return err
		}
		if before, err = s.version(ctx, projectID, id); err != nil {
			return err
		}
	} else if in.Name == nil {
		return failf(http.StatusBadRequest, i18n.CommonNameRequired, 255)
	}
	sets := map[string]any{}
	if in.Name != nil {
		name, err := cleanName(*in.Name, 255)
		if err != nil {
			return err
		}
		if err := s.versionNameTaken(ctx, projectID, name, id); err != nil {
			return err
		}
		sets["name"] = name
	}
	if _, ok := raw["description"]; ok {
		sets["description"] = optDescription(in.Description)
	}
	start, release := any(nil), any(nil)
	if before.StartDate != nil {
		start = *before.StartDate
	}
	if before.ReleaseDate != nil {
		release = *before.ReleaseDate
	}
	if v, ok, err := optDate(raw, "startDate"); err != nil {
		return err
	} else if ok {
		sets["start_date"], start = v, v
	}
	if v, ok, err := optDate(raw, "releaseDate"); err != nil {
		return err
	} else if ok {
		sets["release_date"], release = v, v
	}
	if a, ok := start.(string); ok {
		if b, ok := release.(string); ok && a > b {
			return fail(http.StatusBadRequest, i18n.VersionStartAfterRelease)
		}
	}
	if in.Released != nil {
		sets["released"] = *in.Released
	}
	if in.Archived != nil {
		sets["archived"] = *in.Archived
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if creating {
		res, err := tx.ExecContext(ctx, `INSERT INTO versions (project_id, name, position)
			SELECT ?, ?, COALESCE(MAX(position), 0) + 1 FROM versions WHERE project_id = ?`, projectID, sets["name"], projectID)
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
	}
	for column, v := range sets {
		if _, err := tx.ExecContext(ctx, `UPDATE versions SET `+column+` = ? WHERE id = ?`, v, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	v, err := s.version(ctx, projectID, id)
	if err != nil {
		return err
	}
	target := audit.Target{Type: "version", ID: strconv.FormatInt(id, 10), Label: v.Name}
	project := s.projectKeyOf(ctx, projectID)
	if creating {
		s.auditLog(r, audit.VersionCreate, target, map[string]any{"project": project})
		return writeJSON(w, http.StatusCreated, v)
	}
	changes := map[string]any{}
	for k, pair := range map[string][2]any{
		"name": {before.Name, v.Name}, "description": {derefStr(before.Description), derefStr(v.Description)},
		"startDate":   {derefStr(before.StartDate), derefStr(v.StartDate)},
		"releaseDate": {derefStr(before.ReleaseDate), derefStr(v.ReleaseDate)},
		"released":    {before.Released, v.Released}, "archived": {before.Archived, v.Archived},
	} {
		if pair[0] != pair[1] {
			changes[k] = map[string]any{"from": pair[0], "to": pair[1]}
		}
	}
	if len(changes) > 0 {
		changes["project"] = project
		s.auditLog(r, audit.VersionUpdate, target, changes)
	}
	return writeJSON(w, http.StatusOK, v)
}

// orderVersions sets the order of a project's versions.
func (s *Server) orderVersions(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	list, err := s.versions(ctx, projectID, "TRUE")
	if err != nil {
		return err
	}
	if err := sameIDs(list, in.IDs, func(v versionAdminOut) int64 { return v.ID }); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range in.IDs {
		if _, err := tx.ExecContext(ctx, `UPDATE versions SET position = ? WHERE id = ?`, i+1, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auditLog(r, audit.VersionOrder, audit.Target{Type: "project", ID: strconv.FormatInt(projectID, 10),
		Label: s.projectKeyOf(ctx, projectID)}, map[string]any{"ids": in.IDs})
	out, err := s.versions(ctx, projectID, "TRUE")
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// sameIDs checks that ids lists every item exactly once.
func sameIDs[T any, K comparable](items []T, ids []K, id func(T) K) error {
	if len(items) != len(ids) {
		return fail(http.StatusBadRequest, i18n.CommonOrderInvalid)
	}
	seen := map[K]bool{}
	for _, it := range items {
		seen[id(it)] = true
	}
	for _, k := range ids {
		if !seen[k] {
			return fail(http.StatusBadRequest, i18n.CommonOrderInvalid)
		}
		delete(seen, k)
	}
	return nil
}

// moveVersion takes a version off issues (one kind: fix or affects) and,
// when to is not nil, puts that one instead, with history. With
// unresolvedOnly, only issues without a resolution move.
func moveVersion(ctx context.Context, tx *sql.Tx, from versionAdminOut, to *versionAdminOut, kind string,
	unresolvedOnly bool, author int64, now time.Time) (int, error) {
	q := `SELECT iv.issue_id FROM issue_versions iv JOIN issues i ON i.id = iv.issue_id
		WHERE iv.version_id = ? AND iv.kind = ?`
	if unresolvedOnly {
		q += ` AND i.resolution_id IS NULL`
	}
	issues, err := idsOf(ctx, tx, q, from.ID, kind)
	if err != nil {
		return 0, err
	}
	field := "Fix Version"
	if kind == "affects" {
		field = "Version"
	}
	for _, issue := range issues {
		if _, err := tx.ExecContext(ctx, `DELETE FROM issue_versions WHERE issue_id = ? AND version_id = ? AND kind = ?`,
			issue, from.ID, kind); err != nil {
			return 0, err
		}
		changes := []change{{field: field, oldValue: strconv.FormatInt(from.ID, 10), oldString: from.Name}}
		if to != nil {
			res, err := tx.ExecContext(ctx, `INSERT IGNORE INTO issue_versions (issue_id, version_id, kind) VALUES (?, ?, ?)`,
				issue, to.ID, kind)
			if err != nil {
				return 0, err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				changes = append(changes, change{field: field, newValue: strconv.FormatInt(to.ID, 10), newString: to.Name})
			}
		}
		if err := touchIssue(ctx, tx, issue, now); err != nil {
			return 0, err
		}
		if err := recordChanges(ctx, tx, issue, author, now, changes); err != nil {
			return 0, err
		}
	}
	return len(issues), nil
}

// otherVersion loads the version issues move to: another one of the project.
func (s *Server) otherVersion(ctx context.Context, projectID, self int64, id *int64) (*versionAdminOut, error) {
	if id == nil {
		return nil, nil
	}
	v, err := s.version(ctx, projectID, *id)
	if errors.Is(err, errNotFound) || (err == nil && v.ID == self) {
		return nil, fail(http.StatusBadRequest, i18n.VersionTargetNotFound)
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// releaseVersion marks a version released; its unresolved issues may move
// to another version.
func (s *Server) releaseVersion(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		ReleaseDate      *string `json:"releaseDate"`
		MoveUnresolvedTo *int64  `json:"moveUnresolvedTo"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &in); err != nil {
			return err
		}
	}
	ctx := r.Context()
	v, err := s.version(ctx, projectID, id)
	if err != nil {
		return err
	}
	day := s.today()
	if in.ReleaseDate != nil && *in.ReleaseDate != "" {
		if _, err := time.Parse(time.DateOnly, *in.ReleaseDate); err != nil {
			return failf(http.StatusBadRequest, i18n.CommonDateInvalid, *in.ReleaseDate)
		}
		day = *in.ReleaseDate
	}
	if v.StartDate != nil && *v.StartDate > day {
		return fail(http.StatusBadRequest, i18n.VersionStartAfterRelease)
	}
	to, err := s.otherVersion(ctx, projectID, id, in.MoveUnresolvedTo)
	if err != nil {
		return err
	}
	if to != nil && to.Released {
		return fail(http.StatusBadRequest, i18n.VersionMoveToUnreleased)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	moved := 0
	if to != nil {
		if moved, err = moveVersion(ctx, tx, v, to, "fix", true, currentUser(r).ID, time.Now().UTC()); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE versions SET released = TRUE, release_date = ? WHERE id = ?`, day, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	details := map[string]any{"project": s.projectKeyOf(ctx, projectID), "releaseDate": day, "moved": moved}
	if to != nil {
		details["movedTo"] = to.Name
	}
	s.auditLog(r, audit.VersionRelease, audit.Target{Type: "version", ID: strconv.FormatInt(id, 10), Label: v.Name}, details)
	v, err = s.version(ctx, projectID, id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, v)
}

// mergeVersion moves every issue of a version to another and deletes it.
func (s *Server) mergeVersion(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Into *int64 `json:"into"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	v, err := s.version(ctx, projectID, id)
	if err != nil {
		return err
	}
	if in.Into == nil {
		return fail(http.StatusBadRequest, i18n.VersionTargetNotFound)
	}
	into, err := s.otherVersion(ctx, projectID, id, in.Into)
	if err != nil {
		return err
	}
	return s.removeVersion(w, r, projectID, v, into, into, audit.VersionMerge)
}

// deleteVersion removes a version; its issues lose it, or get the versions
// given.
func (s *Server) deleteVersion(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		FixTo     *int64 `json:"fixTo"`
		AffectsTo *int64 `json:"affectsTo"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &in); err != nil {
			return err
		}
	}
	ctx := r.Context()
	v, err := s.version(ctx, projectID, id)
	if err != nil {
		return err
	}
	fixTo, err := s.otherVersion(ctx, projectID, id, in.FixTo)
	if err != nil {
		return err
	}
	affectsTo, err := s.otherVersion(ctx, projectID, id, in.AffectsTo)
	if err != nil {
		return err
	}
	return s.removeVersion(w, r, projectID, v, fixTo, affectsTo, audit.VersionDelete)
}

func (s *Server) removeVersion(w http.ResponseWriter, r *http.Request, projectID int64, v versionAdminOut,
	fixTo, affectsTo *versionAdminOut, action string) error {
	ctx := r.Context()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now, author := time.Now().UTC(), currentUser(r).ID
	fixed, err := moveVersion(ctx, tx, v, fixTo, "fix", false, author, now)
	if err != nil {
		return err
	}
	affected, err := moveVersion(ctx, tx, v, affectsTo, "affects", false, author, now)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM versions WHERE id = ?`, v.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	details := map[string]any{"project": s.projectKeyOf(ctx, projectID), "fix": fixed, "affects": affected}
	if fixTo != nil {
		details["fixTo"] = fixTo.Name
	}
	if affectsTo != nil {
		details["affectsTo"] = affectsTo.Name
	}
	s.auditLog(r, action, audit.Target{Type: "version", ID: strconv.FormatInt(v.ID, 10), Label: v.Name}, details)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
