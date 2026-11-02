package devseed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"solstice/internal/notify"
	"solstice/internal/perm"
)

// seedGroup is a group of mock users.
type seedGroup struct {
	name, description string
	members           []string
}

// Developers is the group that sees the confidential demo issue and the
// comments and work logs restricted to it; tom is not in it.
const Developers = "developers"

var groups = []seedGroup{
	{Developers, "People who build the products.", []string{"alice", "hans", "giulia", "priya"}},
	{"office", "People who run the office.", []string{"marie", "tom"}},
}

// seedLevel is a security level whose issues only a group sees.
type seedLevel struct{ project, name, description, group string }

// Confidential is the security level of the confidential demo issue.
const Confidential = "Developers only"

var levels = []seedLevel{
	{Project, Confidential, "Security problems, until they are fixed.", Developers},
}

// groupGrant grants permissions of a project to a group, on top of what a
// new project grants.
type groupGrant struct {
	project, group string
	permissions    []string
}

var grants = []groupGrant{
	{Project, Developers, []string{perm.Security, perm.Moderate}},
}

// groupNotice sends a project's e-mails about an event to a group too.
type groupNotice struct{ project, event, group string }

var notices = []groupNotice{
	{"OPS", notify.GroupCreated, "office"},
}

// grant is a project grant as the permissions API reads and writes it.
type grant struct {
	Permission string `json:"permission"`
	Holder     string `json:"holder"`
	ID         int64  `json:"id,omitempty"`
}

// ensureAccess creates the groups and adds their missing members, then the
// security levels, group grants and notification recipients that are
// missing. It returns the level ids by project and name.
func ensureAccess(ctx context.Context, conn *sql.DB, admin *client, ids map[string]int64,
	say func(string, ...any)) (map[[2]string]int64, error) {
	groupIDs := map[string]int64{}
	for _, g := range groups {
		id, err := ensureGroup(ctx, conn, admin, ids, g, say)
		if err != nil {
			return nil, fmt.Errorf("group %s: %w", g.name, err)
		}
		groupIDs[g.name] = id
	}
	levelIDs := map[[2]string]int64{}
	for _, l := range levels {
		id, err := ensureLevel(ctx, conn, admin, l, groupIDs[l.group], say)
		if err != nil {
			return nil, fmt.Errorf("security level %s: %w", l.name, err)
		}
		levelIDs[[2]string{l.project, l.name}] = id
	}
	for _, g := range grants {
		if err := ensureGrant(admin, g, groupIDs[g.group], say); err != nil {
			return nil, fmt.Errorf("permissions of %s: %w", g.project, err)
		}
	}
	for _, n := range notices {
		if err := ensureNotice(admin, n, groupIDs[n.group], say); err != nil {
			return nil, fmt.Errorf("notifications of %s: %w", n.project, err)
		}
	}
	return levelIDs, nil
}

func ensureGroup(ctx context.Context, conn *sql.DB, admin *client, ids map[string]int64, g seedGroup,
	say func(string, ...any)) (int64, error) {
	var id int64
	err := conn.QueryRowContext(ctx, `SELECT id FROM user_groups WHERE name = ?`, g.name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var out struct{ ID int64 }
		if err = admin.call("POST", "/api/admin/groups", map[string]any{"name": g.name, "description": g.description},
			&out); err == nil {
			say("created the group %s", g.name)
			id = out.ID
		}
	}
	if err != nil {
		return 0, err
	}
	var missing []int64
	for _, u := range g.members {
		var n int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM group_members WHERE group_id = ? AND user_id = ?`,
			id, ids[u]).Scan(&n); err != nil {
			return 0, err
		}
		if n == 0 {
			missing = append(missing, ids[u])
		}
	}
	if len(missing) == 0 {
		return id, nil
	}
	if err := admin.call("POST", fmt.Sprintf("/api/admin/groups/%d/members", id), map[string]any{"users": missing},
		nil); err != nil {
		return 0, err
	}
	say("added %d members to the group %s", len(missing), g.name)
	return id, nil
}

func ensureLevel(ctx context.Context, conn *sql.DB, admin *client, l seedLevel, group int64,
	say func(string, ...any)) (int64, error) {
	path := "/api/projects/" + l.project + "/security-levels"
	var id int64
	err := conn.QueryRowContext(ctx, `SELECT l.id FROM security_levels l JOIN projects p ON p.id = l.project_id
		WHERE p.pkey = ? AND l.name = ?`, l.project, l.name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var out struct{ ID int64 }
		if err = admin.call("POST", path, map[string]any{"name": l.name, "description": l.description},
			&out); err == nil {
			say("created the security level %s in the project %s", l.name, l.project)
			id = out.ID
		}
	}
	if err != nil {
		return 0, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM security_level_members
		WHERE level_id = ? AND holder = ? AND group_id = ?`, id, perm.Group, group).Scan(&n); err != nil || n > 0 {
		return id, err
	}
	return id, admin.call("POST", fmt.Sprintf("%s/%d/members", path, id), map[string]any{"holder": perm.Group,
		"id": group}, nil)
}

// ensureGrant adds the permissions a project lacks for a group. The API
// replaces every grant of the project, so the others are sent back.
func ensureGrant(admin *client, g groupGrant, group int64, say func(string, ...any)) error {
	path := "/api/projects/" + g.project + "/permissions"
	var cur struct{ Grants []grant }
	if err := admin.call("GET", path, nil, &cur); err != nil {
		return err
	}
	all := cur.Grants
	for _, p := range g.permissions {
		if want := (grant{p, perm.Group, group}); !slices.Contains(all, want) {
			all = append(all, want)
		}
	}
	if len(all) == len(cur.Grants) {
		return nil
	}
	if err := admin.call("PUT", path, map[string]any{"grants": all}, nil); err != nil {
		return err
	}
	say("granted %d permissions of the project %s to the group %s", len(all)-len(cur.Grants), g.project, g.group)
	return nil
}

// ensureNotice adds a group to a project's recipients of an event. The API
// replaces every recipient, so the others are sent back.
func ensureNotice(admin *client, n groupNotice, group int64, say func(string, ...any)) error {
	path := "/api/projects/" + n.project + "/notifications"
	var cur struct{ Extra []notify.ExtraRecipient }
	if err := admin.call("GET", path, nil, &cur); err != nil {
		return err
	}
	if slices.ContainsFunc(cur.Extra, func(e notify.ExtraRecipient) bool {
		return e.Event == n.event && e.Kind == "group" && e.ID == group
	}) {
		return nil
	}
	extra := append(cur.Extra, notify.ExtraRecipient{Event: n.event, Kind: "group", ID: group})
	if err := admin.call("PUT", path, map[string]any{"extra": extra}, nil); err != nil {
		return err
	}
	say("the project %s notifies the group %s of %s issues", n.project, n.group, n.event)
	return nil
}
