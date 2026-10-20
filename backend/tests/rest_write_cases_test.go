package tests

import (
	"fmt"
	"slices"
	"strconv"
	"testing"
)

// restNames lists the names of a REST list of components or versions.
func restNames(v any) []string {
	out := []string{}
	for _, item := range list(v) {
		out = append(out, str(item, "name"))
	}
	slices.Sort(out)
	return out
}

// TestRESTWriteProjectParts sets components, versions and security levels
// through the REST API: by id or by name ignoring case, with set, add and
// remove, and refuses another project's values.
func TestRESTWriteProjectParts(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	rest := restAs(t, "paul.martin")
	key, other := newProject(t, admin, nil), newProject(t, admin, nil)
	add := func(project, kind, name string) string {
		var out struct{ ID int64 }
		admin.call("POST", "/api/projects/"+project+"/"+kind, map[string]any{"name": name}, 201, &out)
		return strconv.FormatInt(out.ID, 10)
	}
	compA, compB := add(key, "components", "Alpha"), add(key, "components", "Beta")
	v1, _ := add(key, "versions", "1.0"), add(key, "versions", "2.0")
	foreignComp, foreignVersion := add(other, "components", "Gamma"), add(other, "versions", "9.9")
	team, inner := add(key, "security-levels", "Équipe"), add(key, "security-levels", "Interne")
	foreignLevel := add(other, "security-levels", "Ailleurs")
	paulID := userID(t, "paul.martin")
	for _, level := range []string{team, inner} {
		admin.call("POST", fmt.Sprintf("/api/projects/%s/security-levels/%s/members", key, level),
			map[string]any{"holder": "user", "id": paulID}, 201, nil)
	}
	var perms struct{ Grants []grantOut }
	admin.call("GET", "/api/projects/"+key+"/permissions", nil, 200, &perms)
	admin.call("PUT", "/api/projects/"+key+"/permissions", map[string]any{"grants": append(perms.Grants,
		grantOut{Permission: "security", Holder: "user", ID: paulID})}, 200, nil)

	status, created := rest("POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
		"project": map[string]string{"key": key}, "issuetype": map[string]string{"id": standardType(t, paul, key)},
		"summary":    "Parties du projet",
		"components": []any{map[string]string{"name": "alpha"}, map[string]string{"id": compB}},
		"versions":   []any{map[string]string{"name": "1.0"}}, "fixVersions": []any{map[string]string{"id": v1}},
		"security": map[string]string{"name": "équipe"},
	}})
	if status != 201 {
		t.Fatalf("create: %d %v", status, created)
	}
	path := "/rest/api/2/issue/" + str(created, "key")
	read := func() any {
		t.Helper()
		_, issue := rest("GET", path+"?fields=components,versions,fixVersions,security", nil)
		return at(issue, "fields")
	}
	want := func(what string, f any, components, versions, fixVersions []string, level string) {
		t.Helper()
		if got := restNames(at(f, "components")); !slices.Equal(got, components) {
			t.Errorf("%s: components %v, want %v", what, got, components)
		}
		if got := restNames(at(f, "versions")); !slices.Equal(got, versions) {
			t.Errorf("%s: versions %v, want %v", what, got, versions)
		}
		if got := restNames(at(f, "fixVersions")); !slices.Equal(got, fixVersions) {
			t.Errorf("%s: fix versions %v, want %v", what, got, fixVersions)
		}
		if got := str(f, "security", "name"); got != level {
			t.Errorf("%s: security level %q, want %q", what, got, level)
		}
	}
	want("created", read(), []string{"Alpha", "Beta"}, []string{"1.0"}, []string{"1.0"}, "Équipe")

	edit := func(body map[string]any) (int, map[string]any) {
		t.Helper()
		return rest("PUT", path, body)
	}
	// add and remove start from the issue's lists; an item already there, by
	// name or by id, is not added twice.
	if status, e := edit(map[string]any{"update": map[string]any{
		"components":  []any{map[string]any{"remove": map[string]string{"id": compA}}, map[string]any{"add": map[string]string{"name": "BETA"}}},
		"versions":    []any{map[string]any{"add": map[string]string{"name": "2.0"}}, map[string]any{"add": map[string]string{"id": v1}}},
		"fixVersions": []any{map[string]any{"set": []any{}}},
		"security":    []any{map[string]any{"set": map[string]string{"id": inner}}},
	}}); status != 204 {
		t.Fatalf("update operations: %d %v", status, e)
	}
	want("after the update operations", read(), []string{"Beta"}, []string{"1.0", "2.0"}, []string{}, "Interne")

	// add after a set in the same request starts from the set list.
	if status, e := edit(map[string]any{
		"fields": map[string]any{"components": []any{map[string]string{"name": "alpha"}}},
		"update": map[string]any{"components": []any{map[string]any{"add": map[string]string{"id": compB}}}},
	}); status != 204 {
		t.Fatalf("set then add: %d %v", status, e)
	}
	want("after a set then an add", read(), []string{"Alpha", "Beta"}, []string{"1.0", "2.0"}, []string{}, "Interne")

	if status, e := edit(map[string]any{"fields": map[string]any{"security": nil, "versions": nil}}); status != 204 {
		t.Fatalf("clear: %d %v", status, e)
	}
	want("after clearing", read(), []string{"Alpha", "Beta"}, []string{}, []string{}, "")

	for name, c := range map[string]struct {
		field string
		body  map[string]any
	}{
		"component of another project by id":   {"components", map[string]any{"components": []any{map[string]string{"id": foreignComp}}}},
		"component of another project by name": {"components", map[string]any{"components": []any{map[string]string{"name": "Gamma"}}}},
		"component id that is not a number":    {"components", map[string]any{"components": []any{map[string]string{"id": "premier"}}}},
		"components not a list":                {"components", map[string]any{"components": map[string]string{"name": "Alpha"}}},
		"version of another project":           {"fixVersions", map[string]any{"fixVersions": []any{map[string]string{"id": foreignVersion}}}},
		"unknown version name":                 {"versions", map[string]any{"versions": []any{map[string]string{"name": "3.0"}}}},
		"level of another project":             {"security", map[string]any{"security": map[string]string{"id": foreignLevel}}},
		"unknown level name":                   {"security", map[string]any{"security": map[string]string{"name": "Direction"}}},
		"level not an object":                  {"security", map[string]any{"security": []string{"Équipe"}}},
	} {
		c.body["summary"] = "Ne doit pas passer"
		if status, e := edit(map[string]any{"fields": c.body}); status != 400 || at(e, "errors", c.field) == nil {
			t.Errorf("%s: %d %v, want 400 naming %s", name, status, e, c.field)
		}
	}
	if status, e := edit(map[string]any{"update": map[string]any{
		"components": []any{map[string]any{"copy": map[string]string{"id": compA}}}}}); status != 400 || at(e, "errors", "components") == nil {
		t.Errorf("unsupported operation: %d %v", status, e)
	}
	if status, e := edit(map[string]any{"update": map[string]any{
		"security": []any{map[string]any{"add": map[string]string{"id": team}}}}}); status != 400 || at(e, "errors", "security") == nil {
		t.Errorf("add to a field that is not a list: %d %v", status, e)
	}
	f := read()
	want("after the refusals", f, []string{"Alpha", "Beta"}, []string{}, []string{}, "")
	_, issue := rest("GET", path+"?fields=summary", nil)
	if s := str(issue, "fields", "summary"); s != "Parties du projet" {
		t.Errorf("a refused edit changed the summary: %q", s)
	}
}

