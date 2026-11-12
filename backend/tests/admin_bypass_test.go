package tests

import (
	"net/url"
	"slices"
	"testing"

	"solstice/internal/jirafixture"
)

// A global administrator opens and searches a project they created for a
// group they are not in, with another lead, without becoming assignable or a
// member of it.
func TestAdministratorSeesUngrantedProject(t *testing.T) {
	admin := login(t, "admin", "admin")
	chloe := login(t, "chloe.simon", jirafixture.Password) // in "rh"
	if n := pick(t, `SELECT COUNT(*) FROM group_members m JOIN user_groups g ON g.id = m.group_id
		JOIN users u ON u.id = m.user_id WHERE g.name = 'rh' AND u.username = 'admin'`); n != "0" {
		t.Fatal("admin is in the rh group")
	}
	key := newProject(t, admin, map[string]any{"lead": userID(t, "chloe.simon"),
		"access": map[string]any{"holder": "group", "id": groupID(t, "rh")}})
	createIssue(t, chloe, key, "check admin bypass", "")

	admin.call("GET", "/api/projects/"+key, nil, 200, nil)

	type listed struct {
		Key    string `json:"key"`
		Member bool   `json:"member"`
	}
	member := func(c *client) (found, member bool) {
		var list []listed
		c.call("GET", "/api/projects", nil, 200, &list)
		i := slices.IndexFunc(list, func(p listed) bool { return p.Key == key })
		return i >= 0, i >= 0 && list[i].Member
	}
	if found, m := member(admin); !found || m {
		t.Errorf("admin's project list: found %v, member %v; want true, false", found, m)
	}
	if found, m := member(chloe); !found || !m {
		t.Errorf("rh member's project list: found %v, member %v; want true, true", found, m)
	}

	var res struct{ Total int }
	admin.call("GET", "/api/issues?jql="+url.QueryEscape("project = "+key), nil, 200, &res)
	if res.Total != 1 {
		t.Errorf("admin's JQL search finds %d issues, want 1", res.Total)
	}

	var users []userRef
	admin.call("GET", "/api/projects/"+key+"/assignable", nil, 200, &users)
	if slices.ContainsFunc(users, func(u userRef) bool { return u.Username == "admin" }) {
		t.Error("admin is assignable in a project with no grant for them")
	}
}
