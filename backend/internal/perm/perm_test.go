package perm

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

// testRules builds an empty rule set. Tests fill only the grants they need.
func testRules() *rules {
	return &rules{
		grants:   map[int64]map[string][]Grant{},
		leads:    map[int64]int64{},
		archived: map[int64]bool{},
		levels:   map[int64]level{},
	}
}

// grant adds one grant for a permission in a project.
func (r *rules) grant(project int64, p string, g Grant) {
	if r.grants[project] == nil {
		r.grants[project] = map[string][]Grant{}
	}
	r.grants[project][p] = append(r.grants[project][p], g)
}

// subject builds a subject on the rules with no groups or roles.
func subject(r *rules, userID int64, admin bool) *Subject {
	return &Subject{
		UserID:     userID,
		Admin:      admin,
		rules:      r,
		groups:     map[int64]bool{},
		groupNames: map[string]bool{},
		roles:      map[int64]map[int64]bool{},
	}
}

// addGroup puts the subject in a group, with the name used by restricted comments.
func addGroup(s *Subject, id int64, name string) {
	s.groups[id] = true
	if name != "" {
		s.groupNames[name] = true
	}
}

// addRole gives the subject a role in a project.
func addRole(s *Subject, project, role int64) {
	if s.roles[project] == nil {
		s.roles[project] = map[int64]bool{}
	}
	s.roles[project][role] = true
}

// TestCanGrantSources checks that a permission is honoured for each kind of
// holder, and only for the holder it names.
func TestCanGrantSources(t *testing.T) {
	const p, q int64 = 7, 8
	r := testRules()
	r.grant(p, Browse, Grant{Holder: Everyone})
	r.grant(p, Create, Grant{Holder: User, UserID: 42})
	r.grant(p, Edit, Grant{Holder: Group, GroupID: 3})
	r.grant(p, Comment, Grant{Holder: Role, RoleID: 5})
	r.grant(q, Comment, Grant{Holder: Role, RoleID: 5})
	r.grant(p, Transition, Grant{Holder: Lead})
	r.grant(p, Delete, Grant{Holder: "widget"})
	r.leads[p] = 9

	user := subject(r, 42, false)
	member := subject(r, 43, false)
	addGroup(member, 3, "developpeurs")
	roleHolder := subject(r, 44, false)
	addRole(roleHolder, p, 5)
	lead := subject(r, 9, false)
	stranger := subject(r, 45, false)

	cases := []struct {
		name    string
		sub     *Subject
		perm    string
		project int64
		want    bool
	}{
		{"everyone grant matches any user", stranger, Browse, p, true},
		{"direct user grant matches the user", user, Create, p, true},
		{"direct user grant does not match another user", stranger, Create, p, false},
		{"group grant matches a member", member, Edit, p, true},
		{"group grant does not match a non-member", stranger, Edit, p, false},
		{"role grant matches a holder", roleHolder, Comment, p, true},
		{"role grant does not match a different project", roleHolder, Comment, q, false},
		{"lead grant matches the project lead", lead, Transition, p, true},
		{"lead grant does not match a non-lead", stranger, Transition, p, false},
		{"an unknown holder kind matches nobody", stranger, Delete, p, false},
		{"permission not granted is refused", stranger, Moderate, p, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.sub.Can(c.perm, c.project, nil); got != c.want {
				t.Errorf("Can(%q, %d) = %v, want %v", c.perm, c.project, got, c.want)
			}
		})
	}
}

// TestCanRequiresBrowseOnIssues checks that acting on an issue also needs the
// browse permission, while a project-wide check does not.
func TestCanRequiresBrowseOnIssues(t *testing.T) {
	const p int64 = 7
	r := testRules()
	r.grant(p, Edit, Grant{Holder: User, UserID: 42})
	is := &Issue{ProjectID: p, ReporterID: 1, AssigneeID: 2}
	sub := subject(r, 42, false)

	if !sub.Can(Edit, p, nil) {
		t.Error("edit at project level: got false, want true")
	}
	if sub.Can(Edit, p, is) {
		t.Error("edit on an issue without browse: got true, want false")
	}
	r.grant(p, Browse, Grant{Holder: Everyone})
	if !sub.Can(Edit, p, is) {
		t.Error("edit on an issue with browse: got false, want true")
	}
	if !sub.Can(Browse, p, is) {
		t.Error("browse on an issue with browse: got false, want true")
	}
}

