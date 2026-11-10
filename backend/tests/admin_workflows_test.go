package tests

// Global workflows: seeded at startup in the instance language, managed by
// global administrators, invisible to the project endpoints.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"testing"
	"time"

	"solstice/internal/api"
	"solstice/internal/i18n"
	"solstice/internal/jirafixture"
	"solstice/internal/settings"
)

type globalWorkflow struct {
	ID          int64
	Name        string
	Default     bool
	Initial     struct{ ID string }
	Statuses    []struct{ ID, Name string }
	Transitions []struct {
		ID   int64
		Name string
		From *struct{ ID string }
		To   struct{ ID string }
	}
}

// put is the body of a PUT that keeps the workflow as it is.
func (w globalWorkflow) put() map[string]any {
	var statuses []string
	for _, st := range w.Statuses {
		statuses = append(statuses, st.ID)
	}
	var transitions []map[string]any
	for _, tr := range w.Transitions {
		var from *string
		if tr.From != nil {
			from = &tr.From.ID
		}
		transitions = append(transitions, map[string]any{"id": tr.ID, "name": tr.Name, "from": from, "to": tr.To.ID})
	}
	return map[string]any{"name": w.Name, "initial": w.Initial.ID, "statuses": statuses, "transitions": transitions,
		"default": w.Default}
}

func globalWorkflowByID(list []globalWorkflow, id int64) *globalWorkflow {
	i := slices.IndexFunc(list, func(w globalWorkflow) bool { return w.ID == id })
	if i < 0 {
		return nil
	}
	return &list[i]
}

// A global administrator copies, edits and deletes global workflows, and
// each change is audited; others get 403, and the project endpoints do not
// find them. A status a global workflow uses shows it and cannot be deleted.
func TestGlobalWorkflows(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password) // a project administrator
	lucas := login(t, "lucas.bernard", jirafixture.Password)

	var list []globalWorkflow
	admin.call("GET", "/api/admin/workflows", nil, 200, &list)
	var def *globalWorkflow
	for i := range list {
		if list[i].Default {
			if def != nil {
				t.Fatalf("two default global workflows: %+v", list)
			}
			def = &list[i]
		}
	}
	if len(list) < 2 || def == nil {
		t.Fatalf("global workflows: %+v", list)
	}
	defID := def.ID

	tag := fmt.Sprintf("check900-%d", time.Now().UnixNano())
	var created struct{ ID int64 }
	admin.call("POST", "/api/admin/workflows", map[string]any{"name": tag}, 201, &created)
	path := fmt.Sprintf("/api/admin/workflows/%d", created.ID)
	t.Cleanup(func() { admin.do("DELETE", path, nil) })
	admin.call("GET", "/api/admin/workflows", nil, 200, &list)
	def = globalWorkflowByID(list, defID)
	found := globalWorkflowByID(list, created.ID)
	if found == nil {
		t.Fatalf("copy not listed: %+v", list)
	}
	copied := *found // list is decoded into again below
	if def == nil || copied.Default || !def.Default || len(copied.Statuses) != len(def.Statuses) ||
		len(copied.Transitions) != len(def.Transitions) || copied.Initial != def.Initial {
		t.Fatalf("copy of the default: %+v, default %+v", copied, def)
	}
	admin.call("POST", "/api/admin/workflows", map[string]any{"name": tag, "copyFrom": -1}, 400, nil)
	admin.call("POST", "/api/admin/workflows", map[string]any{"name": " "}, 400, nil)

	// A status of its own, first and initial, reached from any status.
	var st struct{ ID string }
	admin.call("POST", "/api/statuses", map[string]any{"name": tag, "category": "todo"}, 201, &st)
	t.Cleanup(func() { admin.do("DELETE", "/api/statuses/"+st.ID, nil) })
	body := copied.put()
	body["name"] = tag + " renamed"
	body["statuses"] = append([]string{st.ID}, body["statuses"].([]string)...)
	body["initial"] = st.ID
	body["transitions"] = append(body["transitions"].([]map[string]any),
		map[string]any{"name": "Back", "from": nil, "to": st.ID})
	admin.call("PUT", path, body, 200, &list)
	got := globalWorkflowByID(list, created.ID)
	if got == nil || got.Name != tag+" renamed" || got.Initial.ID != st.ID || got.Statuses[0].ID != st.ID ||
		len(got.Transitions) != len(copied.Transitions)+1 || got.Default {
		t.Fatalf("after PUT: %+v", got)
	}
	bad := got.put()
	bad["initial"] = "no-such-status"
	admin.call("PUT", path, bad, 400, nil)
	admin.call("PUT", "/api/admin/workflows/0", got.put(), 404, nil)

	var usage struct {
		Workflows []struct {
			ID      int64
			Project *struct{ Key string }
		}
	}
	admin.call("GET", "/api/admin/statuses/"+st.ID+"/usage", nil, 200, &usage)
	if len(usage.Workflows) != 1 || usage.Workflows[0].ID != created.ID || usage.Workflows[0].Project != nil {
		t.Errorf("usage of the status: %+v", usage.Workflows)
	}
	admin.call("DELETE", "/api/statuses/"+st.ID, nil, 409, nil)

	project := fmt.Sprintf("/api/workflows/%d", created.ID)
	admin.call("PUT", project, got.put(), 404, nil)
	admin.call("DELETE", project, nil, 404, nil)
	paul.call("PUT", project, got.put(), 404, nil)

	for name, c := range map[string]*client{"project admin": paul, "user": lucas} {
		for _, req := range []struct {
			method, path string
			body         any
		}{{"GET", "/api/admin/workflows", nil}, {"POST", "/api/admin/workflows", map[string]any{"name": tag}},
			{"PUT", path, got.put()}, {"DELETE", path, nil}} {
			if status, _ := c.do(req.method, req.path, req.body); status != http.StatusForbidden {
				t.Errorf("%s %s %s: %d, want 403", name, req.method, req.path, status)
			}
		}
	}

	admin.call("DELETE", fmt.Sprintf("/api/admin/workflows/%d", defID), nil, 409, nil)
	admin.call("DELETE", path, nil, 204, nil)
	admin.call("DELETE", path, nil, 404, nil)
	admin.call("DELETE", "/api/statuses/"+st.ID, nil, 204, nil)

	for action, q := range map[string]string{"workflow.create": tag, "workflow.update": tag + " renamed",
		"workflow.delete": tag + " renamed"} {
		var audit struct{ Total int }
		admin.call("GET", "/api/admin/audit?action="+action+"&q="+url.QueryEscape(q), nil, 200, &audit)
		if audit.Total != 1 {
			t.Errorf("audit entries %s: %d, want 1", action, audit.Total)
		}
	}
}

