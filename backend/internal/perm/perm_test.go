package perm

import (
	"context"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// testRules builds an empty rule set. Tests fill only the rules they need.
func testRules() *rules {
	return &rules{
		project:  map[int64]map[string][]Rule{},
		global:   map[string][]Rule{},
		leads:    map[int64]int64{},
		archived: map[int64]bool{},
		levels:   map[int64]level{},
		parents:  map[int64]map[int64][]int64{},
		groupID:  map[string]int64{},
	}
}

// member makes group a project member of parent in a project.
func (r *rules) member(project, group, parent int64) {
	if r.parents[project] == nil {
		r.parents[project] = map[int64][]int64{}
	}
	r.parents[project][group] = append(r.parents[project][group], parent)
}

// grant adds an allow for a permission in a project, or a global one
// (project 0).
func (r *rules) grant(project int64, p string, g Grant) { r.add(project, p, Rule{Grant: g}) }

// deny adds a deny for a permission in a project, or a global one (project 0).
func (r *rules) deny(project int64, p string, g Grant) { r.add(project, p, Rule{Grant: g, Deny: true}) }

// subject builds a subject on the rules with no groups.
func subject(r *rules, userID int64, admin bool) *Subject {
	return r.subject(userID, admin, map[int64]bool{}, nil)
}

// addGroup makes the subject a global member of a group, with the name used
// by restricted comments.
func addGroup(s *Subject, id int64, name string) {
	s.global[id] = true
	for _, set := range s.inProject {
		set[id] = true
	}
	if name != "" {
		s.rules.groupID[name] = id
	}
}

// addProjectGroup makes the subject a project member of a group in a project.
func addProjectGroup(s *Subject, project, group int64) {
	if s.inProject[project] == nil {
		s.inProject[project] = maps.Clone(s.global)
	}
	s.inProject[project][group] = true
}

// TestCanGrantSources checks that a permission is honoured for each kind of
// holder, and only for the holder it names.
func TestCanGrantSources(t *testing.T) {
	const p, q int64 = 7, 8
	r := testRules()
	r.grant(p, Browse, Grant{Holder: Everyone})
	r.grant(p, Create, Grant{Holder: User, UserID: 42})
	r.grant(p, Edit, Grant{Holder: Group, GroupID: 3})
	r.grant(p, Comment, Grant{Holder: Group, GroupID: 5})
	r.grant(q, Comment, Grant{Holder: Group, GroupID: 5})
	r.grant(p, Transition, Grant{Holder: Lead})
	r.grant(p, Delete, Grant{Holder: "widget"})
	r.leads[p] = 9

	user := subject(r, 42, false)
	member := subject(r, 43, false)
	addGroup(member, 3, "developpeurs")
	projectMember := subject(r, 44, false)
	addProjectGroup(projectMember, p, 5)
	lead := subject(r, 9, false)
	stranger := subject(r, 45, false)
	if !lead.Leads(p) || lead.Leads(q) || user.Leads(p) {
		t.Error("Leads must name the project's lead only")
	}

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
		{"group grant matches a project member", projectMember, Comment, p, true},
		{"group grant does not match a project member of another project", projectMember, Comment, q, false},
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
// (UserID 0) matches no user, group or lead grant.
func TestAnonymousVisitorGetsNoGrant(t *testing.T) {
	const p int64 = 7
	r := testRules()
	r.grant(p, Browse, Grant{Holder: User, UserID: 42})
	r.grant(p, Edit, Grant{Holder: Group, GroupID: 3})
	r.grant(p, Comment, Grant{Holder: Group, GroupID: 5})
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
		{Holder: Group, GroupID: 5},
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
	projectMember := subject(r, 56, false)
	addProjectGroup(projectMember, p, 5)
	memberElsewhere := subject(r, 57, false)
	addProjectGroup(memberElsewhere, 99, 5)
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
		{"project member of a level group sees the issue", projectMember, secured(1, 2), true},
		{"project member in another project does not see the issue", memberElsewhere, secured(1, 2), false},
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
	r.groupID["administrateurs"] = 4
	r.groupID["relecteurs"] = 5
	sub := subject(r, 42, false)
	addGroup(sub, 3, "developpeurs")
	addProjectGroup(sub, p, 5)

	cases := []struct {
		name    string
		project int64
		group   string
		want    bool
	}{
		{"unrestricted comment is visible", p, "", true},
		{"global member is visible", p, "developpeurs", true},
		{"global member in another project is visible", 99, "developpeurs", true},
		{"another group is hidden", p, "administrateurs", false},
		{"project member is visible", p, "relecteurs", true},
		{"project member of another project is hidden", 99, "relecteurs", false},
		{"unknown group is hidden", p, "disparus", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sub.SeesRestricted(c.project, c.group); got != c.want {
				t.Errorf("SeesRestricted(%d, %q) = %v, want %v", c.project, c.group, got, c.want)
			}
		})
	}
}

// TestProjectMembers checks who is in a group in a project: global members,
// project members, and members of member groups there, recursively.
func TestProjectMembers(t *testing.T) {
	const p, q int64 = 7, 8
	r := testRules()
	r.member(p, 3, 6) // global group 3 is a project member of 6 in p
	r.member(p, 6, 8) // and 6 of 8, so 3's members are in 8 there too
	r.member(q, 9, 3)
	r.groupID["g8"] = 8
	r.grant(p, Browse, Grant{Holder: Everyone})
	r.grant(q, Browse, Grant{Holder: Everyone})
	r.grant(p, Edit, Grant{Holder: Group, GroupID: 8})
	r.grant(q, Edit, Grant{Holder: Group, GroupID: 8})
	r.grant(p, Comment, Grant{Holder: Group, GroupID: 5})
	r.grant(q, Comment, Grant{Holder: Group, GroupID: 5})

	viaGroup := r.subject(42, false, map[int64]bool{3: true}, nil)
	direct := r.subject(43, false, map[int64]bool{}, map[int64][]int64{p: {5}})
	stranger := r.subject(44, false, map[int64]bool{9: true}, nil)

	for _, c := range []struct {
		name           string
		sub            *Subject
		project, group int64
		want           bool
	}{
		{"global member", viaGroup, q, 3, true},
		{"through a member group", viaGroup, p, 6, true},
		{"through a member group, recursively", viaGroup, p, 8, true},
		{"not through a member group of another project", viaGroup, q, 6, false},
		{"direct project member", direct, p, 5, true},
		{"direct project member, another project", direct, q, 5, false},
		{"a member group's parents are not its members", stranger, p, 3, false},
		{"member group in its project", stranger, q, 3, true},
	} {
		if got := c.sub.InGroupIn(c.project, c.group); got != c.want {
			t.Errorf("%s: InGroupIn(%d, %d) = %v, want %v", c.name, c.project, c.group, got, c.want)
		}
	}
	if !direct.Can(Comment, p, nil) || direct.Can(Comment, q, nil) {
		t.Error("a project member must hold its group's grants in that project only")
	}
	if !viaGroup.Can(Edit, p, nil) || viaGroup.Can(Edit, q, nil) {
		t.Error("a nested project member must hold the group's grants in that project only")
	}
	if !viaGroup.SeesRestricted(p, "g8") || viaGroup.SeesRestricted(q, "g8") {
		t.Error("SeesRestricted must follow project membership")
	}
	if !viaGroup.InGroup(8) || !direct.InGroup(5) || direct.InGroup(8) {
		t.Error("InGroup must count membership in any project")
	}
	if got := viaGroup.GroupsIn(p); !slices.Equal(got, []int64{3, 6, 8}) {
		t.Errorf("GroupsIn(p) = %v, want [3 6 8]", got)
	}
	if got := viaGroup.GroupsIn(q); !slices.Equal(got, []int64{3}) {
		t.Errorf("GroupsIn(q) = %v, want [3]", got)
	}

	cycle := testRules()
	cycle.member(p, 1, 2)
	cycle.member(p, 2, 1)
	sub := cycle.subject(45, false, map[int64]bool{1: true}, nil)
	if !sub.InGroupIn(p, 2) || len(cycle.children(p, 1)) != 2 {
		t.Error("a cycle must resolve, not loop")
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

	// The exact SQL and arguments for one project: grants of every kind of
	// holder, a level granted through a project group, and another alias.
	for _, c := range []struct {
		name   string
		grants []Grant // Browse grants on project 7
		level  []Grant // members of level 30 of project 7
		alias  string
		want   string
		args   []any
	}{
		{"reporter and assignee", []Grant{{Holder: Reporter}, {Holder: Assignee}}, nil, "i",
			"((i.project_id IN (?) AND i.reporter_id = ?) OR (i.project_id IN (?) AND i.assignee_id = ?)) AND " +
				"(i.security_level_id IS NULL)",
			[]any{int64(7), int64(42), int64(7), int64(42)}},
		{"full access wins over reporter", []Grant{{Holder: Reporter}, {Holder: Everyone}}, nil, "i",
			"(i.project_id IN (?)) AND (i.security_level_id IS NULL)", []any{int64(7)}},
		{"level through a project group", []Grant{{Holder: Everyone}}, []Grant{{Holder: Group, GroupID: 5}}, "i",
			"(i.project_id IN (?)) AND (i.security_level_id IS NULL OR i.security_level_id IN (?))",
			[]any{int64(7), int64(30)}},
		{"project by reporter, level by assignee", []Grant{{Holder: Reporter}}, []Grant{{Holder: Assignee}}, "i",
			"((i.project_id IN (?) AND i.reporter_id = ?)) AND " +
				"(i.security_level_id IS NULL OR (i.security_level_id IN (?) AND i.assignee_id = ?))",
			[]any{int64(7), int64(42), int64(30), int64(42)}},
		{"alias", []Grant{{Holder: Reporter}}, []Grant{{Holder: Everyone}}, "x",
			"((x.project_id IN (?) AND x.reporter_id = ?)) AND (x.security_level_id IS NULL OR x.security_level_id IN (?))",
			[]any{int64(7), int64(42), int64(30)}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := testRules()
			for _, g := range c.grants {
				r.grant(7, Browse, g)
			}
			if c.level != nil {
				r.levels[30] = level{project: 7, members: c.level}
			}
			sub := subject(r, 42, false)
			addProjectGroup(sub, 7, 5)
			where, args := sub.VisibleSQL(c.alias)
			if where != c.want || !reflect.DeepEqual(args, c.args) {
				t.Errorf("VisibleSQL(%q)\n got %q %v\nwant %q %v", c.alias, where, args, c.want, c.args)
			}
		})
	}

	t.Run("several projects and levels, in id order", func(t *testing.T) {
		r := testRules()
		for _, p := range []int64{9, 7, 8} {
			r.grant(p, Browse, Grant{Holder: Everyone})
		}
		r.grant(12, Browse, Grant{Holder: Reporter})
		r.grant(11, Browse, Grant{Holder: Reporter})
		r.levels[31] = level{project: 7, members: []Grant{{Holder: Everyone}}}
		r.levels[30] = level{project: 8, members: []Grant{{Holder: Everyone}}}
		want := "(i.project_id IN (?, ?, ?) OR (i.project_id IN (?, ?) AND i.reporter_id = ?)) AND " +
			"(i.security_level_id IS NULL OR i.security_level_id IN (?, ?))"
		wantArgs := []any{int64(7), int64(8), int64(9), int64(11), int64(12), int64(42), int64(30), int64(31)}
		for range 20 { // map order differs from run to run
			where, args := subject(r, 42, false).VisibleSQL("i")
			if where != want || !reflect.DeepEqual(args, wantArgs) {
				t.Fatalf("VisibleSQL\n got %q %v\nwant %q %v", where, args, want, wantArgs)
			}
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
			sql:   groupSQL(1),
			args:  []any{int64(3), int64(7), int64(3)},
		},
		{
			name: "a group grant lists the members of its member groups in the project",
			rules: func() *rules {
				r := testRules()
				r.grant(7, Comment, Grant{Holder: Group, GroupID: 5})
				r.member(7, 4, 5)
				r.member(7, 2, 4)
				r.member(8, 1, 5)
				return r
			}(),
			perm: Comment,
			sql:  groupSQL(3),
			args: []any{int64(2), int64(4), int64(5), int64(7), int64(2), int64(4), int64(5)},
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
			sql:  "SELECT ? UNION " + groupSQL(1),
			args: []any{int64(42), int64(3), int64(7), int64(3)},
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

// groupSQL is the query listing the members of a group in a project, with n
// groups (the group and its member groups there).
func groupSQL(n int) string {
	marks := strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
	return "SELECT user_id FROM group_members WHERE group_id IN (" + marks + ")" +
		" UNION SELECT user_id FROM group_project_members WHERE project_id = ? AND user_id IS NOT NULL" +
		" AND group_id IN (" + marks + ")"
}

// TestMembersSQL checks the query listing a group's members in a project,
// or in any project.
func TestMembersSQL(t *testing.T) {
	r := testRules()
	r.member(7, 4, 5)
	r.member(8, 1, 6)
	s := &Store{rules: r, loadedAt: time.Now()}

	q, args, err := s.MembersSQL(context.Background(), 7, 5)
	if err != nil || q != groupSQL(2) || !reflect.DeepEqual(args, []any{int64(4), int64(5), int64(7), int64(4), int64(5)}) {
		t.Errorf("MembersSQL(7, 5) = %q %v %v", q, args, err)
	}
	q, args, err = s.MembersSQL(context.Background(), 0, 5)
	want := "SELECT user_id FROM group_members WHERE group_id = ? UNION " +
		"SELECT user_id FROM group_project_members WHERE group_id = ? AND user_id IS NOT NULL UNION " + groupSQL(2)
	if err != nil || q != want ||
		!reflect.DeepEqual(args, []any{int64(5), int64(5), int64(4), int64(5), int64(7), int64(4), int64(5)}) {
		t.Errorf("MembersSQL(0, 5) = %q %v %v", q, args, err)
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
	r.grant(7, Admin, Grant{Holder: Group, GroupID: 5})
	r.grant(9, Browse, Grant{Holder: Everyone})
	r.grant(9, Admin, Grant{Holder: Group, GroupID: 5})
	r.archived[9] = true
	projectAdmin := subject(r, 42, false)
	addProjectGroup(projectAdmin, 7, 5)
	addProjectGroup(projectAdmin, 9, 5)
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
	addProjectGroup(sub, 7, 5)

	if !sub.Archived(7) || sub.Archived(8) {
		t.Error("Archived: wrong project")
	}
	if !sub.InGroup(3) || sub.InGroup(4) {
		t.Error("InGroup: wrong group")
	}
	if !sub.InGroupIn(7, 5) || sub.InGroupIn(7, 6) || sub.InGroupIn(8, 5) || !sub.InGroupIn(8, 3) {
		t.Error("InGroupIn: wrong group")
	}
}

// TestRuleLevels checks the order in which the four levels of rules decide:
// a user rule of the project, a global user rule, the group rules of the
// project, then the global group rules, deny beating allow within a level.
func TestRuleLevels(t *testing.T) {
	const p int64 = 7
	reported := &Issue{ProjectID: p, ReporterID: 42}
	for _, c := range []struct {
		name   string
		rules  func(r *rules)
		groups []int64 // project groups of user 42 in p
		perm   string
		is     *Issue
		want   bool
	}{
		{"a project user allow beats a project group deny", func(r *rules) {
			r.deny(p, Edit, Grant{Holder: Group, GroupID: 5})
			r.grant(p, Edit, Grant{Holder: User, UserID: 42})
		}, []int64{5}, Edit, nil, true},
		{"a project user deny beats a global user allow", func(r *rules) {
			r.grant(0, Edit, Grant{Holder: User, UserID: 42})
			r.deny(p, Edit, Grant{Holder: User, UserID: 42})
		}, nil, Edit, nil, false},
		{"a project group deny beats a global group allow", func(r *rules) {
			r.grant(0, Edit, Grant{Holder: Everyone})
			r.deny(p, Edit, Grant{Holder: Group, GroupID: 5})
		}, []int64{5}, Edit, nil, false},
		{"a global group allow applies without a project rule", func(r *rules) {
			r.grant(0, Edit, Grant{Holder: Everyone})
			r.deny(p, Edit, Grant{Holder: Group, GroupID: 5})
		}, nil, Edit, nil, true},
		{"a global user deny beats a project group allow", func(r *rules) {
			r.grant(p, Edit, Grant{Holder: Group, GroupID: 5})
			r.deny(0, Edit, Grant{Holder: User, UserID: 42})
		}, []int64{5}, Edit, nil, false},
		{"two groups at one level, one allow and one deny: deny", func(r *rules) {
			r.grant(p, Edit, Grant{Holder: Group, GroupID: 5})
			r.deny(p, Edit, Grant{Holder: Group, GroupID: 6})
		}, []int64{5, 6}, Edit, nil, false},
		{"a deny of a group the user is not in leaves the allow", func(r *rules) {
			r.grant(p, Edit, Grant{Holder: Group, GroupID: 5})
			r.deny(p, Edit, Grant{Holder: Group, GroupID: 6})
		}, []int64{5}, Edit, nil, true},
		{"a built-in holder deny beats a group allow at its level", func(r *rules) {
			r.grant(p, Browse, Grant{Holder: Everyone})
			r.grant(p, Edit, Grant{Holder: Group, GroupID: 5})
			r.deny(p, Edit, Grant{Holder: Reporter})
		}, []int64{5}, Edit, reported, false},
		{"a rule of another permission does not decide", func(r *rules) {
			r.deny(p, Comment, Grant{Holder: User, UserID: 42})
			r.grant(p, Edit, Grant{Holder: Everyone})
		}, nil, Edit, nil, true},
		{"no rule at any level: not granted", func(r *rules) {
			r.grant(p, Edit, Grant{Holder: Group, GroupID: 6})
		}, []int64{5}, Edit, nil, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := testRules()
			c.rules(r)
			sub := subject(r, 42, false)
			for _, g := range c.groups {
				addProjectGroup(sub, p, g)
			}
			if got := sub.Can(c.perm, p, c.is); got != c.want {
				t.Errorf("Can(%s) = %v, want %v", c.perm, got, c.want)
			}
		})
	}
}

// TestGlobalReporterRule checks that a global rule for the built-in reporter
// grants Edit on the issues a user reported, in every project.
func TestGlobalReporterRule(t *testing.T) {
	r := testRules()
	r.grant(0, Browse, Grant{Holder: Everyone})
	r.grant(0, Edit, Grant{Holder: Reporter})
	sub := subject(r, 42, false)
	for _, p := range []int64{7, 8} {
		if !sub.Can(Edit, p, &Issue{ProjectID: p, ReporterID: 42}) {
			t.Errorf("project %d: the reporter may not edit its issue", p)
		}
		if sub.Can(Edit, p, &Issue{ProjectID: p, ReporterID: 43}) {
			t.Errorf("project %d: edits someone else's issue", p)
		}
		if sub.Can(Edit, p, nil) {
			t.Errorf("project %d: a reporter rule matched without an issue", p)
		}
	}
}

// TestBrowseDeny checks that a Browse deny hides the project from VisibleSQL
// and CanBrowseProject, also for an issue the user reported.
func TestBrowseDeny(t *testing.T) {
	const p int64 = 7
	for _, c := range []struct {
		name   string
		rules  func(r *rules)
		groups []int64
	}{
		{"user deny", func(r *rules) {
			r.grant(p, Browse, Grant{Holder: Everyone})
			r.grant(p, Browse, Grant{Holder: Reporter})
			r.deny(p, Browse, Grant{Holder: User, UserID: 42})
		}, nil},
		{"group deny at the reporter's level", func(r *rules) {
			r.grant(p, Browse, Grant{Holder: Reporter})
			r.deny(p, Browse, Grant{Holder: Group, GroupID: 5})
		}, []int64{5}},
		{"global user deny over a global reporter allow", func(r *rules) {
			r.grant(0, Browse, Grant{Holder: Reporter})
			r.deny(0, Browse, Grant{Holder: User, UserID: 42})
		}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := testRules()
			r.leads[p] = 0
			c.rules(r)
			sub := subject(r, 42, false)
			for _, g := range c.groups {
				addProjectGroup(sub, p, g)
			}
			if where, args := sub.VisibleSQL("i"); where != "FALSE" || args != nil {
				t.Errorf("VisibleSQL = %q %v, want FALSE", where, args)
			}
			if sub.CanBrowseProject(p) {
				t.Error("CanBrowseProject = true")
			}
			if sub.CanSee(Issue{ProjectID: p, ReporterID: 42, AssigneeID: 42}) {
				t.Error("CanSee of a reported issue = true")
			}
		})
	}
}

// TestVisibleSQLRules checks the filter for what only rules beyond
// project allows give: global rules and relation masks from denies.
func TestVisibleSQLRules(t *testing.T) {
	t.Run("a global allow covers every project", func(t *testing.T) {
		r := testRules()
		r.leads[8], r.leads[7] = 0, 0
		r.grant(0, Browse, Grant{Holder: Everyone})
		where, args := subject(r, 42, false).VisibleSQL("i")
		want := "(i.project_id IN (?, ?)) AND (i.security_level_id IS NULL)"
		if where != want || !reflect.DeepEqual(args, []any{int64(7), int64(8)}) {
			t.Errorf("VisibleSQL = %q %v, want %q [7 8]", where, args, want)
		}
	})

	t.Run("everything but the reported issues", func(t *testing.T) {
		r := testRules()
		r.grant(7, Browse, Grant{Holder: Everyone})
		r.deny(7, Browse, Grant{Holder: Reporter})
		sub := subject(r, 42, false)
		where, args := sub.VisibleSQL("i")
		want := "((i.project_id IN (?) AND ((NOT (i.reporter_id <=> ?) AND NOT (i.assignee_id <=> ?)) OR " +
			"(NOT (i.reporter_id <=> ?) AND i.assignee_id = ?)))) AND (i.security_level_id IS NULL)"
		wantArgs := []any{int64(7), int64(42), int64(42), int64(42), int64(42)}
		if where != want || !reflect.DeepEqual(args, wantArgs) {
			t.Errorf("VisibleSQL\n got %q %v\nwant %q %v", where, args, want, wantArgs)
		}
		if !sub.CanBrowseProject(7) {
			t.Error("CanBrowseProject = false")
		}
	})
}

// TestAdministersWithDeny checks that a global administrator still
// administers a project where Admin is denied to it.
func TestAdministersWithDeny(t *testing.T) {
	r := testRules()
	r.deny(7, Admin, Grant{Holder: User, UserID: 1})
	r.deny(0, Admin, Grant{Holder: Everyone})
	if !subject(r, 1, true).Administers(7) {
		t.Error("a global administrator denied Admin does not administer")
	}
	if subject(r, 1, false).Administers(7) {
		t.Error("a user denied Admin administers")
	}
}

// TestHoldersSQLWithDenies checks the query listing holders once a deny
// applies: one term per level, less the users a more specific level decided.
func TestHoldersSQLWithDenies(t *testing.T) {
	r := testRules()
	r.grant(7, Edit, Grant{Holder: Group, GroupID: 3})
	r.deny(7, Edit, Grant{Holder: User, UserID: 42})
	r.grant(0, Edit, Grant{Holder: User, UserID: 43})
	r.deny(0, Edit, Grant{Holder: Everyone})
	s := &Store{rules: r, loadedAt: time.Now()}
	q, args, err := s.HoldersSQL(context.Background(), 7, Edit)
	want := "SELECT id FROM users WHERE " +
		"(id NOT IN (SELECT ?) AND id IN (SELECT ?)) OR " +
		"(id NOT IN (SELECT ?) AND id NOT IN (SELECT ?) AND id IN (" + groupSQL(1) + "))"
	wantArgs := []any{int64(42), int64(43), int64(42), int64(43), int64(3), int64(7), int64(3)}
	if err != nil || q != want || !reflect.DeepEqual(args, wantArgs) {
		t.Errorf("HoldersSQL\n got %q %v %v\nwant %q %v", q, args, err, want, wantArgs)
	}
}

// TestAccess checks that Access names the level and the rule that decide,
// as Can decides.
func TestAccess(t *testing.T) {
	const p int64 = 7
	r := testRules()
	r.grant(0, Browse, Grant{Holder: Everyone})
	r.grant(0, Create, Grant{Holder: Group, GroupID: 3})
	r.deny(p, Create, Grant{Holder: Group, GroupID: 3})
	r.grant(p, Create, Grant{Holder: Everyone})
	r.grant(0, Edit, Grant{Holder: Group, GroupID: 3})
	r.deny(0, Edit, Grant{Holder: User, UserID: 42})
	r.deny(0, Comment, Grant{Holder: Everyone})
	r.grant(p, Comment, Grant{Holder: User, UserID: 42})
	r.leads[p] = 9

	user := subject(r, 42, false)
	addGroup(user, 3, "")
	for _, c := range []struct {
		perm    string
		granted bool
		level   string
		rule    Rule
	}{
		{Browse, true, LevelGlobalGroup, Rule{Grant: Grant{Holder: Everyone}}},
		{Create, false, LevelProjectGroup, Rule{Grant: Grant{Holder: Group, GroupID: 3}, Deny: true}},
		{Edit, false, LevelGlobalUser, Rule{Grant: Grant{Holder: User, UserID: 42}, Deny: true}},
		{Comment, true, LevelProjectUser, Rule{Grant: Grant{Holder: User, UserID: 42}}},
		{Delete, false, LevelNone, Rule{}},
	} {
		a := user.Access(c.perm, p)
		if a != (Access{c.granted, c.level, c.rule}) {
			t.Errorf("Access(%s) = %+v, want %v %s %+v", c.perm, a, c.granted, c.level, c.rule)
		}
		if a.Granted != user.Can(c.perm, p, nil) {
			t.Errorf("Access(%s).Granted = %v, Can disagrees", c.perm, a.Granted)
		}
	}

	r.archived[p] = true
	if a := user.Access(Comment, p); a.Granted || a.Level != LevelProjectUser {
		t.Errorf("archived: Access(comment) = %+v, want refused, still naming the project user rule", a)
	}
	if a := user.Access(Browse, p); !a.Granted {
		t.Errorf("archived: Access(browse) = %+v, want granted", a)
	}
	admin := subject(r, 1, true)
	if a := admin.Access(Admin, p); !a.Granted || a.Level != LevelNone {
		t.Errorf("global admin: Access(admin) = %+v, want granted with no rule", a)
	}
}
