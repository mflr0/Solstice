package perm

import (
	"context"
	"maps"
	"slices"
	"strings"

	"solstice/internal/sqlq"
)

// HoldersSQL returns a query selecting the ids of the users who hold a
// permission in a project, with its arguments. Reporter and assignee grants
// are left out: they depend on a particular issue.
func (s *Store) HoldersSQL(ctx context.Context, project int64, p string) (string, []any, error) {
	r, err := s.load(ctx)
	if err != nil {
		return "", nil, err
	}
	var parts []string
	var args []any
	for _, g := range r.grants[project][p] {
		switch g.Holder {
		case Everyone:
			parts = append(parts, "SELECT id FROM users WHERE active")
		case User:
			parts = append(parts, "SELECT ?")
			args = append(args, g.UserID)
		case Group:
			q, a := r.membersSQL(project, g.GroupID)
			parts = append(parts, q)
			args = append(args, a...)
		case Lead:
			parts = append(parts, "SELECT lead_id FROM projects WHERE id = ? AND lead_id IS NOT NULL")
			args = append(args, project)
		}
	}
	if len(parts) == 0 {
		return "SELECT NULL FROM DUAL WHERE FALSE", nil, nil
	}
	return strings.Join(parts, " UNION "), args, nil
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