// TestEveryPermissionConstantIsIndependent grants each permission of the
// package in project A and checks that it does not leak to project B.
func TestEveryPermissionConstantIsIndependent(t *testing.T) {
	for _, p := range All {
		t.Run(p, func(t *testing.T) {
			const a, b int64 = 10, 11
			r := testRules()
			r.grant(a, p, Grant{Holder: User, UserID: 1})
			sub := subject(r, 1, false)
			if !sub.Can(p, a, nil) {
				t.Errorf("granted %q in project A: got false", p)
			}
			if sub.Can(p, b, nil) {
				t.Errorf("grant of %q in project A leaked to project B", p)
			}
		})
	}
}

// TestArchivedProjectIsReadOnly checks that an archived project keeps browse
// and refuses every other action.
func TestArchivedProjectIsReadOnly(t *testing.T) {
	const p int64 = 7
	r := testRules()
	r.archived[p] = true
	for _, perm := range All {
		r.grant(p, perm, Grant{Holder: Everyone})
	}
	sub := subject(r, 42, false)

	if !sub.Archived(p) {
		t.Error("Archived: got false, want true")
	}
	if !sub.Can(Browse, p, nil) {
		t.Error("browse on an archived project: got false, want true")
	}
	is := &Issue{ProjectID: p}
	for _, perm := range All {
		if perm == Browse {
			continue
		}
		if sub.Can(perm, p, is) {
			t.Errorf("%q on an archived project: got true, want false", perm)
		}
	}
}

// TestAdministratorHasNoImplicitAccess checks that the admin flag grants
// nothing by itself and does not bypass a security level.
func TestAdministratorHasNoImplicitAccess(t *testing.T) {
	const p int64 = 7
	r := testRules()
	admin := subject(r, 1, true)
	if admin.Can(Browse, p, nil) {
		t.Error("admin without a grant sees the project")
	}
	if admin.CanSee(Issue{ProjectID: p}) {
		t.Error("admin without a grant sees an issue")
	}
	if admin.CanBrowseProject(p) {
		t.Error("admin without a grant browses the project")
	}

	// With the same grant, an administrator and a plain user are equal.
	r.grant(p, Browse, Grant{Holder: User, UserID: 1})
	if !admin.Can(Browse, p, nil) || !subject(r, 1, false).Can(Browse, p, nil) {
		t.Error("admin and plain user should both get the direct grant")
	}

	// A security level still hides the issue from an administrator outside it.
	r.levels[21] = level{project: p, members: []Grant{{Holder: User, UserID: 2}}}
	is := Issue{ProjectID: p, SecurityLevel: 21}
	if admin.CanSee(is) {
		t.Error("admin outside the security level sees the issue")
	}
}

// TestAnonymousVisitorGetsNoGrant checks that a subject with no account
// (UserID 0) matches no user, group, role or lead grant.
func TestAnonymousVisitorGetsNoGrant(t *testing.T) {
	const p int64 = 7
	r := testRules()
	r.grant(p, Browse, Grant{Holder: User, UserID: 42})
	r.grant(p, Edit, Grant{Holder: Group, GroupID: 3})
	r.grant(p, Comment, Grant{Holder: Role, RoleID: 5})
	r.grant(p, Transition, Grant{Holder: Lead})
	r.leads[p] = 9
	anon := subject(r, 0, false)

	for _, perm := range All {
		if anon.Can(perm, p, nil) {
			t.Errorf("anonymous visitor got %q", perm)
		}
	}
	if anon.CanSee(Issue{ProjectID: p, ReporterID: 0, AssigneeID: 0}) {
		t.Error("anonymous visitor sees an issue")
	}
	if anon.CanBrowseProject(p) {
		t.Error("anonymous visitor browses the project")
	}
}

// TestLeadGrantWithNoLeadMatchesAnonymous checks that a project whose lead_id
// is NULL (cached as lead 0) does not let its Lead grants match a subject
// whose UserID is 0 (an anonymous visitor).
func TestLeadGrantWithNoLeadMatchesAnonymous(t *testing.T) {
	const p int64 = 7
	r := testRules()
	r.grant(p, Browse, Grant{Holder: Lead})
	r.leads[p] = 0
	if subject(r, 0, false).Can(Browse, p, nil) {
		t.Error("anonymous visitor matched the Lead grant of a project with no lead")
	}
}