// TestRESTWriteCustomFields sets the select family, the user picker and the
// labels custom fields of the seed through the REST API, and checks what is
// stored, refused and recorded in the history.
func TestRESTWriteCustomFields(t *testing.T) {
	rest := restAs(t, "paul.martin")
	option := func(field int, value string) string {
		return pick(t, `SELECT id FROM custom_field_options WHERE field_id = ? AND value = ?`, field, value)
	}
	task := pick(t, `SELECT it.name FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND NOT it.subtask AND it.name NOT IN ('Epic', 'Épopée')
		ORDER BY t.position LIMIT 1`)

	// customfield_10101 select, 10102 multiselect, 10103 cascading select,
	// 10105 user picker, 10109 labels.
	status, created := rest("POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
		"project": map[string]string{"key": "DEMO"}, "issuetype": map[string]string{"name": task},
		"summary":           "Champs personnalisés",
		"customfield_10101": map[string]string{"value": "recette"},
		"customfield_10102": []any{map[string]string{"value": "Front"}, map[string]string{"id": option(10102, "Data")}},
		"customfield_10103": map[string]any{"value": "France", "child": map[string]string{"value": "lyon"}},
		"customfield_10105": map[string]string{"name": "chloe.simon"},
		"customfield_10109": []string{"a b", "a  b", " ", "z", "a_b"},
	}})
	if status != 201 {
		t.Fatalf("create: %d %v", status, created)
	}
	path := "/rest/api/2/issue/" + str(created, "key")
	fields := "?fields=customfield_10101,customfield_10102,customfield_10103,customfield_10105,customfield_10109"
	read := func() any {
		t.Helper()
		_, issue := rest("GET", path+fields, nil)
		return at(issue, "fields")
	}
	multi := func(f any) []string {
		out := []string{}
		for _, v := range list(at(f, "customfield_10102")) {
			out = append(out, str(v, "value"))
		}
		return out
	}
	f := read()
	if got := str(f, "customfield_10101", "value"); got != "Recette" {
		t.Errorf("select by value ignoring case: %q", got)
	}
	if got := multi(f); !slices.Equal(got, []string{"Front", "Data"}) {
		t.Errorf("multiselect by value and id: %v", got)
	}
	if str(f, "customfield_10103", "value") != "France" || str(f, "customfield_10103", "child", "value") != "Lyon" {
		t.Errorf("cascading select: %v", at(f, "customfield_10103"))
	}
	if got := str(f, "customfield_10105", "name"); got != "chloe.simon" {
		t.Errorf("user picker: %q", got)
	}
	var labels []string
	for _, l := range list(at(f, "customfield_10109")) {
		labels = append(labels, l.(string))
	}
	if !slices.Equal(labels, []string{"a_b", "z"}) {
		t.Errorf("labels field: %q, want spaces joined by _, blanks and duplicates dropped, sorted", labels)
	}

	edit := func(body map[string]any) (int, map[string]any) {
		t.Helper()
		return rest("PUT", path, map[string]any{"fields": body})
	}
	if status, e := edit(map[string]any{
		"customfield_10101": map[string]string{"id": option(10101, "Production")},
		"customfield_10102": []any{map[string]string{"value": "back"}},
		"customfield_10103": map[string]any{"id": option(10103, "Belgique")},
		"customfield_10105": "hugo.robert",
	}); status != 204 {
		t.Fatalf("edit: %d %v", status, e)
	}
	f = read()
	if str(f, "customfield_10101", "value") != "Production" || !slices.Equal(multi(f), []string{"Back"}) ||
		str(f, "customfield_10103", "value") != "Belgique" || at(f, "customfield_10103", "child") != nil ||
		str(f, "customfield_10105", "name") != "hugo.robert" {
		t.Errorf("after the edit: %v", f)
	}

	// The history names the field and shows the values as text. A creation
	// records none, as in Jira; setting the same values again records nothing.
	history := func() map[string][]string {
		t.Helper()
		_, issue := rest("GET", path+"?fields=summary&expand=changelog", nil)
		out := map[string][]string{}
		for _, h := range list(at(issue, "changelog", "histories")) {
			for _, it := range list(at(h, "items")) {
				out[str(it, "field")] = append(out[str(it, "field")], str(it, "fromString")+" → "+str(it, "toString"))
			}
		}
		return out
	}
	before := history()
	for field, want := range map[string][]string{
		"Environnement client": {"Recette → Production"},
		"Équipes concernées":   {"Front, Data → Back"},
		"Lieu":                 {"France › Lyon → Belgique"},
		"Référent métier":      {"Chloé Simon → Hugo Robert"},
		"Thèmes":               nil,
	} {
		got := slices.Sorted(slices.Values(before[field]))
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("history of %s: %q, want %q", field, got, want)
		}
	}
	if status, e := edit(map[string]any{
		"customfield_10101": map[string]string{"value": "Production"},
		"customfield_10102": []any{map[string]string{"id": option(10102, "Back")}},
		"customfield_10109": []string{"z", "a_b"},
	}); status != 204 {
		t.Fatalf("edit with the same values: %d %v", status, e)
	}
	if after := history(); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("an edit that changes nothing recorded history: %v, was %v", after, before)
	}

	for name, c := range map[string]struct {
		field string
		value any
	}{
		"disabled option":                {"customfield_10101", map[string]string{"value": "Préproduction"}},
		"unknown option":                 {"customfield_10101", map[string]string{"value": "Staging"}},
		"option of another field":        {"customfield_10101", map[string]string{"id": option(10102, "Front")}},
		"multiselect not a list":         {"customfield_10102", map[string]string{"value": "Front"}},
		"one unknown option in the list": {"customfield_10102", []any{map[string]string{"value": "Front"}, map[string]string{"value": "Ops"}}},
		"child of another parent":        {"customfield_10103", map[string]any{"value": "France", "child": map[string]string{"value": "Bruxelles"}}},
		"child as a parent":              {"customfield_10103", map[string]string{"value": "Paris"}},
		"unknown user":                   {"customfield_10105", map[string]string{"name": "personne"}},
		"labels not a list":              {"customfield_10109", "a"},
	} {
		if status, e := edit(map[string]any{"summary": "Ne doit pas passer", c.field: c.value}); status != 400 ||
			at(e, "errors", c.field) == nil {
			t.Errorf("%s: %d %v, want 400 naming %s", name, status, e, c.field)
		}
	}
	f = read()
	if str(f, "customfield_10101", "value") != "Production" || !slices.Equal(multi(f), []string{"Back"}) ||
		str(f, "customfield_10103", "value") != "Belgique" || str(f, "customfield_10105", "name") != "hugo.robert" {
		t.Errorf("a refused edit changed the fields: %v", f)
	}

	if status, e := edit(map[string]any{"customfield_10101": nil, "customfield_10102": []any{}, "customfield_10103": nil,
		"customfield_10105": nil, "customfield_10109": nil}); status != 204 {
		t.Fatalf("clear: %d %v", status, e)
	}
	f = read()
	for _, field := range []string{"customfield_10101", "customfield_10102", "customfield_10103", "customfield_10105", "customfield_10109"} {
		if v := at(f, field); v != nil {
			t.Errorf("%s after clearing: %v", field, v)
		}
	}
}
