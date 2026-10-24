package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strings"

	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

type projectOut struct {
	ID          int64    `json:"id"`
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Kind        string   `json:"kind"`
	Lead        *userRef `json:"lead"`
	Category    *string  `json:"category"`
	CategoryID  *int64   `json:"categoryId"`
	Open        int      `json:"open"`
	Archived    bool     `json:"archived"`
}

// projectRow loads a project by key (or a former key).
func (s *Server) projectByKey(ctx context.Context, key string) (id int64, err error) {
	key = strings.ToUpper(key)
	err = s.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE pkey = ?
		UNION SELECT project_id FROM project_keys WHERE pkey = ? LIMIT 1`, key, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errNotFound
	}
	return id, err
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	open := map[int64]int{}
	vis, args := sub.VisibleSQL("i")
	if err := sqlq.Each(ctx, s.db, `SELECT i.project_id, COUNT(*) FROM issues i JOIN statuses st ON st.id = i.status_id
		WHERE `+vis+` AND st.category <> ? GROUP BY i.project_id`, func(rows *sql.Rows) error {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return err
		}
		open[id] = n
		return nil
	}, append(args, issue.Done)...); err != nil {
		return err
	}
	var list []projectOut
	var leads []int64
	leadOf := map[int64]sql.NullInt64{}
	// Archived projects only when asked for.
	withArchived := r.URL.Query().Get("archived") == "1" || r.URL.Query().Get("archived") == "true"
	if err := sqlq.Each(ctx, s.db, `SELECT p.id, p.pkey, p.name, p.kind, p.lead_id, c.id, c.name, p.archived FROM projects p
		LEFT JOIN project_categories c ON c.id = p.category_id ORDER BY p.name`, func(rows *sql.Rows) error {
		var p projectOut
		var lead, categoryID sql.NullInt64
		var category sql.NullString
		if err := rows.Scan(&p.ID, &p.Key, &p.Name, &p.Kind, &lead, &categoryID, &category, &p.Archived); err != nil {
			return err
		}
		if !sub.CanBrowseProject(p.ID) || (p.Archived && !withArchived) {
			return nil
		}
		p.Category = sqlq.String(category)
		if categoryID.Valid {
			p.CategoryID = &categoryID.Int64
		}
		p.Open = open[p.ID]
		leadOf[p.ID] = lead
		leads = append(leads, lead.Int64)
		list = append(list, p)
		return nil
	}); err != nil {
		return err
	}
	users, err := s.users(ctx, leads...)
	if err != nil {
		return err
	}
	for i := range list {
		list[i].Lead = refOf(users, leadOf[list[i].ID])
	}
	if list == nil {
		list = []projectOut{}
	}
	return writeJSON(w, http.StatusOK, list)
}

type versionOut struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Released    bool    `json:"released"`
	Archived    bool    `json:"archived"`
	ReleaseDate *string `json:"releaseDate"`
}

type componentOut struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	id, err := s.projectByKey(ctx, r.PathValue("key"))
	if err != nil {
		return err
	}
	if !sub.CanBrowseProject(id) {
		return errNotFound
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	var p projectOut
	var lead, categoryID sql.NullInt64
	var desc, category sql.NullString
	var defaultAssignee string
	if err := s.db.QueryRowContext(ctx, `SELECT p.id, p.pkey, p.name, p.description, p.kind, p.lead_id, c.id, c.name,
		p.default_assignee, p.archived FROM projects p LEFT JOIN project_categories c ON c.id = p.category_id
		WHERE p.id = ?`, id).
		Scan(&p.ID, &p.Key, &p.Name, &desc, &p.Kind, &lead, &categoryID, &category, &defaultAssignee, &p.Archived); err != nil {
		return err
	}
	p.Description, p.Category = sqlq.String(desc), sqlq.String(category)
	if categoryID.Valid {
		p.CategoryID = &categoryID.Int64
	}
	users, err := s.users(ctx, lead.Int64)
	if err != nil {
		return err
	}
	p.Lead = refOf(users, lead)

	types := []typeRef{}
	if err := sqlq.Each(ctx, s.db, `SELECT issue_type_id FROM project_issue_types WHERE project_id = ? ORDER BY position`,
		func(rows *sql.Rows) error {
			var t string
			if err := rows.Scan(&t); err != nil {
				return err
			}
			types = append(types, cat.typeOf(t))
			return nil
		}, id); err != nil {
		return err
	}
	components := []componentOut{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name, archived FROM components WHERE project_id = ? ORDER BY name`,
		func(rows *sql.Rows) error {
			var c componentOut
			if err := rows.Scan(&c.ID, &c.Name, &c.Archived); err != nil {
				return err
			}
			components = append(components, c)
			return nil
		}, id); err != nil {
		return err
	}
	versions := []versionOut{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name, released, archived, release_date FROM versions
		WHERE project_id = ? ORDER BY position, name`, func(rows *sql.Rows) error {
		var v versionOut
		var release sql.NullTime
		if err := rows.Scan(&v.ID, &v.Name, &v.Released, &v.Archived, &release); err != nil {
			return err
		}
		v.ReleaseDate = dateOnly(release)
		versions = append(versions, v)
		return nil
	}, id); err != nil {
		return err
	}
	levels := []idName{}
	if sub.Can(perm.Security, id, nil) {
		if err := sqlq.Each(ctx, s.db, `SELECT id, name FROM security_levels WHERE project_id = ? ORDER BY name`,
			func(rows *sql.Rows) error {
				var v idName
				if err := rows.Scan(&v.ID, &v.Name); err != nil {
					return err
				}
				levels = append(levels, v)
				return nil
			}, id); err != nil {
			return err
		}
	}
	can := map[string]bool{}
	for _, pm := range perm.All {
		can[pm] = sub.Can(pm, id, nil)
	}
	can["configure"] = sub.Administers(id)
	return writeJSON(w, http.StatusOK, map[string]any{
		"project": p, "issueTypes": types, "components": components, "versions": versions,
		"securityLevels": levels, "defaultAssignee": defaultAssignee, "can": can,
	})
}

// assignableUsers lists who can be assigned issues in a project.
func (s *Server) assignableUsers(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	id, err := s.projectByKey(ctx, r.PathValue("key"))
	if err != nil {
		return err
	}
	if !sub.CanBrowseProject(id) {
		return errNotFound
	}
	holders, args, err := s.perms.HoldersSQL(ctx, id, perm.Assignable)
	if err != nil {
		return err
	}
	like := "%" + sqlq.EscapeLike(strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))) + "%"
	out := []userRef{}
	err = sqlq.Each(ctx, s.db, `SELECT id, username, display_name, active FROM users
		WHERE active AND id IN (`+holders+`) AND (username LIKE ? OR LOWER(display_name) LIKE ?)
		ORDER BY display_name LIMIT 30`, func(rows *sql.Rows) error {
		var u userRef
		if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Active); err != nil {
			return err
		}
		out = append(out, u)
		return nil
	}, append(args, like, like)...)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, out)
}

// Project permissions: the "no scheme" settings page. Each project lists its
// own grants; editing them never affects another project.

type grantOut struct {
	Permission string `json:"permission"`
	Holder     string `json:"holder"`
	ID         int64  `json:"id,omitempty"` // user, group or role id
	Name       string `json:"name,omitempty"`
}

type roleOut struct {
	ID     int64     `json:"id"`
	Name   string    `json:"name"`
	Users  []userRef `json:"users"`
	Groups []idName  `json:"groups"`
}

// browsableProject loads the subject and the project of the path's key,
// which the subject must see (a global administrator sees every project).
// A project it cannot see is "not found", so project keys cannot be probed.
func (s *Server) browsableProject(r *http.Request) (*perm.Subject, int64, error) {
	sub, err := s.subject(r)
	if err != nil {
		return nil, 0, err
	}
	id, err := s.projectByKey(r.Context(), r.PathValue("key"))
	if err != nil {
		return nil, 0, err
	}
	if !sub.Admin && !sub.CanBrowseProject(id) {
		return nil, 0, errNotFound
	}
	return sub, id, nil
}

// configurableProject loads the project of the path's key, whose settings
// the subject may change: a project it sees but does not administer is
// forbidden.
func (s *Server) configurableProject(r *http.Request) (int64, error) {
	sub, id, err := s.browsableProject(r)
	if err != nil {
		return 0, err
	}
	if !sub.Administers(id) {
		return 0, errForbidden
	}
	return id, nil
}

func (s *Server) getPermissions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	grants := []grantOut{}
	if err := sqlq.Each(ctx, s.db, `SELECT g.permission, g.holder, COALESCE(g.user_id, g.group_id, g.role_id, 0),
		COALESCE(u.display_name, ug.name, pr.name, '')
		FROM project_permissions g
		LEFT JOIN users u ON u.id = g.user_id
		LEFT JOIN user_groups ug ON ug.id = g.group_id
		LEFT JOIN project_roles pr ON pr.id = g.role_id
		WHERE g.project_id = ? ORDER BY g.permission, g.holder, 4`, func(rows *sql.Rows) error {
		var g grantOut
		if err := rows.Scan(&g.Permission, &g.Holder, &g.ID, &g.Name); err != nil {
			return err
		}
		grants = append(grants, g)
		return nil
	}, id); err != nil {
		return err
	}
	out, err := s.projectRoles(ctx, id)
	if err != nil {
		return err
	}
	roles := make([]roleOut, len(out))
	for i, ro := range out {
		roles[i] = roleOut{ID: ro.ID, Name: ro.Name, Users: ro.Users, Groups: ro.Groups}
	}
	groups := []idName{}
	if err := sqlq.Each(ctx, s.db, `SELECT id, name FROM user_groups ORDER BY name`, func(rows *sql.Rows) error {
		var g idName
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return err
		}
		groups = append(groups, g)
		return nil
	}); err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, map[string]any{
		"permissions": perm.All, "grants": grants, "roles": roles, "groups": groups,
	})
}

func (s *Server) putPermissions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	id, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	var in struct {
		Grants []grantOut `json:"grants"`
	}
	if err := readJSON(r, &in); err != nil {
		return err
	}
	before, err := s.projectGrants(ctx, id)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM project_permissions WHERE project_id = ?`, id); err != nil {
		return err
	}
	seen := map[grantOut]bool{}
	for _, g := range in.Grants {
		g.Name = ""
		if !slices.Contains(perm.All, g.Permission) || seen[g] {
			continue
		}
		seen[g] = true
		var user, group, role any
		table := ""
		switch g.Holder {
		case perm.Everyone, perm.Lead, perm.Reporter, perm.Assignee:
		case perm.User:
			user, table = g.ID, "users"
		case perm.Group:
			group, table = g.ID, "user_groups"
		case perm.Role:
			role, table = g.ID, "project_roles"
		default:
			return failf(http.StatusBadRequest, i18n.ProjectGranteeUnknown, g.Holder)
		}
		if table != "" {
			var n int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE id = ?`, g.ID).Scan(&n); err != nil {
				return err
			}
			if n == 0 {
				return fail(http.StatusBadRequest, i18n.ProjectGranteeNotFound)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_permissions (project_id, permission, holder, user_id,
			group_id, role_id) VALUES (?, ?, ?, ?, ?, ?)`, id, g.Permission, g.Holder, user, group, role); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.perms.Invalidate()
	s.auditGrants(r, id, before)
	return s.getPermissions(w, r)
}

func (s *Server) projectKeyOf(ctx context.Context, id int64) string {
	var key string
	s.db.QueryRowContext(ctx, `SELECT pkey FROM projects WHERE id = ?`, id).Scan(&key)
	return key
}
