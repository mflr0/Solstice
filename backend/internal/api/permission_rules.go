package api

// Global permission rules of a group, a built-in holder or a user, and the
// effective access of a user, for global administrators.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// ruleOut is a global rule of a holder: one per permission at most.
type ruleOut struct {
	Permission string `json:"permission"`
	Effect     string `json:"effect"`
}

// ruleHolder is whom the global rules of a request are for.
type ruleHolder struct {
	holder string // perm.Group, perm.User or a built-in holder
	id     int64  // group or user id
	action string
	target audit.Target
}

// builtinHolders are the holders other than a group or a user that can hold global rules.
var builtinHolders = []string{perm.Everyone, perm.Lead, perm.Reporter, perm.Assignee}

func (s *Server) groupHolder(r *http.Request) (ruleHolder, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return ruleHolder{}, err
	}
	g, err := s.groupByID(r.Context(), id)
	if err != nil {
		return ruleHolder{}, err
	}
	return ruleHolder{perm.Group, id, audit.GroupPerms, target(audit.TargetGroup, id, g.Name)}, nil
}

func (s *Server) builtinHolder(r *http.Request) (ruleHolder, error) {
	h := r.PathValue("holder")
	if !slices.Contains(builtinHolders, h) {
		return ruleHolder{}, errNotFound
	}
	return ruleHolder{h, 0, audit.GroupPerms, audit.Target{Type: audit.TargetGroup, ID: h, Label: h}}, nil
}

func (s *Server) userHolder(r *http.Request) (ruleHolder, error) {
	id, err := pathID(r, "id")
	if err != nil {
		return ruleHolder{}, err
	}
	var username string
	err = s.db.QueryRowContext(r.Context(), `SELECT username FROM users WHERE id = ?`, id).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		return ruleHolder{}, errNotFound
	}
	if err != nil {
		return ruleHolder{}, err
	}
	return ruleHolder{perm.User, id, audit.UserPerms, target(audit.TargetUser, id, username)}, nil
}

// globalRules returns the global rules of a holder, in the order of perm.All.
func (s *Server) globalRules(ctx context.Context, h ruleHolder) ([]ruleOut, error) {
	out := []ruleOut{}
	err := sqlq.Each(ctx, s.db, `SELECT permission, effect FROM permission_rules
		WHERE project_id IS NULL AND holder = ? AND COALESCE(user_id, group_id, 0) = ?`,
		func(rows *sql.Rows) error {
			var rl ruleOut
			if err := rows.Scan(&rl.Permission, &rl.Effect); err != nil {
				return err
			}
			out = append(out, rl)
			return nil
		}, h.holder, h.id)
	slices.SortStableFunc(out, func(a, b ruleOut) int {
		return slices.Index(perm.All, a.Permission) - slices.Index(perm.All, b.Permission)
	})
	return out, err
}

// getRules answers the global rules of the holder resolve finds.
func (s *Server) getRules(resolve func(*http.Request) (ruleHolder, error)) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := requireAdmin(r); err != nil {
			return err
		}
		h, err := resolve(r)
		if err != nil {
			return err
		}
		rules, err := s.globalRules(r.Context(), h)
		if err != nil {
			return err
		}
		return writeJSON(w, http.StatusOK, rules)
	}
}

// putRules replaces the global rules of the holder resolve finds.
func (s *Server) putRules(resolve func(*http.Request) (ruleHolder, error)) handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := requireAdmin(r); err != nil {
			return err
		}
		h, err := resolve(r)
		if err != nil {
			return err
		}
		var in []ruleOut
		if err := readJSON(r, &in); err != nil {
			return err
		}
		seen := map[string]bool{}
		for i, rl := range in {
			if !slices.Contains(perm.All, rl.Permission) {
				return failf(http.StatusBadRequest, i18n.PermissionUnknown, rl.Permission)
			}
			if seen[rl.Permission] {
				return failf(http.StatusBadRequest, i18n.PermissionTwice, rl.Permission)
			}
			seen[rl.Permission] = true
			if in[i].Effect, err = ruleEffect(rl.Effect); err != nil {
				return err
			}
		}

		ctx := r.Context()
		before, err := s.globalRules(ctx, h)
		if err != nil {
			return err
		}
		var user, group any
		switch h.holder {
		case perm.User:
			user = h.id
		case perm.Group:
			group = h.id
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, `DELETE FROM permission_rules
			WHERE project_id IS NULL AND holder = ? AND COALESCE(user_id, group_id, 0) = ?`, h.holder, h.id); err != nil {
			return err
		}
		for _, rl := range in {
			if _, err := tx.ExecContext(ctx, `INSERT INTO permission_rules (permission, effect, holder, user_id, group_id)
				VALUES (?, ?, ?, ?, ?)`, rl.Permission, rl.Effect, h.holder, user, group); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		s.perms.Invalidate()

		after, err := s.globalRules(ctx, h)
		if err != nil {
			return err
		}
		label := func(rl ruleOut) string { return rl.Permission + ": " + rl.Effect }
		added, removed := []string{}, []string{}
		for _, rl := range after {
			if !slices.Contains(before, rl) {
				added = append(added, label(rl))
			}
		}
		for _, rl := range before {
			if !slices.Contains(after, rl) {
				removed = append(removed, label(rl))
			}
		}
		if len(added)+len(removed) > 0 {
			s.auditLog(r, h.action, h.target, map[string]any{"added": added, "removed": removed})
		}
		return writeJSON(w, http.StatusOK, after)
	}
}

