package tests

// Tests of a filter's columns and of folded gadgets.
// They need SOLSTICE_TEST_DB and are skipped without it.

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
)

type filterColumnsOut struct {
	ID      int64    `json:"id"`
	Columns []string `json:"columns"`
}

func TestFilterColumns(t *testing.T) {
	marie := login(t, "marie.durand", jirafixture.Password)
	read := func(id int64) []string {
		t.Helper()
		var list []filterColumnsOut
		marie.call("GET", "/api/filters", nil, 200, &list)
		for _, f := range list {
			if f.ID == id {
				return f.Columns
			}
		}
		t.Fatalf("filter %d not listed", id)
		return nil
	}

	var f filterColumnsOut
	marie.call("POST", "/api/filters", map[string]any{"name": unique("Colonnes "), "query": "project = DEMO",
		"columns": []string{"status", "key", "assignee", "summary"}}, 201, &f)
	t.Cleanup(func() { marie.do("DELETE", fmt.Sprintf("/api/filters/%d", f.ID), nil) })
	// The key and the summary are always shown, so they aren't kept.
	if want := []string{"status", "assignee"}; !slices.Equal(f.Columns, want) || !slices.Equal(read(f.ID), want) {
		t.Errorf("created with columns: %q, read back %q", f.Columns, read(f.ID))
	}

	// A patch without columns keeps them.
	marie.call("PATCH", fmt.Sprintf("/api/filters/%d", f.ID), map[string]any{"description": "d"}, 200, nil)
	if got := read(f.ID); !slices.Equal(got, []string{"status", "assignee"}) {
		t.Errorf("columns after a patch without them: %q", got)
	}
	marie.call("PATCH", fmt.Sprintf("/api/filters/%d", f.ID), map[string]any{"columns": []string{"priority"}}, 200, nil)
	if got := read(f.ID); !slices.Equal(got, []string{"priority"}) {
		t.Errorf("columns patched: %q", got)
	}

	for name, cols := range map[string]any{
		"unknown":  []string{"status", "nonsense"},
		"too many": slices.Repeat([]string{"status"}, 31),
		"not ids":  "status",
	} {
		if status, body := marie.do("PATCH", fmt.Sprintf("/api/filters/%d", f.ID), map[string]any{"columns": cols}); status != 400 {
			t.Errorf("%s columns: %d %s", name, status, body)
		}
	}
	if status, body := marie.do("POST", "/api/filters", map[string]any{"name": unique("Colonnes "), "query": "project = DEMO",
		"columns": []string{"nonsense"}}); status != 400 {
		t.Errorf("filter created with an unknown column: %d %s", status, body)
	}
	if got := read(f.ID); !slices.Equal(got, []string{"priority"}) {
		t.Errorf("columns after refused patches: %q", got)
	}

	// null goes back to the default columns.
	marie.call("PATCH", fmt.Sprintf("/api/filters/%d", f.ID), map[string]any{"columns": nil}, 200, nil)
	if got := read(f.ID); got != nil {
		t.Errorf("columns reset: %q", got)
	}
}

func TestDashboardCollapsed(t *testing.T) {
	resetDashboard(t, "hugo.robert")
	t.Cleanup(func() { resetDashboard(t, "hugo.robert") })
	hugo := login(t, "hugo.robert", jirafixture.Password)

	dashboard(hugo, "PUT", map[string]any{"gadgets": []map[string]any{
		{"kind": "assigned", "config": map[string]any{"size": 5, "collapsed": true}},
		{"kind": "recent"},
	}})
	d := dashboard(hugo, "GET", nil)
	if len(d.Gadgets) != 2 || !d.Gadgets[0].Config.Collapsed || d.Gadgets[1].Config.Collapsed {
		t.Errorf("collapsed gadget read back: %+v", d)
	}
}

// Migration 0028 gives a filter the columns of its owner's first results
// gadget on it that has some, and leaves the gadgets alone.
func TestMigrationFilterColumns(t *testing.T) {
	body, err := os.ReadFile("../internal/db/migrations/0028_filter_columns.sql")
	if err != nil {
		t.Fatal(err)
	}
	var update string
	for _, stmt := range strings.Split(string(body), ";\n") {
		if s := strings.TrimSpace(stmt); strings.HasPrefix(s, "UPDATE") {
			update = s
		}
	}
	if update == "" {
		t.Fatal("no UPDATE in the migration")
	}

	for _, u := range []string{"marie.durand", "paul.martin"} {
		resetDashboard(t, u)
	}
	t.Cleanup(func() {
		for _, u := range []string{"marie.durand", "paul.martin"} {
			resetDashboard(t, u)
		}
	})
	marie := login(t, "marie.durand", jirafixture.Password)
	paul := login(t, "paul.martin", jirafixture.Password)
	newFilter := func(c *client) int64 {
		var f filterOut
		c.call("POST", "/api/filters", map[string]any{"name": unique("Migration colonnes "), "query": "project = DEMO",
			"shares": []map[string]any{{"kind": "everyone"}}}, 201, &f)
		t.Cleanup(func() { c.do("DELETE", fmt.Sprintf("/api/filters/%d", f.ID), nil) })
		return f.ID
	}
	shared, others := newFilter(marie), newFilter(marie)

	dashboard(marie, "PUT", map[string]any{"gadgets": []map[string]any{
		{"kind": "results", "filter": shared},
		{"kind": "count", "filter": shared, "config": map[string]any{"field": "status"}},
		{"kind": "results", "filter": shared, "config": map[string]any{"columns": []string{"status", "assignee"}}},
		{"kind": "results", "filter": shared, "config": map[string]any{"columns": []string{"priority"}}},
	}})
	paulBefore := dashboard(paul, "PUT", map[string]any{"gadgets": []map[string]any{
		{"kind": "results", "filter": shared, "config": map[string]any{"columns": []string{"created"}}},
		{"kind": "results", "filter": others, "config": map[string]any{"columns": []string{"reporter"}}},
	}})
	marieBefore := dashboard(marie, "GET", nil)

	if _, err := testDB.Exec(update); err != nil {
		t.Fatal(err)
	}

	var list []filterColumnsOut
	marie.call("GET", "/api/filters", nil, 200, &list)
	got := map[int64][]string{}
	for _, f := range list {
		got[f.ID] = f.Columns
	}
	if !slices.Equal(got[shared], []string{"status", "assignee"}) {
		t.Errorf("columns of the owner's filter: %q", got[shared])
	}
	if got[others] != nil {
		t.Errorf("columns of a filter only another user has a gadget on: %q", got[others])
	}
	for name, c := range map[string]struct {
		cl     *client
		before dashboardOut
	}{"marie.durand": {marie, marieBefore}, "paul.martin": {paul, paulBefore}} {
		before, _ := json.Marshal(c.before)
		after, _ := json.Marshal(dashboard(c.cl, "GET", nil))
		if string(after) != string(before) {
			t.Errorf("%s's dashboard changed:\n%s\n%s", name, before, after)
		}
	}
}
