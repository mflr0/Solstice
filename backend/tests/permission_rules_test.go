package tests

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"solstice/internal/jirafixture"
)

// TestPermissionsRoundTrip checks that putting back a project's rules,
// denies included, changes none of them, and that the denies apply.
func TestPermissionsRoundTrip(t *testing.T) {
	admin := login(t, "admin", "admin")
	key := newProject(t, admin, nil)
	projectID := pick(t, `SELECT id FROM projects WHERE pkey = ?`, key)

	// Paul may no longer be assigned issues here, through a project user deny
	// over the developpeurs' allow.
	if _, err := testDB.ExecContext(context.Background(), `INSERT INTO permission_rules
		(project_id, permission, effect, holder, user_id) VALUES (?, 'assignable', 'deny', 'user', ?)`,
		projectID, userID(t, "paul.martin")); err != nil {
		t.Fatal(err)
	}
	var before, after struct {
		Grants []grantOut `json:"grants"`
	}
	admin.get("/api/projects/"+key+"/permissions", &before)
	if !slices.ContainsFunc(before.Grants, func(g grantOut) bool { return g.Effect == "deny" }) {
		t.Fatalf("the deny is not listed: %+v", before.Grants)
	}
	admin.call("PUT", "/api/projects/"+key+"/permissions", map[string]any{"grants": before.Grants}, 200, nil)
	admin.get("/api/projects/"+key+"/permissions", &after)
	if !reflect.DeepEqual(before.Grants, after.Grants) {
		t.Errorf("grants changed by putting them back:\nbefore %+v\nafter  %+v", before.Grants, after.Grants)
	}
	if n := pick(t, `SELECT COUNT(*) FROM permission_rules WHERE project_id = ? AND effect = 'deny'`, projectID); n != "1" {
		t.Errorf("%s denies after the put, want 1", n)
	}

	var assignable []struct{ Username string }
	admin.get("/api/projects/"+key+"/assignable", &assignable)
	names := []string{}
	for _, u := range assignable {
		names = append(names, u.Username)
	}
	if slices.Contains(names, "paul.martin") || !slices.Contains(names, "jeanne.dupont") {
		t.Errorf("assignable = %v, want jeanne.dupont and not paul.martin", names)
	}

	admin.call("PUT", "/api/projects/"+key+"/permissions", map[string]any{"grants": []map[string]any{
		{"permission": "browse", "holder": "everyone", "effect": "maybe"}}}, 400, nil)
}

// ruleEnv is a project, and a user of a group of their own, none of which
// any other test uses.
type ruleEnv struct {
	admin, user *client
	key         string
	userID      int64
	groupID     int64
}

func newRuleEnv(t *testing.T) ruleEnv {
	t.Helper()
	admin := login(t, "admin", "admin")
	e := ruleEnv{admin: admin, key: newProject(t, admin, nil)}
	name := unique("rules")
	var u, g struct{ ID int64 }
	admin.call("POST", "/api/admin/users", map[string]any{"username": name, "name": "Règles " + name,
		"email": name + "@exemple.fr", "password": linkPassword}, 201, &u)
	admin.call("POST", "/api/admin/groups", map[string]any{"name": name}, 201, &g)
	e.userID, e.groupID = u.ID, g.ID
	t.Cleanup(func() {
		admin.do("DELETE", fmt.Sprintf("/api/admin/groups/%d", g.ID), nil)
		admin.do("PATCH", fmt.Sprintf("/api/admin/users/%d", u.ID), map[string]any{"active": false})
	})
	admin.call("POST", fmt.Sprintf("/api/admin/groups/%d/members", g.ID), map[string]any{"users": []int64{u.ID}}, 200, nil)
	e.user = login(t, name, linkPassword)
	return e
}

// addProjectRules adds rules to the project's.
func (e ruleEnv) addProjectRules(t *testing.T, rules ...grantOut) {
	t.Helper()
	var perms struct {
		Grants []grantOut `json:"grants"`
	}
	e.admin.get("/api/projects/"+e.key+"/permissions", &perms)
	e.admin.call("PUT", "/api/projects/"+e.key+"/permissions",
		map[string]any{"grants": append(perms.Grants, rules...)}, 200, nil)
}

