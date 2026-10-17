package api

// Routes of the administration (see ADMIN_API.md, the contract with the
// interface).

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"solstice/internal/i18n"
)

func (s *Server) adminRoutes(route func(string, handler, bool)) {
	// Instance settings.
	route("GET /api/settings/public", s.publicSettings, false)
	route("GET /api/admin/settings", s.getSettings, true)
	route("PUT /api/admin/settings", s.putSettings, true)

	// People.
	route("GET /api/admin/users/{id}", s.adminGetUser, true)
	route("PUT /api/admin/users/{id}/groups", s.adminUserGroups, true)
	route("POST /api/admin/users/{id}/reset-password", s.adminResetPassword, true)
	route("GET /api/admin/groups", s.listGroups, true)
	route("POST /api/admin/groups", s.createGroup, true)
	route("GET /api/admin/groups/{id}", s.getGroup, true)
	route("PATCH /api/admin/groups/{id}", s.updateGroup, true)
	route("DELETE /api/admin/groups/{id}", s.deleteGroup, true)
	route("POST /api/admin/groups/{id}/members", s.addGroupMembers, true)
	route("DELETE /api/admin/groups/{id}/members/{user}", s.removeGroupMember, true)
	route("GET /api/admin/roles", s.listRoles, true)
	route("POST /api/admin/roles", s.createRole, true)
	route("PATCH /api/admin/roles/{id}", s.updateRole, true)
	route("DELETE /api/admin/roles/{id}", s.deleteRole, true)
	route("GET /api/projects/{key}/roles", s.getProjectRoles, true)
	route("PUT /api/projects/{key}/roles/{role}", s.putProjectRole, true)

	// Projects.
	route("POST /api/projects", s.createProject, true)
	route("GET /api/admin/projects", s.adminProjects, true)
	route("GET /api/admin/project-key", s.projectKeyCheck, true)
	route("GET /api/projects/{key}/settings", s.getProjectSettings, true)
	route("PATCH /api/projects/{key}", s.updateProject, true)
	route("DELETE /api/projects/{key}", s.deleteProject, true)
	route("POST /api/projects/{key}/archive", s.archiveProject(true), true)
	route("POST /api/projects/{key}/unarchive", s.archiveProject(false), true)
	route("GET /api/project-categories", s.listCategories, true)
	route("POST /api/admin/project-categories", s.saveCategory, true)
	route("PATCH /api/admin/project-categories/{id}", s.saveCategory, true)
	route("DELETE /api/admin/project-categories/{id}", s.deleteCategory, true)
	route("GET /api/projects/{key}/components", s.listComponents, true)
	route("POST /api/projects/{key}/components", s.saveComponent, true)
	route("PATCH /api/projects/{key}/components/{id}", s.saveComponent, true)
	route("DELETE /api/projects/{key}/components/{id}", s.deleteComponent, true)
	route("GET /api/projects/{key}/versions", s.listVersions, true)
	route("POST /api/projects/{key}/versions", s.saveVersion, true)
	route("POST /api/projects/{key}/versions/order", s.orderVersions, true)
	route("PATCH /api/projects/{key}/versions/{id}", s.saveVersion, true)
	route("DELETE /api/projects/{key}/versions/{id}", s.deleteVersion, true)
	route("POST /api/projects/{key}/versions/{id}/release", s.releaseVersion, true)
	route("POST /api/projects/{key}/versions/{id}/merge", s.mergeVersion, true)
	route("GET /api/projects/{key}/fields", s.projectFields, true)

	// Catalogues.
	for kind, c := range catalogueKinds {
		route("GET /api/admin/"+kind, s.listCatalogue(c), true)
		route("POST /api/admin/"+kind, s.saveCatalogueItem(c), true)
		route("POST /api/admin/"+kind+"/order", s.orderCatalogue(c), true)
		route("PATCH /api/admin/"+kind+"/{id}", s.saveCatalogueItem(c), true)
		route("DELETE /api/admin/"+kind+"/{id}", s.deleteCatalogueItem(c), true)
	}
	route("GET /api/admin/statuses", s.adminStatuses, true)
	route("POST /api/admin/statuses/order", s.orderStatuses, true)
	route("GET /api/admin/statuses/{id}/usage", s.adminStatusUsage, true)

	// Custom fields.
	route("GET /api/admin/fields", s.adminFields, true)
	route("GET /api/admin/fields/types", s.fieldTypes, true)
	route("POST /api/admin/fields", s.createField, true)
	route("GET /api/admin/fields/{id}", s.adminField, true)
	route("PATCH /api/admin/fields/{id}", s.updateField, true)
	route("DELETE /api/admin/fields/{id}", s.deleteField, true)
	route("POST /api/admin/fields/{id}/options", s.addFieldOption, true)
	route("POST /api/admin/fields/{id}/options/order", s.orderFieldOptions, true)
	route("PATCH /api/admin/fields/{id}/options/{option}", s.updateFieldOption, true)
	route("DELETE /api/admin/fields/{id}/options/{option}", s.deleteFieldOption, true)

	// System and import.
	route("GET /api/admin/system", s.systemInfo, true)
	route("GET /api/admin/import", s.adminImportState, true)
	route("POST /api/admin/import/test", s.adminImportTest, true)
	route("POST /api/admin/import", s.adminImportStart, true)
}

// readJSONFields decodes a body like readJSON, and also tells which keys it
// holds: for the fields where absent (keep) and null (clear) differ.
func readJSONFields(r *http.Request, v any) (map[string]json.RawMessage, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 4<<20))
	if err != nil {
		return nil, badBody(err)
	}
	keys := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, failf(http.StatusBadRequest, i18n.RequestInvalid, err.Error())
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	return keys, readJSON(r, v)
}
