package perm

import (
	"context"
	"maps"
	"slices"
	"strings"

	"solstice/internal/sqlq"
)

// HoldersSQL returns a query selecting the ids of the users who hold a
// permission in a project, with its arguments. Reporter and assignee rules
// are left out: they depend on a particular issue.
func (s *Store) HoldersSQL(ctx context.Context, project int64, p string) (string, []any, error) {
	r, err := s.load(ctx)
	if err != nil {
		return "", nil, err
	}
	scopes := [2][]Rule{r.project[project][p], r.global[p]}
	if !slices.ContainsFunc(scopes[0], isDeny) && !slices.ContainsFunc(scopes[1], isDeny) {
		// With allows only, every user an allow matches holds the permission.
		q, args := r.holdersSQL(project, slices.Concat(scopes[0], scopes[1]), false)
		if q == "" {
			return "SELECT NULL FROM DUAL WHERE FALSE", nil, nil
		}
		return q, args, nil
	}

	// One term per level: the users an allow of the level matches, less
	// those a deny matches and those a more specific level decided for.
	var terms, passed []string
	var args, passedArgs []any
	for _, users := range []bool{true, false} {
		for _, rs := range scopes {
			rs = slices.DeleteFunc(slices.Clone(rs), func(rl Rule) bool { return (rl.Holder == User) != users })
			allow, allowArgs := r.holdersSQL(project, rs, false)
			deny, denyArgs := r.holdersSQL(project, rs, true)
			if allow != "" {
				t := append(slices.Clone(passed), "id IN ("+allow+")")
				args = append(append(args, passedArgs...), allowArgs...)
				if deny != "" {
					t = append(t, "id NOT IN ("+deny+")")
					args = append(args, denyArgs...)
				}
				terms = append(terms, "("+strings.Join(t, " AND ")+")")
			}
			for _, q := range []struct {
				sql  string
				args []any
			}{{allow, allowArgs}, {deny, denyArgs}} {
				if q.sql != "" {
					passed = append(passed, "id NOT IN ("+q.sql+")")
					passedArgs = append(passedArgs, q.args...)
				}
			}
		}
	}
	if len(terms) == 0 {
		return "SELECT NULL FROM DUAL WHERE FALSE", nil, nil
	}
	return "SELECT id FROM users WHERE " + strings.Join(terms, " OR "), args, nil
}

func isDeny(rl Rule) bool { return rl.Deny }

// holdersSQL returns the union of the users the denies (deny) or allows of
// rs match without an issue, or "" when there are none. None of its queries
// yields NULL, so that it can follow NOT IN.
func (r *rules) holdersSQL(project int64, rs []Rule, deny bool) (string, []any) {
	var parts []string
	var args []any
	for _, rl := range rs {
		if rl.Deny != deny {
			continue
		}
		switch rl.Holder {
		case Everyone:
			if deny {
				// Can matches inactive users too: they must not pass a deny.
				parts = append(parts, "SELECT id FROM users")
			} else {
				parts = append(parts, "SELECT id FROM users WHERE active")
			}
		case User:
			parts = append(parts, "SELECT ?")
			args = append(args, rl.UserID)
		case Group:
			q, a := r.membersSQL(project, rl.GroupID)
			parts = append(parts, q)
			args = append(args, a...)
		case Lead:
			parts = append(parts, "SELECT lead_id FROM projects WHERE id = ? AND lead_id IS NOT NULL")
			args = append(args, project)
		}
	}
	return strings.Join(parts, " UNION "), args
}

// MembersSQL returns a query selecting the ids of the users in a group in a
// project, with its arguments. With project 0, it selects its members in any
// project: its global members and its project members of every project.
func (s *Store) MembersSQL(ctx context.Context, project, group int64) (string, []any, error) {
	r, err := s.load(ctx)
	if err != nil {
		return "", nil, err
	}
	if project != 0 {
		q, a := r.membersSQL(project, group)
		return q, a, nil
	}
	parts := []string{"SELECT user_id FROM group_members WHERE group_id = ?",
		"SELECT user_id FROM group_project_members WHERE group_id = ? AND user_id IS NOT NULL"}
	args := []any{group, group}
	for _, p := range slices.Sorted(maps.Keys(r.parents)) {
		if len(r.children(p, group)) > 1 {
			q, a := r.membersSQL(p, group)
			parts = append(parts, q)
			args = append(args, a...)
		}
	}
	return strings.Join(parts, " UNION "), args, nil
}

// membersSQL selects the users in a group in a project: the global and
// project members of the group and of its member groups there.
func (r *rules) membersSQL(project, group int64) (string, []any) {
	groups := r.children(project, group)
	marks := sqlq.Marks(len(groups))
	args := make([]any, 0, 2*len(groups)+1)
	for _, g := range groups {
		args = append(args, g)
	}
	args = append(args, project)
	for _, g := range groups {
		args = append(args, g)
	}
	return "SELECT user_id FROM group_members WHERE group_id IN (" + marks + ")" +
		" UNION SELECT user_id FROM group_project_members WHERE project_id = ? AND user_id IS NOT NULL" +
		" AND group_id IN (" + marks + ")", args
}