// auditCount counts the entries of an action on a target.
func auditCount(t *testing.T, action, targetType, targetID string) int {
	t.Helper()
	n, _ := strconv.Atoi(pick(t, `SELECT COUNT(*) FROM audit_log WHERE action = ? AND target_type = ? AND target_id = ?`,
		action, targetType, targetID))
	return n
}

// A project deny for a user beats a global allow of a group they are in,
// and each write of global rules is audited.
func TestProjectDenyBeatsGlobalGroupAllow(t *testing.T) {
	e := newRuleEnv(t)
	groupPath := fmt.Sprintf("/api/admin/groups/%d/permissions", e.groupID)
	gid := strconv.FormatInt(e.groupID, 10)
	e.admin.call("PUT", groupPath, []map[string]any{{"permission": "browse"}, {"permission": "create", "effect": "allow"}}, 200, nil)
	if n := auditCount(t, "group.permissions", "group", gid); n != 1 {
		t.Errorf("%d group.permissions entries, want 1", n)
	}
	var rules []map[string]any
	e.admin.get(groupPath, &rules)
	want := []map[string]any{{"permission": "browse", "effect": "allow"}, {"permission": "create", "effect": "allow"}}
	if !reflect.DeepEqual(rules, want) {
		t.Errorf("group rules = %v, want %v", rules, want)
	}

	issue := map[string]any{"project": e.key, "type": standardType(t, e.admin, e.key), "summary": "Règles"}
	e.user.call("POST", "/api/issues", issue, 201, nil)
	before := auditCount(t, "project.permissions", "project", pick(t, `SELECT id FROM projects WHERE pkey = ?`, e.key))
	e.addProjectRules(t, grantOut{Permission: "create", Holder: "user", ID: e.userID, Effect: "deny"})
	if n := auditCount(t, "project.permissions", "project", pick(t, `SELECT id FROM projects WHERE pkey = ?`, e.key)); n != before+1 {
		t.Errorf("%d project.permissions entries, want %d", n, before+1)
	}
	e.user.call("POST", "/api/issues", issue, 403, nil)

	e.admin.call("PUT", groupPath, []map[string]any{{"permission": "browse"}, {"permission": "browse", "effect": "deny"}}, 400, nil)
	e.admin.call("PUT", groupPath, []map[string]any{{"permission": "fly"}}, 400, nil)
	e.admin.call("PUT", groupPath, []map[string]any{{"permission": "browse", "effect": "maybe"}}, 400, nil)
	e.admin.call("PUT", "/api/admin/groups/999999999/permissions", []any{}, 404, nil)
	e.admin.call("PUT", "/api/admin/builtin-groups/robots/permissions", []any{}, 404, nil)
	e.admin.call("PUT", groupPath, []any{}, 200, nil)
}

// Global rules are for global administrators: a project administrator can
// neither read nor write them, nor write one through the project's rules.
func TestGlobalRulesNeedGlobalAdmin(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password)
	key := newProject(t, admin, map[string]any{"lead": userID(t, "paul.martin")})
	paulID := userID(t, "paul.martin")
	for _, path := range []string{
		fmt.Sprintf("/api/admin/groups/%d/permissions", groupID(t, "developpeurs")),
		"/api/admin/builtin-groups/everyone/permissions",
		fmt.Sprintf("/api/admin/users/%d/permissions", paulID),
	} {
		paul.call("GET", path, nil, 403, nil)
		paul.call("PUT", path, []map[string]any{{"permission": "admin"}}, 403, nil)
	}
	paul.call("GET", fmt.Sprintf("/api/admin/users/%d/access?project=%s", paulID, key), nil, 403, nil)

	globals := pick(t, `SELECT COUNT(*) FROM permission_rules WHERE project_id IS NULL`)
	var perms struct {
		Grants []grantOut `json:"grants"`
	}
	paul.get("/api/projects/"+key+"/permissions", &perms)
	paul.call("PUT", "/api/projects/"+key+"/permissions", map[string]any{"grants": append(perms.Grants,
		grantOut{Permission: "delete", Holder: "user", ID: paulID, Effect: "deny"})}, 200, nil)
	paul.call("PUT", "/api/projects/"+key+"/permissions", map[string]any{"grants": perms.Grants, "project": nil}, 400, nil)
	if after := pick(t, `SELECT COUNT(*) FROM permission_rules WHERE project_id IS NULL`); after != globals {
		t.Errorf("%s global rules after the project admin's puts, want %s", after, globals)
	}
	if n := pick(t, `SELECT COUNT(*) FROM permission_rules r JOIN projects p ON p.id = r.project_id
		WHERE p.pkey = ? AND r.effect = 'deny' AND r.user_id = ?`, key, paulID); n != "1" {
		t.Errorf("%s project denies for paul, want 1", n)
	}
}

