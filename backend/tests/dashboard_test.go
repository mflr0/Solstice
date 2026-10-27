package tests

// Tests of the personal dashboard (GET and PUT /api/dashboard).
// They need SOLSTICE_TEST_DB and are skipped without it.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
)

type gadgetOut struct {
	ID     int64  `json:"id"`
	Kind   string `json:"kind"`
	Filter *struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Query string `json:"query"`
	} `json:"filter"`
	Unavailable bool `json:"unavailable"`
	Config      struct {
		Columns []string `json:"columns"`
		Size    int      `json:"size"`
		Field   string   `json:"field"`
	} `json:"config"`
}

type dashboardOut struct {
	Customised bool        `json:"customised"`
	Gadgets    []gadgetOut `json:"gadgets"`
}

// dashboard reads or saves a dashboard, failing the test unless it answers 200.
func dashboard(c *client, method string, body any) dashboardOut {
	var d dashboardOut
	c.call(method, "/api/dashboard", body, 200, &d)
	return d
}

func resetDashboard(t *testing.T, username string) {
	t.Helper()
	id := userID(t, username)
	testDB.Exec(`DELETE FROM dashboard_gadgets WHERE user_id = ?`, id)
	testDB.Exec(`DELETE FROM dashboards WHERE user_id = ?`, id)
}

func TestDashboardDefaults(t *testing.T) {
	resetDashboard(t, "hugo.robert")
	t.Cleanup(func() { resetDashboard(t, "hugo.robert") })
	hugo := login(t, "hugo.robert", jirafixture.Password)

	d := dashboard(hugo, "GET", nil)
	if d.Customised || len(d.Gadgets) != 3 || d.Gadgets[0].Kind != "assigned" || d.Gadgets[1].Kind != "reported" ||
		d.Gadgets[2].Kind != "recent" {
		t.Fatalf("default dashboard: %+v", d)
	}

	// Removing every gadget leaves an empty dashboard, not the defaults.
	d = dashboard(hugo, "PUT", map[string]any{"gadgets": []any{}})
	if !d.Customised || len(d.Gadgets) != 0 {
		t.Errorf("emptied dashboard: %+v", d)
	}
	d = dashboard(hugo, "GET", nil)
	if !d.Customised || len(d.Gadgets) != 0 {
		t.Errorf("emptied dashboard, read back: %+v", d)
	}
}

func TestDashboardValidation(t *testing.T) {
	resetDashboard(t, "hugo.robert")
	t.Cleanup(func() { resetDashboard(t, "hugo.robert") })
	hugo := login(t, "hugo.robert", jirafixture.Password)
	marie := login(t, "marie.durand", jirafixture.Password)

	var private filterOut
	marie.call("POST", "/api/filters", map[string]any{"name": unique("Privé tableau de bord "), "query": "project = DEMO"}, 201, &private)
	t.Cleanup(func() { marie.do("DELETE", fmt.Sprintf("/api/filters/%d", private.ID), nil) })
	var own filterOut
	hugo.call("POST", "/api/filters", map[string]any{"name": unique("Mien tableau de bord "), "query": "project = DEMO"}, 201, &own)
	t.Cleanup(func() { hugo.do("DELETE", fmt.Sprintf("/api/filters/%d", own.ID), nil) })

	many := make([]map[string]any, 21)
	for i := range many {
		many[i] = map[string]any{"kind": "recent"}
	}
	for name, gadgets := range map[string]any{
		"unknown kind":      []map[string]any{{"kind": "burndown"}},
		"no filter":         []map[string]any{{"kind": "results"}},
		"someone's private": []map[string]any{{"kind": "results", "filter": private.ID}},
		"missing filter":    []map[string]any{{"kind": "count", "filter": 999999999, "config": map[string]any{"field": "status"}}},
		"count field":       []map[string]any{{"kind": "assigned"}, {"kind": "count", "filter": own.ID, "config": map[string]any{"field": "summary"}}},
		"size":              []map[string]any{{"kind": "recent", "config": map[string]any{"size": 51}}},
		"column":            []map[string]any{{"kind": "recent", "config": map[string]any{"columns": []string{"status", "nonsense"}}}},
		"too many":          many,
	} {
		if status, body := hugo.do("PUT", "/api/dashboard", map[string]any{"gadgets": gadgets}); status != 400 {
			t.Errorf("%s: %d %s", name, status, body)
		}
	}
	d := dashboard(hugo, "GET", nil)
	if d.Customised {
		t.Errorf("a refused save changed the dashboard: %+v", d)
	}
}