// TestEveryoneGrantMatchesAnonymous checks that the Everyone holder does not
// reach a subject without a user: the API never builds one for anonymous
// requests, and such a subject holds no grant at all.
func TestEveryoneGrantMatchesAnonymous(t *testing.T) {
	const p int64 = 7
	r := testRules()
	r.grant(p, Browse, Grant{Holder: Everyone})
	if subject(r, 0, false).Can(Browse, p, nil) {
		t.Error("Everyone grant matched a subject with UserID 0")
	}
	if !subject(r, 1, false).Can(Browse, p, nil) {
		t.Error("Everyone grant did not match a signed-in user")
	}
}

// TestSecurityLevels checks who passes an issue's security level.
func TestSecurityLevels(t *testing.T) {
	const p int64 = 8
	r := testRules()
	r.grant(p, Browse, Grant{Holder: Everyone})
	r.grant(p, Edit, Grant{Holder: Everyone})
	r.leads[p] = 9
	const levelID int64 = 21
	r.levels[levelID] = level{project: p, members: []Grant{
		{Holder: Group, GroupID: 3},
		{Holder: User, UserID: 55},
		{Holder: Role, RoleID: 5},
		{Holder: Reporter},
		{Holder: Assignee},
		{Holder: Lead},
	}}
	// A level with no reporter or assignee member: only the group sees it.
	const groupOnlyLevel int64 = 25
	r.levels[groupOnlyLevel] = level{project: p, members: []Grant{{Holder: Group, GroupID: 3}}}

	member := subject(r, 43, false)
	addGroup(member, 3, "developpeurs")
	named := subject(r, 55, false)
	roleHolder := subject(r, 56, false)
	addRole(roleHolder, p, 5)
	roleElsewhere := subject(r, 57, false)
	addRole(roleElsewhere, 99, 5)
	lead := subject(r, 9, false)
	outsider := subject(r, 58, false)

	secured := func(reporter, assignee int64) Issue {
		return Issue{ProjectID: p, ReporterID: reporter, AssigneeID: assignee, SecurityLevel: levelID}
	}
	// groupSecured is restricted to a group with no reporter or assignee member.
	groupSecured := func(reporter, assignee int64) Issue {
		return Issue{ProjectID: p, ReporterID: reporter, AssigneeID: assignee, SecurityLevel: groupOnlyLevel}
	}

	cases := []struct {
		name string
		sub  *Subject
		is   Issue
		want bool
	}{
		{"group member sees the issue", member, secured(1, 2), true},
		{"user member sees the issue", named, secured(1, 2), true},
		{"role member sees the issue", roleHolder, secured(1, 2), true},
		{"role held in another project does not see the issue", roleElsewhere, secured(1, 2), false},
		{"lead sees the issue", lead, secured(1, 2), true},
		{"browse permission outside the level does not see the issue", outsider, secured(1, 2), false},
		{"reporter in the level sees the issue", outsider, secured(58, 2), true},
		{"reporter outside the level does not see the issue", subject(r, 59, false), groupSecured(59, 2), false},
		{"assignee in the level sees the issue", outsider, secured(1, 58), true},
		{"assignee outside the level does not see the issue", subject(r, 59, false), groupSecured(1, 59), false},
		{"unsecured issue is visible with browse", outsider, Issue{ProjectID: p, ReporterID: 1, AssigneeID: 2}, true},
		{"unknown security level hides the issue from everyone", member, Issue{ProjectID: p, SecurityLevel: 999}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.sub.CanSee(c.is); got != c.want {
				t.Errorf("CanSee(%+v) = %v, want %v", c.is, got, c.want)
			}
		})
	}

	// A level member without the browse grant still cannot see the issue.
	noBrowse := testRules()
	noBrowse.levels[levelID] = r.levels[levelID]
	bare := subject(noBrowse, 43, false)
	addGroup(bare, 3, "developpeurs")
	if bare.CanSee(secured(1, 2)) {
		t.Error("membership in a security level granted browse by itself")
	}
}

