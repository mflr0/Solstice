package api

// Groups. A group has global members and, per project, project members
// (users and groups). Comments and work logs restricted to a group (by name)
// must never become visible to more people: renaming a group carries them
// along, and a group they still use cannot be deleted.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strings"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/sqlq"
)

type groupOut struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Members     int     `json:"members"`
}

type groupUsage struct {
	Grants         int `json:"grants"`
	ProjectMembers int `json:"projectMembers"`
	SecurityLevels int `json:"securityLevels"`
	Comments       int `json:"comments"`
	Worklogs       int `json:"worklogs"`
}

// checkGroups verifies that every id names a group.
func (s *Server) checkGroups(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_groups WHERE id = ?`, id).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fail(http.StatusBadRequest, i18n.GroupNotFound)
		}
	}
	return nil
}

// checkUsers verifies that every id names an account.
func (s *Server) checkUsers(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id = ?`, id).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return fail(http.StatusBadRequest, i18n.AccountNotFound)
		}
	}
	return nil
}

// groupNames names groups, sorted, for the audit log.
func (s *Server) groupNames(ctx context.Context, ids []int64) []string {
	return s.namesOf(ctx, `SELECT name FROM user_groups WHERE id = ?`, ids)
}

func (s *Server) userNames(ctx context.Context, ids []int64) []string {
	return s.namesOf(ctx, `SELECT username FROM users WHERE id = ?`, ids)
}

func (s *Server) namesOf(ctx context.Context, q string, ids []int64) []string {
	out := []string{}
	for _, id := range ids {
		var n string
		if err := s.db.QueryRowContext(ctx, q, id).Scan(&n); err == nil {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// optDescription reads an optional description: empty stores NULL.
func optDescription(v *string) any {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	return strings.TrimSpace(*v)
}

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	match, args := sqlq.LikeWords(strings.ToLower(r.URL.Query().Get("q")), "g.name")
	out := []groupOut{}
	err := sqlq.Each(r.Context(), s.db, `SELECT g.id, g.name, g.description,
		(SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id)
		FROM user_groups g WHERE `+match+` ORDER BY g.name`, func(rows *sql.Rows) error {
		var g groupOut
		var desc sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &desc, &g.Members); err != nil {
			return err
		}
		g.Description = sqlq.String(desc)
		out = append(out, g)
		return nil
	}, args...)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) groupByID(ctx context.Context, id int64) (groupOut, error) {
	var g groupOut
	var desc sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT g.id, g.name, g.description,
		(SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id) FROM user_groups g WHERE g.id = ?`, id).
		Scan(&g.ID, &g.Name, &desc, &g.Members)
	if errors.Is(err, sql.ErrNoRows) {
		return g, errNotFound
	}
	g.Description = sqlq.String(desc)
	return g, err
}

func (s *Server) groupUsage(ctx context.Context, g groupOut) (groupUsage, error) {
	var u groupUsage
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM project_permissions WHERE group_id = ?),
		(SELECT COUNT(*) FROM group_project_members WHERE group_id = ? OR member_group_id = ?),
		(SELECT COUNT(*) FROM security_level_members WHERE group_id = ?),
		(SELECT COUNT(*) FROM comments WHERE visibility_group = ?),
		(SELECT COUNT(*) FROM worklogs WHERE visibility_group = ?)`, g.ID, g.ID, g.ID, g.ID, g.Name, g.Name).
		Scan(&u.Grants, &u.ProjectMembers, &u.SecurityLevels, &u.Comments, &u.Worklogs)
	return u, err
}

func (s *Server) writeGroup(w http.ResponseWriter, r *http.Request, id int64, status int) error {
	ctx := r.Context()
	g, err := s.groupByID(ctx, id)
	if err != nil {
		return err
	}
	usage, err := s.groupUsage(ctx, g)
	if err != nil {
		return err
	}
	members := []userRef{}
	if err := sqlq.Each(ctx, s.db, `SELECT u.id, u.username, u.display_name, u.active FROM group_members m
		JOIN users u ON u.id = m.user_id WHERE m.group_id = ? ORDER BY u.display_name`, func(rows *sql.Rows) error {
		var u userRef
		if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Active); err != nil {
			return err
		}
		members = append(members, u)
		return nil
	}, id); err != nil {
		return err
	}
	return writeJSON(w, status, map[string]any{"id": g.ID, "name": g.Name, "description": g.Description,
		"members": members, "usage": usage})
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	return s.writeGroup(w, r, id, http.StatusOK)
}

