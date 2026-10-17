package api

// Versions and components through the REST API: reading one by its id, and
// creating, editing and deleting them. The interface's handlers do the work
// (they check that the user administers the project, validate, write the
// history and the audit log); these translate Jira's requests and answers.

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"solstice/internal/i18n"
)

// restBody reads a flat Jira body. Keys in writable are returned; keys in
// echoed are what a client sends back after reading the object (read-only
// values, ignored); any other key is refused, as unknown fields are in an
// issue's "fields".
func restBody(r *http.Request, writable []string, echoed ...string) (map[string]json.RawMessage, error) {
	var in map[string]json.RawMessage
	if err := readRESTJSON(r, &in); err != nil {
		return nil, err
	}
	for _, k := range slices.Sorted(maps.Keys(in)) {
		if !slices.Contains(writable, k) && !slices.Contains(echoed, k) {
			return nil, fieldError(k, failf(http.StatusBadRequest, i18n.FieldNotSettable, k))
		}
	}
	return in, nil
}

// restPartProject finds the project of a version or a component (table) from
// its id.
func (s *Server) restPartProject(r *http.Request, table string) (id, projectID int64, key string, err error) {
	if id, err = strconv.ParseInt(r.PathValue("id"), 10, 64); err != nil {
		return 0, 0, "", errNotFound
	}
	err = s.db.QueryRowContext(r.Context(), `SELECT p.id, p.pkey FROM `+table+` t JOIN projects p ON p.id = t.project_id
		WHERE t.id = ?`, id).Scan(&projectID, &key)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, "", errNotFound
	}
	return id, projectID, key, err
}

// restPartBrowse is restPartProject for reading: the user must see the project.
func (s *Server) restPartBrowse(r *http.Request, table string) (id, projectID int64, key string, err error) {
	if id, projectID, key, err = s.restPartProject(r, table); err != nil {
		return 0, 0, "", err
	}
	sub, err := s.subject(r)
	if err != nil {
		return 0, 0, "", err
	}
	if !sub.Admin && !sub.CanBrowseProject(projectID) {
		return 0, 0, "", errNotFound
	}
	return id, projectID, key, nil
}

// restBodyProject finds the project a new version or component belongs to:
// "project" (its key) or "projectId".
func (s *Server) restBodyProject(ctx context.Context, in map[string]json.RawMessage) (string, error) {
	var ref string
	if raw, ok := in["project"]; ok && string(raw) != "null" {
		if json.Unmarshal(raw, &ref) != nil || ref == "" {
			return "", invalidValue("project")
		}
	} else if raw, ok := in["projectId"]; ok && string(raw) != "null" {
		var n json.Number
		if json.Unmarshal(raw, &n) != nil {
			return "", invalidValue("projectId")
		}
		ref = n.String()
	} else {
		return "", invalidValue("project")
	}
	id, err := s.restProjectID(ctx, ref)
	if err != nil {
		return "", fieldError("project", err)
	}
	return s.projectKeyOf(ctx, id), nil
}

// copyFields puts the fields of a Jira body that the interface's API names
// the same into its body.
func copyFields(ui map[string]any, in map[string]json.RawMessage, names ...string) {
	for _, k := range names {
		if v, ok := in[k]; ok {
			ui[k] = v
		}
	}
}

// restMoveTo reads the id of a query parameter naming where issues move.
func restMoveTo(r *http.Request, name string) (*int64, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil, invalidValue(name)
	}
	return &id, nil
}

// Versions ---------------------------------------------------------------------

// versionBodyFields are what a version's body may set.
var versionBodyFields = []string{"name", "description", "startDate", "releaseDate", "released", "archived"}

// partVersion is a version as Jira shows one: the project's list shows less.
func (o *restOut) partVersion(v versionAdminOut, projectID int64) map[string]any {
	var release sql.NullTime
	if v.ReleaseDate != nil {
		t, err := time.Parse(time.DateOnly, *v.ReleaseDate)
		release = sql.NullTime{Time: t, Valid: err == nil}
	}
	out := o.version(versionData{ID: v.ID, Name: v.Name, Released: v.Released, Archived: v.Archived, Release: release},
		projectID)
	if v.Description != nil {
		out["description"] = *v.Description
	}
	if v.StartDate != nil {
		out["startDate"] = *v.StartDate
	}
	return out
}

