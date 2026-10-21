package api

// Groups and project roles. Groups are global; roles too, but who holds a
// role is set per project (users and groups), and grants name roles.
// Comments and work logs restricted to a group (by name) or a role must
// never become visible to more people: renaming a group carries them along,
// and a group or role they still use cannot be deleted.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

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
	Roles          int `json:"roles"`
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

// cleanName trims a name and checks its length.
func cleanName(name string, max int) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > max {
		return "", failf(http.StatusBadRequest, i18n.CommonNameRequired, max)
	}
	return name, nil
}

// optDescription reads an optional description: empty stores NULL.
func optDescription(v *string) any {
	if v == nil || strings.TrimSpace(*v) == "" {
		return nil
	}
	return strings.TrimSpace(*v)
}

// pathID reads a numeric path value; a value that is not a number is 404.
func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		return 0, errNotFound
	}
	return id, nil
}

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	like := "%" + sqlq.EscapeLike(strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))) + "%"
	out := []groupOut{}
	err := sqlq.Each(r.Context(), s.db, `SELECT g.id, g.name, g.description,
		(SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id)
		FROM user_groups g WHERE g.name LIKE ? ORDER BY g.name`, func(rows *sql.Rows) error {
		var g groupOut
		var desc sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &desc, &g.Members); err != nil {
			return err
		}
		g.Description = sqlq.String(desc)
		out = append(out, g)
		return nil
	}, like)
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
		(SELECT COUNT(*) FROM project_role_members WHERE group_id = ?),
		(SELECT COUNT(*) FROM security_level_members WHERE group_id = ?),
		(SELECT COUNT(*) FROM comments WHERE visibility_group = ?),
		(SELECT COUNT(*) FROM worklogs WHERE visibility_group = ?)`, g.ID, g.ID, g.ID, g.Name, g.Name).
		Scan(&u.Grants, &u.Roles, &u.SecurityLevels, &u.Comments, &u.Worklogs)
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
	name, err := cleanName(name, 255)
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
	s.auditLog(r, audit.GroupCreate, audit.Target{Type: "group", ID: strconv.FormatInt(id, 10), Label: name}, nil)
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
	if name != g.Name {
		details["name"] = map[string]string{"from": g.Name, "to": name}
	}
	if in.Description != nil {
		details["description"] = desc
	}
	s.auditLog(r, audit.GroupUpdate, audit.Target{Type: "group", ID: strconv.FormatInt(id, 10), Label: name}, details)
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
	s.auditLog(r, audit.GroupDelete, audit.Target{Type: "group", ID: strconv.FormatInt(id, 10), Label: g.Name},
		map[string]any{"members": g.Members, "grants": usage.Grants, "roles": usage.Roles,
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
	var added []int64
	for _, u := range in.Users {
		res, err := s.db.ExecContext(ctx, `INSERT IGNORE INTO group_members (group_id, user_id) VALUES (?, ?)`, id, u)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			added = append(added, u)
		}
	}
	s.perms.Invalidate()
	if len(added) > 0 {
		s.auditLog(r, audit.GroupMembers, audit.Target{Type: "group", ID: strconv.FormatInt(id, 10), Label: g.Name},
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
	s.auditLog(r, audit.GroupMembers, audit.Target{Type: "group", ID: strconv.FormatInt(id, 10), Label: g.Name},
		map[string]any{"removed": s.userNames(ctx, []int64{user})})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type roleAdminOut struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Projects    int     `json:"projects"`
	Grants      int     `json:"grants"`
}

func (s *Server) rolesList(ctx context.Context, where string, args ...any) ([]roleAdminOut, error) {
	out := []roleAdminOut{}
	err := sqlq.Each(ctx, s.db, `SELECT r.id, r.name, r.description,
		(SELECT COUNT(DISTINCT m.project_id) FROM project_role_members m WHERE m.role_id = r.id),
		(SELECT COUNT(*) FROM project_permissions p WHERE p.role_id = r.id)
			+ (SELECT COUNT(*) FROM security_level_members l WHERE l.role_id = r.id)
		FROM project_roles r WHERE `+where+` ORDER BY r.name`, func(rows *sql.Rows) error {
		var ro roleAdminOut
		var desc sql.NullString
		if err := rows.Scan(&ro.ID, &ro.Name, &desc, &ro.Projects, &ro.Grants); err != nil {
			return err
		}
		ro.Description = sqlq.String(desc)
		out = append(out, ro)
		return nil
	}, args...)
	return out, err
}

func (s *Server) listRoles(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	out, err := s.rolesList(r.Context(), "TRUE")
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) roleByID(ctx context.Context, id int64) (roleAdminOut, error) {
	list, err := s.rolesList(ctx, "r.id = ?", id)
	if err != nil {
		return roleAdminOut{}, err
	}
	if len(list) == 0 {
		return roleAdminOut{}, errNotFound
	}
	return list[0], nil
}

func (s *Server) createRole(w http.ResponseWriter, r *http.Request) error {
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
	name, err := cleanName(in.Name, 255)
	if err != nil {
		return err
	}
	ctx := r.Context()
	if err := s.nameTaken(ctx, "project_roles", name, 0, i18n.RoleExists); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO project_roles (name, description) VALUES (?, ?)`, name,
		optDescription(in.Description))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	s.auditLog(r, audit.RoleCreate, audit.Target{Type: "role", ID: strconv.FormatInt(id, 10), Label: name}, nil)
	ro, err := s.roleByID(ctx, id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusCreated, ro)
}

