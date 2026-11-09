package tests

// Required fields: what the create template of a type requires, both APIs
// enforce on create and refuse to clear on edit. Clone, move and a type
// change carry the issue's data as it is.

import (
	"fmt"
	"testing"
)

func TestIssueLayoutsRequired(t *testing.T) {
	admin := login(t, "admin", "admin")
	rest := restBasicAs(t, "admin")

	var typ, other catalogueItem
	admin.call("POST", "/api/admin/issue-types", map[string]any{"name": unique("Obligatoire ")}, 201, &typ)
	admin.call("POST", "/api/admin/issue-types", map[string]any{"name": unique("Libre ")}, 201, &other)
	t.Cleanup(func() {
		admin.do("DELETE", "/api/admin/issue-types/"+typ.id(), nil)
		admin.do("DELETE", "/api/admin/issue-types/"+other.id(), nil)
	})
	types := map[string]any{"issueTypes": []string{typ.id(), other.id()}}
	key, dest := newProject(t, admin, types), newProject(t, admin, types)

	name := unique("Exigé ")
	var field fieldDetail
	admin.call("POST", "/api/admin/fields", map[string]any{"name": name, "type": "text"}, 201, &field)
	t.Cleanup(func() {
		admin.do("DELETE", fmt.Sprintf("/api/admin/fields/%d", field.ID), map[string]string{"confirm": name})
	})
	custom := fmt.Sprintf("customfield_%d", field.ID)

	// Issues made before the template: they lack the fields it requires.
	var bare, untyped struct{ Key string }
	admin.call("POST", "/api/issues", map[string]any{"project": key, "type": typ.id(), "summary": "Sans rien"}, 201, &bare)
	admin.call("POST", "/api/issues", map[string]any{"project": key, "type": other.id(), "summary": "Autre type"}, 201, &untyped)

	admin.call("PUT", "/api/admin/issue-types/"+typ.id()+"/layouts/create", map[string]any{"fields": []layoutField{
		{Field: custom, Required: true}, {Field: "duedate", Required: true}, {Field: "labels", Required: true},
		{Field: "description"}, {Field: "environment", Hidden: true},
	}}, 200, nil)

	full := map[string]any{"project": key, "type": typ.id(), "summary": "Complet", "due": "2026-12-01",
		"labels": []string{"exigé"}, "customFields": map[string]any{custom: "valeur"}}
	without := func(k string) map[string]any {
		out := map[string]any{}
		for kk, v := range full {
			if kk != k {
				out[kk] = v
			}
		}
		return out
	}

	// Interface API: each missing field gives 400, naming it.
	for k, id := range map[string]string{"customFields": custom, "due": "duedate", "labels": "labels"} {
		status, msg, body := admin.errorIn("en", "POST", "/api/issues", without(k))
		if status != 400 || body["field"] != id || msg == "" {
			t.Errorf("create without %s: %d %v", k, status, body)
		}
	}
	if _, _, body := admin.errorIn("en", "POST", "/api/issues", without("customFields")); body["error"] != "Field "+name+" is required." {
		t.Errorf("message of a missing custom field: %v", body["error"])
	}
	var made struct{ Key string }
	admin.call("POST", "/api/issues", full, 201, &made)

	// REST API: the same, in Jira's errors map.
	restFull := map[string]any{"project": map[string]any{"key": key}, "issuetype": map[string]any{"id": typ.id()},
		"summary": "Complet", "duedate": "2026-12-01", "labels": []string{"exigé"}, custom: "valeur"}
	for _, missing := range []string{custom, "duedate", "labels"} {
		in := map[string]any{}
		for k, v := range restFull {
			if k != missing {
				in[k] = v
			}
		}
		status, out := rest("POST", "/rest/api/2/issue", map[string]any{"fields": in})
		if errs, _ := at(out, "errors").(map[string]any); status != 400 || errs[missing] == nil {
			t.Errorf("REST create without %s: %d %v", missing, status, out)
		}
	}
	if status, out := rest("POST", "/rest/api/2/issue", map[string]any{"fields": restFull}); status != 201 {
		t.Errorf("REST create with every required field: %d %v", status, out)
	}

	// Clearing a required field by edit gives 400; another field may change.
	issue := "/api/issues/" + made.Key
	for _, body := range []map[string]any{{"due": nil}, {"labels": []string{}}, {"customFields": map[string]any{custom: nil}}} {
		if status, raw := admin.do("PATCH", issue, body); status != 400 {
			t.Errorf("PATCH %v: %d %s", body, status, raw)
		}
	}
	admin.call("PATCH", issue, map[string]any{"description": "Autre chose"}, 200, nil)
	for _, fields := range []map[string]any{{"duedate": nil}, {custom: ""}} {
		if status, out := rest("PUT", "/rest/api/2/issue/"+made.Key, map[string]any{"fields": fields}); status != 400 {
			t.Errorf("REST PUT %v: %d %v", fields, status, out)
		}
	}
	got := bulk(t, admin, map[string]any{"op": "edit", "keys": []string{made.Key},
		"labels": map[string]any{"mode": "remove", "values": []string{"exigé"}}})
	if got[made.Key].OK {
		t.Errorf("bulk edit cleared the required labels: %+v", got)
	}
	var d issueDetail
	admin.get(issue, &d)
	if due := pick(t, `SELECT COALESCE(due_date, '') FROM issues WHERE id = ?`, issueID(t, made.Key)); due == "" || len(d.Labels) != 1 {
		t.Errorf("after the refused edits: due %q, labels %v", due, d.Labels)
	}

	// Clone, move and type change carry what the issue has.
	cloneOf(t, admin, "en", bare.Key, map[string]any{})
	admin.call("POST", "/api/issues/"+bare.Key+"/move", map[string]any{"project": dest}, 200, nil)
	admin.call("PATCH", "/api/issues/"+untyped.Key, map[string]any{"type": typ.id()}, 200, nil)

	// createmeta: the template's required fields, and not its hidden ones.
	_, meta := rest("GET", "/rest/api/2/issue/createmeta?expand=projects.issuetypes.fields&projectKeys="+key+
		"&issuetypeIds="+typ.id(), nil)
	form := at(meta, "projects", 0, "issuetypes", 0, "fields")
	for _, id := range []string{custom, "duedate", "labels"} {
		if at(form, id, "required") != true {
			t.Errorf("createmeta %s: %v", id, at(form, id))
		}
	}
	if at(form, "description", "required") != false {
		t.Errorf("createmeta description: %v", at(form, "description"))
	}
	for _, id := range []string{"environment", "versions"} {
		if at(form, id) != nil {
			t.Errorf("createmeta shows %s, which the template hides", id)
		}
	}
	_, edit := rest("GET", "/rest/api/2/issue/"+made.Key+"/editmeta", nil)
	if at(edit, "fields", "environment") == nil || at(edit, "fields", "duedate", "required") != false {
		t.Errorf("editmeta changed: %v", at(edit, "fields"))
	}
}