func (s *Server) restVersion(w http.ResponseWriter, r *http.Request) error {
	id, projectID, _, err := s.restPartBrowse(r, "versions")
	if err != nil {
		return err
	}
	v, err := s.version(r.Context(), projectID, id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, s.restOut(r).partVersion(v, projectID))
}

// restSaveVersion creates a version (no id in the path) or changes one.
func (s *Server) restSaveVersion(w http.ResponseWriter, r *http.Request) error {
	creating := r.PathValue("id") == ""
	in, err := restBody(r, append([]string{"project", "projectId"}, versionBodyFields...),
		"self", "id", "overdue", "userStartDate", "userReleaseDate")
	if err != nil {
		return err
	}
	ui := map[string]any{}
	copyFields(ui, in, versionBodyFields...)
	var out versionAdminOut
	var projectID int64
	if creating {
		key, err := s.restBodyProject(r.Context(), in)
		if err != nil {
			return err
		}
		if projectID, err = s.projectByKey(r.Context(), key); err != nil {
			return err
		}
		err = s.uiCall(r, s.saveVersion, http.MethodPost, "/api/projects/"+key+"/versions",
			map[string]string{"key": key}, ui, &out)
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusCreated, s.restOut(r).partVersion(out, projectID))
	}
	id, projectID, key, err := s.restPartProject(r, "versions")
	if err != nil {
		return err
	}
	sid := strconv.FormatInt(id, 10)
	err = s.uiCall(r, s.saveVersion, http.MethodPatch, "/api/projects/"+key+"/versions/"+sid,
		map[string]string{"key": key, "id": sid}, ui, &out)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, s.restOut(r).partVersion(out, projectID))
}