// groupName checks a group name: stored lower-case, as Jira does.
func groupName(name string) (string, error) {
	name, err := cleanName(name, sqlq.VarcharWidth)
	return strings.ToLower(name), err
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	var in struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	name, err := groupName(in.Name)
	if err != nil {
		return err
	}
	ctx := r.Context()
	res, err := s.db.ExecContext(ctx, `INSERT INTO user_groups (name, description) VALUES (?, ?)`,
		name, optDescription(in.Description))
	if isDuplicate(err) {
		return failf(http.StatusConflict, i18n.GroupExists, name)
	}
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	s.perms.Invalidate() // restrictions find groups by name
	s.auditLog(r, audit.GroupCreate, target(audit.TargetGroup, id, name), nil)
	g, err := s.groupByID(ctx, id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, g)
}

func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	g, err := s.groupByID(ctx, id)
	if err != nil {
		return err
	}
	name := g.Name
	if in.Name != nil {
		if name, err = groupName(*in.Name); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	desc := any(nil)
	if g.Description != nil {
		desc = *g.Description
	}
	if in.Description != nil {
		desc = optDescription(in.Description)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_groups SET name = ?, description = ? WHERE id = ?`, name, desc, id); err != nil {
		if isDuplicate(err) {
			return failf(http.StatusConflict, i18n.GroupExists, name)
		}
		return err
	}
	if name != g.Name {
		// Restrictions name the group: they follow it.
		for _, table := range []string{"comments", "worklogs"} {
			if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET visibility_group = ? WHERE visibility_group = ?`,
				name, g.Name); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	details := map[string]any{}
	diff(details, "name", g.Name, name)
	if in.Description != nil {
		details["description"] = desc
	}
	s.auditLog(r, audit.GroupUpdate, target(audit.TargetGroup, id, name), details)
	g, err = s.groupByID(ctx, id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, g)
}

func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	ctx := r.Context()
	g, err := s.groupByID(ctx, id)
	if err != nil {
		return err
	}
	usage, err := s.groupUsage(ctx, g)
	if err != nil {
		return err
	}
	if usage.Comments+usage.Worklogs > 0 {
		return &httpError{status: http.StatusConflict, extra: map[string]any{"usage": usage},
			msg: i18n.M(i18n.GroupRestricted)}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM user_groups WHERE id = ?`, id); err != nil {
		return err
	}
	s.perms.Invalidate()
	s.auditLog(r, audit.GroupDelete, target(audit.TargetGroup, id, g.Name),
		map[string]any{"members": g.Members, "grants": usage.Grants, "projectMembers": usage.ProjectMembers,
			"securityLevels": usage.SecurityLevels})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) addGroupMembers(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Users []int64 `json:"users"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	g, err := s.groupByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.checkUsers(ctx, in.Users); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var added []int64
	for _, u := range in.Users {
		res, err := tx.ExecContext(ctx, `INSERT IGNORE INTO group_members (group_id, user_id) VALUES (?, ?)`, id, u)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			added = append(added, u)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	if len(added) > 0 {
		s.auditLog(r, audit.GroupMembers, target(audit.TargetGroup, id, g.Name),
			map[string]any{"added": s.userNames(ctx, added)})
	}
	return s.writeGroup(w, r, id, http.StatusOK)
}

func (s *Server) removeGroupMember(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	user, err := pathID(r, "user")
	if err != nil {
		return err
	}
	ctx := r.Context()
	g, err := s.groupByID(ctx, id)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = ? AND user_id = ?`, id, user)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	s.perms.Invalidate()
	s.auditLog(r, audit.GroupMembers, target(audit.TargetGroup, id, g.Name),
		map[string]any{"removed": s.userNames(ctx, []int64{user})})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type projectGroupOut struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   *string   `json:"description"`
	FromDirectory bool      `json:"fromDirectory"`
	Users         []userRef `json:"users"`
	Groups        []idName  `json:"groups"`
}

// projectGroups lists every group with its project members in a project.
func (s *Server) projectGroups(ctx context.Context, projectID int64) ([]projectGroupOut, error) {
	out := []projectGroupOut{}
	index := map[int64]int{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name, description, from_directory FROM user_groups ORDER BY name`,
		func(rows *sql.Rows) error {
			var g projectGroupOut
			var desc sql.NullString
			if err := rows.Scan(&g.ID, &g.Name, &desc, &g.FromDirectory); err != nil {
				return err
			}
			g.Description = sqlq.String(desc)
			g.Users, g.Groups = []userRef{}, []idName{}
			index[g.ID] = len(out)
			out = append(out, g)
			return nil
		}); err != nil {
		return nil, err
	}
	err := sqlq.Each(ctx, s.db, `SELECT m.group_id, u.id, u.username, u.display_name, u.active, g.id, g.name
		FROM group_project_members m LEFT JOIN users u ON u.id = m.user_id
		LEFT JOIN user_groups g ON g.id = m.member_group_id
		WHERE m.project_id = ? ORDER BY u.display_name, g.name`, func(rows *sql.Rows) error {
		var group int64
		var uid, gid sql.NullInt64
		var username, name, gname sql.NullString
		var active sql.NullBool
		if err := rows.Scan(&group, &uid, &username, &name, &active, &gid, &gname); err != nil {
			return err
		}
		i, ok := index[group]
		if !ok {
			return nil
		}
		if uid.Valid {
			out[i].Users = append(out[i].Users, userRef{uid.Int64, username.String, name.String, active.Bool})
		}
		if gid.Valid {
			out[i].Groups = append(out[i].Groups, idName{gid.Int64, gname.String})
		}
		return nil
	}, projectID)
	return out, err
}

