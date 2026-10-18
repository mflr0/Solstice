package tests

import (
	"encoding/json"
	"testing"
)

type resolutionItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

func TestDefaultResolution(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")

	list := func() (items []resolutionItem, def string) {
		t.Helper()
		if status := admin.get("/api/admin/resolutions", &items); status != 200 {
			t.Fatalf("list: %d", status)
		}
		for _, it := range items {
			if it.Default {
				if def != "" {
					t.Fatalf("two default resolutions: %s and %s", def, it.ID)
				}
				def = it.ID
			}
		}
		return items, def
	}
	setDefault := func(id string, on bool) {
		t.Helper()
		if status, body := admin.do("PATCH", "/api/admin/resolutions/"+id, map[string]any{"default": on}); status != 200 {
			t.Fatalf("default %s = %v: %d %s", id, on, status, body)
		}
	}

	items, imported := list()
	if imported != pick(t, `SELECT id FROM resolutions WHERE name = 'Fixed'`) {
		t.Fatalf("the import must keep Jira's default (Fixed), got %q in %+v", imported, items)
	}
	t.Cleanup(func() { setDefault(imported, true) })

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
	// resolve creates an issue and moves it to a done status, without
	// naming a resolution.
	resolve := func() *namedRef {
		t.Helper()
		status, body := paul.do("POST", "/api/issues", map[string]any{"project": "DEMO", "type": task,
			"summary": "Résolution par défaut"})
		if status != 201 {
			t.Fatalf("create: %d %s", status, body)
		}
		var created struct{ Key string }
		json.Unmarshal(body, &created)
		t.Cleanup(func() { admin.do("DELETE", "/api/issues/"+created.Key, nil) })
		var d issueDetail
		paul.get("/api/issues/"+created.Key, &d)
		var done string
		for _, s := range d.Statuses {
			if s.Category == "done" {
				done = s.ID
			}
		}
		status, body = paul.do("PATCH", "/api/issues/"+created.Key, map[string]any{"status": done})
		if status != 200 {
			t.Fatalf("transition: %d %s", status, body)
		}
		json.Unmarshal(body, &d)
		if d.Status.ID != done {
			t.Fatalf("transition did not reach %s: %+v", done, d.Status)
		}
		return d.Resolution
	}

	if r := resolve(); r == nil || r.ID != imported {
		t.Errorf("resolved as %+v, want the default %s", r, imported)
	}

	// Setting another default moves it.
	var other string
	for _, it := range items {
		if it.ID != imported {
			other = it.ID
			break
		}
	}
	setDefault(other, true)
	if _, def := list(); def != other {
		t.Fatalf("default is %q, want %q", def, other)
	}
	var meta catalog
	paul.get("/api/meta", &meta)
	if meta.DefaultResolution != other {
		t.Errorf("meta gives default %q, want %q", meta.DefaultResolution, other)
	}
	if r := resolve(); r == nil || r.ID != other {
		t.Errorf("resolved as %+v, want the new default %s", r, other)
	}

	// Without a default, done issues stay unresolved.
	setDefault(other, false)
	if _, def := list(); def != "" {
		t.Fatalf("default %q left after unsetting it", def)
	}
	if r := resolve(); r != nil {
		t.Errorf("resolved as %+v without a default resolution", r)
	}
}