// restDeleteVersion deletes a version; moveFixIssuesTo and moveAffectedIssuesTo
// name the versions its issues get instead.
func (s *Server) restDeleteVersion(w http.ResponseWriter, r *http.Request) error {
	id, _, key, err := s.restPartProject(r, "versions")
	if err != nil {
		return err
	}
	fixTo, err := restMoveTo(r, "moveFixIssuesTo")
	if err != nil {
		return err
	}
	affectsTo, err := restMoveTo(r, "moveAffectedIssuesTo")
	if err != nil {
		return err
	}
	ui := map[string]any{}
	if fixTo != nil {
		ui["fixTo"] = *fixTo
	}
	if affectsTo != nil {
		ui["affectsTo"] = *affectsTo
	}
	sid := strconv.FormatInt(id, 10)
	if err := s.uiCall(r, s.deleteVersion, http.MethodDelete, "/api/projects/"+key+"/versions/"+sid,
		map[string]string{"key": key, "id": sid}, ui, nil); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Components -------------------------------------------------------------------

// partComponent is a component as Jira shows one: the project's list shows less.
func (s *Server) partComponent(ctx context.Context, o *restOut, c componentAdminOut, projectID int64,
	key string) (map[string]any, error) {
	out := o.components(&projectDetail{Project: projectOut{ID: projectID, Key: key},
		Components: []componentOut{{ID: c.ID, Name: c.Name, Archived: c.Archived}}})[0].(map[string]any)
	out["assigneeType"] = strings.ToUpper(c.AssigneeType)
	if c.Description != nil {
		out["description"] = *c.Description
	}
	if c.Lead != nil {
		users, err := s.jiraUsers(ctx, o, c.Lead.ID)
		if err != nil {
			return nil, err
		}
		out["lead"] = userOf(users, c.Lead.ID)
	}
	return out, nil
}

func (s *Server) restComponent(w http.ResponseWriter, r *http.Request) error {
	id, projectID, key, err := s.restPartBrowse(r, "components")
	if err != nil {
		return err
	}
	list, err := s.components(r.Context(), projectID, "c.id = ?", id)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return errNotFound
	}
	out, err := s.partComponent(r.Context(), s.restOut(r), list[0], projectID, key)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// restSaveComponent creates a component (no id in the path) or changes one.
func (s *Server) restSaveComponent(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	creating := r.PathValue("id") == ""
	in, err := restBody(r, []string{"project", "projectId", "name", "description", "leadUserName", "assigneeType",
		"archived"}, "self", "id", "lead", "isAssigneeTypeValid", "assignee", "realAssignee", "realAssigneeType")
	if err != nil {
		return err
	}
	ui := map[string]any{}
	copyFields(ui, in, "name", "description", "archived")
	if raw, ok := in["assigneeType"]; ok {
		var kind string
		if json.Unmarshal(raw, &kind) != nil {
			return invalidValue("assigneeType")
		}
		ui["assigneeType"] = strings.ToLower(kind)
	}
	if raw, ok := in["leadUserName"]; ok {
		var name *string
		if json.Unmarshal(raw, &name) != nil {
			return invalidValue("leadUserName")
		}
		ui["lead"] = nil
		if name != nil && *name != "" {
			lead, found := s.userByName(ctx, *name)
			if !found {
				return unknownValue("leadUserName", *name)
			}
			ui["lead"] = lead
		}
	}
	var out componentAdminOut
	var projectID int64
	var key string
	if creating {
		if key, err = s.restBodyProject(ctx, in); err != nil {
			return err
		}
		if projectID, err = s.projectByKey(ctx, key); err != nil {
			return err
		}
		err = s.uiCall(r, s.saveComponent, http.MethodPost, "/api/projects/"+key+"/components",
			map[string]string{"key": key}, ui, &out)
	} else {
		var id int64
		if id, projectID, key, err = s.restPartProject(r, "components"); err != nil {
			return err
		}
		sid := strconv.FormatInt(id, 10)
		err = s.uiCall(r, s.saveComponent, http.MethodPatch, "/api/projects/"+key+"/components/"+sid,
			map[string]string{"key": key, "id": sid}, ui, &out)
	}
	if err != nil {
		return err
	}
	jc, err := s.partComponent(ctx, s.restOut(r), out, projectID, key)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if creating {
		status = http.StatusCreated
	}
	return writeJSON(w, status, jc)
}

// restDeleteComponent deletes a component; moveIssuesTo names the component
// its issues get instead.
func (s *Server) restDeleteComponent(w http.ResponseWriter, r *http.Request) error {
	id, _, key, err := s.restPartProject(r, "components")
	if err != nil {
		return err
	}
	moveTo, err := restMoveTo(r, "moveIssuesTo")
	if err != nil {
		return err
	}
	ui := map[string]any{}
	if moveTo != nil {
		ui["moveTo"] = *moveTo
	}
	sid := strconv.FormatInt(id, 10)
	if err := s.uiCall(r, s.deleteComponent, http.MethodDelete, "/api/projects/"+key+"/components/"+sid,
		map[string]string{"key": key, "id": sid}, ui, nil); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Filters ----------------------------------------------------------------------

// restShare is one of a filter's sharePermissions, as Jira writes it.
type restShare struct {
	Type  string `json:"type"`
	Group *struct {
		Name string `json:"name"`
	} `json:"group"`
	Project *struct {
		ID  json.Number `json:"id"`
		Key string      `json:"key"`
	} `json:"project"`
	Role *struct {
		ID json.Number `json:"id"`
	} `json:"role"`
	User *struct {
		Name string `json:"name"`
		Key  string `json:"key"`
	} `json:"user"`
}

// filterShares turns sharePermissions into the shares the interface knows:
// everyone, a group, a project (or a role in it), a person.
func (s *Server) filterShares(ctx context.Context, raw json.RawMessage) ([]filterShare, error) {
	var list []restShare
	if json.Unmarshal(raw, &list) != nil {
		return nil, invalidValue("sharePermissions")
	}
	shares := []filterShare{}
	for _, in := range list {
		switch in.Type {
		case "global":
			shares = append(shares, filterShare{Kind: "everyone"})
		case "group":
			if in.Group == nil {
				return nil, invalidValue("sharePermissions")
			}
			var id int64
			if err := s.db.QueryRowContext(ctx, `SELECT id FROM user_groups WHERE name = ?`, in.Group.Name).Scan(&id); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, unknownValue("sharePermissions", in.Group.Name)
				}
				return nil, err
			}
			shares = append(shares, filterShare{Kind: "group", ID: id})
		case "project", "projectRole":
			if in.Project == nil {
				return nil, invalidValue("sharePermissions")
			}
			ref := in.Project.ID.String()
			if ref == "" {
				ref = in.Project.Key
			}
			id, err := s.restProjectID(ctx, ref)
			if errors.Is(err, errNotFound) {
				return nil, unknownValue("sharePermissions", ref)
			} else if err != nil {
				return nil, err
			}
			share := filterShare{Kind: "project", ID: id}
			if in.Type == "projectRole" {
				if in.Role == nil {
					return nil, invalidValue("sharePermissions")
				}
				if share.Role, err = strconv.ParseInt(in.Role.ID.String(), 10, 64); err != nil {
					return nil, invalidValue("sharePermissions")
				}
			}
			shares = append(shares, share)
		case "user":
			if in.User == nil {
				return nil, invalidValue("sharePermissions")
			}
			name := cmp.Or(in.User.Name, in.User.Key)
			id, found := s.userByName(ctx, name)
			if !found {
				return nil, unknownValue("sharePermissions", name)
			}
			shares = append(shares, filterShare{Kind: "user", ID: id})
		default:
			return nil, fieldError("sharePermissions", failf(http.StatusBadRequest, i18n.FilterShareKindUnknown, in.Type))
		}
	}
	return shares, nil
}

