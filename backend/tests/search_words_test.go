package tests

// The people, group and audit searches: a row is kept when every word of the
// query matches one of its columns, in any order, with % and _ literal.

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
)

// wordQueries are the queries every search answers for a row named
// "<first> <last>", first having four letters or more, and whether each finds it.
func wordQueries(first, last string) map[string]bool {
	return map[string]bool{
		last + " " + first:                       true,  // words in reverse order
		strings.ToUpper(first) + " " + last:      true,  // case as today
		last + " bob":                            false, // a word matching no column
		last + " " + first[:1] + "%" + first[4:]: false, // % is literal
		last + " " + first[:2] + "_" + first[3:]: false, // _ is literal
		"  " + last + "  " + first + " ":         true,  // spaces around the words
	}
}

func TestSearchesMatchEveryWord(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password)
	last := unique("smith")
	var created struct{ ID int64 }
	admin.call("POST", "/api/admin/users", map[string]any{"username": unique("mots."), "name": "Alice " + last,
		"password": "une phrase assez longue", "groups": []int64{groupID(t, "developpeurs")}}, 201, &created)
	// Other tests count the members of developpeurs.
	t.Cleanup(func() {
		admin.do("PUT", fmt.Sprintf("/api/admin/users/%d/groups", created.ID), map[string]any{"groups": []int64{}})
	})
	key := newProject(t, admin, nil) // the new account is assignable there through developpeurs
	var group struct{ ID int64 }
	admin.call("POST", "/api/admin/groups", map[string]any{"name": "Alice " + last}, 201, &group)
	t.Cleanup(func() { admin.do("DELETE", fmt.Sprintf("/api/admin/groups/%d", group.ID), nil) })

	found := func(c *client, path string, id int64) bool {
		t.Helper()
		var rows []struct{ ID int64 }
		c.call("GET", path, nil, 200, &rows)
		for _, r := range rows {
			if r.ID == id {
				return true
			}
		}
		return false
	}
	for q, want := range wordQueries("alice", last) {
		v := url.QueryEscape(q)
		for _, tc := range []struct {
			name string
			got  func() bool
		}{
			{"admin users", func() bool { return found(admin, "/api/admin/users?q="+v, created.ID) }},
			{"user picker", func() bool { return found(paul, "/api/users?q="+v, created.ID) }},
			{"assignable", func() bool { return found(paul, "/api/projects/"+key+"/assignable?q="+v, created.ID) }},
			{"groups", func() bool { return found(admin, "/api/admin/groups?q="+v, group.ID) }},
			{"audit", func() bool {
				var page struct{ Total int }
				admin.call("GET", "/api/admin/audit?action=user.create&q="+v, nil, 200, &page)
				return page.Total == 1
			}},
		} {
			if got := tc.got(); got != want {
				t.Errorf("%s, q=%q: found %v, want %v", tc.name, q, got, want)
			}
		}
	}
}
