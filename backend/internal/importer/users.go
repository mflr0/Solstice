package importer

import (
	"database/sql"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Jira resolves a user name against its user directories in order of
// position: the first active directory holding the name wins.
type directory struct {
	position int64
	active   bool
}

type cwdUser struct {
	directory  int64
	display    string
	email      string
	credential string
	active     bool
	rank       [2]int64 // (inactive directory, position): lower wins
}

// importUsers creates one Solstice user per Jira user key (app_user), plus the
// directory users that never got a key. Internal-directory passwords are kept
// (Solstice verifies Jira's PKCS5S2 hashes); external-directory users get none.
func (r *run) importUsers() error {
	dirs := map[int64]directory{}
	if err := r.each(`SELECT id, directory_position, active FROM cwd_directory`, func(rows *sql.Rows) error {
		var id int64
		var pos, active sql.NullInt64
		if err := rows.Scan(&id, &pos, &active); err != nil {
			return err
		}
		dirs[id] = directory{position: pos.Int64, active: active.Int64 == 1}
		return nil
	}); err != nil {
		return err
	}

	best := map[string]cwdUser{}
	if err := r.each(`SELECT directory_id, lower_user_name, display_name, email_address, credential, active
		FROM cwd_user`, func(rows *sql.Rows) error {
		var dir int64
		var lower, display, email, credential sql.NullString
		var active sql.NullInt64
		if err := rows.Scan(&dir, &lower, &display, &email, &credential, &active); err != nil {
			return err
		}
		if !lower.Valid || lower.String == "" {
			return nil
		}
		d := dirs[dir]
		inactiveDir := int64(1)
		if d.active {
			inactiveDir = 0
		}
		u := cwdUser{
			directory:  dir,
			display:    orDefault(display, lower.String),
			email:      email.String,
			credential: credential.String,
			active:     active.Int64 == 1 && d.active,
			rank:       [2]int64{inactiveDir, d.position},
		}
		if cur, ok := best[lower.String]; !ok || lessRank(u.rank, cur.rank) {
			best[lower.String] = u
		}
		return nil
	}); err != nil {
		return err
	}

	type appUser struct{ key, lower string }
	var appUsers []appUser
	if err := r.each(`SELECT user_key, lower_user_name FROM app_user ORDER BY id`, func(rows *sql.Rows) error {
		var key, lower sql.NullString
		if err := rows.Scan(&key, &lower); err != nil {
			return err
		}
		if key.Valid && lower.Valid && key.String != "" && lower.String != "" {
			appUsers = append(appUsers, appUser{key.String, lower.String})
		}
		return nil
	}); err != nil {
		return err
	}

	r.users = map[string]int64{}
	byName := map[string]int64{}
	ins := r.inserter("users", "id", "username", "user_key", "display_name", "email", "password_hash", "active")
	var id int64
	noPassword, placeholders := 0, 0
	add := func(lower string, key any, u cwdUser, found bool) error {
		id++
		byName[lower] = id
		if !found {
			placeholders++
			return ins.add(id, lower, key, lower, "", nil, false)
		}
		if u.active {
			r.activeUsers = append(r.activeUsers, id)
		}
		var hash any
		if strings.HasPrefix(u.credential, "{PKCS5S2}") {
			hash = u.credential
		} else if u.active {
			noPassword++
		}
		return ins.add(id, lower, key, truncate(u.display, 255), truncate(u.email, 255), hash, u.active)
	}
	for _, a := range appUsers {
		if _, dup := byName[a.lower]; dup {
			continue
		}
		u, found := best[a.lower]
		if err := add(a.lower, a.key, u, found); err != nil {
			return err
		}
		r.users[a.key] = id
	}
	// Directory users without a key: Jira would give them their user name as key.
	for _, lower := range slices.Sorted(maps.Keys(best)) {
		u := best[lower]
		if _, ok := byName[lower]; ok {
			continue
		}
		var key any
		if _, taken := r.users[lower]; !taken {
			key = lower
		}
		if err := add(lower, key, u, true); err != nil {
			return err
		}
		if key != nil {
			r.users[lower] = id
		}
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.nextUserID = id
	r.cwd = best
	r.usersByName = byName

	r.rep.step("utilisateurs", ins.total, 0)
	if noPassword > 0 {
		r.rep.warn("%s sans mot de passe utilisable (annuaire externe ou ancien format) : "+
			"définissez-en un avec « solstice user passwd »", plural(noPassword, "compte actif", "comptes actifs"))
	}
	if placeholders > 0 {
		r.rep.warn("%s (auteurs d'anciens tickets)", plural(placeholders,
			"utilisateur supprimé de Jira conservé comme compte inactif",
			"utilisateurs supprimés de Jira conservés comme comptes inactifs"))
	}
	return nil
}

func lessRank(a, b [2]int64) bool {
	if a[0] != b[0] {
		return a[0] < b[0]
	}
	return a[1] < b[1]
}

// importGroups copies groups and flattens membership: a user belongs to the
// groups of its winning directory, including groups that contain those groups.
// Members of the ADMINISTER / SYSTEM_ADMIN groups become Solstice administrators.
func (r *run) importGroups() error {
	r.groups = map[string]int64{}
	gins := r.inserter("user_groups", "id", "name", "description")
	var gid int64
	if err := r.each(`SELECT lower_group_name, description FROM cwd_group ORDER BY directory_id, id`, func(rows *sql.Rows) error {
		var lower, desc sql.NullString
		if err := rows.Scan(&lower, &desc); err != nil {
			return err
		}
		if !lower.Valid || lower.String == "" {
			return nil
		}
		if _, ok := r.groups[lower.String]; ok {
			return nil
		}
		gid++
		r.groups[lower.String] = gid
		var d any
		if desc.Valid {
			d = truncate(desc.String, 255)
		}
		return gins.add(gid, lower.String, d)
	}); err != nil {
		return err
	}
	if err := gins.flush(); err != nil {
		return err
	}

	direct := map[string][]string{}            // user → groups, in the user's directory
	parents := map[int64]map[string][]string{} // directory → group → groups containing it
	if err := r.each(`SELECT directory_id, membership_type, lower_parent_name, lower_child_name
		FROM cwd_membership`, func(rows *sql.Rows) error {
		var dir int64
		var kind, parent, child sql.NullString
		if err := rows.Scan(&dir, &kind, &parent, &child); err != nil {
			return err
		}
		switch kind.String {
		case "GROUP_USER":
			if u, ok := r.cwd[child.String]; ok && u.directory == dir {
				direct[child.String] = append(direct[child.String], parent.String)
			}
		case "GROUP_GROUP":
			if parents[dir] == nil {
				parents[dir] = map[string][]string{}
			}
			parents[dir][child.String] = append(parents[dir][child.String], parent.String)
		}
		return nil
	}); err != nil {
		return err
	}

	r.userGroups = map[int64][]int64{}
	mins := r.inserter("group_members", "group_id", "user_id")
	for lower, groups := range direct {
		uid := r.usersByName[lower]
		seen := map[string]bool{}
		queue := append([]string(nil), groups...)
		for len(queue) > 0 {
			g := queue[0]
			queue = queue[1:]
			if seen[g] {
				continue
			}
			seen[g] = true
			queue = append(queue, parents[r.cwd[lower].directory][g]...)
			if id, ok := r.groups[g]; ok {
				r.userGroups[uid] = append(r.userGroups[uid], id)
				if err := mins.add(id, uid); err != nil {
					return err
				}
			}
		}
	}
	if err := mins.flush(); err != nil {
		return err
	}
	r.rep.step("groupes", gins.total, 0)
	r.rep.step("appartenances aux groupes", mins.total, 0)

	adminGroups := map[int64]bool{}
	if err := r.each(`SELECT permission, group_id FROM globalpermissionentry`, func(rows *sql.Rows) error {
		var perm, group sql.NullString
		if err := rows.Scan(&perm, &group); err != nil {
			return err
		}
		if perm.String == "ADMINISTER" || perm.String == "SYSTEM_ADMIN" {
			if id, ok := r.groups[strings.ToLower(group.String)]; ok {
				adminGroups[id] = true
			}
		}
		return nil
	}); err != nil {
		return err
	}
	var admins []any
	for uid, groups := range r.userGroups {
		for _, g := range groups {
			if adminGroups[g] {
				admins = append(admins, uid)
				break
			}
		}
	}
	for chunk := range slices.Chunk(admins, 1000) {
		q := "UPDATE users SET is_admin = TRUE WHERE id IN (" + placeholders(len(chunk)) + ")"
		if _, err := r.dst.ExecContext(r.ctx, q, chunk...); err != nil {
			return err
		}
	}
	r.rep.step("administrateurs", len(admins), 0)
	return r.applyApplicationAccess(adminGroups)
}

// applyApplicationAccess reproduces Jira's application access: only members of
// a licensed group (or administrators) may sign in. Accounts without it, for
// instance people removed from jira-software-users when they left, are
// imported inactive. Databases without licence groups are left alone.
func (r *run) applyApplicationAccess(adminGroups map[int64]bool) error {
	licensed := map[int64]bool{}
	r.licences = map[string][]int64{}
	found := false
	if err := r.each(`SELECT license_role_name, group_id FROM licenserolesgroup`, func(rows *sql.Rows) error {
		var role, group sql.NullString
		if err := rows.Scan(&role, &group); err != nil {
			return err
		}
		found = true
		if id, ok := r.groups[strings.ToLower(group.String)]; ok {
			licensed[id] = true
			r.licences[role.String] = append(r.licences[role.String], id)
		}
		return nil
	}); err != nil {
		return err
	}
	if !found {
		return nil
	}
	var noAccess []any
	for _, uid := range r.activeUsers {
		allowed := false
		for _, g := range r.userGroups[uid] {
			if licensed[g] || adminGroups[g] {
				allowed = true
				break
			}
		}
		if !allowed {
			noAccess = append(noAccess, uid)
		}
	}
	for chunk := range slices.Chunk(noAccess, 1000) {
		q := "UPDATE users SET active = FALSE WHERE id IN (" + placeholders(len(chunk)) + ")"
		if _, err := r.dst.ExecContext(r.ctx, q, chunk...); err != nil {
			return err
		}
	}
	if len(noAccess) > 0 {
		r.rep.warn("%s sans accès applicatif dans Jira (hors des groupes de licence) : importés inactifs",
			plural(len(noAccess), "compte", "comptes"))
	}
	return nil
}

// plural writes "1 compte" or "3 comptes".
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}