// TestRestrictedComments checks the audience of a restricted comment.
func TestRestrictedComments(t *testing.T) {
	const p int64 = 7
	r := testRules()
	sub := subject(r, 42, false)
	addGroup(sub, 3, "developpeurs")
	addRole(sub, p, 5)

	cases := []struct {
		name    string
		project int64
		group   string
		role    int64
		want    bool
	}{
		{"unrestricted comment is visible", p, "", 0, true},
		{"group name match is visible", p, "developpeurs", 0, true},
		{"another group is hidden", p, "administrateurs", 0, false},
		{"role match is visible", p, "", 5, true},
		{"another role is hidden", p, "", 6, false},
		{"role held in another project is hidden", 99, "", 5, false},
		{"group and role both held is visible", p, "developpeurs", 5, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sub.SeesRestricted(c.project, c.group, c.role); got != c.want {
				t.Errorf("SeesRestricted(%d, %q, %d) = %v, want %v", c.project, c.group, c.role, got, c.want)
			}
		})
	}
}

// TestCanBrowseProject checks that a project is listed when it has a full
// browse grant, or a reporter/assignee grant that shows only some issues.
func TestCanBrowseProject(t *testing.T) {
	const p int64 = 7
	r := testRules()
	r.grant(p, Browse, Grant{Holder: Reporter})
	r.grant(p, Browse, Grant{Holder: Assignee})
	r.grant(p, Browse, Grant{Holder: User, UserID: 42})

	if !subject(r, 42, false).CanBrowseProject(p) {
		t.Error("direct browse grant: got false")
	}
	if !subject(r, 43, false).CanBrowseProject(p) {
		t.Error("reporter and assignee grants should make the project visible")
	}
	if subject(r, 42, false).CanBrowseProject(8) {
		t.Error("browse grant leaked to another project")
	}
	if subject(testRules(), 42, false).CanBrowseProject(p) {
		t.Error("project without browse grant: got true")
	}
}

// TestVisibleSQL checks the SQL filter that keeps only visible issues.
func TestVisibleSQL(t *testing.T) {
	t.Run("no browse grant matches nothing", func(t *testing.T) {
		r := testRules()
		r.grant(7, Edit, Grant{Holder: Everyone})
		where, args := subject(r, 42, false).VisibleSQL("i")
		if where != "FALSE" || args != nil {
			t.Errorf("VisibleSQL = %q, %v, want \"FALSE\", nil", where, args)
		}
	})

	t.Run("full browse on a project", func(t *testing.T) {
		r := testRules()
		r.grant(7, Browse, Grant{Holder: Everyone})
		where, args := subject(r, 42, false).VisibleSQL("i")
		want := "(i.project_id IN (?)) AND (i.security_level_id IS NULL)"
		if where != want || !reflect.DeepEqual(args, []any{int64(7)}) {
			t.Errorf("VisibleSQL = %q, %v, want %q, [7]", where, args, want)
		}
	})

	t.Run("reporter grant limits to own issues", func(t *testing.T) {
		r := testRules()
		r.grant(7, Browse, Grant{Holder: Reporter})
		where, args := subject(r, 42, false).VisibleSQL("i")
		want := "((i.project_id IN (?) AND i.reporter_id = ?)) AND (i.security_level_id IS NULL)"
		if where != want || !reflect.DeepEqual(args, []any{int64(7), int64(42)}) {
			t.Errorf("VisibleSQL = %q, %v, want %q, [7 42]", where, args, want)
		}
	})

	t.Run("assignee grant limits to assigned issues", func(t *testing.T) {
		r := testRules()
		r.grant(7, Browse, Grant{Holder: Assignee})
		where, args := subject(r, 42, false).VisibleSQL("i")
		want := "((i.project_id IN (?) AND i.assignee_id = ?)) AND (i.security_level_id IS NULL)"
		if where != want || !reflect.DeepEqual(args, []any{int64(7), int64(42)}) {
			t.Errorf("VisibleSQL = %q, %v, want %q, [7 42]", where, args, want)
		}
	})

	t.Run("open security level widens the filter", func(t *testing.T) {
		r := testRules()
		r.grant(7, Browse, Grant{Holder: Everyone})
		r.levels[21] = level{project: 7, members: []Grant{{Holder: Everyone}}}
		where, args := subject(r, 42, false).VisibleSQL("i")
		want := "(i.project_id IN (?)) AND (i.security_level_id IS NULL OR i.security_level_id IN (?))"
		if where != want || !reflect.DeepEqual(args, []any{int64(7), int64(21)}) {
			t.Errorf("VisibleSQL = %q, %v, want %q, [7 21]", where, args, want)
		}
	})

	t.Run("level restricted to the reporter follows the reporter", func(t *testing.T) {
		r := testRules()
		r.grant(7, Browse, Grant{Holder: Everyone})
		r.levels[22] = level{project: 7, members: []Grant{{Holder: Reporter}}}
		where, args := subject(r, 42, false).VisibleSQL("i")
		want := "(i.project_id IN (?)) AND (i.security_level_id IS NULL OR (i.security_level_id IN (?) AND i.reporter_id = ?))"
		if where != want || !reflect.DeepEqual(args, []any{int64(7), int64(22), int64(42)}) {
			t.Errorf("VisibleSQL = %q, %v, want %q, [7 22 42]", where, args, want)
		}
	})

	t.Run("level restricted to the assignee follows the assignee", func(t *testing.T) {
		r := testRules()
		r.grant(7, Browse, Grant{Holder: Everyone})
		r.levels[23] = level{project: 7, members: []Grant{{Holder: Assignee}}}
		where, args := subject(r, 42, false).VisibleSQL("i")
		want := "(i.project_id IN (?)) AND (i.security_level_id IS NULL OR (i.security_level_id IN (?) AND i.assignee_id = ?))"
		if where != want || !reflect.DeepEqual(args, []any{int64(7), int64(23), int64(42)}) {
			t.Errorf("VisibleSQL = %q, %v, want %q, [7 23 42]", where, args, want)
		}
	})

	t.Run("a level the subject does not hold is left out", func(t *testing.T) {
		r := testRules()
		r.grant(7, Browse, Grant{Holder: Everyone})
		r.levels[24] = level{project: 7, members: []Grant{{Holder: User, UserID: 99}}}
		where, args := subject(r, 42, false).VisibleSQL("i")
		if strings.Contains(where, "24") || !reflect.DeepEqual(args, []any{int64(7)}) {
			t.Errorf("VisibleSQL = %q, %v: an unmatched level was included", where, args)
		}
		if where == "FALSE" {
			t.Error("VisibleSQL hid a browsable, unsecured project")
		}
	})

	t.Run("full and reporter grants combine", func(t *testing.T) {
		r := testRules()
		r.grant(7, Browse, Grant{Holder: Everyone})
		r.grant(8, Browse, Grant{Holder: Reporter})
		where, args := subject(r, 42, false).VisibleSQL("i")
		if !strings.Contains(where, "i.project_id IN (?)") ||
			!strings.Contains(where, "i.reporter_id = ?") {
			t.Errorf("VisibleSQL = %q: missing a clause", where)
		}
		if !reflect.DeepEqual(args, []any{int64(7), int64(8), int64(42)}) {
			t.Errorf("VisibleSQL args = %v, want [7 8 42]", args)
		}
	})
}

