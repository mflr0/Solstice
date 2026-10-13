package perm

import (
	"context"
	"strings"
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
			parts = append(parts, "SELECT user_id FROM group_members WHERE group_id = ?")
			args = append(args, g.GroupID)
		case Role:
			parts = append(parts,
				"SELECT user_id FROM project_role_members WHERE project_id = ? AND role_id = ? AND user_id IS NOT NULL",
				`SELECT m.user_id FROM project_role_members r JOIN group_members m ON m.group_id = r.group_id
				 WHERE r.project_id = ? AND r.role_id = ?`)
			args = append(args, project, g.RoleID, project, g.RoleID)
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
