package tests

import (
	"slices"
	"testing"
)

// An edit that adds and removes several components and versions writes their
// history lines in id order, whatever the order of the request.
func TestIssueSetHistoryOrder(t *testing.T) {
	admin := login(t, "admin", "admin")
	key := newProject(t, admin, nil)

	// Created so that the names sort against the ids.
	names := []string{"Zeta", "Mu", "Kappa", "Alpha", "Epsilon"}
	sets := []struct{ field, path, line string }{
		{"components", "components", "Component"},
		{"fixVersions", "versions", "Fix Version"},
		{"affectsVersions", "versions", "Version"},
	}
	ids := map[string][]int64{}
	for _, s := range sets {
		if ids[s.path] != nil {
			continue
		}
		for _, n := range names {
			var c struct{ ID int64 }
			admin.call("POST", "/api/projects/"+key+"/"+s.path, map[string]any{"name": n}, 201, &c)
			ids[s.path] = append(ids[s.path], c.ID)
		}
	}

	var created struct{ Key string }
	admin.call("POST", "/api/issues", map[string]any{"project": key, "type": standardType(t, admin, key),
		"summary": "Ensembles"}, 201, &created)

	edit := func(order []int) {
		t.Helper()
		in := map[string]any{}
		for _, s := range sets {
			var v []int64
			for _, i := range order {
				v = append(v, ids[s.path][i])
			}
			in[s.field] = v
		}
		admin.call("PATCH", "/api/issues/"+created.Key, in, 200, nil)
	}
	type line struct{ from, to string }
	last := func(field string) []line {
		t.Helper()
		var history []historyEntry
		admin.get("/api/issues/"+created.Key+"/history", &history)
		var out []line
		for _, it := range history[len(history)-1].Items {
			if it.Field != field {
				continue
			}
			var l line
			if it.From != nil {
				l.from = *it.From
			}
			if it.To != nil {
				l.to = *it.To
			}
			out = append(out, l)
		}
		return out
	}

	edit([]int{3, 0, 2})
	for _, s := range sets {
		if got, want := last(s.line), []line{{to: "Zeta"}, {to: "Kappa"}, {to: "Alpha"}}; !slices.Equal(got, want) {
			t.Errorf("%s added: %+v, want %+v", s.field, got, want)
		}
	}
	edit([]int{4, 2, 1})
	for _, s := range sets {
		if got, want := last(s.line), []line{{from: "Zeta"}, {from: "Alpha"}, {to: "Mu"}, {to: "Epsilon"}}; !slices.Equal(got, want) {
			t.Errorf("%s replaced: %+v, want %+v", s.field, got, want)
		}
	}
}
