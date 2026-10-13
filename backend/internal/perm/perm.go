// Package perm decides who may see and do what in a project.
//
// Each project lists its own grants (there are no shared schemes): a
// permission is granted to everyone signed in, a group, a user, a project
// role, or to the project lead, the issue's reporter or its assignee. An issue
// security level further restricts who sees an issue.
//
// Global administrators configure Solstice but get no implicit access to
// content, as in Jira: a confidential HR issue stays confidential.
package perm

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"time"
)

// Permissions. Solstice keeps fewer, broader permissions than Jira: resolving
// and closing are part of moving an issue along, and authors may always edit
// or delete their own comments.
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
	Moderate   = "moderate"   // edit or delete anyone's comments and attachments
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
	Role     = "role"
	Lead     = "lead"
	Reporter = "reporter"
	Assignee = "assignee"
)

// Grant gives a permission (or a security level) to a holder.
type Grant struct {
	Holder  string
	UserID  int64
	GroupID int64
	RoleID  int64
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
	grants map[int64]map[string][]Grant // project → permission → grants
	leads  map[int64]int64              // project → lead user id
	levels map[int64]level
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

// Invalidate must be called after any change to grants, roles, leads or levels.
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
		grants: map[int64]map[string][]Grant{},
		leads:  map[int64]int64{},
		levels: map[int64]level{},
	}
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, permission, holder,
		COALESCE(user_id, 0), COALESCE(group_id, 0), COALESCE(role_id, 0) FROM project_permissions`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var project int64
		var p string
		var g Grant
		if err := rows.Scan(&project, &p, &g.Holder, &g.UserID, &g.GroupID, &g.RoleID); err != nil {
			rows.Close()
			return nil, err
		}
		if r.grants[project] == nil {
			r.grants[project] = map[string][]Grant{}
		}
		r.grants[project][p] = append(r.grants[project][p], g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.QueryContext(ctx, `SELECT id, COALESCE(lead_id, 0) FROM projects`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var project, lead int64
		if err := rows.Scan(&project, &lead); err != nil {
			rows.Close()
			return nil, err
		}
		r.leads[project] = lead
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.QueryContext(ctx, `SELECT l.id, l.project_id, m.holder,
		COALESCE(m.user_id, 0), COALESCE(m.group_id, 0), COALESCE(m.role_id, 0)
		FROM security_levels l LEFT JOIN security_level_members m ON m.level_id = l.id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id, project int64
		var holder sql.NullString
		var g Grant
		if err := rows.Scan(&id, &project, &holder, &g.UserID, &g.GroupID, &g.RoleID); err != nil {
			rows.Close()
			return nil, err
		}
		lv := r.levels[id]
		lv.project = project
		if holder.Valid {
			g.Holder = holder.String
			lv.members = append(lv.members, g)
		}
		r.levels[id] = lv
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	s.rules, s.loadedAt = r, time.Now()
	return r, nil
}

// Subject is a signed-in user, ready to be checked against the rules.
type Subject struct {
	UserID     int64
	Admin      bool
	rules      *rules
	groups     map[int64]bool
	groupNames map[string]bool
	roles      map[int64]map[int64]bool // project → role ids
}

