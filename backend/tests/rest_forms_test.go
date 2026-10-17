package tests

// REST API: createmeta and editmeta. They must agree with what the write
// routes accept: every field they list can be set, and the fields they do not
// list are refused.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"testing"
)

// formUsers see different things: developers, a user with fewer rights, an
// administrator.
var formUsers = map[string]string{
	"paul.martin": "motdepasse", "chloe.simon": "motdepasse", "hugo.robert": "motdepasse",
	"lucas.bernard": "motdepasse", "admin": "admin",
}

// restCall sends a REST request as one user and decodes the answer.
type restCall func(method, path string, body any) (int, any)

func restAs(t *testing.T, username string) restCall {
	t.Helper()
	needDB(t)
	return func(method, path string, body any) (int, any) {
		t.Helper()
		status, _, raw := call(t, nil, testURL, method, path, body, basic(username, formUsers[username]))
		var v any
		json.Unmarshal(raw, &v)
		return status, v
	}
}

func list(v any) []any { l, _ := v.([]any); return l }

// column runs a query returning one text column.
func column(t *testing.T, query string, args ...any) []string {
	t.Helper()
	needDB(t)
	rows, err := testDB.Query(query, args...)
	if err != nil {
		t.Fatalf("fixture lookup failed: %v\n%s", err, query)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }

// sortedKeys lists the keys of a map.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func ids(v any) []string {
	var out []string
	for _, item := range list(v) {
		out = append(out, str(item, "id"))
	}
	sort.Strings(out)
	return out
}

// sampler makes valid values for fields.
type sampler struct {
	t       *testing.T
	rest    restCall
	user    string
	project string
	parent  string // an issue to be the parent of a sub-task
	epic    string // an epic the user sees, or ""
}

// value gives a valid value for a field as the forms describe it (schema,
// allowedValues, autoCompleteUrl); false when it has none to give.
func (s *sampler) value(key string, f map[string]any) (any, bool) {
	typ, items, custom := str(f, "schema", "type"), str(f, "schema", "items"), str(f, "schema", "custom")
	allowed := list(f["allowedValues"])
	ref := func(i int) any {
		if i >= len(allowed) {
			return nil
		}
		return map[string]any{"id": str(allowed[i], "id")}
	}
	switch {
	case strings.HasSuffix(custom, ":gh-sprint"):
		return nil, false
	case strings.HasSuffix(custom, ":gh-epic-link"):
		return s.epic, s.epic != ""
	}
	switch typ {
	case "string":
		switch {
		case strings.HasSuffix(custom, ":url"):
			return "https://example.org/forms", true
		case key == "duedate":
			return "2026-12-01", true
		}
		return "texte " + key, true
	case "number":
		return 3.5, true
	case "date":
		return "2026-12-01", true
	case "datetime":
		return "2026-12-01T10:00:00.000+0000", true
	case "timetracking":
		return map[string]any{"originalEstimate": "2h"}, true
	case "user":
		if url := str(f, "autoCompleteUrl"); strings.Contains(url, "assignable") {
			status, users := s.rest("GET", strings.TrimPrefix(url, testURL), nil)
			if status != 200 || len(list(users)) == 0 {
				s.t.Fatalf("assignable users %s: %d %v", url, status, users)
			}
			// Oneself when possible: not everyone assigns others.
			name := str(users, 0, "name")
			for _, u := range list(users) {
				if str(u, "name") == s.user {
					name = s.user
				}
			}
			return map[string]any{"name": name}, true
		}
		return map[string]any{"name": "paul.martin"}, true
	case "option", "priority", "resolution", "securitylevel", "issuetype":
		v := ref(0)
		return v, v != nil
	case "option-with-child":
		if len(allowed) == 0 {
			return nil, false
		}
		v := map[string]any{"id": str(allowed[0], "id")}
		if children := list(at(allowed[0], "cascadingOptions")); len(children) > 0 {
			v["child"] = map[string]any{"id": str(children[0], "id")}
		}
		return v, true
	case "project":
		return map[string]any{"key": s.project}, true
	case "issuelink":
		return map[string]any{"key": s.parent}, true
	case "array":
		switch items {
		case "string":
			return []string{"forms"}, true
		case "option", "component", "version":
			v := ref(0)
			return []any{v}, v != nil
		case "user":
			return []any{map[string]any{"name": "paul.martin"}}, true
		}
	}
	return nil, false
}