// restSaveFilter creates a filter (no id in the path) or changes one.
func (s *Server) restSaveFilter(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	creating := r.PathValue("id") == ""
	in, err := restBody(r, []string{"name", "description", "jql", "favourite", "sharePermissions"},
		"self", "id", "owner", "viewUrl", "searchUrl", "subscriptions")
	if err != nil {
		return err
	}
	ui := map[string]any{}
	copyFields(ui, in, "name")
	if raw, ok := in["description"]; ok {
		// Null clears the description, which the interface does by an empty one.
		ui["description"] = raw
		if string(raw) == "null" {
			ui["description"] = ""
		}
	}
	if raw, ok := in["jql"]; ok {
		ui["query"] = raw
	}
	if raw, ok := in["sharePermissions"]; ok {
		if ui["shares"], err = s.filterShares(ctx, raw); err != nil {
			return err
		}
	}
	var favourite *bool
	if raw, ok := in["favourite"]; ok {
		if json.Unmarshal(raw, &favourite) != nil {
			return invalidValue("favourite")
		}
	}
	var saved filterOut
	// A body with only "favourite" changes nothing else, so there is nothing to save.
	favouriteOnly := !creating && len(ui) == 0 && favourite != nil
	id := r.PathValue("id")
	switch {
	case creating:
		err = s.uiCall(r, s.createFilter, http.MethodPost, "/api/filters", nil, ui, &saved)
	case favouriteOnly:
		saved.ID, _ = strconv.ParseInt(id, 10, 64)
	default:
		err = s.uiCall(r, s.updateFilter, http.MethodPatch, "/api/filters/"+id, map[string]string{"id": id}, ui, &saved)
		if err == nil && saved.ID == 0 {
			saved.ID, _ = strconv.ParseInt(id, 10, 64)
		}
	}
	if err != nil {
		return err
	}
	if favourite != nil {
		id, method := strconv.FormatInt(saved.ID, 10), http.MethodDelete
		if *favourite {
			method = http.MethodPut
		}
		// The save and the favourite are two steps, not one transaction. When
		// the save went through, a failing favourite is logged, not answered
		// with an error: the client would take the saved change for a failed one.
		if err := s.uiCall(r, s.setFavourite(*favourite), method, "/api/filters/"+id+"/favourite",
			map[string]string{"id": id}, nil, nil); err != nil {
			if favouriteOnly {
				return err
			}
			s.log.Warn("setting the favourite of a saved filter failed", "filter", saved.ID, "err", err)
		}
	}
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	rows, err := s.loadFilters(ctx, sub, saved.ID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		// An administrator unshared someone else's filter: they no longer see it.
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	out, err := s.jiraFilter(ctx, s.restOut(r), rows[0].out)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}