// TestHoldersSQL checks the query that lists the users holding a permission.
// The rule cache is primed so the store never touches its database.
func TestHoldersSQL(t *testing.T) {
	cached := func(r *rules) *Store { return &Store{rules: r, loadedAt: time.Now()} }

	cases := []struct {
		name  string
		rules *rules
		perm  string
		sql   string
		frag  []string
		args  []any
	}{
		{
			name:  "everyone lists active users",
			rules: func() *rules { r := testRules(); r.grant(7, Browse, Grant{Holder: Everyone}); return r }(),
			perm:  Browse,
			sql:   "SELECT id FROM users WHERE active",
		},
		{
			name:  "a user grant lists that user",
			rules: func() *rules { r := testRules(); r.grant(7, Create, Grant{Holder: User, UserID: 42}); return r }(),
			perm:  Create,
			sql:   "SELECT ?",
			args:  []any{int64(42)},
		},
		{
			name:  "a group grant lists its members",
			rules: func() *rules { r := testRules(); r.grant(7, Edit, Grant{Holder: Group, GroupID: 3}); return r }(),
			perm:  Edit,
			sql:   "SELECT user_id FROM group_members WHERE group_id = ?",
			args:  []any{int64(3)},
		},
		{
			name:  "a role grant lists both direct and group members",
			rules: func() *rules { r := testRules(); r.grant(7, Comment, Grant{Holder: Role, RoleID: 5}); return r }(),
			perm:  Comment,
			frag:  []string{"SELECT user_id FROM project_role_members WHERE project_id = ? AND role_id = ?", "JOIN group_members"},
			args:  []any{int64(7), int64(5), int64(7), int64(5)},
		},
		{
			name:  "a lead grant lists the project lead",
			rules: func() *rules { r := testRules(); r.grant(7, Transition, Grant{Holder: Lead}); return r }(),
			perm:  Transition,
			sql:   "SELECT lead_id FROM projects WHERE id = ? AND lead_id IS NOT NULL",
			args:  []any{int64(7)},
		},
		{
			name: "reporter and assignee grants list nobody",
			rules: func() *rules {
				r := testRules()
				r.grant(7, Browse, Grant{Holder: Reporter})
				r.grant(7, Browse, Grant{Holder: Assignee})
				return r
			}(),
			perm: Browse,
			sql:  "SELECT NULL FROM DUAL WHERE FALSE",
		},
		{
			name:  "no grant lists nobody",
			rules: testRules(),
			perm:  Browse,
			sql:   "SELECT NULL FROM DUAL WHERE FALSE",
		},
		{
			name: "several grants are unioned",
			rules: func() *rules {
				r := testRules()
				r.grant(7, Browse, Grant{Holder: User, UserID: 42})
				r.grant(7, Browse, Grant{Holder: Group, GroupID: 3})
				return r
			}(),
			perm: Browse,
			sql:  "SELECT ? UNION SELECT user_id FROM group_members WHERE group_id = ?",
			args: []any{int64(42), int64(3)},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sql, args, err := cached(c.rules).HoldersSQL(context.Background(), 7, c.perm)
			if err != nil {
				t.Fatalf("HoldersSQL: %v", err)
			}
			if c.sql != "" && sql != c.sql {
				t.Errorf("HoldersSQL = %q, want %q", sql, c.sql)
			}
			for _, f := range c.frag {
				if !strings.Contains(sql, f) {
					t.Errorf("HoldersSQL = %q, missing %q", sql, f)
				}
			}
			if !reflect.DeepEqual(args, c.args) {
				t.Errorf("HoldersSQL args = %v, want %v", args, c.args)
			}
		})
	}
}