func (s *Server) updateRole(w http.ResponseWriter, r *http.Request) error {
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
	ro, err := s.roleByID(ctx, id)
	if err != nil {
		return err
	}
	details := map[string]any{}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if in.Name != nil {
		name, err := cleanName(*in.Name, 255)
		if err != nil {
			return err
		}
		if err := s.nameTaken(ctx, "project_roles", name, id, i18n.RoleExists); err != nil {
			return err
		}
		if name != ro.Name {
			details["name"] = map[string]string{"from": ro.Name, "to": name}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE project_roles SET name = ? WHERE id = ?`, name, id); err != nil {
			return err
		}
	}
	if in.Description != nil {
		details["description"] = optDescription(in.Description)
		if _, err := tx.ExecContext(ctx, `UPDATE project_roles SET description = ? WHERE id = ?`,
			optDescription(in.Description), id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	ro, err = s.roleByID(ctx, id)
	if err != nil {
		return err
	}
	if len(details) > 0 {
		s.auditLog(r, audit.RoleUpdate, audit.Target{Type: "role", ID: strconv.FormatInt(id, 10), Label: ro.Name}, details)
	}
	return writeJSON(w, http.StatusOK, ro)
}

func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	ctx := r.Context()
	ro, err := s.roleByID(ctx, id)
	if err != nil {
		return err
	}
	var comments, worklogs int
	if err := s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM comments WHERE visibility_role_id = ?),
		(SELECT COUNT(*) FROM worklogs WHERE visibility_role_id = ?)`, id, id).Scan(&comments, &worklogs); err != nil {
		return err
	}
	if comments+worklogs > 0 {
		return &httpError{status: http.StatusConflict,
			extra: map[string]any{"usage": map[string]int{"comments": comments, "worklogs": worklogs}},
			msg:   i18n.M(i18n.RoleRestricted)}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM project_roles WHERE id = ?`, id); err != nil {
		return err
	}
	s.perms.Invalidate()
	s.auditLog(r, audit.RoleDelete, audit.Target{Type: "role", ID: strconv.FormatInt(id, 10), Label: ro.Name},
		map[string]any{"projects": ro.Projects, "grants": ro.Grants})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type projectRoleOut struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Users       []userRef `json:"users"`
	Groups      []idName  `json:"groups"`
}

func (s *Server) projectRoles(ctx context.Context, projectID int64) ([]projectRoleOut, error) {
	out := []projectRoleOut{}
	index := map[int64]int{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name, description FROM project_roles ORDER BY name`, func(rows *sql.Rows) error {
		var ro projectRoleOut
		var desc sql.NullString
		if err := rows.Scan(&ro.ID, &ro.Name, &desc); err != nil {
			return err
		}
		ro.Description = sqlq.String(desc)
		ro.Users, ro.Groups = []userRef{}, []idName{}
		index[ro.ID] = len(out)
		out = append(out, ro)
		return nil
	}); err != nil {
		return nil, err
	}
	err := sqlq.Each(ctx, s.db, `SELECT m.role_id, u.id, u.username, u.display_name, u.active, g.id, g.name
		FROM project_role_members m LEFT JOIN users u ON u.id = m.user_id LEFT JOIN user_groups g ON g.id = m.group_id
		WHERE m.project_id = ? ORDER BY u.display_name, g.name`, func(rows *sql.Rows) error {
		var role int64
		var uid, gid sql.NullInt64
		var username, name, gname sql.NullString
		var active sql.NullBool
		if err := rows.Scan(&role, &uid, &username, &name, &active, &gid, &gname); err != nil {
			return err
		}
		i, ok := index[role]
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

func (s *Server) getProjectRoles(w http.ResponseWriter, r *http.Request) error {
	id, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	out, err := s.projectRoles(r.Context(), id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// putProjectRole replaces who holds a role in a project.
func (s *Server) putProjectRole(w http.ResponseWriter, r *http.Request) error {
	projectID, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	roleID, err := pathID(r, "role")
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
	role, err := s.roleByID(ctx, roleID)
	if err != nil {
		return err
	}
	if err := s.checkUsers(ctx, in.Users); err != nil {
		return err
	}
	if err := s.checkGroups(ctx, in.Groups); err != nil {
		return err
	}
	before, err := s.projectRoles(ctx, projectID)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_role_members WHERE project_id = ? AND role_id = ?`,
		projectID, roleID); err != nil {
		return err
	}
	slices.Sort(in.Users)
	slices.Sort(in.Groups)
	for _, u := range slices.Compact(in.Users) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_role_members (project_id, role_id, user_id) VALUES (?, ?, ?)`,
			projectID, roleID, u); err != nil {
			return err
		}
	}
	for _, g := range slices.Compact(in.Groups) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_role_members (project_id, role_id, group_id) VALUES (?, ?, ?)`,
			projectID, roleID, g); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	after, err := s.projectRoles(ctx, projectID)
	if err != nil {
		return err
	}
	var entry projectRoleOut
	for _, ro := range after {
		if ro.ID == roleID {
			entry = ro
		}
	}
	var old projectRoleOut
	for _, ro := range before {
		if ro.ID == roleID {
			old = ro
		}
	}
	names := func(ro projectRoleOut) map[string][]string {
		out := map[string][]string{"users": {}, "groups": {}}
		for _, u := range ro.Users {
			out["users"] = append(out["users"], u.Username)
		}
		for _, g := range ro.Groups {
			out["groups"] = append(out["groups"], g.Name)
		}
		slices.Sort(out["users"])
		slices.Sort(out["groups"])
		return out
	}
	from, to := names(old), names(entry)
	if !slices.Equal(from["users"], to["users"]) || !slices.Equal(from["groups"], to["groups"]) {
		key := s.auditProjectKey(ctx, projectID)
		s.auditLog(r, audit.ProjectRoles, audit.Target{Type: "project", ID: strconv.FormatInt(projectID, 10), Label: key},
			map[string]any{"role": role.Name, "from": from, "to": to})
	}
	return writeJSON(w, http.StatusOK, entry)
}
