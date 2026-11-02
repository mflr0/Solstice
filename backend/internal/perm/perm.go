// Package perm decides who may see and do what in a project.
//
// Each project lists its own grants (there are no shared schemes): a
// permission is granted to everyone signed in, a group, a user, or to the
// project lead, the issue's reporter or its assignee. An issue security level
// further restricts who sees an issue.
//
// A group has global members and, per project, project members: users or
// other groups. Being in a group in a project means a global member, or a
// project member there, directly or through a member group (recursively).
//
// Global administrators configure Solstice but get no implicit access to
// content, as in Jira: a confidential HR issue stays confidential.
package perm

import (
	"context"
	"database/sql"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"solstice/internal/sqlq"
)

// Permissions. Solstice keeps fewer, broader permissions than Jira: resolving
// and closing are part of moving an issue along, and authors may always edit
// or delete their own comments and work logs.
const (
	Browse     = "browse"     // see the project and its issues
	Create     = "create"     // create issues
	Edit       = "edit"       // edit issues
	Comment    = "comment"    // add comments
	Transition = "transition" // change status
	Assign     = "assign"     // assign issues to someone
	Assignable = "assignable" // be assigned issues
	Attach     = "attach"     // add attachments
	Link       = "link"       // link issues
	Worklog    = "worklog"    // log work
	Delete     = "delete"     // delete issues
	Moderate   = "moderate"   // edit or delete anyone's comments, work logs and attachments
	Watchers   = "watchers"   // manage other people's watching
	Security   = "security"   // set the security level
	Admin      = "admin"      // configure the project
)

// All lists the permissions in display order.
var All = []string{
	Browse, Create, Edit, Comment, Transition, Assign, Assignable,
	Attach, Link, Worklog, Delete, Moderate, Watchers, Security, Admin,
}

// Holder kinds.
const (
	Everyone = "everyone"
	User     = "user"
	Group    = "group"
	Lead     = "lead"
	Reporter = "reporter"
	Assignee = "assignee"
)

// Grant gives a permission (or a security level) to a holder.
type Grant struct {
	Holder  string
	UserID  int64
	GroupID int64
}

// Issue is what checks need to know about an issue.
type Issue struct {
	ProjectID     int64
	ReporterID    int64
	AssigneeID    int64
	SecurityLevel int64 // 0: not secured
}

type level struct {
	project int64
	members []Grant
}

// rules are the grants of every project, shared by all requests.
type rules struct {
	grants   map[int64]map[string][]Grant // project → permission → grants
	leads    map[int64]int64              // project → lead user id
	archived map[int64]bool               // archived projects: read-only
	levels   map[int64]level
	// parents: project → group → the groups it is a project member of there.
	parents map[int64]map[int64][]int64
	groupID map[string]int64 // group name → id
}

// rulesTTL bounds how long rules are cached: an import run by another
// process (solstice import -replace) is picked up without a restart.
const rulesTTL = 30 * time.Second