// groupRuleOut is a global rule of a group or a built-in holder.
type groupRuleOut struct {
	Permission string `json:"permission"`
	Holder     string `json:"holder"`
	ID         int64  `json:"id,omitempty"` // group id
	Effect     string `json:"effect"`
}

// groupGlobalRules returns the global rules of every group and built-in holder.
func (s *Server) groupGlobalRules(ctx context.Context) ([]groupRuleOut, error) {
	out := []groupRuleOut{}
	err := sqlq.Each(ctx, s.db, `SELECT permission, holder, COALESCE(group_id, 0), effect FROM permission_rules
		WHERE project_id IS NULL AND holder <> ? ORDER BY holder, 3, permission`, func(rows *sql.Rows) error {
		var rl groupRuleOut
		if err := rows.Scan(&rl.Permission, &rl.Holder, &rl.ID, &rl.Effect); err != nil {
			return err
		}
		out = append(out, rl)
		return nil
	}, perm.User)
	return out, err
}

// permUserOut is what a user inherits in a project when no project rule of
// theirs decides: their global rules, and the groups they are in there.
type permUserOut struct {
	ID     int64     `json:"id"`
	Name   string    `json:"name"`
	Groups []string  `json:"groups"` // builtin:<holder> or group:<id>
	Rules  []ruleOut `json:"rules"`
}

// permUser loads what a user inherits in a project. Reporter and assignee
// are left out: they depend on an issue.
func (s *Server) permUser(ctx context.Context, project, userID int64) (permUserOut, error) {
	u := permUserOut{ID: userID}
	err := s.db.QueryRowContext(ctx, `SELECT display_name FROM users WHERE id = ?`, userID).Scan(&u.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return u, errNotFound
	}
	if err != nil {
		return u, err
	}
	sub, err := s.perms.Subject(ctx, userID, false)
	if err != nil {
		return u, err
	}
	u.Groups = []string{"builtin:" + perm.Everyone}
	if sub.Leads(project) {
		u.Groups = append(u.Groups, "builtin:"+perm.Lead)
	}
	for _, g := range sub.GroupsIn(project) {
		u.Groups = append(u.Groups, "group:"+strconv.FormatInt(g, 10))
	}
	u.Rules, err = s.globalRules(ctx, ruleHolder{holder: perm.User, id: userID})
	return u, err
}

// getPermissionUser answers what a user inherits in a project, for a user row
// added to the project's rules before they are saved.
func (s *Server) getPermissionUser(w http.ResponseWriter, r *http.Request) error {
	project, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	u, err := s.permUser(r.Context(), project, id)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, u)
}

// ruleEffect checks the effect of a rule: allow when left out.
func ruleEffect(effect string) (string, error) {
	switch effect {
	case "":
		return perm.Allow, nil
	case perm.Allow, perm.Deny:
		return effect, nil
	}
	return "", failf(http.StatusBadRequest, i18n.PermissionEffectUnknown, effect)
}

// accessOut is whether a user has a permission in a project, and which rule
// decided.
type accessOut struct {
	Permission string     `json:"permission"`
	Granted    bool       `json:"granted"`
	Level      string     `json:"level"`
	Effect     *string    `json:"effect"`
	Source     *sourceOut `json:"source"`
}

type sourceOut struct {
	Kind string `json:"kind"` // user, group or builtin
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name"`
}

func (s *Server) userAccess(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(r); err != nil {
		return err
	}
	ctx := r.Context()
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var name string
	var admin bool
	err = s.db.QueryRowContext(ctx, `SELECT display_name, is_admin FROM users WHERE id = ?`, id).Scan(&name, &admin)
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound
	}
	if err != nil {
		return err
	}
	key := r.URL.Query().Get("project")
	if key == "" {
		return failf(http.StatusBadRequest, i18n.RequestInvalid, "project")
	}
	project, err := s.projectByKey(ctx, key)
	if err != nil {
		return err
	}
	sub, err := s.perms.Subject(ctx, id, admin)
	if err != nil {
		return err
	}

	groups := map[int64]string{}
	out := make([]accessOut, 0, len(perm.All))
	for _, p := range perm.All {
		a := sub.Access(p, project)
		o := accessOut{Permission: p, Granted: a.Granted, Level: a.Level}
		if a.Level != perm.LevelNone {
			effect := perm.Allow
			if a.Rule.Deny {
				effect = perm.Deny
			}
			o.Effect = &effect
			switch a.Rule.Holder {
			case perm.User:
				o.Source = &sourceOut{Kind: "user", ID: id, Name: name}
			case perm.Group:
				g, ok := groups[a.Rule.GroupID]
				if !ok {
					if err := s.db.QueryRowContext(ctx, `SELECT name FROM user_groups WHERE id = ?`,
						a.Rule.GroupID).Scan(&g); err != nil && !errors.Is(err, sql.ErrNoRows) {
						return err
					}
					groups[a.Rule.GroupID] = g
				}
				o.Source = &sourceOut{Kind: "group", ID: a.Rule.GroupID, Name: g}
			default:
				o.Source = &sourceOut{Kind: "builtin", Name: a.Rule.Holder}
			}
		}
		out = append(out, o)
	}
	return writeJSON(w, http.StatusOK, out)
}

// projectRuleLabel names a project rule in the audit log: the effect shows
// for a deny only, so allows read as they always did.
func projectRuleLabel(p, effect, holder, name string) string {
	if effect == perm.Deny {
		holder = perm.Deny + " " + holder
	}
	return strings.TrimSpace(p + ": " + holder + " " + name)
}
