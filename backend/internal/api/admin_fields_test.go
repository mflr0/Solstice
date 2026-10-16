package api

import (
	"encoding/json"
	"strconv"
	"testing"
)

type fieldOptionOut struct {
	ID       int64  `json:"id"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
	Issues   int    `json:"issues"`
	Children int    `json:"children"`
}

type fieldDetailOut struct {
	ID      int64            `json:"id"`
	Options []fieldOptionOut `json:"options"`
}

// TestDeleteFieldOption covers the removal of a custom field's options: an
// unused option goes, a used one is refused until its values move to another
// option or are cleared, and a text field has no options to remove.
func TestDeleteFieldOption(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")

	var field fieldDetailOut
	status, body := admin.do("POST", "/api/admin/fields", map[string]any{
		"name": "Option removal test", "type": "multiselect", "options": []string{"Red", "Green", "Blue"}})
	if status != 201 {
		t.Fatalf("create field: %d %s", status, body)
	}
	json.Unmarshal(body, &field)
	fieldPath := "/api/admin/fields/" + strconv.FormatInt(field.ID, 10)
	t.Cleanup(func() { admin.do("DELETE", fieldPath, map[string]string{"confirm": "Option removal test"}) })
	if len(field.Options) != 3 {
		t.Fatalf("options: %+v", field.Options)
	}
	red, green, blue := field.Options[0], field.Options[1], field.Options[2]
	optionPath := func(o fieldOptionOut) string { return fieldPath + "/options/" + strconv.FormatInt(o.ID, 10) }

	// Issues choosing options.
	var project struct {
		IssueTypes []typeRef `json:"issueTypes"`
	}
	paul.get("/api/projects/DEMO", &project)
	var task string
	for _, it := range project.IssueTypes {
		if !it.Subtask {
			task = it.ID
			break
		}
	}
	create := func(options ...fieldOptionOut) string {
		t.Helper()
		ids := []map[string]string{}
		for _, o := range options {
			ids = append(ids, map[string]string{"id": strconv.FormatInt(o.ID, 10)})
		}
		status, body := paul.do("POST", "/api/issues", map[string]any{"project": "DEMO", "type": task,
			"summary": "Option removal", "customFields": map[string]any{strconv.FormatInt(field.ID, 10): ids}})
		if status != 201 {
			t.Fatalf("create issue: %d %s", status, body)
		}
		var created struct{ Key string }
		json.Unmarshal(body, &created)
		t.Cleanup(func() { admin.do("DELETE", "/api/issues/"+created.Key, nil) })
		return created.Key
	}
	create(red)
	create(red, green)
	detail := func() []fieldOptionOut {
		t.Helper()
		var f fieldDetailOut
		if status := admin.get(fieldPath, &f); status != 200 {
			t.Fatalf("detail: %d", status)
		}
		return f.Options
	}
	if o := detail(); o[0].Issues != 2 || o[1].Issues != 1 || o[2].Issues != 0 {
		t.Fatalf("issues per option: %+v", o)
	}

	// Not an administrator, unknown option, option of another field.
	if status, _ := paul.do("DELETE", optionPath(blue), nil); status != 403 {
		t.Errorf("non-admin delete: %d", status)
	}
	if status, _ := admin.do("DELETE", fieldPath+"/options/999999999", nil); status != 404 {
		t.Errorf("unknown option: %d", status)
	}

	// Unused: it goes.
	if status, body := admin.do("DELETE", optionPath(blue), nil); status != 200 {
		t.Fatalf("delete unused: %d %s", status, body)
	}
	if o := detail(); len(o) != 2 {
		t.Fatalf("after delete: %+v", o)
	}

	// Used: refused with the usage, whatever the replacement.
	status, body = admin.do("DELETE", optionPath(red), nil)
	if status != 409 {
		t.Fatalf("delete used: %d %s", status, body)
	}
	var refused struct {
		Usage map[string]int `json:"usage"`
	}
	json.Unmarshal(body, &refused)
	if refused.Usage["issues"] != 2 {
		t.Errorf("usage: %s", body)
	}
	if status, _ := admin.do("DELETE", optionPath(red), map[string]int64{"moveTo": red.ID}); status != 400 {
		t.Errorf("move to itself: %d", status)
	}
	if status, _ := admin.do("DELETE", optionPath(red), map[string]int64{"moveTo": 999999999}); status != 400 {
		t.Errorf("move to a stranger: %d", status)
	}

	// Moved: the issue holding both keeps one value, the other gets Green.
	if status, body := admin.do("DELETE", optionPath(red), map[string]int64{"moveTo": green.ID}); status != 200 {
		t.Fatalf("move: %d %s", status, body)
	}
	o := detail()
	if len(o) != 1 || o[0].ID != green.ID || o[0].Issues != 2 {
		t.Fatalf("after move: %+v", o)
	}
	var rows int
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM custom_field_values WHERE field_id = ?`, field.ID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Errorf("value rows after move: %d, want 2", rows)
	}

	// Cleared: the values go, the issues stay.
	if status, body := admin.do("DELETE", optionPath(green), map[string]bool{"clear": true}); status != 200 {
		t.Fatalf("clear: %d %s", status, body)
	}
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM custom_field_values WHERE field_id = ?`, field.ID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 || len(detail()) != 0 {
		t.Errorf("after clear: %d value rows, options %+v", rows, detail())
	}

	// A field without options refuses the route.
	var text fieldDetailOut
	status, body = admin.do("POST", "/api/admin/fields", map[string]any{"name": "Option removal text", "type": "text"})
	if status != 201 {
		t.Fatalf("create text field: %d %s", status, body)
	}
	json.Unmarshal(body, &text)
	textPath := "/api/admin/fields/" + strconv.FormatInt(text.ID, 10)
	t.Cleanup(func() { admin.do("DELETE", textPath, nil) })
	if status, _ := admin.do("DELETE", textPath+"/options/1", nil); status != 400 {
		t.Errorf("text field option: %d", status)
	}
}

// TestDeleteCascadingOption checks that removing a parent option takes its
// children and their values with it, and that their values count as the
// parent's.
func TestDeleteCascadingOption(t *testing.T) {
	admin := login(t, "admin", "admin")
	issue := pick(t, `SELECT CAST(id AS CHAR) FROM issues ORDER BY id LIMIT 1`)
	exec := func(query string, args ...any) int64 {
		t.Helper()
		res, err := testDB.Exec(query, args...)
		if err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
		id, _ := res.LastInsertId()
		return id
	}
	field := exec(`INSERT INTO custom_fields (type_key, name) VALUES (?, 'Cascade removal test')`,
		jiraFieldPrefix+"cascadingselect")
	t.Cleanup(func() { exec(`DELETE FROM custom_fields WHERE id = ?`, field) })
	parent := exec(`INSERT INTO custom_field_options (field_id, position, value) VALUES (?, 1, 'Europe')`, field)
	child := exec(`INSERT INTO custom_field_options (field_id, parent_id, position, value) VALUES (?, ?, 1, 'Paris')`, field, parent)
	exec(`INSERT INTO custom_field_values (issue_id, field_id, string_value) VALUES (?, ?, ?)`, issue, field, strconv.FormatInt(parent, 10))
	exec(`INSERT INTO custom_field_values (issue_id, field_id, parent_key, string_value) VALUES (?, ?, ?, ?)`,
		issue, field, strconv.FormatInt(parent, 10), strconv.FormatInt(child, 10))

	path := "/api/admin/fields/" + strconv.FormatInt(field, 10)
	var f fieldDetailOut
	admin.get(path, &f)
	if len(f.Options) != 1 || f.Options[0].Issues != 1 || f.Options[0].Children != 1 {
		t.Fatalf("detail: %+v", f.Options)
	}
	optionPath := path + "/options/" + strconv.FormatInt(parent, 10)
	if status, _ := admin.do("DELETE", optionPath, nil); status != 409 {
		t.Fatalf("delete used parent: %d", status)
	}
	if status, body := admin.do("DELETE", optionPath, map[string]bool{"clear": true}); status != 200 {
		t.Fatalf("clear: %d %s", status, body)
	}
	var options, values int
	testDB.QueryRow(`SELECT COUNT(*) FROM custom_field_options WHERE field_id = ?`, field).Scan(&options)
	testDB.QueryRow(`SELECT COUNT(*) FROM custom_field_values WHERE field_id = ?`, field).Scan(&values)
	if options != 0 || values != 0 {
		t.Errorf("left behind: %d options, %d values", options, values)
	}
}