// Store loads rules and keeps them until Invalidate is called or rulesTTL
// has passed.
type Store struct {
	db       *sql.DB
	mu       sync.Mutex
	rules    *rules
	loadedAt time.Time
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Invalidate must be called after any change to grants, groups, project
// members, leads or levels.
func (s *Store) Invalidate() {
	s.mu.Lock()
	s.rules = nil
	s.mu.Unlock()
}

func (s *Store) load(ctx context.Context) (*rules, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rules != nil && time.Since(s.loadedAt) < rulesTTL {
		return s.rules, nil
	}
	// A request cancelled midway must not leave a truncated rule set behind.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	r := &rules{
		grants:   map[int64]map[string][]Grant{},
		leads:    map[int64]int64{},
		archived: map[int64]bool{},
		levels:   map[int64]level{},
		parents:  map[int64]map[int64][]int64{},
		groupID:  map[string]int64{},
	}
	err := sqlq.Each(ctx, s.db, `SELECT project_id, permission, holder,
		COALESCE(user_id, 0), COALESCE(group_id, 0) FROM project_permissions`,
		func(rows *sql.Rows) error {
			var project int64
			var p string
			var g Grant
			if err := rows.Scan(&project, &p, &g.Holder, &g.UserID, &g.GroupID); err != nil {
				return err
			}
			if r.grants[project] == nil {
				r.grants[project] = map[string][]Grant{}
			}
			r.grants[project][p] = append(r.grants[project][p], g)
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = sqlq.Each(ctx, s.db, `SELECT id, COALESCE(lead_id, 0), archived FROM projects`,
		func(rows *sql.Rows) error {
			var project, lead int64
			var archived bool
			if err := rows.Scan(&project, &lead, &archived); err != nil {
				return err
			}
			r.leads[project] = lead
			if archived {
				r.archived[project] = true
			}
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = sqlq.Each(ctx, s.db, `SELECT l.id, l.project_id, m.holder,
		COALESCE(m.user_id, 0), COALESCE(m.group_id, 0)
		FROM security_levels l LEFT JOIN security_level_members m ON m.level_id = l.id`,
		func(rows *sql.Rows) error {
			var id, project int64
			var holder sql.NullString
			var g Grant
			if err := rows.Scan(&id, &project, &holder, &g.UserID, &g.GroupID); err != nil {
				return err
			}
			lv := r.levels[id]
			lv.project = project
			if holder.Valid {
				g.Holder = holder.String
				lv.members = append(lv.members, g)
			}
			r.levels[id] = lv
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = sqlq.Each(ctx, s.db, `SELECT project_id, member_group_id, group_id FROM group_project_members
		WHERE member_group_id IS NOT NULL`,
		func(rows *sql.Rows) error {
			var project, member, group int64
			if err := rows.Scan(&project, &member, &group); err != nil {
				return err
			}
			if r.parents[project] == nil {
				r.parents[project] = map[int64][]int64{}
			}
			r.parents[project][member] = append(r.parents[project][member], group)
			return nil
		})
	if err != nil {
		return nil, err
	}

	err = sqlq.Each(ctx, s.db, `SELECT id, name FROM user_groups`, func(rows *sql.Rows) error {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		r.groupID[name] = id
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.rules, s.loadedAt = r, time.Now()
	return r, nil
}

// closure adds to set every group that one of its groups is a project
// member of in project, recursively.
func (r *rules) closure(project int64, set map[int64]bool) {
	queue := make([]int64, 0, len(set))
	for g := range set {
		queue = append(queue, g)
	}
	for len(queue) > 0 {
		g := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, p := range r.parents[project][g] {
			if !set[p] {
				set[p] = true
				queue = append(queue, p)
			}
		}
	}
}

// children returns group and the groups that are, recursively, its project
// members in project: their members are the group's members there.
func (r *rules) children(project, group int64) []int64 {
	out := []int64{group}
	seen := map[int64]bool{group: true}
	for i := 0; i < len(out); i++ {
		for member, parents := range r.parents[project] {
			if !seen[member] && slices.Contains(parents, out[i]) {
				seen[member] = true
				out = append(out, member)
			}
		}
	}
	slices.Sort(out)
	return out
}

// Subject is a signed-in user, ready to be checked against the rules.
type Subject struct {
	UserID int64
	Admin  bool
	rules  *rules
	global map[int64]bool // groups the user is a global member of
	// inProject: project → every group the user is in there, for the
	// projects where that differs from global.
	inProject map[int64]map[int64]bool
}

// Subject loads the groups of a user, globally and in each project.
func (s *Store) Subject(ctx context.Context, userID int64, admin bool) (*Subject, error) {
	r, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	global := map[int64]bool{}
	err = sqlq.Each(ctx, s.db, `SELECT group_id FROM group_members WHERE user_id = ?`,
		func(rows *sql.Rows) error {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			global[id] = true
			return nil
		}, userID)
	if err != nil {
		return nil, err
	}

	direct := map[int64][]int64{}
	err = sqlq.Each(ctx, s.db, `SELECT project_id, group_id FROM group_project_members WHERE user_id = ?`,
		func(rows *sql.Rows) error {
			var project, group int64
			if err := rows.Scan(&project, &group); err != nil {
				return err
			}
			direct[project] = append(direct[project], group)
			return nil
		}, userID)
	if err != nil {
		return nil, err
	}
	return r.subject(userID, admin, global, direct), nil
}

// subject builds a subject from the groups the user is a global member of
// and those it is a project member of, by project.
func (r *rules) subject(userID int64, admin bool, global map[int64]bool, direct map[int64][]int64) *Subject {
	sub := &Subject{UserID: userID, Admin: admin, rules: r, global: global, inProject: map[int64]map[int64]bool{}}
	projects := map[int64]bool{}
	for p := range direct {
		projects[p] = true
	}
	for p := range r.parents {
		projects[p] = true
	}
	for p := range projects {
		set := maps.Clone(global)
		for _, g := range direct[p] {
			set[g] = true
		}
		r.closure(p, set)
		if len(set) > len(global) {
			sub.inProject[p] = set
		}
	}
	return sub
}

// InGroupIn reports whether the subject is in a group in a project.
func (s *Subject) InGroupIn(project, group int64) bool {
	if set, ok := s.inProject[project]; ok {
		return set[group]
	}
	return s.global[group]
}

// holds reports whether the subject matches a grant. Without an issue,
// reporter and assignee grants never match. A subject without a user
// (UserID 0) holds nothing: a project with no lead is cached with lead 0, and
// an Everyone grant is meant for accounts, not for a visitor with none.
func (s *Subject) holds(g Grant, project int64, is *Issue) bool {
	if s.UserID == 0 {
		return false
	}
	switch g.Holder {
	case Everyone:
		return true
	case User:
		return g.UserID == s.UserID
	case Group:
		return s.InGroupIn(project, g.GroupID)
	case Lead:
		return s.rules.leads[project] == s.UserID
	case Reporter:
		return is != nil && is.ReporterID == s.UserID
	case Assignee:
		return is != nil && is.AssigneeID == s.UserID
	}
	return false
}

// Can reports whether the subject has permission p in a project, or on an
// issue when is is not nil. Any action on an issue also requires seeing it.
// An archived project is read-only: only Browse is ever granted there.
func (s *Subject) Can(p string, project int64, is *Issue) bool {
	if p != Browse && s.rules.archived[project] {
		return false
	}
	if !s.granted(p, project, is) {
		return false
	}
	if is != nil && p != Browse && !s.granted(Browse, project, is) {
		return false
	}
	return is == nil || s.passesLevel(is)
}

// CanSee reports whether the subject may see an issue.
func (s *Subject) CanSee(is Issue) bool { return s.Can(Browse, is.ProjectID, &is) }

// CanBrowseProject reports whether the subject sees the project at all,
// possibly only the issues it reported or is assigned.
func (s *Subject) CanBrowseProject(project int64) bool {
	for _, g := range s.rules.grants[project][Browse] {
		if g.Holder == Reporter || g.Holder == Assignee || s.holds(g, project, nil) {
			return true
		}
	}
	return false
}

func (s *Subject) granted(p string, project int64, is *Issue) bool {
	for _, g := range s.rules.grants[project][p] {
		if s.holds(g, project, is) {
			return true
		}
	}
	return false
}

func (s *Subject) passesLevel(is *Issue) bool {
	if is.SecurityLevel == 0 {
		return true
	}
	lv, ok := s.rules.levels[is.SecurityLevel]
	if !ok {
		return false
	}
	for _, g := range lv.members {
		if s.holds(g, lv.project, is) {
			return true
		}
	}
	return false
}

// SeesRestricted reports whether the subject sees a comment or work log of
// a project restricted to a group (by name; "" for none).
func (s *Subject) SeesRestricted(project int64, group string) bool {
	if group == "" {
		return true
	}
	id, ok := s.rules.groupID[group]
	return ok && s.InGroupIn(project, id)
}

// VisibleSQL returns a SQL condition on the issues table (under alias) that
// keeps only the issues the subject may see, with its bind arguments, or
// "FALSE" and no arguments when it sees no project. Ids come in ascending
// order, so a subject gets the same SQL every time.
func (s *Subject) VisibleSQL(alias string) (string, []any) {
	var projects visible
	for project, perms := range s.rules.grants {
		projects.add(project, perms[Browse], func(g Grant) bool { return s.holds(g, project, nil) })
	}
	cond, args := projects.where(alias, "project_id", s.UserID)
	if len(cond) == 0 {
		return "FALSE", nil
	}
	var levels visible
	for id, lv := range s.rules.levels {
		levels.add(id, lv.members, func(g Grant) bool { return s.holds(g, lv.project, nil) })
	}
	levelCond, levelArgs := levels.where(alias, "security_level_id", s.UserID)
	sec := append([]string{alias + ".security_level_id IS NULL"}, levelCond...)
	return "(" + strings.Join(cond, " OR ") + ") AND (" + strings.Join(sec, " OR ") + ")", append(args, levelArgs...)
}

// visible groups project or security level ids by what a subject sees of
// their issues: all of them, or only those it reports or is assigned.
type visible struct {
	all, byReporter, byAssignee []int64
}

// add puts id in v by its grants gs: in all when the subject holds one of
// them outright, else in byReporter or byAssignee when one goes to the
// issue's reporter or assignee, which only a row can tell.
func (v *visible) add(id int64, gs []Grant, holds func(Grant) bool) {
	if slices.ContainsFunc(gs, holds) {
		v.all = append(v.all, id)
		return
	}
	if slices.ContainsFunc(gs, func(g Grant) bool { return g.Holder == Reporter }) {
		v.byReporter = append(v.byReporter, id)
	}
	if slices.ContainsFunc(gs, func(g Grant) bool { return g.Holder == Assignee }) {
		v.byAssignee = append(v.byAssignee, id)
	}
}

// where returns the conditions, to be joined with OR, that keep the rows of
// alias whose col is in v (the reported or assigned ones of user for
// byReporter and byAssignee), and their arguments.
func (v visible) where(alias, col string, user int64) ([]string, []any) {
	var cond []string
	var args []any
	in := func(ids []int64, owner string) {
		if len(ids) == 0 {
			return
		}
		c := alias + "." + col + " IN (" + sqlq.Marks(len(ids)) + ")"
		for _, id := range slices.Sorted(slices.Values(ids)) {
			args = append(args, id)
		}
		if owner != "" {
			c = "(" + c + " AND " + alias + "." + owner + " = ?)"
			args = append(args, user)
		}
		cond = append(cond, c)
	}
	in(v.all, "")
	in(v.byReporter, "reporter_id")
	in(v.byAssignee, "assignee_id")
	return cond, args
}

// Administers reports whether the subject may administer a project: a
// global administrator, or a holder of the Admin permission there (which an
// archived project never grants).
func (s *Subject) Administers(project int64) bool {
	return s.Admin || s.Can(Admin, project, nil)
}

// Archived reports whether a project is archived.
func (s *Subject) Archived(project int64) bool { return s.rules.archived[project] }

// InGroup reports whether the subject belongs to a group globally or in
// any project.
func (s *Subject) InGroup(group int64) bool {
	if s.global[group] {
		return true
	}
	for _, set := range s.inProject {
		if set[group] {
			return true
		}
	}
	return false
}