// A gadget on a filter shared then unshared says so and gives no query; it
// can still be moved, but not added again. Deleting the filter keeps it too.
func TestDashboardFilterGadgets(t *testing.T) {
	resetDashboard(t, "paul.martin")
	t.Cleanup(func() { resetDashboard(t, "paul.martin") })
	paul := login(t, "paul.martin", jirafixture.Password)
	marie := login(t, "marie.durand", jirafixture.Password)

	name := unique("Partagé tableau de bord ")
	var f filterOut
	marie.call("POST", "/api/filters", map[string]any{"name": name, "query": "project = DEMO",
		"shares": []map[string]any{{"kind": "user", "id": userID(t, "paul.martin")}}}, 201, &f)
	t.Cleanup(func() { marie.do("DELETE", fmt.Sprintf("/api/filters/%d", f.ID), nil) })

	d := dashboard(paul, "PUT", map[string]any{"gadgets": []map[string]any{
		{"kind": "assigned", "config": map[string]any{"size": 5}},
		{"kind": "results", "filter": f.ID, "config": map[string]any{"columns": []string{"status", "assignee"}, "size": 10}},
		{"kind": "count", "filter": f.ID, "config": map[string]any{"field": "status"}},
	}})
	if len(d.Gadgets) != 3 {
		t.Fatalf("saved: %+v", d)
	}
	res, count := d.Gadgets[1], d.Gadgets[2]
	if res.Kind != "results" || res.Filter == nil || res.Filter.Name != name || res.Filter.Query != "project = DEMO" ||
		res.Unavailable || res.Config.Size != 10 || len(res.Config.Columns) != 2 {
		t.Errorf("results gadget: %+v", res)
	}
	if count.Kind != "count" || count.Filter == nil || count.Config.Field != "status" || count.Unavailable {
		t.Errorf("count gadget: %+v", count)
	}

	unshare := func() {
		marie.call("PATCH", fmt.Sprintf("/api/filters/%d", f.ID), map[string]any{"shares": []any{}}, 200, nil)
	}
	unshare()
	d = dashboard(paul, "GET", nil)
	for _, g := range d.Gadgets[1:] {
		if !g.Unavailable || g.Filter != nil {
			t.Errorf("gadget on an unshared filter: %+v", g)
		}
	}
	if raw, _ := json.Marshal(d); strings.Contains(string(raw), "project = DEMO") {
		t.Errorf("the unshared filter's query leaks: %s", raw)
	}

	// Moved, the gadget keeps its filter, though unavailable.
	d = dashboard(paul, "PUT", map[string]any{"gadgets": []map[string]any{
		{"id": res.ID, "kind": "results", "filter": f.ID, "config": map[string]any{"size": 20}},
		{"id": d.Gadgets[0].ID, "kind": "assigned"},
	}})
	if len(d.Gadgets) != 2 || d.Gadgets[0].ID != res.ID || !d.Gadgets[0].Unavailable || d.Gadgets[0].Config.Size != 20 {
		t.Errorf("unavailable gadget moved: %+v", d)
	}
	// A new gadget on it is refused.
	if status, body := paul.do("PUT", "/api/dashboard", map[string]any{"gadgets": []map[string]any{
		{"id": res.ID, "kind": "results", "filter": f.ID},
		{"kind": "count", "filter": f.ID, "config": map[string]any{"field": "status"}},
	}}); status != 400 {
		t.Errorf("new gadget on an unshared filter: %d %s", status, body)
	}

	// Shared again, it shows again.
	marie.call("PATCH", fmt.Sprintf("/api/filters/%d", f.ID), map[string]any{
		"shares": []map[string]any{{"kind": "user", "id": userID(t, "paul.martin")}}}, 200, nil)
	d = dashboard(paul, "GET", nil)
	if g := d.Gadgets[0]; g.Unavailable || g.Filter == nil || g.Filter.ID != f.ID {
		t.Errorf("gadget on a filter shared again: %+v", g)
	}

	// Deleted, the filter leaves the gadget without one; it can still be kept.
	marie.call("DELETE", fmt.Sprintf("/api/filters/%d", f.ID), nil, 204, nil)
	d = dashboard(paul, "GET", nil)
	if g := d.Gadgets[0]; !g.Unavailable || g.Filter != nil {
		t.Errorf("gadget on a deleted filter: %+v", g)
	}
	d = dashboard(paul, "PUT", map[string]any{"gadgets": []map[string]any{
		{"id": d.Gadgets[1].ID, "kind": "assigned"},
		{"id": res.ID, "kind": "results", "filter": nil},
	}})
	if len(d.Gadgets) != 2 || d.Gadgets[1].ID != res.ID || !d.Gadgets[1].Unavailable {
		t.Errorf("gadget on a deleted filter, kept: %+v", d)
	}

	// Someone else's dashboard is untouched by all this.
	resetDashboard(t, "marie.durand")
	other := dashboard(marie, "GET", nil)
	if other.Customised {
		t.Errorf("marie.durand's dashboard: %+v", other)
	}
}