// universe describes, with values taken from the database, every field a
// request could try to set, whether the forms list it or not.
func (s *sampler) universe() map[string]map[string]any {
	s.t.Helper()
	status, fields := s.rest("GET", "/rest/api/2/field", nil)
	if status != 200 {
		s.t.Fatalf("fields: %d", status)
	}
	options := func(rows []string) []any {
		out := []any{}
		for _, id := range rows {
			out = append(out, map[string]any{"id": id})
		}
		return out
	}
	project := pick(s.t, `SELECT id FROM projects WHERE pkey = ?`, s.project)
	out := map[string]map[string]any{}
	for _, f := range list(fields) {
		key := str(f, "id")
		d := map[string]any{"schema": at(f, "schema")}
		switch key {
		case "priority":
			d["allowedValues"] = options(column(s.t, `SELECT id FROM priorities ORDER BY position LIMIT 1`))
		case "resolution":
			d["allowedValues"] = options(column(s.t, `SELECT id FROM resolutions ORDER BY position LIMIT 1`))
		case "components":
			d["allowedValues"] = options(column(s.t, `SELECT id FROM components WHERE project_id = ? LIMIT 1`, project))
		case "fixVersions", "versions":
			d["allowedValues"] = options(column(s.t, `SELECT id FROM versions WHERE project_id = ? LIMIT 1`, project))
		case "security":
			d["allowedValues"] = options(column(s.t, `SELECT id FROM security_levels WHERE project_id = ? LIMIT 1`, project))
		}
		if custom := str(f, "schema", "customId"); custom != "" {
			d["allowedValues"] = options(column(s.t, `SELECT id FROM custom_field_options WHERE field_id = ?
				AND parent_id IS NULL AND NOT disabled ORDER BY position, id LIMIT 1`, custom))
		}
		out[key] = d
	}
	return out
}

// settings turns a form into values to send: fields to set, and update
// operations for those that cannot be set (the comment).
func (s *sampler) settings(form any) (fields, update map[string]any) {
	s.t.Helper()
	fields, update = map[string]any{}, map[string]any{}
	for _, key := range sortedKeys(asMap(form)) {
		f, _ := at(form, key).(map[string]any)
		if key == "project" || key == "issuetype" {
			continue
		}
		if !slices.Contains(list(f["operations"]), any("set")) {
			update[key] = []any{map[string]any{"add": map[string]any{"body": "Ajouté par le formulaire."}}}
			continue
		}
		v, ok := s.value(key, f)
		if !ok {
			// Cannot be made up: the sprint, an epic link when there is no epic, and
			// values of a project that has none to choose from.
			if custom := str(f, "schema", "custom"); !strings.HasSuffix(custom, ":gh-sprint") &&
				!strings.HasSuffix(custom, ":gh-epic-link") && !(f["allowedValues"] != nil && len(list(f["allowedValues"])) == 0) {
				s.t.Errorf("no sample for %s: %v", key, f)
			}
			continue
		}
		fields[key] = v
	}
	return fields, update
}

// listed tells whether a form lists a field.
func listed(form any, key string) bool { return at(form, key) != nil }

// empty tells whether a field shows no value.
func empty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case []any:
		return len(x) == 0
	case string:
		return x == ""
	}
	return false
}

// creatorOf finds a user who can create issues in a project, among formUsers.
func creatorOf(t *testing.T, project string) string {
	t.Helper()
	for _, u := range []string{"paul.martin", "hugo.robert", "lucas.bernard", "chloe.simon", "admin"} {
		status, meta := restAs(t, u)("GET", "/rest/api/2/issue/createmeta?projectKeys="+project, nil)
		if status == 200 && len(list(at(meta, "projects"))) == 1 {
			return u
		}
	}
	return ""
}

// userProjects lists the keys of the projects a user sees.
func userProjects(t *testing.T, rest restCall) []string {
	t.Helper()
	status, projects := rest("GET", "/rest/api/2/project", nil)
	if status != 200 {
		t.Fatalf("projects: %d", status)
	}
	var keys []string
	for _, p := range list(projects) {
		keys = append(keys, str(p, "key"))
	}
	return keys
}

