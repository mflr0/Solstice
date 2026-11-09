package tests

// Tests of the personal dashboard (GET and PUT /api/dashboard).
// They need SOLSTICE_TEST_DB and are skipped without it.

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
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
		Size int `json:"size"`
		Sort *struct {
			Field string `json:"field"`
			Desc  bool   `json:"desc"`
		} `json:"sort"`
		Collapsed bool `json:"collapsed"`
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

// rawGadget is a dashboard_gadgets row, for kinds and settings the API no longer takes.
type rawGadget struct {
	kind   string
	filter int64
	config string
}

// insertGadgets gives the user a dashboard of these gadgets, bypassing the API.
func insertGadgets(t *testing.T, username string, gadgets ...rawGadget) {
	t.Helper()
	id := userID(t, username)
	if _, err := testDB.Exec(`INSERT INTO dashboards (user_id, updated_at) VALUES (?, NOW())`, id); err != nil {
		t.Fatal(err)
	}
	for i, g := range gadgets {
		if _, err := testDB.Exec(`INSERT INTO dashboard_gadgets (user_id, position, kind, filter_id, config) VALUES (?, ?, ?, NULLIF(?, 0), ?)`,
			id, i, g.kind, g.filter, g.config); err != nil {
			t.Fatal(err)
		}
	}
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

// The ready-made views beyond the defaults are built-in kinds too, with no
// filter, and the queries the interface gives them run.
func TestDashboardWatchingAndDue(t *testing.T) {
	resetDashboard(t, "hugo.robert")
	t.Cleanup(func() { resetDashboard(t, "hugo.robert") })
	hugo := login(t, "hugo.robert", jirafixture.Password)

	d := dashboard(hugo, "PUT", map[string]any{"gadgets": []map[string]any{
		{"kind": "watching", "filter": 999999999, "config": map[string]any{"size": 5}},
		{"kind": "due", "config": map[string]any{"sort": map[string]any{"field": "due", "desc": false}}},
	}})
	if len(d.Gadgets) != 2 || d.Gadgets[0].Kind != "watching" || d.Gadgets[0].Filter != nil || d.Gadgets[0].Config.Size != 5 ||
		d.Gadgets[1].Kind != "due" || d.Gadgets[1].Config.Sort == nil {
		t.Fatalf("saved: %+v", d)
	}

	// As frontend/src/core/dashboard.ts writes them.
	for _, q := range []string{
		"watcher = currentUser() ORDER BY updated DESC",
		"statusCategory != Done AND due >= startOfDay() AND due <= 7d ORDER BY due ASC",
	} {
		if status, body := hugo.do("GET", "/api/issues?jql="+url.QueryEscape(q), nil); status != 200 {
			t.Errorf("%s: %d %s", q, status, body)
		}
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
		"missing filter":    []map[string]any{{"kind": "results", "filter": 999999999}},
		"count":             []map[string]any{{"kind": "assigned"}, {"kind": "count", "filter": own.ID}},
		"size":              []map[string]any{{"kind": "recent", "config": map[string]any{"size": 51}}},
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

// A list gadget keeps the sort saved on it; one ORDER BY can't take is dropped.
func TestDashboardSort(t *testing.T) {
	resetDashboard(t, "hugo.robert")
	t.Cleanup(func() { resetDashboard(t, "hugo.robert") })
	hugo := login(t, "hugo.robert", jirafixture.Password)
	var own filterOut
	hugo.call("POST", "/api/filters", map[string]any{"name": unique("Tri tableau de bord "), "query": "project = DEMO ORDER BY created DESC"}, 201, &own)
	t.Cleanup(func() { hugo.do("DELETE", fmt.Sprintf("/api/filters/%d", own.ID), nil) })
	var cf int64
	if err := testDB.QueryRow(`SELECT MIN(id) FROM custom_fields`).Scan(&cf); err != nil {
		t.Fatal(err)
	}
	custom := fmt.Sprintf("cf[%d]", cf)

	sort := func(field string, desc bool) map[string]any {
		return map[string]any{"sort": map[string]any{"field": field, "desc": desc}}
	}
	d := dashboard(hugo, "PUT", map[string]any{"gadgets": []map[string]any{
		{"kind": "assigned", "config": sort("updated", false)},
		{"kind": "results", "filter": own.ID, "config": sort(custom, true)},
		{"kind": "recent", "config": sort("nonsense", true)},
		{"kind": "reported", "config": sort("cf[999999999]", true)},
	}})
	read := dashboard(hugo, "GET", nil)
	for i, g := range read.Gadgets {
		got := "none"
		if s := g.Config.Sort; s != nil {
			got = fmt.Sprintf("%s %v", s.Field, s.Desc)
		}
		want := []string{"updated false", custom + " true", "none", "none"}[i]
		if got != want {
			t.Errorf("gadget %d (%s): sort %s, want %s", i, g.Kind, got, want)
		}
	}
	if len(d.Gadgets) != 4 {
		t.Fatalf("saved dashboard: %+v", d)
	}

	if status, body := hugo.do("PUT", "/api/dashboard", map[string]any{"gadgets": []map[string]any{
		{"kind": "recent", "config": map[string]any{"sort": map[string]any{"field": "updated", "desc": "yes"}}},
	}}); status != 400 {
		t.Errorf("unreadable direction: %d %s", status, body)
	}

	// The saved filter keeps its own order.
	if f := read.Gadgets[1].Filter; f == nil || !strings.HasSuffix(f.Query, "ORDER BY created DESC") {
		t.Errorf("filter changed: %+v", f)
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
		{"kind": "results", "filter": f.ID, "config": map[string]any{"size": 10}},
		{"kind": "results", "filter": f.ID, "config": map[string]any{"collapsed": true}},
	}})
	if len(d.Gadgets) != 3 {
		t.Fatalf("saved: %+v", d)
	}
	res, folded := d.Gadgets[1], d.Gadgets[2]
	if res.Kind != "results" || res.Filter == nil || res.Filter.Name != name || res.Filter.Query != "project = DEMO" ||
		res.Unavailable || res.Config.Size != 10 {
		t.Errorf("results gadget: %+v", res)
	}
	if folded.Filter == nil || folded.Filter.ID != f.ID || !folded.Config.Collapsed || folded.Unavailable {
		t.Errorf("second gadget on the filter: %+v", folded)
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

	// Moved without naming a filter, the gadget keeps its own, though unavailable.
	d = dashboard(paul, "PUT", map[string]any{"gadgets": []map[string]any{
		{"id": res.ID, "kind": "results", "config": map[string]any{"size": 20}},
		{"id": d.Gadgets[0].ID, "kind": "assigned"},
	}})
	if len(d.Gadgets) != 2 || d.Gadgets[0].ID != res.ID || !d.Gadgets[0].Unavailable || d.Gadgets[0].Config.Size != 20 {
		t.Errorf("unavailable gadget moved: %+v", d)
	}
	// A new gadget on it is refused.
	if status, body := paul.do("PUT", "/api/dashboard", map[string]any{"gadgets": []map[string]any{
		{"id": res.ID, "kind": "results"},
		{"kind": "results", "filter": f.ID},
	}}); status != 400 {
		t.Errorf("new gadget on an unshared filter: %d %s", status, body)
	}
	// So is pointing the built-in gadget at it.
	if status, body := paul.do("PUT", "/api/dashboard", map[string]any{"gadgets": []map[string]any{
		{"id": d.Gadgets[1].ID, "kind": "results", "filter": f.ID},
	}}); status != 400 {
		t.Errorf("built-in gadget turned to an unshared filter: %d %s", status, body)
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

// Migration 0029 turns a count gadget into a list of the same filter, in its
// place, and leaves the other gadgets alone.
func TestMigrationCountGadgets(t *testing.T) {
	body, err := os.ReadFile("../internal/db/migrations/0029_count_gadgets.sql")
	if err != nil {
		t.Fatal(err)
	}
	resetDashboard(t, "hugo.robert")
	t.Cleanup(func() { resetDashboard(t, "hugo.robert") })
	hugo := login(t, "hugo.robert", jirafixture.Password)
	var f filterOut
	hugo.call("POST", "/api/filters", map[string]any{"name": unique("Migration décompte "), "query": "project = DEMO"}, 201, &f)
	t.Cleanup(func() { hugo.do("DELETE", fmt.Sprintf("/api/filters/%d", f.ID), nil) })
	insertGadgets(t, "hugo.robert",
		rawGadget{kind: "recent", config: `{"size":12}`},
		rawGadget{kind: "count", filter: f.ID, config: `{"field":"status","collapsed":true}`},
		rawGadget{kind: "results", filter: f.ID, config: `{"size":5,"columns":["status"]}`},
	)
	before := dashboard(hugo, "GET", nil)

	if _, err := testDB.Exec(string(body)); err != nil {
		t.Fatal(err)
	}

	d := dashboard(hugo, "GET", nil)
	if len(d.Gadgets) != 3 {
		t.Fatalf("dashboard after the migration: %+v", d)
	}
	if g := d.Gadgets[1]; g.ID != before.Gadgets[1].ID || g.Kind != "results" || g.Filter == nil || g.Filter.ID != f.ID ||
		g.Unavailable || !g.Config.Collapsed {
		t.Errorf("count gadget after the migration: %+v", g)
	}
	for _, i := range []int{0, 2} {
		was, _ := json.Marshal(before.Gadgets[i])
		now, _ := json.Marshal(d.Gadgets[i])
		if string(was) != string(now) {
			t.Errorf("gadget %d changed:\n%s\n%s", i, was, now)
		}
	}
	// It still saves as it reads.
	var in []map[string]any
	for _, g := range d.Gadgets {
		in = append(in, map[string]any{"id": g.ID, "kind": g.Kind})
	}
	dashboard(hugo, "PUT", map[string]any{"gadgets": in})
}