// /access names the level and the rule that decide each permission, and
// writes of built-in and user rules are audited.
func TestUserAccess(t *testing.T) {
	e := newRuleEnv(t)
	uid := strconv.FormatInt(e.userID, 10)
	userPath := fmt.Sprintf("/api/admin/users/%d/permissions", e.userID)
	t.Cleanup(func() { e.admin.do("PUT", userPath, []any{}) })
	e.admin.call("PUT", fmt.Sprintf("/api/admin/groups/%d/permissions", e.groupID),
		[]map[string]any{{"permission": "browse"}}, 200, nil)
	e.admin.call("PUT", userPath, []map[string]any{{"permission": "edit", "effect": "deny"}}, 200, nil)
	if n := auditCount(t, "user.permissions", "user", uid); n != 1 {
		t.Errorf("%d user.permissions entries, want 1", n)
	}
	e.addProjectRules(t,
		grantOut{Permission: "comment", Holder: "user", ID: e.userID},
		grantOut{Permission: "create", Holder: "group", ID: e.groupID, Effect: "deny"})

	// A built-in group's global rule, removed at once: it applies everywhere.
	const builtin = "/api/admin/builtin-groups/assignee/permissions"
	var kept []map[string]any
	e.admin.get(builtin, &kept)
	t.Cleanup(func() { e.admin.do("PUT", builtin, kept) })
	n := auditCount(t, "group.permissions", "group", "assignee")
	changed := slices.DeleteFunc(slices.Clone(kept), func(r map[string]any) bool { return r["permission"] == "watchers" })
	if len(changed) == len(kept) {
		changed = append(changed, map[string]any{"permission": "watchers", "effect": "allow"})
	}
	e.admin.call("PUT", builtin, changed, 200, nil)
	e.admin.call("PUT", builtin, kept, 200, nil)
	if got := auditCount(t, "group.permissions", "group", "assignee"); got != n+2 {
		t.Errorf("%d group.permissions entries for assignee, want %d", got, n+2)
	}

	type source struct {
		Kind string `json:"kind"`
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	var access []struct {
		Permission string  `json:"permission"`
		Granted    bool    `json:"granted"`
		Level      string  `json:"level"`
		Effect     *string `json:"effect"`
		Source     *source `json:"source"`
	}
	e.admin.call("GET", fmt.Sprintf("/api/admin/users/%d/access?project=%s", e.userID, e.key), nil, 200, &access)
	byPerm := map[string]int{}
	for i, a := range access {
		byPerm[a.Permission] = i
	}
	group := pick(t, `SELECT name FROM user_groups WHERE id = ?`, e.groupID)
	name := pick(t, `SELECT display_name FROM users WHERE id = ?`, e.userID)
	for _, c := range []struct {
		perm, level, effect string
		granted             bool
		source              *source
	}{
		{"comment", "projectUser", "allow", true, &source{"user", e.userID, name}},
		{"edit", "globalUser", "deny", false, &source{"user", e.userID, name}},
		{"create", "projectGroup", "deny", false, &source{"group", e.groupID, group}},
		{"browse", "globalGroup", "allow", true, &source{"group", e.groupID, group}},
		{"security", "none", "", false, nil},
	} {
		i, ok := byPerm[c.perm]
		if !ok {
			t.Errorf("%s missing from %+v", c.perm, access)
			continue
		}
		a := access[i]
		effect := ""
		if a.Effect != nil {
			effect = *a.Effect
		}
		if a.Granted != c.granted || a.Level != c.level || effect != c.effect || !reflect.DeepEqual(a.Source, c.source) {
			t.Errorf("%s: %+v (source %+v), want %v %s %s %+v", c.perm, a, a.Source, c.granted, c.level, c.effect, c.source)
		}
	}
	if len(access) != len(byPerm) || len(access) < 15 {
		t.Errorf("%d permissions answered", len(access))
	}

	e.admin.call("GET", fmt.Sprintf("/api/admin/users/%d/access", e.userID), nil, 400, nil)
	e.admin.call("GET", fmt.Sprintf("/api/admin/users/%d/access?project=NOPE9", e.userID), nil, 404, nil)
}

// The project's rules come with what they inherit: the global rules of the
// groups, and for each user with a project rule, their global rules and the
// groups they are in there.
func TestProjectPermissionsInherited(t *testing.T) {
	e := newRuleEnv(t)
	groupPath := fmt.Sprintf("/api/admin/groups/%d/permissions", e.groupID)
	userPath := fmt.Sprintf("/api/admin/users/%d/permissions", e.userID)
	t.Cleanup(func() { e.admin.do("PUT", userPath, []any{}) })
	e.admin.call("PUT", groupPath, []map[string]any{{"permission": "edit"}}, 200, nil)
	e.admin.call("PUT", userPath, []map[string]any{{"permission": "delete", "effect": "deny"}}, 200, nil)
	e.addProjectRules(t, grantOut{Permission: "comment", Holder: "user", ID: e.userID})

	type user struct {
		ID     int64               `json:"id"`
		Name   string              `json:"name"`
		Groups []string            `json:"groups"`
		Rules  []map[string]string `json:"rules"`
	}
	var perms struct {
		Globals []grantOut `json:"globals"`
		Users   []user     `json:"users"`
	}
	e.admin.get("/api/projects/"+e.key+"/permissions", &perms)
	if !slices.Contains(perms.Globals, grantOut{Permission: "edit", Holder: "group", ID: e.groupID, Effect: "allow"}) {
		t.Errorf("the group's global rule is missing from %+v", perms.Globals)
	}
	if slices.ContainsFunc(perms.Globals, func(g grantOut) bool { return g.Holder == "user" }) {
		t.Errorf("globals list a user rule: %+v", perms.Globals)
	}
	i := slices.IndexFunc(perms.Users, func(u user) bool { return u.ID == e.userID })
	if i < 0 {
		t.Fatalf("the user with a project rule is missing from %+v", perms.Users)
	}
	u := perms.Users[i]
	if !slices.Contains(u.Groups, "builtin:everyone") || !slices.Contains(u.Groups, fmt.Sprintf("group:%d", e.groupID)) ||
		slices.Contains(u.Groups, "builtin:lead") {
		t.Errorf("groups = %v, want everyone and group:%d, not lead", u.Groups, e.groupID)
	}
	if want := []map[string]string{{"permission": "delete", "effect": "deny"}}; !reflect.DeepEqual(u.Rules, want) {
		t.Errorf("rules = %v, want %v", u.Rules, want)
	}

	var one user
	e.admin.get(fmt.Sprintf("/api/projects/%s/permissions/users/%d", e.key, e.userID), &one)
	if !reflect.DeepEqual(one, u) {
		t.Errorf("one user = %+v, want %+v", one, u)
	}
	e.admin.call("GET", fmt.Sprintf("/api/projects/%s/permissions/users/999999999", e.key), nil, 404, nil)
	e.user.call("GET", fmt.Sprintf("/api/projects/%s/permissions/users/%d", e.key, e.userID), nil, 404, nil)

	// The project's lead administers it without being a global administrator.
	lead := login(t, "paul.martin", jirafixture.Password)
	key := newProject(t, e.admin, map[string]any{"lead": userID(t, "paul.martin")})
	var mine user
	lead.get(fmt.Sprintf("/api/projects/%s/permissions/users/%d", key, userID(t, "paul.martin")), &mine)
	if !slices.Contains(mine.Groups, "builtin:lead") {
		t.Errorf("the lead's groups = %v, want builtin:lead", mine.Groups)
	}
}
