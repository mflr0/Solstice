package tests

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

// TestRESTMyPermissionsBothParameters sends both the key and the id of an
// issue, and of a project: the key is read, never the two joined.
func TestRESTMyPermissionsBothParameters(t *testing.T) {
	rest := restAs(t, "paul.martin")
	// The last issue: its key followed by any id names no issue.
	key := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' AND i.security_level_id IS NULL ORDER BY i.num DESC LIMIT 1`)
	otherID := pick(t, `SELECT i.id FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	projectID := pick(t, `SELECT id FROM projects WHERE pkey = 'DEMO'`)

	for _, c := range []struct{ one, both string }{
		{"issueKey=" + key, "issueKey=" + key + "&issueId=" + otherID},
		{"projectKey=DEMO", "projectKey=DEMO&projectId=" + projectID},
	} {
		status, want := rest("GET", "/rest/api/2/mypermissions?"+c.one, nil)
		if status != 200 {
			t.Fatalf("%s: %d %v", c.one, status, want)
		}
		status, got := rest("GET", "/rest/api/2/mypermissions?"+c.both, nil)
		if status != 200 {
			t.Fatalf("%s: %d %v", c.both, status, got)
		}
		for name, p := range at(want, "permissions").(map[string]any) {
			if at(got, "permissions", name, "havePermission") != at(p, "havePermission") {
				t.Errorf("%s: %s is %v, %v with %s", c.both, name, at(got, "permissions", name, "havePermission"),
					at(p, "havePermission"), c.one)
			}
		}
	}
}

// TestRESTEditErrorOrder sends edits with several bad fields: the answer
// names the first of them in alphabetical order, every time.
func TestRESTEditErrorOrder(t *testing.T) {
	rest := restAs(t, "admin")
	key := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' ORDER BY i.num LIMIT 1`)
	path := "/rest/api/2/issue/" + key

	for _, c := range []struct {
		name  string
		body  map[string]any
		field string
		text  string
	}{
		{"fields", map[string]any{"fields": map[string]any{"summary": 5, "labels": 5, "duedate": 5, "description": 5}},
			"description", "description"},
		{"update operations", map[string]any{"update": map[string]any{
			"labels": []any{map[string]any{"frob": "x"}}, "components": []any{map[string]any{"frob": "x"}},
			"versions": []any{map[string]any{"frob": "x"}}}}, "components", "frob"},
		{"verbs", map[string]any{"update": map[string]any{
			"labels": []any{map[string]any{"zap": "x", "frob": "x", "quux": "x"}}}}, "labels", "frob"},
		{"time tracking", map[string]any{"fields": map[string]any{
			"timetracking": map[string]any{"remainingEstimate": "soon", "originalEstimate": "never", "zzz": "1h"}}},
			"timetracking", "1w 2d"},
	} {
		for range 20 {
			status, e := rest("PUT", path, c.body)
			errs, _ := at(e, "errors").(map[string]any)
			if status != 400 || !slices.Equal(slices.Collect(maps.Keys(errs)), []string{c.field}) {
				t.Fatalf("%s: %d %v, want an error on %s only", c.name, status, e, c.field)
			}
			if msg, _ := errs[c.field].(string); !strings.Contains(msg, c.text) {
				t.Fatalf("%s: %q does not name %q", c.name, msg, c.text)
			}
		}
	}
}