// TestStoreCacheAndInvalidate checks that a fresh cache is used as is and that
// Invalidate drops it.
func TestStoreCacheAndInvalidate(t *testing.T) {
	if NewStore(nil) == nil {
		t.Fatal("NewStore returned nil")
	}
	r := testRules()
	s := &Store{rules: r, loadedAt: time.Now()}
	got, err := s.load(context.Background())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != r {
		t.Error("load did not return the cached rules")
	}
	s.Invalidate()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rules != nil {
		t.Error("Invalidate did not drop the cached rules")
	}
}

// TestAccessors checks the small lookups on a subject.
func TestAdministers(t *testing.T) {
	r := testRules()
	r.grant(7, Browse, Grant{Holder: Everyone})
	r.grant(7, Admin, Grant{Holder: Role, RoleID: 5})
	r.grant(9, Browse, Grant{Holder: Everyone})
	r.grant(9, Admin, Grant{Holder: Role, RoleID: 5})
	r.archived[9] = true
	projectAdmin := subject(r, 42, false)
	addRole(projectAdmin, 7, 5)
	addRole(projectAdmin, 9, 5)
	for _, c := range []struct {
		name    string
		sub     *Subject
		project int64
		want    bool
	}{
		{"global administrator", subject(r, 1, true), 8, true},
		{"global administrator, archived project", subject(r, 1, true), 9, true},
		{"holder of Admin", projectAdmin, 7, true},
		{"holder of Admin, another project", projectAdmin, 8, false},
		{"holder of Admin, archived project", projectAdmin, 9, false},
		{"browser only", subject(r, 43, false), 7, false},
		{"no user", subject(r, 0, false), 7, false},
	} {
		if got := c.sub.Administers(c.project); got != c.want {
			t.Errorf("%s: Administers(%d) = %v, want %v", c.name, c.project, got, c.want)
		}
	}
}

func TestAccessors(t *testing.T) {
	r := testRules()
	r.archived[7] = true
	sub := subject(r, 42, false)
	addGroup(sub, 3, "developpeurs")
	addRole(sub, 7, 5)

	if !sub.Archived(7) || sub.Archived(8) {
		t.Error("Archived: wrong project")
	}
	if !sub.InGroup(3) || sub.InGroup(4) {
		t.Error("InGroup: wrong group")
	}
	if !sub.HasRole(7, 5) || sub.HasRole(7, 6) || sub.HasRole(8, 5) {
		t.Error("HasRole: wrong role")
	}
}