// A new instance gets Software (the default) and Simple, named in its
// language; another start does not seed them again. Making one the default
// takes the flag from the other, and the last one cannot be deleted.
func TestGlobalWorkflowsSeed(t *testing.T) {
	dsn := emptyDB(t)
	s, c := unlocked(t, api.SetupOptions{})
	if status, body := c.call("POST", "/api/setup/database", "fr", dbFields(t, dsn)); status != 200 {
		t.Fatalf("database: %d %v", status, body)
	}
	if status, body := c.call("POST", "/api/setup/admin", "fr", map[string]string{"username": "chef",
		"email": "chef@example.org", "password": "une phrase de passe longue"}); status != 200 {
		t.Fatalf("admin: %d %v", status, body)
	}
	if err := settings.New(s.DB()).SetJSON(context.Background(), "general.default_language", "fr"); err != nil {
		t.Fatal(err)
	}
	start := func() *client {
		ts := httptest.NewServer(api.New(s.DB(), t.TempDir(), time.UTC, quiet()).Handler(http.NotFoundHandler()))
		t.Cleanup(ts.Close)
		return loginAt(t, ts.URL, "chef", "une phrase de passe longue")
	}
	admin := start()

	var list []globalWorkflow
	admin.call("GET", "/api/admin/workflows", nil, 200, &list)
	fr := func(keys ...i18n.Key) []string {
		var out []string
		for _, k := range keys {
			out = append(out, i18n.M(k).In("fr"))
		}
		return out
	}
	want := []struct {
		name     string
		def      bool
		statuses []string
	}{
		{i18n.M(i18n.DefaultWorkflowSoftware).In("fr"), true, fr(i18n.DefaultStatusToDo, i18n.DefaultStatusInProgress,
			i18n.DefaultStatusInReview, i18n.DefaultStatusDone)},
		{i18n.M(i18n.DefaultWorkflowSimple).In("fr"), false, fr(i18n.DefaultStatusToDo, i18n.DefaultStatusInProgress,
			i18n.DefaultStatusDone)},
	}
	if len(list) != len(want) {
		t.Fatalf("seeded workflows: %+v", list)
	}
	for i, w := range want {
		got := list[i]
		var names []string
		for _, st := range got.Statuses {
			names = append(names, st.Name)
		}
		if got.Name != w.name || got.Default != w.def || !slices.Equal(names, w.statuses) ||
			got.Initial.ID != got.Statuses[0].ID || len(got.Transitions) != len(got.Statuses) {
			t.Errorf("seeded workflow %d: %+v, want %+v", i, got, w)
		}
		for j, tr := range got.Transitions {
			if tr.From != nil || tr.To.ID != got.Statuses[j].ID {
				t.Errorf("%s: transition %+v, want from any status to %s", got.Name, tr, got.Statuses[j].ID)
			}
		}
	}
	software, simple := list[0], list[1]

	body := simple.put()
	body["default"] = true
	admin.call("PUT", fmt.Sprintf("/api/admin/workflows/%d", simple.ID), body, 200, &list)
	if !list[0].Default || list[0].ID != simple.ID || list[1].Default {
		t.Fatalf("after making Simple the default: %+v", list)
	}
	admin.call("DELETE", fmt.Sprintf("/api/admin/workflows/%d", software.ID), nil, 204, nil)

	admin = start()
	admin.call("GET", "/api/admin/workflows", nil, 200, &list)
	if len(list) != 1 || list[0].ID != simple.ID {
		t.Fatalf("after a restart: %+v", list)
	}
	var conflict struct{ Error string }
	raw := admin.call("DELETE", fmt.Sprintf("/api/admin/workflows/%d", simple.ID), nil, 409, nil)
	json.Unmarshal(raw, &conflict)
	if conflict.Error == "" {
		t.Errorf("delete the last workflow: no message in %s", raw)
	}
}