func (s *Server) getProjectGroups(w http.ResponseWriter, r *http.Request) error {
	id, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	out, err := s.projectGroups(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// memberCycle reports whether making members project members of group in
// a project would make group, through them, a member of itself there.
func memberCycle(ctx context.Context, tx *sql.Tx, projectID, group int64, members []int64) (bool, error) {
	children := map[int64][]int64{} // group → its member groups in the project
	if err := sqlq.Each(ctx, tx, `SELECT group_id, member_group_id FROM group_project_members
		WHERE project_id = ? AND member_group_id IS NOT NULL AND group_id <> ?`, func(rows *sql.Rows) error {
		var parent, child int64
		err := rows.Scan(&parent, &child)
		children[parent] = append(children[parent], child)
		return err
	}, projectID, group); err != nil {
		return false, err
	}
	seen := map[int64]bool{}
	queue := slices.Clone(members)
	for len(queue) > 0 {
		g := queue[0]
		queue = queue[1:]
		if g == group {
			return true, nil
		}
		if !seen[g] {
			seen[g] = true
			queue = append(queue, children[g]...)
		}
	}
	return false, nil
}

// putProjectGroup replaces the project members of a group in a project.
func (s *Server) putProjectGroup(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	groupID, err := pathID(r, "group")
	if err != nil {
		return err
	}
	var in struct {
		Users  []int64 `json:"users"`
		Groups []int64 `json:"groups"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	group, err := s.groupByID(ctx, groupID)
	if err != nil {
		return err
	}
	if err := s.checkUsers(ctx, in.Users); err != nil {
		return err
	}
	if err := s.checkGroups(ctx, in.Groups); err != nil {
		return err
	}
	before, err := s.projectGroups(ctx, projectID)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serializes concurrent changes to the project's members, so two of them
	// cannot close a cycle together.
	if _, err := tx.ExecContext(ctx, `SELECT id FROM projects WHERE id = ? FOR UPDATE`, projectID); err != nil {
		return err
	}
	slices.Sort(in.Users)
	slices.Sort(in.Groups)
	in.Users, in.Groups = slices.Compact(in.Users), slices.Compact(in.Groups)
	if cycle, err := memberCycle(ctx, tx, projectID, groupID, in.Groups); err != nil {
		return err
	} else if cycle {
		return fail(http.StatusBadRequest, i18n.GroupMemberCycle)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_project_members WHERE project_id = ? AND group_id = ?`,
		projectID, groupID); err != nil {
		return err
	}
	for _, u := range in.Users {
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_project_members (group_id, project_id, user_id)
			VALUES (?, ?, ?)`, groupID, projectID, u); err != nil {
			return err
		}
	}
	for _, g := range in.Groups {
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_project_members (group_id, project_id, member_group_id)
			VALUES (?, ?, ?)`, groupID, projectID, g); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	after, err := s.projectGroups(ctx, projectID)
	if err != nil {
		return err
	}
	find := func(list []projectGroupOut) projectGroupOut {
		for _, g := range list {
			if g.ID == groupID {
				return g
			}
		}
		return projectGroupOut{}
	}
	names := func(g projectGroupOut) map[string][]string {
		out := map[string][]string{"users": {}, "groups": {}}
		for _, u := range g.Users {
			out["users"] = append(out["users"], u.Username)
		}
		for _, m := range g.Groups {
			out["groups"] = append(out["groups"], m.Name)
		}
		slices.Sort(out["users"])
		slices.Sort(out["groups"])
		return out
	}
	entry := find(after)
	from, to := names(find(before)), names(entry)
	if changed(from, to) {
		s.auditLog(r, audit.ProjectGroups, target(audit.TargetProject, projectID, s.auditProjectKey(ctx, projectID)),
			map[string]any{"group": group.Name, "from": from, "to": to})
	}
	return writeJSON(w, http.StatusOK, entry)
}
