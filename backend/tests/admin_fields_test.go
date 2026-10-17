package tests

// Custom fields made in Solstice: their context, their options, values on
// issues and in searches, and deletion.

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
)

type fieldDetail struct {
	ID      int64
	Name    string
	Type    *string
	TypeKey string
	Issues  int
	Options []struct {
		ID       int64
		Value    string
		Disabled bool
	}
	Context struct {
		Projects   []struct{ Key string }
		IssueTypes []typeRef `json:"issueTypes"`
	}
}

type formField struct {
	ID      int64
	Name    string
	Type    string
	Options []fieldOption
}

type fieldOption struct {
	ID    int64
	Value string
}

func offered(t *testing.T, c *client, project string, id int64) *formField {
	t.Helper()
	var list []formField
	c.call("GET", "/api/projects/"+project+"/fields", nil, 200, &list)
	if i := slices.IndexFunc(list, func(f formField) bool { return f.ID == id }); i >= 0 {
		return &list[i]
	}
	return nil
}

func TestCustomFields(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	paul.call("GET", "/api/admin/fields", nil, 403, nil)
	var types []struct{ Type string }
	admin.call("GET", "/api/admin/fields/types", nil, 200, &types)
	if !slices.ContainsFunc(types, func(x struct{ Type string }) bool { return x.Type == "select" }) {
		t.Errorf("field types: %+v", types)
	}
	key := newProject(t, admin, nil)
	name := unique("Gravité ")
	for _, bad := range []struct {
		in   map[string]any
		want int
	}{
		{map[string]any{"name": name, "type": "couleur"}, 400},
		{map[string]any{"name": name, "type": "text", "options": []string{"a"}}, 400},
		{map[string]any{"name": name, "type": "select", "options": []string{"Haut", "haut"}}, 409},
		{map[string]any{"name": name, "type": "select", "context": map[string]any{"projects": []string{"NOPE"}}}, 400},
		{map[string]any{"name": "Story Points", "type": "number"}, 409},
	} {
		if status, body := admin.do("POST", "/api/admin/fields", bad.in); status != bad.want {
			t.Errorf("create %v: %d %s, want %d", bad.in, status, body, bad.want)
		}
	}

	var f fieldDetail
	admin.call("POST", "/api/admin/fields", map[string]any{"name": name, "type": "select", "options": []string{"Bas", "Haut"},
		"context": map[string]any{"projects": []string{key}}}, 201, &f)
	path := fmt.Sprintf("/api/admin/fields/%d", f.ID)
	t.Cleanup(func() { admin.do("DELETE", path, map[string]string{"confirm": name}) })
	if f.Type == nil || *f.Type != "select" || len(f.Options) != 2 || len(f.Context.Projects) != 1 || f.Context.Projects[0].Key != key {
		t.Fatalf("created field: %+v", f)
	}
	low, high := f.Options[0].ID, f.Options[1].ID

	// Offered in its project only.
	if o := offered(t, paul, key, f.ID); o == nil || len(o.Options) != 2 {
		t.Fatalf("the field in %s: %+v", key, o)
	}
	if offered(t, paul, "DEMO", f.ID) != nil {
		t.Error("the field is offered in DEMO, outside its context")
	}

	// Values on issues, by option value or id, and in searches.
	field := fmt.Sprint(f.ID)
	var c struct{ Key string }
	paul.call("POST", "/api/issues", map[string]any{"project": key, "type": standardType(t, paul, key),
		"summary": "Avec gravité", "customFields": map[string]any{field: "Haut"}}, 201, &c)
	var d issueDetail
	paul.get("/api/issues/"+c.Key, &d)
	if i := slices.IndexFunc(d.CustomFields, func(v customValue) bool {
		return v.ID == f.ID
	}); i < 0 || !slices.Equal(d.CustomFields[i].Values, []string{"Haut"}) {
		t.Errorf("custom values: %+v", d.CustomFields)
	}
	paul.call("PATCH", "/api/issues/"+c.Key, map[string]any{"customFields": map[string]any{"customfield_" + field: map[string]any{"id": fmt.Sprint(low)}}}, 200, nil)
	paul.call("PATCH", "/api/issues/"+c.Key, map[string]any{"customFields": map[string]any{field: "Inconnue"}}, 400, nil)
	demo := scratchIssue(t, paul, "Hors contexte")
	paul.call("PATCH", "/api/issues/"+demo, map[string]any{"customFields": map[string]any{field: "Haut"}}, 400, nil)
	for _, q := range []string{`"` + name + `" = Bas`, `cf[` + field + `] = Bas`} {
		var res struct{ Total int }
		paul.call("GET", "/api/issues?jql="+url.QueryEscape("project = "+key+" AND "+q), nil, 200, &res)
		if res.Total != 1 {
			t.Errorf("%s: %d issues", q, res.Total)
		}
	}

	// Options: added, disabled (no longer offered), reordered.
	admin.call("POST", path+"/options", map[string]any{"value": "Moyen"}, 201, &f)
	admin.call("POST", path+"/options", map[string]any{"value": "moyen"}, 409, nil)
	middle := f.Options[2].ID
	admin.call("PATCH", fmt.Sprintf("%s/options/%d", path, low), map[string]any{"disabled": true}, 200, nil)
	if o := offered(t, paul, key, f.ID); o == nil || len(o.Options) != 2 || slices.ContainsFunc(o.Options, func(x fieldOption) bool { return x.ID == low }) {
		t.Errorf("options offered after disabling Bas: %+v", o)
	}
	paul.get("/api/issues/"+c.Key, &d)
	if !slices.ContainsFunc(d.CustomFields, func(v customValue) bool {
		return v.ID == f.ID && slices.Equal(v.Values, []string{"Bas"})
	}) {
		t.Error("a disabled option's value is gone from the issue")
	}
	admin.call("POST", path+"/options/order", map[string]any{"ids": []int64{high, middle}}, 400, nil)
	admin.call("POST", path+"/options/order", map[string]any{"ids": []int64{high, middle, low}}, 200, &f)
	if f.Options[0].ID != high || f.Options[2].ID != low {
		t.Errorf("options after reordering: %+v", f.Options)
	}

	// The context widens to every project.
	admin.call("PATCH", path, map[string]any{"name": name + " bis", "context": nil}, 200, &f)
	if f.Name != name+" bis" || len(f.Context.Projects) != 0 {
		t.Errorf("after PATCH: %+v", f)
	}
	if offered(t, paul, "DEMO", f.ID) == nil {
		t.Error("the field is not offered in DEMO with an empty context")
	}

	// A field holding values needs its name typed to go.
	status, body := admin.do("DELETE", path, nil)
	if status != 409 || num(obj(t, body), "usage", "issues") != 1 {
		t.Fatalf("deleting a field with values: %d %s", status, body)
	}
	admin.call("DELETE", path, map[string]string{"confirm": name + " bis"}, 204, nil)
	paul.get("/api/issues/"+c.Key, &d)
	for _, v := range d.CustomFields {
		if v.ID == f.ID {
			t.Errorf("the deleted field still shows: %+v", v)
		}
	}
	if status, body := paul.do("GET", "/api/issues?jql="+url.QueryEscape(`"`+name+` bis" = Bas`), nil); status != 400 ||
		!strings.Contains(string(body), "JQL") {
		t.Errorf("searching a deleted field: %d %s", status, body)
	}
}