// Subject loads the groups and project roles of a user.
func (s *Store) Subject(ctx context.Context, userID int64, admin bool) (*Subject, error) {
	r, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	sub := &Subject{
		UserID:     userID,
		Admin:      admin,
		rules:      r,
		groups:     map[int64]bool{},
		groupNames: map[string]bool{},
		roles:      map[int64]map[int64]bool{},
	}
	rows, err := s.db.QueryContext(ctx, `SELECT g.id, g.name FROM group_members m
		JOIN user_groups g ON g.id = m.group_id WHERE m.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		sub.groups[id] = true
		sub.groupNames[name] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.QueryContext(ctx, `SELECT DISTINCT r.project_id, r.role_id FROM project_role_members r
		LEFT JOIN group_members m ON m.group_id = r.group_id
		WHERE r.user_id = ? OR m.user_id = ?`, userID, userID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var project, role int64
		if err := rows.Scan(&project, &role); err != nil {
			rows.Close()
			return nil, err
		}
		if sub.roles[project] == nil {
			sub.roles[project] = map[int64]bool{}
		}
		sub.roles[project][role] = true
	}
	rows.Close()
	return sub, rows.Err()
}

// holds reports whether the subject matches a grant. Without an issue,
// reporter and assignee grants never match.
func (s *Subject) holds(g Grant, project int64, is *Issue) bool {
	switch g.Holder {
	case Everyone:
		return true
	case User:
		return g.UserID == s.UserID
	case Group:
		return s.groups[g.GroupID]
	case Role:
		return s.roles[project][g.RoleID]
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
func (s *Subject) Can(p string, project int64, is *Issue) bool {
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

// SeesRestricted reports whether the subject sees a comment or work log
// restricted to a group (by name) and/or a project role.
func (s *Subject) SeesRestricted(project int64, group string, role int64) bool {
	if group != "" && !s.groupNames[group] {
		return false
	}
	if role != 0 && !s.roles[project][role] {
		return false
	}
	return true
}

// VisibleSQL returns a SQL condition on the issues table (under alias) that
// keeps only the issues the subject may see, with its bind arguments.
func (s *Subject) VisibleSQL(alias string) (string, []any) {
	col := func(c string) string { return alias + "." + c }
	var full, byReporter, byAssignee []any
	for project, perms := range s.rules.grants {
		all, rep, asg := false, false, false
		for _, g := range perms[Browse] {
			switch g.Holder {
			case Reporter:
				rep = true
			case Assignee:
				asg = true
			default:
				all = all || s.holds(g, project, nil)
			}
		}
		switch {
		case all:
			full = append(full, project)
		default:
			if rep {
				byReporter = append(byReporter, project)
			}
			if asg {
				byAssignee = append(byAssignee, project)
			}
		}
	}

	var cond []string
	var args []any
	if len(full) > 0 {
		cond = append(cond, col("project_id")+" IN ("+marks(len(full))+")")
		args = append(args, full...)
	}
	if len(byReporter) > 0 {
		cond = append(cond, "("+col("project_id")+" IN ("+marks(len(byReporter))+") AND "+col("reporter_id")+" = ?)")
		args = append(append(args, byReporter...), s.UserID)
	}
	if len(byAssignee) > 0 {
		cond = append(cond, "("+col("project_id")+" IN ("+marks(len(byAssignee))+") AND "+col("assignee_id")+" = ?)")
		args = append(append(args, byAssignee...), s.UserID)
	}
	if len(cond) == 0 {
		return "FALSE", nil
	}
	where := "(" + strings.Join(cond, " OR ") + ")"

	var open, levelByReporter, levelByAssignee []any
	for id, lv := range s.rules.levels {
		all, rep, asg := false, false, false
		for _, g := range lv.members {
			switch g.Holder {
			case Reporter:
				rep = true
			case Assignee:
				asg = true
			default:
				all = all || s.holds(g, lv.project, nil)
			}
		}
		switch {
		case all:
			open = append(open, id)
		default:
			if rep {
				levelByReporter = append(levelByReporter, id)
			}
			if asg {
				levelByAssignee = append(levelByAssignee, id)
			}
		}
	}
	sec := []string{col("security_level_id") + " IS NULL"}
	if len(open) > 0 {
		sec = append(sec, col("security_level_id")+" IN ("+marks(len(open))+")")
		args = append(args, open...)
	}
	if len(levelByReporter) > 0 {
		sec = append(sec, "("+col("security_level_id")+" IN ("+marks(len(levelByReporter))+") AND "+col("reporter_id")+" = ?)")
		args = append(append(args, levelByReporter...), s.UserID)
	}
	if len(levelByAssignee) > 0 {
		sec = append(sec, "("+col("security_level_id")+" IN ("+marks(len(levelByAssignee))+") AND "+col("assignee_id")+" = ?)")
		args = append(append(args, levelByAssignee...), s.UserID)
	}
	return where + " AND (" + strings.Join(sec, " OR ") + ")", args
}

func marks(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// InGroup reports whether the subject belongs to a group.
func (s *Subject) InGroup(group int64) bool { return s.groups[group] }

// HasRole reports whether the subject holds a role in a project.
func (s *Subject) HasRole(project, role int64) bool { return s.roles[project][role] }
