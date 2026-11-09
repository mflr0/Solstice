package tests

// Issue layouts: the type's templates belong to global administrators, a
// project's overrides to its administrators, and everyone who browses the
// project reads the layout that applies.

import (
	"fmt"
	"slices"
	"testing"

	"solstice/internal/jirafixture"
)

type layoutField struct {
	Field    string `json:"field"`
	Zone     string `json:"zone,omitempty"`
	Hidden   bool   `json:"hidden"`
	Required bool   `json:"required,omitempty"`
}

type layoutAnswer struct {
	Form   string
	Source string
	Fields []layoutField
}

func (l layoutAnswer) ids() []string {
	out := []string{}
	for _, f := range l.Fields {
		out = append(out, f.Field)
	}
	return out
}

func layoutRows(t *testing.T, where string, args ...any) string {
	t.Helper()
	return pick(t, `SELECT COUNT(*) FROM issue_layout_fields WHERE `+where, args...)
}

func TestIssueLayouts(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password)
	jeanne := login(t, "jeanne.dupont", jirafixture.Password)

	var typ catalogueItem
	admin.call("POST", "/api/admin/issue-types", map[string]any{"name": unique("Mise en page ")}, 201, &typ)
	key := newProject(t, admin, map[string]any{"issueTypes": []string{typ.id()}})
	t.Cleanup(func() { admin.do("DELETE", "/api/admin/issue-types/"+typ.id(), nil) })

	// paul.martin administers the project, and is not a global administrator.
	var perms struct {
		Grants []grantOut `json:"grants"`
	}
	admin.call("GET", "/api/projects/"+key+"/permissions", nil, 200, &perms)
	grants := append(perms.Grants, grantOut{Permission: "admin", Holder: "user", ID: userID(t, "paul.martin")})
	admin.call("PUT", "/api/projects/"+key+"/permissions", map[string]any{"grants": grants}, 200, nil)

	name := unique("Gabarit ")
	var field fieldDetail
	admin.call("POST", "/api/admin/fields", map[string]any{"name": name, "type": "text"}, 201, &field)
	t.Cleanup(func() {
		admin.do("DELETE", fmt.Sprintf("/api/admin/fields/%d", field.ID), map[string]string{"confirm": name})
	})
	custom := fmt.Sprintf("customfield_%d", field.ID)

	resolved := "/api/issue-layouts?project=" + key + "&issueType=" + typ.id() + "&form="
	override := "/api/projects/" + key + "/issue-types/" + typ.id() + "/layouts/"
	template := "/api/admin/issue-types/" + typ.id() + "/layouts/"

	// Nothing customised: the default, with the custom field appended, shown.
	var got layoutAnswer
	jeanne.call("GET", resolved+"create", nil, 200, &got)
	if got.Source != "default" || got.ids()[0] != "priority" {
		t.Errorf("default create layout: %+v", got)
	}
	if i := slices.Index(got.ids(), custom); i < 0 || got.Fields[i].Hidden || got.Fields[i].Required {
		t.Errorf("offered custom field in the default: %+v", got.Fields)
	}

	// A project administrator replaces and resets the override, and may not
	// touch the type's template. A plain user may do neither.
	create := map[string]any{"fields": []layoutField{{Field: custom, Required: true}, {Field: "labels"}}}
	paul.call("PUT", override+"create", create, 200, &got)
	if got.Source != "project" || !slices.Equal(got.ids(), []string{custom, "labels"}) {
		t.Errorf("override after PUT: %+v", got)
	}
	jeanne.call("GET", resolved+"create", nil, 200, &got)
	if got.Source != "project" || !slices.Equal(got.ids()[:2], []string{custom, "labels"}) || !got.Fields[0].Required {
		t.Errorf("resolved override: %+v", got)
	}
	if n := pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = 'issuelayout.update' AND target_type = 'project'
		AND target_id = (SELECT id FROM projects WHERE pkey = ?)`, key); n != "1" {
		t.Errorf("audit entries of the override: %s", n)
	}
	for _, c := range []struct {
		who          *client
		method, path string
		body         any
		want         int
	}{
		{paul, "GET", template + "create", nil, 403},
		{paul, "PUT", template + "create", create, 403},
		{paul, "DELETE", template + "create", nil, 403},
		{jeanne, "GET", override + "create", nil, 403},
		{jeanne, "PUT", override + "create", create, 403},
		{jeanne, "DELETE", override + "create", nil, 403},
		{jeanne, "PUT", template + "create", create, 403},
		{paul, "PUT", override + "create", map[string]any{"fields": []layoutField{{Field: "summary"}}}, 400},
		{admin, "PUT", template + "view", map[string]any{"fields": []layoutField{{Field: "issuetype", Zone: "side"}}}, 400},
		{paul, "PUT", override + "view", map[string]any{"fields": []layoutField{{Field: "labels", Zone: "side", Required: true}}}, 400},
		{paul, "PUT", override + "create", map[string]any{"fields": []layoutField{{Field: "customfield_999999999"}}}, 400},
		{paul, "GET", override + "edit", nil, 404},
	} {
		if status, body := c.who.do(c.method, c.path, c.body); status != c.want {
			t.Errorf("%s %s: %d %s, want %d", c.method, c.path, status, body, c.want)
		}
	}
	if status, msg, _ := paul.errorIn("fr", "PUT", override+"create",
		map[string]any{"fields": []layoutField{{Field: "summary"}}}); status != 400 || msg == "" {
		t.Errorf("fixed field: %d %q", status, msg)
	}

	// The type's template applies once the override is reset.
	admin.call("PUT", template+"create", map[string]any{"fields": []layoutField{{Field: "description"}}}, 200, nil)
	paul.call("DELETE", override+"create", nil, 204, nil)
	paul.call("GET", override+"create", nil, 200, &got)
	if got.Source != "type" || got.ids()[0] != "description" {
		t.Errorf("override after DELETE: %+v", got)
	}
	jeanne.call("GET", resolved+"create", nil, 200, &got)
	if got.Source != "type" || got.ids()[0] != "description" {
		t.Errorf("resolved after the reset: %+v", got)
	}
	admin.call("DELETE", template+"create", nil, 204, nil)
	admin.call("GET", template+"create", nil, 200, &got)
	if got.Source != "default" {
		t.Errorf("template after DELETE: %+v", got)
	}

	// Deleting a custom field, or a type, takes its rows with it.
	admin.call("PUT", template+"view", map[string]any{"fields": []layoutField{{Field: custom, Zone: "main"}}}, 200, nil)
	paul.call("PUT", override+"create", create, 200, nil)
	if n := layoutRows(t, "custom_field_id = ?", field.ID); n != "2" {
		t.Fatalf("rows of the custom field: %s", n)
	}
	admin.call("DELETE", fmt.Sprintf("/api/admin/fields/%d", field.ID), map[string]string{"confirm": name}, 204, nil)
	if n := layoutRows(t, "custom_field_id = ?", field.ID); n != "0" {
		t.Errorf("rows of the deleted custom field: %s", n)
	}

	var other catalogueItem
	admin.call("POST", "/api/admin/issue-types", map[string]any{"name": unique("Mise en page ")}, 201, &other)
	admin.call("PUT", "/api/admin/issue-types/"+other.id()+"/layouts/create",
		map[string]any{"fields": []layoutField{{Field: "labels"}}}, 200, nil)
	if n := layoutRows(t, "issue_type_id = ?", other.id()); n != "1" {
		t.Fatalf("rows of the type: %s", n)
	}
	admin.call("DELETE", "/api/admin/issue-types/"+other.id(), nil, 204, nil)
	if n := layoutRows(t, "issue_type_id = ?", other.id()); n != "0" {
		t.Errorf("rows of the deleted type: %s", n)
	}
}