// epicOf finds an epic of a project that anyone who sees the project sees.
func epicOf(t *testing.T, project string) string {
	t.Helper()
	keys := column(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN issue_types it ON it.id = i.issue_type_id WHERE p.pkey = ? AND it.name IN ('Epic', 'Épopée')
		AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`, project)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

func TestRESTCreateMeta(t *testing.T) {
	paul := restAs(t, "paul.martin")
	task := pick(t, `SELECT it.id FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND NOT it.subtask AND it.name NOT IN ('Epic', 'Épopée')
		ORDER BY t.position LIMIT 1`)
	taskName := pick(t, `SELECT name FROM issue_types WHERE id = ?`, task)

	// Without the expand, projects and issue types only.
	status, meta := paul("GET", "/rest/api/2/issue/createmeta?projectKeys=DEMO", nil)
	if status != 200 || len(list(at(meta, "projects"))) != 1 || str(meta, "projects", 0, "key") != "DEMO" ||
		str(meta, "projects", 0, "id") == "" || len(list(at(meta, "projects", 0, "issuetypes"))) == 0 {
		t.Fatalf("createmeta: %d %v", status, meta)
	}
	for _, it := range list(at(meta, "projects", 0, "issuetypes")) {
		if at(it, "fields") != nil || str(it, "id") == "" || str(it, "name") == "" {
			t.Errorf("issue type without the expand: %v", it)
		}
	}

	// With it, the fields of each issue type.
	path := "/rest/api/2/issue/createmeta?expand=projects.issuetypes.fields&projectKeys=demo&issuetypeIds=" + task
	status, meta = paul("GET", path, nil)
	if status != 200 || len(list(at(meta, "projects", 0, "issuetypes"))) != 1 || str(meta, "projects", 0, "issuetypes", 0, "id") != task {
		t.Fatalf("createmeta for %s: %d %v", task, status, meta)
	}
	form := at(meta, "projects", 0, "issuetypes", 0, "fields")
	for _, key := range []string{"project", "issuetype", "summary"} {
		if at(form, key, "required") != true {
			t.Errorf("%s is not required: %v", key, at(form, key))
		}
	}
	if listed(form, "parent") {
		t.Errorf("a parent for a standard issue type: %v", at(form, "parent"))
	}
	if at(form, "description", "required") != false || str(form, "summary", "schema", "type") != "string" ||
		str(form, "summary", "schema", "system") != "summary" || str(form, "summary", "key") != "summary" ||
		str(form, "summary", "name") == "" || str(form, "summary", "operations", 0) != "set" {
		t.Errorf("summary: %v", at(form, "summary"))
	}
	_, priorities := paul("GET", "/rest/api/2/priority", nil)
	if got, want := ids(at(form, "priority", "allowedValues")), ids(priorities); !slices.Equal(got, want) || len(want) == 0 {
		t.Errorf("priorities %v, want %v", got, want)
	}
	_, components := paul("GET", "/rest/api/2/project/DEMO/components", nil)
	if got, want := ids(at(form, "components", "allowedValues")), ids(components); !slices.Equal(got, want) {
		t.Errorf("components %v, want %v", got, want)
	}
	_, versions := paul("GET", "/rest/api/2/project/DEMO/versions", nil)
	for _, key := range []string{"fixVersions", "versions"} {
		if got, want := ids(at(form, key, "allowedValues")), ids(versions); !slices.Equal(got, want) {
			t.Errorf("%s %v, want %v", key, got, want)
		}
	}
	if ops := fmt.Sprint(at(form, "labels", "operations")); ops != "[add set remove]" {
		t.Errorf("labels operations: %s", ops)
	}
	if str(form, "assignee", "autoCompleteUrl") == "" || str(form, "assignee", "schema", "type") != "user" {
		t.Errorf("assignee: %v", at(form, "assignee"))
	}
	// A number field and a select field, as the field list describes them.
	if str(form, "customfield_10100", "schema", "type") != "number" || str(form, "customfield_10100", "name") != "Story Points" ||
		str(form, "customfield_10100", "key") != "customfield_10100" || num(form, "customfield_10100", "schema", "customId") != 10100 {
		t.Errorf("customfield_10100: %v", at(form, "customfield_10100"))
	}
	enabled := column(t, `SELECT value FROM custom_field_options WHERE field_id = 10101 AND parent_id IS NULL AND NOT disabled
		ORDER BY position, id`)
	var offered []string
	for _, v := range list(at(form, "customfield_10101", "allowedValues")) {
		offered = append(offered, str(v, "value"))
	}
	if !slices.Equal(offered, enabled) || len(enabled) == 0 || str(form, "customfield_10101", "schema", "type") != "option" {
		t.Errorf("customfield_10101 offers %v, want %v", offered, enabled)
	}
	_, all := paul("GET", "/rest/api/2/field", nil)
	for _, f := range list(all) {
		if id := str(f, "id"); strings.HasPrefix(id, "customfield_") && listed(form, id) &&
			fmt.Sprint(at(f, "schema")) != fmt.Sprint(at(form, id, "schema")) {
			t.Errorf("%s: schema %v in the form, %v in the field list", id, at(form, id, "schema"), at(f, "schema"))
		}
	}

	// A sub-task type needs a parent.
	subtask := column(t, `SELECT it.id FROM project_issue_types t JOIN issue_types it ON it.id = t.issue_type_id
		JOIN projects p ON p.id = t.project_id WHERE p.pkey = 'DEMO' AND it.subtask LIMIT 1`)
	if len(subtask) == 1 {
		_, sub := paul("GET", "/rest/api/2/issue/createmeta?expand=projects.issuetypes.fields&projectKeys=DEMO&issuetypeIds="+subtask[0], nil)
		if f := at(sub, "projects", 0, "issuetypes", 0, "fields", "parent"); at(f, "required") != true ||
			str(f, "schema", "type") != "issuelink" {
			t.Errorf("parent of a sub-task: %v", f)
		}
	}

	// Filters.
	for _, c := range []struct {
		query string
		want  int // projects
	}{
		{"projectKeys=DEMO&issuetypeNames=" + strings.ToUpper(taskName), 1},
		{"projectKeys=DEMO&issuetypeNames=Licorne", 0},
		{"projectKeys=NOPE", 0},
		{"projectKeys=NOPE,DEMO", 1},
		{"projectIds=" + pick(t, `SELECT id FROM projects WHERE pkey = 'DEMO'`), 1},
		{"projectKeys=RH", 0}, // paul.martin does not see it
	} {
		status, meta := paul("GET", "/rest/api/2/issue/createmeta?"+c.query, nil)
		if status != 200 || len(list(at(meta, "projects"))) != c.want {
			t.Errorf("createmeta?%s: %d %v, want %d projects", c.query, status, meta, c.want)
		}
	}
	if status, meta := paul("GET", "/rest/api/2/issue/createmeta?projectKeys=DEMO&issuetypeNames="+taskName, nil); status != 200 ||
		len(list(at(meta, "projects", 0, "issuetypes"))) != 1 || str(meta, "projects", 0, "issuetypes", 0, "name") != taskName {
		t.Errorf("issuetypeNames: %d %v", status, meta)
	}

	// The projects are those where the interface lets the user create issues.
	for username := range formUsers {
		rest := restAs(t, username)
		var want []string
		c := login(t, username, formUsers[username])
		var projects []struct{ Key string }
		c.get("/api/projects", &projects)
		for _, p := range projects {
			var detail struct{ Can map[string]bool }
			c.call("GET", "/api/projects/"+p.Key, nil, 200, &detail)
			if detail.Can["create"] {
				want = append(want, p.Key)
			}
		}
		status, meta := rest("GET", "/rest/api/2/issue/createmeta", nil)
		var got []string
		for _, p := range list(at(meta, "projects")) {
			got = append(got, str(p, "key"))
		}
		sort.Strings(got)
		sort.Strings(want)
		if status != 200 || !slices.Equal(got, want) {
			t.Errorf("%s: createmeta lists %v, the interface lets create in %v", username, got, want)
		}
	}
}

// TestRESTCreateMetaMatchesCreate: for every user, project and issue type, an
// issue created with a value for every field createmeta lists is created
// with them, and a field it does not list is refused.
func TestRESTCreateMetaMatchesCreate(t *testing.T) {
	var accepted, refused int
	t.Cleanup(func() {
		if t.Failed() || t.Skipped() {
			return
		}
		if accepted < 100 || refused < 50 {
			t.Errorf("the test checked little: %d listed fields accepted, %d unlisted ones refused", accepted, refused)
		}
	})
	for username := range formUsers {
		rest := restAs(t, username)
		for _, project := range userProjects(t, rest) {
			t.Run(username+"/"+project, func(t *testing.T) {
				rest := restAs(t, username)
				status, meta := rest("GET", "/rest/api/2/issue/createmeta?expand=projects.issuetypes.fields&projectKeys="+project, nil)
				if status != 200 {
					t.Fatalf("createmeta: %d", status)
				}
				if len(list(at(meta, "projects"))) == 0 {
					// Not listed: creating is refused.
					status, _ := rest("POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
						"project": map[string]string{"key": project}, "summary": "Refusé",
						"issuetype": map[string]string{"id": column(t, `SELECT issue_type_id FROM project_issue_types t
							JOIN projects p ON p.id = t.project_id WHERE p.pkey = ? LIMIT 1`, project)[0]}}})
					if status < 400 || status >= 500 {
						t.Errorf("createmeta does not list %s, yet creating there answers %d", project, status)
					}
					return
				}
				s := &sampler{t: t, rest: rest, user: username, project: project, epic: epicOf(t, project)}
				types := list(at(meta, "projects", 0, "issuetypes"))
				// A parent for sub-tasks: a standard issue of the project.
				for _, it := range types {
					if at(it, "subtask") == false {
						status, created := rest("POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
							"project": map[string]string{"key": project}, "summary": "Parent",
							"issuetype": map[string]string{"id": str(it, "id")}}})
						if status != 201 {
							t.Fatalf("parent: %d %v", status, created)
						}
						s.parent = str(created, "key")
						break
					}
				}
				universe := s.universe()
				for _, it := range types {
					typeID, form := str(it, "id"), at(it, "fields")
					subtask := at(it, "subtask") == true
					if subtask && s.parent == "" {
						continue
					}
					base := func() map[string]any {
						f := map[string]any{"project": map[string]string{"key": project},
							"issuetype": map[string]string{"id": typeID}, "summary": "Formulaire " + typeID}
						if subtask {
							f["parent"] = map[string]string{"key": s.parent}
						}
						return f
					}

					// Every listed field is accepted, all at once.
					fields, update := s.settings(form)
					for k, v := range base() {
						fields[k] = v
					}
					status, created := rest("POST", "/rest/api/2/issue", map[string]any{"fields": fields, "update": update})
					if status != 201 {
						t.Errorf("type %s: creating with %v: %d %v", typeID, sortedKeys(fields), status, created)
						continue
					}
					accepted += len(fields) + len(update)
					if status, issue := rest("GET", "/rest/api/2/issue/"+str(created, "key"), nil); status == 200 {
						for k := range fields {
							got := at(issue, "fields", k)
							switch k {
							case "project", "security":
							case "timetracking":
								got = at(issue, "fields", k, "originalEstimate")
							}
							if k != "project" && k != "security" && empty(got) {
								t.Errorf("type %s: %s was listed, accepted, and not stored: %v", typeID, k, at(issue, "fields"))
							}
						}
						if len(update) > 0 && num(issue, "fields", "comment", "total") != 1 {
							t.Errorf("type %s: the comment was not added", typeID)
						}
					}

					// A field not listed is refused. Three are accepted and ignored: the reporter,
					// which can only be oneself, the resolution when creating, and the security
					// level of a sub-task, which is its parent's.
					for _, key := range sortedKeys(universe) {
						if listed(form, key) || key == "reporter" || key == "resolution" || (key == "security" && subtask) {
							continue
						}
						fields, update := base(), map[string]any{}
						var v any = "x"
						if key == "comment" {
							update[key] = []any{map[string]any{"add": map[string]any{"body": "x"}}}
						} else {
							var ok bool
							if v, ok = s.value(key, universe[key]); !ok {
								continue
							}
							fields[key] = v
						}
						if status, body := rest("POST", "/rest/api/2/issue", map[string]any{"fields": fields, "update": update}); status < 400 || status >= 500 {
							t.Errorf("type %s: %s is not listed, yet creating with %v: %d %v", typeID, key, v, status, body)
						}
						refused++
					}
				}
			})
		}
	}
}

// TestRESTEditMeta: the fields editmeta lists can be edited, and the others
// cannot.
func TestRESTEditMeta(t *testing.T) {
	var accepted, refused int
	t.Cleanup(func() {
		if t.Failed() || t.Skipped() {
			return
		}
		if accepted < 50 || refused < 50 {
			t.Errorf("the test checked little: %d listed fields accepted, %d unlisted ones refused", accepted, refused)
		}
	})
	for username := range formUsers {
		for _, project := range userProjects(t, restAs(t, username)) {
			creator := creatorOf(t, project)
			if creator == "" {
				continue
			}
			t.Run(username+"/"+project, func(t *testing.T) {
				rest, by := restAs(t, username), restAs(t, creator)
				_, meta := by("GET", "/rest/api/2/issue/createmeta?projectKeys="+project, nil)
				var typeID string
				for _, it := range list(at(meta, "projects", 0, "issuetypes")) {
					if at(it, "subtask") == false {
						typeID = str(it, "id")
						break
					}
				}
				status, created := by("POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
					"project": map[string]string{"key": project}, "summary": "Édition", "issuetype": map[string]string{"id": typeID}}})
				if status != 201 {
					t.Fatalf("create: %d %v", status, created)
				}
				key := str(created, "key")
				s := &sampler{t: t, rest: rest, user: username, project: project, parent: key, epic: epicOf(t, project)}
				universe := s.universe()

				status, edit := rest("GET", "/rest/api/2/issue/"+key+"/editmeta", nil)
				if status == 404 {
					return // the user does not see it
				}
				form := at(edit, "fields")
				if status != 200 || form == nil {
					t.Fatalf("editmeta: %d %v", status, edit)
				}
				// Compare with the interface's own idea of what the user can do.
				var detail issueDetail
				login(t, username, formUsers[username]).get("/api/issues/"+key, &detail)
				if got := listed(form, "summary"); got != detail.Can["edit"] {
					t.Errorf("summary listed: %v, the interface lets edit: %v", got, detail.Can["edit"])
				}
				if got := listed(form, "comment"); got != detail.Can["comment"] {
					t.Errorf("comment listed: %v, the interface lets comment: %v", got, detail.Can["comment"])
				}

				// Each listed field is accepted by itself, the type and the security level last.
				fields, update := s.settings(form)
				order := sortedKeys(fields)
				order = slices.DeleteFunc(order, func(k string) bool { return k == "issuetype" || k == "security" })
				for _, k := range []string{"issuetype", "security"} {
					if _, ok := fields[k]; ok {
						order = append(order, k)
					}
				}
				for _, k := range order {
					v := fields[k]
					if k == "issuetype" {
						// Another type than the current one, when there is one.
						for _, a := range list(at(form, k, "allowedValues")) {
							if str(a, "id") != typeID {
								v = map[string]any{"id": str(a, "id")}
							}
						}
					}
					if status, body := rest("PUT", "/rest/api/2/issue/"+key, map[string]any{"fields": map[string]any{k: v}}); status != 204 {
						t.Errorf("%s is listed, yet editing it with %v: %d %v", k, v, status, body)
					}
					accepted++
				}
				for k, op := range update {
					if status, body := rest("PUT", "/rest/api/2/issue/"+key, map[string]any{"update": map[string]any{k: op}}); status != 204 {
						t.Errorf("%s is listed, yet adding to it: %d %v", k, status, body)
					}
				}
				// The operations announced work.
				for _, k := range []string{"labels", "components", "fixVersions", "versions"} {
					if !listed(form, k) {
						continue
					}
					v, ok := s.value(k, at(form, k).(map[string]any))
					if !ok {
						continue
					}
					item := v
					if l, isList := v.([]string); isList {
						item = l[0]
					} else if l, isList := v.([]any); isList {
						item = l[0]
					}
					for _, op := range []string{"add", "remove"} {
						if status, body := rest("PUT", "/rest/api/2/issue/"+key, map[string]any{"update": map[string]any{
							k: []any{map[string]any{op: item}}}}); status != 204 {
							t.Errorf("%s %s %v: %d %v", k, op, item, status, body)
						}
					}
				}

				// The fields not listed are refused (the reporter can only stay as it is, the
				// project too, and the resolution of an issue that is not done is ignored).
				for _, k := range sortedKeys(universe) {
					if listed(form, k) || k == "reporter" || k == "project" || k == "resolution" {
						continue
					}
					var v any = "x"
					body := map[string]any{"update": map[string]any{k: []any{map[string]any{"add": map[string]any{"body": "x"}}}}}
					if k != "comment" {
						var ok bool
						if v, ok = s.value(k, universe[k]); !ok {
							continue
						}
						body = map[string]any{"fields": map[string]any{k: v}}
					}
					if status, answer := rest("PUT", "/rest/api/2/issue/"+key, body); status < 400 || status >= 500 {
						t.Errorf("%s is not listed, yet editing it with %v: %d %v", k, v, status, answer)
					}
					refused++
				}
			})
		}
	}
}

// The resolution can be edited on an issue that is done, and only then.
func TestRESTEditMetaResolution(t *testing.T) {
	paul := restAs(t, "paul.martin")
	status, meta := paul("GET", "/rest/api/2/issue/createmeta?projectKeys=DEMO", nil)
	if status != 200 {
		t.Fatalf("createmeta: %d", status)
	}
	var typeID string
	for _, it := range list(at(meta, "projects", 0, "issuetypes")) {
		if at(it, "subtask") == false {
			typeID = str(it, "id")
			break
		}
	}
	status, created := paul("POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
		"project": map[string]string{"key": "DEMO"}, "summary": "Résolution", "issuetype": map[string]string{"id": typeID}}})
	if status != 201 {
		t.Fatalf("create: %d %v", status, created)
	}
	key := str(created, "key")
	_, edit := paul("GET", "/rest/api/2/issue/"+key+"/editmeta", nil)
	if listed(at(edit, "fields"), "resolution") {
		t.Errorf("resolution listed on an issue that is not done")
	}
	_, tr := paul("GET", "/rest/api/2/issue/"+key+"/transitions", nil)
	var done string
	for _, v := range list(at(tr, "transitions")) {
		if str(v, "to", "statusCategory", "key") == "done" {
			done = str(v, "id")
		}
	}
	if done == "" {
		t.Skip("no transition to done from the initial status")
	}
	if status, _ := paul("POST", "/rest/api/2/issue/"+key+"/transitions", map[string]any{"transition": map[string]string{"id": done}}); status != 204 {
		t.Fatalf("transition: %d", status)
	}
	_, edit = paul("GET", "/rest/api/2/issue/"+key+"/editmeta", nil)
	f := at(edit, "fields", "resolution")
	if f == nil || len(list(at(f, "allowedValues"))) == 0 {
		t.Fatalf("resolution not listed on a done issue: %v", at(edit, "fields"))
	}
	if status, body := paul("PUT", "/rest/api/2/issue/"+key, map[string]any{"fields": map[string]any{
		"resolution": map[string]string{"id": str(f, "allowedValues", 0, "id")}}}); status != 204 {
		t.Errorf("editing the resolution: %d %v", status, body)
	}
}

// Issues one does not see have no edit form.
func TestRESTEditMetaVisibility(t *testing.T) {
	paul := restAs(t, "paul.martin")
	hidden := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	if status, _ := paul("GET", "/rest/api/2/issue/"+hidden+"/editmeta", nil); status != http.StatusNotFound {
		t.Errorf("editmeta of an issue paul.martin does not see: %d", status)
	}
	if status, _ := paul("GET", "/rest/api/2/issue/NOPE-1/editmeta", nil); status != http.StatusNotFound {
		t.Errorf("editmeta of an unknown issue: %d", status)
	}
}

// Issue picker -----------------------------------------------------------------

// pickerKeys lists the issues of a picker section.
func pickerKeys(answer any, section string) []string {
	var out []string
	for _, s := range list(at(answer, "sections")) {
		if str(s, "id") == section {
			for _, it := range list(at(s, "issues")) {
				out = append(out, str(it, "key"))
			}
		}
	}
	return out
}

func TestRESTIssuePicker(t *testing.T) {
	paul, chloe := restAs(t, "paul.martin"), restAs(t, "chloe.simon")
	picker := func(rest restCall, query string) (int, any) {
		t.Helper()
		return rest("GET", "/rest/api/2/issue/picker?"+query, nil)
	}
	demo := pick(t, `SELECT id FROM projects WHERE pkey = 'DEMO'`)
	base := `SELECT ` + keyExpr + `, i.summary FROM issues i JOIN projects p ON p.id = i.project_id WHERE p.pkey = ?
		AND i.security_level_id IS NULL AND i.parent_id IS NULL AND CHAR_LENGTH(i.summary) > 8 ORDER BY i.num LIMIT 1`
	var key, summary string
	if err := testDB.QueryRow(base, "DEMO").Scan(&key, &summary); err != nil {
		t.Fatal(err)
	}

	// Jira's answer: two sections, issues with their key and summary.
	word := strings.Fields(summary)[0]
	status, answer := picker(paul, "query="+url.QueryEscape(strings.ToUpper(word)))
	if status != 200 || len(list(at(answer, "sections"))) != 2 || str(answer, "sections", 0, "id") != "hs" ||
		str(answer, "sections", 1, "id") != "cs" {
		t.Fatalf("picker: %d %v", status, answer)
	}
	cs := list(at(answer, "sections", 1, "issues"))
	if len(cs) == 0 || len(cs) > 10 {
		t.Fatalf("current search: %v", cs)
	}
	for _, it := range cs {
		if num(it, "id") == 0 || str(it, "key") == "" || str(it, "summaryText") == "" ||
			!strings.Contains(strings.ToLower(str(it, "summaryText")+str(it, "key")), strings.ToLower(word)) ||
			!strings.Contains(str(it, "keyHtml")+str(it, "summary"), "<b>") {
			t.Errorf("picker issue for %q: %v", word, it)
		}
	}

	// By key, in any case, and by number in the current project.
	for _, q := range []string{"query=" + strings.ToLower(key), "query=" + strings.Split(key, "-")[1] + "&currentProjectId=" + demo} {
		if status, answer := picker(paul, q); status != 200 || !slices.Contains(pickerKeys(answer, "cs"), key) {
			t.Errorf("picker?%s: %d %v", q, status, pickerKeys(answer, "cs"))
		}
	}
	if _, answer := picker(paul, "query="+strings.Split(key, "-")[1]); slices.Contains(pickerKeys(answer, "cs"), key) &&
		len(pickerKeys(answer, "cs")) == 1 {
		t.Errorf("a bare number found %s without a current project", key)
	}
	// No query: the latest issues.
	if _, answer := picker(paul, ""); len(pickerKeys(answer, "cs")) != 10 {
		t.Errorf("picker without a query: %v", pickerKeys(answer, "cs"))
	}

	// Only issues the user sees: RH and the restricted level are hidden from paul.martin.
	var rhKey, rhSummary string
	if err := testDB.QueryRow(base, "RH").Scan(&rhKey, &rhSummary); err != nil {
		t.Fatal(err)
	}
	restricted := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		JOIN security_levels l ON l.id = i.security_level_id JOIN users u ON u.id = i.reporter_id
		WHERE l.name = 'Restreint' AND u.username NOT IN ('paul.martin', 'marie.durand') AND u.active
		ORDER BY i.num LIMIT 1`)
	for _, k := range []string{rhKey, restricted} {
		_, mine := picker(paul, "query="+k)
		_, hers := picker(chloe, "query="+k)
		if slices.Contains(pickerKeys(mine, "cs"), k) || slices.Contains(pickerKeys(mine, "hs"), k) {
			t.Errorf("paul.martin is offered %s: %v", k, mine)
		}
		if k == rhKey && !slices.Contains(pickerKeys(hers, "cs"), k) {
			t.Errorf("chloe.simon is not offered %s: %v", k, hers)
		}
		// Not by its text either.
		_, mine = picker(paul, "query="+url.QueryEscape(rhSummary))
		if slices.Contains(pickerKeys(mine, "cs"), rhKey) {
			t.Errorf("paul.martin finds %s by its summary", rhKey)
		}
	}

	// The current search follows the JQL the client shows.
	_, answer = picker(chloe, "query="+rhKey+"&currentJQL="+url.QueryEscape("project = DEMO"))
	if len(pickerKeys(answer, "cs")) != 0 {
		t.Errorf("%s is outside the current search: %v", rhKey, pickerKeys(answer, "cs"))
	}
	_, answer = picker(chloe, "currentJQL="+url.QueryEscape("project = RH ORDER BY key"))
	for _, k := range pickerKeys(answer, "cs") {
		if !strings.HasPrefix(k, "RH-") {
			t.Errorf("current search of project = RH has %s", k)
		}
	}
	if len(pickerKeys(answer, "cs")) == 0 {
		t.Errorf("current search of project = RH is empty")
	}
	if status, _ := picker(paul, "currentJQL="+url.QueryEscape("project = (")); status != 400 {
		t.Errorf("invalid currentJQL: %d", status)
	}

	// The current issue is left out; sub-tasks and the parent can be too.
	_, answer = picker(paul, "query="+key+"&currentIssueKey="+key)
	if slices.Contains(pickerKeys(answer, "cs"), key) {
		t.Errorf("the current issue %s is offered", key)
	}
	var child, childSummary, parent string
	if err := testDB.QueryRow(`SELECT `+keyExpr+`, i.summary, CONCAT(pp.pkey, '-', par.num) FROM issues i
		JOIN projects p ON p.id = i.project_id JOIN issues par ON par.id = i.parent_id
		JOIN projects pp ON pp.id = par.project_id WHERE p.pkey = 'DEMO' AND i.security_level_id IS NULL
		AND par.security_level_id IS NULL ORDER BY i.num LIMIT 1`).Scan(&child, &childSummary, &parent); err != nil {
		t.Skipf("no sub-task in DEMO: %v", err)
	}
	for query, wantChild := range map[string]bool{"": true, "&showSubTasks=false": false, "&showSubTasks=true": true} {
		_, answer := picker(paul, "query="+child+query)
		if got := slices.Contains(pickerKeys(answer, "cs"), child); got != wantChild {
			t.Errorf("sub-task %s offered: %v, want %v (%s)", child, got, wantChild, query)
		}
	}
	for query, wantParent := range map[string]bool{"": true, "&showSubTaskParent=false": false} {
		_, answer := picker(paul, "query="+parent+"&currentIssueKey="+child+query)
		if got := slices.Contains(pickerKeys(answer, "cs"), parent); got != wantParent {
			t.Errorf("parent %s of the current sub-task offered: %v, want %v (%s)", parent, got, wantParent, query)
		}
	}

	// The history holds what the user worked on, for them alone, and the text
	// is escaped.
	title := `<img src=x onerror=1> & ` + unique("Picker")
	status, created := paul("POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
		"project": map[string]string{"key": "DEMO"}, "summary": title,
		"issuetype": map[string]string{"id": column(t, `SELECT it.id FROM project_issue_types t
			JOIN issue_types it ON it.id = t.issue_type_id JOIN projects p ON p.id = t.project_id
			WHERE p.pkey = 'DEMO' AND NOT it.subtask ORDER BY t.position LIMIT 1`)[0]}}})
	if status != 201 {
		t.Fatalf("create: %d %v", status, created)
	}
	mine := str(created, "key")
	if status, _ := paul("POST", "/rest/api/2/issue/"+mine+"/comment", map[string]any{"body": "Vu."}); status != 201 {
		t.Fatalf("comment: %d", status)
	}
	_, answer = picker(paul, "query="+url.QueryEscape("IMG src"))
	if got := pickerKeys(answer, "hs"); len(got) != 1 || got[0] != mine {
		t.Errorf("history: %v, want [%s]", got, mine)
	}
	item := at(answer, "sections", 0, "issues", 0)
	if str(item, "summaryText") != title || strings.Contains(str(item, "summary"), "<img") ||
		!strings.Contains(str(item, "summary"), "&lt;<b>img src</b>=x") {
		t.Errorf("picker text: %v", item)
	}
	if _, other := picker(restAs(t, "hugo.robert"), "query="+url.QueryEscape("IMG src")); len(pickerKeys(other, "hs")) != 0 ||
		!slices.Contains(pickerKeys(other, "cs"), mine) {
		t.Errorf("hugo.robert's history holds %s: %v", mine, other)
	}
	if status, _, _ := call(t, nil, testURL, "GET", "/rest/api/2/issue/picker", nil, nil); status != 401 {
		t.Errorf("picker without credentials: %d", status)
	}
}
