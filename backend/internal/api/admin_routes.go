package api

// Routes of the administration (see ADMIN_API.md, the contract with the
// interface).

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"solstice/internal/auth"
	"solstice/internal/i18n"
)

func (s *Server) adminRoutes(route routeFunc) {
	// Instance settings.
	route("GET /api/settings/public", s.publicSettings, scopePublic)
	route("GET /api/admin/settings", s.getSettings, auth.ScopeAdmin)
	route("PUT /api/admin/settings", s.putSettings, auth.ScopeAdmin)

	// People.
	route("GET /api/admin/users/{id}", s.adminGetUser, auth.ScopeAdmin)
	route("PUT /api/admin/users/{id}/groups", s.adminUserGroups, auth.ScopeAdmin)
	route("PUT /api/admin/users/{id}/avatar", s.adminPutAvatar, auth.ScopeAdmin)
	route("DELETE /api/admin/users/{id}/avatar", s.adminDeleteAvatar, auth.ScopeAdmin)
	route("POST /api/admin/users/{id}/reset-password", s.adminResetPassword, auth.ScopeAdmin)
	route("GET /api/admin/groups", s.listGroups, auth.ScopeAdmin)
	route("POST /api/admin/groups", s.createGroup, auth.ScopeAdmin)
	route("GET /api/admin/groups/{id}", s.getGroup, auth.ScopeAdmin)
	route("PATCH /api/admin/groups/{id}", s.updateGroup, auth.ScopeAdmin)
	route("DELETE /api/admin/groups/{id}", s.deleteGroup, auth.ScopeAdmin)
	route("POST /api/admin/groups/{id}/members", s.addGroupMembers, auth.ScopeAdmin)
	route("DELETE /api/admin/groups/{id}/members/{user}", s.removeGroupMember, auth.ScopeAdmin)
	route("GET /api/admin/roles", s.listRoles, auth.ScopeAdmin)
	route("POST /api/admin/roles", s.createRole, auth.ScopeAdmin)
	route("PATCH /api/admin/roles/{id}", s.updateRole, auth.ScopeAdmin)
	route("DELETE /api/admin/roles/{id}", s.deleteRole, auth.ScopeAdmin)
	route("GET /api/projects/{key}/roles", s.getProjectRoles, auth.ScopeRead)
	route("PUT /api/projects/{key}/roles/{role}", s.putProjectRole, auth.ScopeProjects)

	// Projects.
	route("POST /api/projects", s.createProject, auth.ScopeProjects)
	route("GET /api/admin/projects", s.adminProjects, auth.ScopeAdmin)
	route("GET /api/admin/project-key", s.projectKeyCheck, auth.ScopeAdmin)
	route("GET /api/projects/{key}/settings", s.getProjectSettings, auth.ScopeRead)
	route("PATCH /api/projects/{key}", s.updateProject, auth.ScopeProjects)
	route("DELETE /api/projects/{key}", s.deleteProject, auth.ScopeProjects)
	route("POST /api/projects/{key}/rename", s.renameProject, auth.ScopeProjects)
	route("POST /api/projects/{key}/archive", s.archiveProject(true), auth.ScopeProjects)
	route("POST /api/projects/{key}/unarchive", s.archiveProject(false), auth.ScopeProjects)
	route("GET /api/project-categories", s.listCategories, auth.ScopeRead)
	route("POST /api/admin/project-categories", s.saveCategory, auth.ScopeAdmin)
	route("PATCH /api/admin/project-categories/{id}", s.saveCategory, auth.ScopeAdmin)
	route("DELETE /api/admin/project-categories/{id}", s.deleteCategory, auth.ScopeAdmin)
	route("GET /api/projects/{key}/components", s.listComponents, auth.ScopeRead)
	route("POST /api/projects/{key}/components", s.saveComponent, auth.ScopeProjects)
	route("PATCH /api/projects/{key}/components/{id}", s.saveComponent, auth.ScopeProjects)
	route("DELETE /api/projects/{key}/components/{id}", s.deleteComponent, auth.ScopeProjects)
	route("GET /api/projects/{key}/versions", s.listVersions, auth.ScopeRead)
	route("POST /api/projects/{key}/versions", s.saveVersion, auth.ScopeProjects)
	route("POST /api/projects/{key}/versions/order", s.orderVersions, auth.ScopeProjects)
	route("PATCH /api/projects/{key}/versions/{id}", s.saveVersion, auth.ScopeProjects)
	route("DELETE /api/projects/{key}/versions/{id}", s.deleteVersion, auth.ScopeProjects)
	route("POST /api/projects/{key}/versions/{id}/release", s.releaseVersion, auth.ScopeProjects)
	route("POST /api/projects/{key}/versions/{id}/merge", s.mergeVersion, auth.ScopeProjects)
	route("GET /api/projects/{key}/fields", s.projectFields, auth.ScopeRead)
	route("GET /api/projects/{key}/security-levels", s.listSecurityLevels, auth.ScopeRead)
	route("POST /api/projects/{key}/security-levels", s.saveSecurityLevel, auth.ScopeProjects)
	route("PATCH /api/projects/{key}/security-levels/{id}", s.saveSecurityLevel, auth.ScopeProjects)
	route("DELETE /api/projects/{key}/security-levels/{id}", s.deleteSecurityLevel, auth.ScopeProjects)
	route("POST /api/projects/{key}/security-levels/{id}/members", s.addSecurityLevelMember, auth.ScopeProjects)
	route("DELETE /api/projects/{key}/security-levels/{id}/members/{member}", s.removeSecurityLevelMember, auth.ScopeProjects)

	// Catalogues.
	for kind, c := range catalogueKinds {
		route("GET /api/admin/"+kind, s.listCatalogue(c), auth.ScopeAdmin)
		route("POST /api/admin/"+kind, s.saveCatalogueItem(c), auth.ScopeAdmin)
		route("POST /api/admin/"+kind+"/order", s.orderCatalogue(c), auth.ScopeAdmin)
		route("PATCH /api/admin/"+kind+"/{id}", s.saveCatalogueItem(c), auth.ScopeAdmin)
		route("DELETE /api/admin/"+kind+"/{id}", s.deleteCatalogueItem(c), auth.ScopeAdmin)
	}
	route("GET /api/admin/statuses", s.adminStatuses, auth.ScopeAdmin)
	route("POST /api/admin/statuses/order", s.orderStatuses, auth.ScopeAdmin)
	route("GET /api/admin/statuses/{id}/usage", s.adminStatusUsage, auth.ScopeAdmin)

	// Custom fields.
	route("GET /api/admin/fields", s.adminFields, auth.ScopeAdmin)
	route("GET /api/admin/fields/types", s.fieldTypes, auth.ScopeAdmin)
	route("POST /api/admin/fields", s.createField, auth.ScopeAdmin)
	route("GET /api/admin/fields/{id}", s.adminField, auth.ScopeAdmin)
	route("PATCH /api/admin/fields/{id}", s.updateField, auth.ScopeAdmin)
	route("DELETE /api/admin/fields/{id}", s.deleteField, auth.ScopeAdmin)
	route("POST /api/admin/fields/{id}/options", s.addFieldOption, auth.ScopeAdmin)
	route("POST /api/admin/fields/{id}/options/order", s.orderFieldOptions, auth.ScopeAdmin)
	route("PATCH /api/admin/fields/{id}/options/{option}", s.updateFieldOption, auth.ScopeAdmin)
	route("DELETE /api/admin/fields/{id}/options/{option}", s.deleteFieldOption, auth.ScopeAdmin)

	// System and import.
	route("GET /api/admin/system", s.systemInfo, auth.ScopeAdmin)
	route("GET /api/admin/import", s.adminImportState, auth.ScopeAdmin)
	route("POST /api/admin/import/test", s.adminImportTest, auth.ScopeAdmin)
	route("POST /api/admin/import", s.adminImportStart, auth.ScopeAdmin)
}

// readJSONFields decodes a body like readJSON, and also tells which keys it
// holds: for the fields where absent (keep) and null (clear) differ.
func readJSONFields(r *http.Request, v any) (map[string]json.RawMessage, error) {
	raw, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, maxJSONBody))
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
