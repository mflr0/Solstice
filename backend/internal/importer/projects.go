package importer

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"solstice/internal/issue"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// jiraPermissions maps Jira permission keys to Solstice permissions. Jira keys
// that are absent are folded into broader Solstice permissions or dropped (see
// perm for the rationale). Moderation is handled apart: it needs both
// EDIT_ALL_COMMENTS and DELETE_ALL_COMMENTS.
var jiraPermissions = map[string]string{
	"BROWSE_PROJECTS":     perm.Browse,
	"CREATE_ISSUES":       perm.Create,
	"EDIT_ISSUES":         perm.Edit,
	"ADD_COMMENTS":        perm.Comment,
	"TRANSITION_ISSUES":   perm.Transition,
	"ASSIGN_ISSUES":       perm.Assign,
	"ASSIGNABLE_USER":     perm.Assignable,
	"CREATE_ATTACHMENTS":  perm.Attach,
	"LINK_ISSUES":         perm.Link,
	"WORK_ON_ISSUES":      perm.Worklog,
	"DELETE_ISSUES":       perm.Delete,
	"MANAGE_WATCHERS":     perm.Watchers,
	"SET_ISSUE_SECURITY":  perm.Security,
	"ADMINISTER_PROJECTS": perm.Admin,
}

// guessDefaultResolution marks a resolution as the default when Jira names
// none: Done or its usual synonyms, else the first. Migration 0011 applies
// the same guess to instances that predate the default.
const guessDefaultResolution = `UPDATE resolutions SET is_default = TRUE WHERE id = (
	SELECT id FROM (SELECT id FROM resolutions
		ORDER BY CASE LOWER(name) WHEN 'done' THEN 1 WHEN 'terminé' THEN 2 WHEN 'fixed' THEN 3 WHEN 'corrigé' THEN 4
			ELSE 5 END, position, name
		LIMIT 1) x)`

// defaultResolution marks Jira's default resolution as Solstice's, or
// guesses one when Jira has none.
func (r *run) defaultResolution() error {
	var id sql.NullString
	if r.hasColumn("propertyentry", "property_key") && r.hasColumn("propertystring", "propertyvalue") {
		err := r.src.QueryRowContext(r.ctx, `SELECT ps.propertyvalue FROM propertyentry pe
			JOIN propertystring ps ON ps.id = pe.id
			WHERE pe.entity_name = 'jira.properties' AND pe.entity_id = 1
			AND pe.property_key = 'jira.constant.default.resolution'`).Scan(&id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("reading Jira's default resolution: %w", err)
		}
	}
	if r.resolutions[id.String] {
		_, err := r.dst.ExecContext(r.ctx, `UPDATE resolutions SET is_default = TRUE WHERE id = ?`, id.String)
		return err
	}
	_, err := r.dst.ExecContext(r.ctx, guessDefaultResolution)
	return err
}

func (r *run) importCatalogues() error {
	r.issueTypes = map[string]bool{}
	ins := r.inserter("issue_types", "id", "name", "description", "subtask", "position")
	if err := r.each(`SELECT id, pname, description, pstyle, sequence FROM issuetype`, func(rows *sql.Rows) error {
		var id string
		var name, desc, style sql.NullString
		var seq sql.NullInt64
		if err := rows.Scan(&id, &name, &desc, &style, &seq); err != nil {
			return err
		}
		r.issueTypes[id] = true
		return ins.add(id, truncate(orDefault(name, id), 255), sqlq.Value(desc), style.String == "jira_subtask", seq.Int64)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("issue types", ins.total, 0)

	r.statuses = map[string]bool{}
	ins = r.inserter("statuses", "id", "name", "description", "category", "position")
	hasCategory := r.hasColumn("issuestatus", "statuscategory")
	q := `SELECT id, pname, description, sequence, NULL FROM issuestatus`
	if hasCategory {
		q = `SELECT id, pname, description, sequence, statuscategory FROM issuestatus`
	}
	if err := r.each(q, func(rows *sql.Rows) error {
		var id string
		var name, desc sql.NullString
		var seq, cat sql.NullInt64
		if err := rows.Scan(&id, &name, &desc, &seq, &cat); err != nil {
			return err
		}
		r.statuses[id] = true
		return ins.add(id, truncate(orDefault(name, id), 255), sqlq.Value(desc), string(issue.FromJiraStatusCategory(cat.Int64)),
			seq.Int64)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("statuses", ins.total, 0)

	r.priorities = map[string]bool{}
	ins = r.inserter("priorities", "id", "name", "description", "color", "position")
	if err := r.each(`SELECT id, pname, description, status_color, sequence FROM priority`, func(rows *sql.Rows) error {
		var id string
		var name, desc, color sql.NullString
		var seq sql.NullInt64
		if err := rows.Scan(&id, &name, &desc, &color, &seq); err != nil {
			return err
		}
		r.priorities[id] = true
		return ins.add(id, truncate(orDefault(name, id), 255), sqlq.Value(desc), sqlq.Value(color), seq.Int64)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("priorities", ins.total, 0)

	r.resolutions = map[string]bool{}
	ins = r.inserter("resolutions", "id", "name", "description", "position")
	if err := r.each(`SELECT id, pname, description, sequence FROM resolution`, func(rows *sql.Rows) error {
		var id string
		var name, desc sql.NullString
		var seq sql.NullInt64
		if err := rows.Scan(&id, &name, &desc, &seq); err != nil {
			return err
		}
		r.resolutions[id] = true
		return ins.add(id, truncate(orDefault(name, id), 255), sqlq.Value(desc), seq.Int64)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("resolutions", ins.total, 0)
	if err := r.defaultResolution(); err != nil {
		return err
	}

	// Jira encodes sub-tasks as links of a hidden type; Solstice has a parent
	// column instead, so those types are not copied.
	r.linkTypes = map[int64]bool{}
	r.subtaskLink = map[int64]bool{}
	r.epicLink = map[int64]bool{}
	ins = r.inserter("link_types", "id", "name", "inward", "outward", "position")
	pos := 0
	if err := r.each(`SELECT id, linkname, inward, outward, pstyle FROM issuelinktype ORDER BY id`, func(rows *sql.Rows) error {
		var id int64
		var name, inward, outward, style sql.NullString
		if err := rows.Scan(&id, &name, &inward, &outward, &style); err != nil {
			return err
		}
		switch style.String {
		case "jira_subtask":
			r.subtaskLink[id] = true
			return nil
		case "jira_gh_epic_story":
			// Epic → issue links become the issue's epic.
			r.epicLink[id] = true
			return nil
		}
		r.linkTypes[id] = true
		pos++
		return ins.add(id, truncate(name.String, 255), truncate(inward.String, 255), truncate(outward.String, 255), pos)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("link types", ins.total, 0)
	return nil
}

func (r *run) importProjects() error {
	cats := map[int64]bool{}
	ins := r.inserter("project_categories", "id", "name", "description")
	if err := r.each(`SELECT id, cname, description FROM projectcategory`, func(rows *sql.Rows) error {
		var id int64
		var name, desc sql.NullString
		if err := rows.Scan(&id, &name, &desc); err != nil {
			return err
		}
		cats[id] = true
		return ins.add(id, truncate(name.String, 255), sqlq.Value(desc))
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}

	projectCategory, err := r.projectAssociations("ProjectCategory")
	if err != nil {
		return err
	}

	originalKey := "pkey"
	if r.hasColumn("project", "originalkey") {
		originalKey = "COALESCE(originalkey, pkey)"
	}
	projectType := "NULL"
	if r.hasColumn("project", "projecttype") {
		projectType = "projecttype"
	}
	r.projects = map[int64]*project{}
	ins = r.inserter("projects", "id", "pkey", "name", "description", "lead_id", "category_id",
		"url", "kind", "default_assignee", "issue_counter")
	if err := r.each(`SELECT id, pkey, pname, description, `+r.quote("lead")+`, url, pcounter, assigneetype, `+
		projectType+`, `+originalKey+` FROM project`, func(rows *sql.Rows) error {
		var id int64
		var key, name, desc, lead, url, kind, orig sql.NullString
		var counter, assigneeType sql.NullInt64
		if err := rows.Scan(&id, &key, &name, &desc, &lead, &url, &counter, &assigneeType, &kind, &orig); err != nil {
			return err
		}
		leadID, err := r.user(lead)
		if err != nil {
			return err
		}
		p := &project{key: key.String, originalKey: orDefault(orig, key.String), maxNum: counter.Int64}
		if id, ok := leadID.(int64); ok {
			p.leadID = id
		}
		r.projects[id] = p
		var category any
		if c, ok := projectCategory[id]; ok && cats[c] {
			category = c
		}
		defaultAssignee := "unassigned"
		if assigneeType.Int64 == 2 {
			defaultAssignee = "lead"
		}
		return ins.add(id, key.String, truncate(orDefault(name, key.String), 255), sqlq.Value(desc), leadID, category,
			sqlq.Value(url), orDefault(kind, "software"), defaultAssignee, counter.Int64)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("projects", ins.total, 0)

	// Former keys of renamed projects, so old issue keys keep working.
	ins = r.inserter("project_keys", "pkey", "project_id")
	seen := map[string]bool{}
	if err := r.each(`SELECT project_id, project_key FROM project_key`, func(rows *sql.Rows) error {
		var pid int64
		var key sql.NullString
		if err := rows.Scan(&pid, &key); err != nil {
			return err
		}
		p, ok := r.projects[pid]
		if !ok || key.String == "" || key.String == p.key || seen[key.String] {
			return nil
		}
		seen[key.String] = true
		return ins.add(key.String, pid)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("old project keys", ins.total, 0)
	return nil
}

// projectAssociations reads Jira's generic project → configuration links
// (nodeassociation): which category, permission scheme, etc. a project uses.
func (r *run) projectAssociations(sinkEntity string) (map[int64]int64, error) {
	out := map[int64]int64{}
	err := r.each(`SELECT source_node_id, sink_node_id FROM nodeassociation
		WHERE source_node_entity = 'Project' AND sink_node_entity = '`+sinkEntity+`'`, func(rows *sql.Rows) error {
		var project, sink int64
		if err := rows.Scan(&project, &sink); err != nil {
			return err
		}
		out[project] = sink
		return nil
	})
	return out, err
}

func (r *run) importRoles() error {
	r.roles = map[int64]bool{}
	ins := r.inserter("project_roles", "id", "name", "description")
	if err := r.each(`SELECT id, name, description FROM projectrole`, func(rows *sql.Rows) error {
		var id int64
		var name, desc sql.NullString
		if err := rows.Scan(&id, &name, &desc); err != nil {
			return err
		}
		r.roles[id] = true
		return ins.add(id, truncate(name.String, 255), sqlq.Value(desc))
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("roles", ins.total, 0)

	ins = r.inserter("project_role_members", "project_id", "role_id", "user_id", "group_id")
	skipped := 0
	if err := r.each(`SELECT pid, projectroleid, roletype, roletypeparameter FROM projectroleactor
		WHERE pid IS NOT NULL`, func(rows *sql.Rows) error {
		var pid, role int64
		var kind, param sql.NullString
		if err := rows.Scan(&pid, &role, &kind, &param); err != nil {
			return err
		}
		if r.projects[pid] == nil || !r.roles[role] {
			skipped++
			return nil
		}
		switch kind.String {
		case "atlassian-user-role-actor":
			if uid, ok := r.users[param.String]; ok {
				return ins.add(pid, role, uid, nil)
			}
		case "atlassian-group-role-actor":
			if gid, ok := r.groups[strings.ToLower(param.String)]; ok {
				return ins.add(pid, role, nil, gid)
			}
		}
		skipped++
		return nil
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("role members", ins.total, skipped)
	return nil
}

// importProjectIssueTypes resolves each project's issue type scheme into the
// list of issue types offered in that project.
func (r *run) importProjectIssueTypes() error {
	var defaultScheme int64 = -1
	schemeOf := map[int64]int64{}
	if err := r.each(`SELECT project, fieldconfigscheme FROM configurationcontext
		WHERE customfield = 'issuetype'`, func(rows *sql.Rows) error {
		var project, scheme sql.NullInt64
		if err := rows.Scan(&project, &scheme); err != nil {
			return err
		}
		if !project.Valid {
			defaultScheme = scheme.Int64
		} else {
			schemeOf[project.Int64] = scheme.Int64
		}
		return nil
	}); err != nil {
		return err
	}
	configOf := map[int64]int64{}
	if err := r.each(`SELECT fieldconfigscheme, fieldconfiguration FROM fieldconfigschemeissuetype
		WHERE issuetype IS NULL`, func(rows *sql.Rows) error {
		var scheme, config sql.NullInt64
		if err := rows.Scan(&scheme, &config); err != nil {
			return err
		}
		configOf[scheme.Int64] = config.Int64
		return nil
	}); err != nil {
		return err
	}
	type option struct {
		typeID string
		seq    int64
	}
	optionsOf := map[int64][]option{}
	if err := r.each(`SELECT fieldconfig, optionid, sequence FROM optionconfiguration
		WHERE fieldid = 'issuetype'`, func(rows *sql.Rows) error {
		var config, seq sql.NullInt64
		var typeID sql.NullString
		if err := rows.Scan(&config, &typeID, &seq); err != nil {
			return err
		}
		optionsOf[config.Int64] = append(optionsOf[config.Int64], option{typeID.String, seq.Int64})
		return nil
	}); err != nil {
		return err
	}
	used := map[int64][]string{}
	if err := r.each(`SELECT DISTINCT project, issuetype FROM jiraissue`, func(rows *sql.Rows) error {
		var project sql.NullInt64
		var typeID sql.NullString
		if err := rows.Scan(&project, &typeID); err != nil {
			return err
		}
		used[project.Int64] = append(used[project.Int64], typeID.String)
		return nil
	}); err != nil {
		return err
	}

	allTypes := make([]string, 0, len(r.issueTypes))
	for id := range r.issueTypes {
		allTypes = append(allTypes, id)
	}
	sort.Strings(allTypes)

	ins := r.inserter("project_issue_types", "project_id", "issue_type_id", "position")
	for _, pid := range r.projectIDs() {
		scheme, ok := schemeOf[pid]
		if !ok {
			scheme = defaultScheme
		}
		opts := slices.Clone(optionsOf[configOf[scheme]])
		sort.SliceStable(opts, func(i, j int) bool { return opts[i].seq < opts[j].seq })
		var types []string
		for _, o := range opts {
			types = append(types, o.typeID)
		}
		if len(types) == 0 {
			types = slices.Clone(allTypes)
		}
		// Issues may use a type the scheme no longer lists: keep it offered.
		for _, t := range used[pid] {
			if !slices.Contains(types, t) {
				types = append(types, t)
			}
		}
		for i, t := range types {
			if !r.issueTypes[t] {
				continue
			}
			if err := ins.add(pid, t, i); err != nil {
				return err
			}
		}
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("issue types per project", ins.total, 0)
	return nil
}

// holder converts a Jira grantee (permission or security level) into a Solstice
// grant. Grantees Solstice cannot express (custom-field based ones, Service
// Management portal access) are refused, which only ever removes access.
func (r *run) holder(kind, param sql.NullString) ([]perm.Grant, bool) {
	one := func(g perm.Grant, ok bool) ([]perm.Grant, bool) { return []perm.Grant{g}, ok }
	switch kind.String {
	case "applicationRole":
		// No parameter: anyone with access to Jira, i.e. any active Solstice
		// account (accounts without access are imported inactive). With one:
		// the users of that application only, i.e. its licensed groups.
		if param.String == "" {
			return one(perm.Grant{Holder: perm.Everyone}, true)
		}
		var grants []perm.Grant
		for _, gid := range r.licences[param.String] {
			grants = append(grants, perm.Grant{Holder: perm.Group, GroupID: gid})
		}
		return grants, len(grants) > 0
	case "group":
		if param.String == "" {
			return one(perm.Grant{Holder: perm.Everyone}, true)
		}
		gid, ok := r.groups[strings.ToLower(param.String)]
		return one(perm.Grant{Holder: perm.Group, GroupID: gid}, ok)
	case "user":
		uid, ok := r.users[param.String]
		return one(perm.Grant{Holder: perm.User, UserID: uid}, ok)
	case "projectrole":
		rid, err := strconv.ParseInt(param.String, 10, 64)
		return one(perm.Grant{Holder: perm.Role, RoleID: rid}, err == nil && r.roles[rid])
	case "lead":
		return one(perm.Grant{Holder: perm.Lead}, true)
	case "reporter":
		return one(perm.Grant{Holder: perm.Reporter}, true)
	case "assignee":
		return one(perm.Grant{Holder: perm.Assignee}, true)
	}
	return nil, false
}

func grantArgs(g perm.Grant) (any, any, any) {
	var u, gr, ro any
	if g.UserID != 0 {
		u = g.UserID
	}
	if g.GroupID != 0 {
		gr = g.GroupID
	}
	if g.RoleID != 0 {
		ro = g.RoleID
	}
	return u, gr, ro
}

// requireSchema refuses a Jira database older than version 7.
func (r *run) requireSchema() error {
	if !r.hasColumn("schemepermissions", "permission_key") {
		return errors.New("this Jira version is too old (no schemepermissions.permission_key): upgrade Jira to 7 or later first")
	}
	return nil
}

// importPermissions copies each project's permission scheme into the
// project's own grants.
func (r *run) importPermissions() error {
	if err := r.requireSchema(); err != nil {
		return err
	}
	type grant struct {
		key         string
		kind, param sql.NullString
	}
	byScheme := map[int64][]grant{}
	if err := r.each(`SELECT scheme, permission_key, perm_type, perm_parameter FROM schemepermissions`, func(rows *sql.Rows) error {
		var scheme sql.NullInt64
		var key, kind, param sql.NullString
		if err := rows.Scan(&scheme, &key, &kind, &param); err != nil {
			return err
		}
		if scheme.Valid {
			byScheme[scheme.Int64] = append(byScheme[scheme.Int64], grant{key.String, kind, param})
		}
		return nil
	}); err != nil {
		return err
	}
	schemeOf, err := r.projectAssociations("PermissionScheme")
	if err != nil {
		return err
	}

	ins := r.inserter("project_permissions", "project_id", "permission", "holder", "user_id", "group_id", "role_id")
	unsupported := map[string]int{}
	for _, pid := range r.projectIDs() {
		p := r.projects[pid]
		scheme, ok := schemeOf[pid]
		if !ok {
			scheme = 0 // Jira's default permission scheme
		}
		grants, ok := byScheme[scheme]
		if !ok {
			r.rep.warn("project %s: no permission scheme in Jira, nobody has access "+
				"until an administrator configures it", p.key)
			continue
		}
		seen := map[string]bool{}
		add := func(sp string, h perm.Grant) error {
			dedupe := fmt.Sprint(sp, h)
			if seen[dedupe] {
				return nil
			}
			seen[dedupe] = true
			u, gr, ro := grantArgs(h)
			return ins.add(pid, sp, h.Holder, u, gr, ro)
		}
		editAll, deleteAll := map[perm.Grant]bool{}, map[perm.Grant]bool{}
		for _, g := range grants {
			sp, mapped := jiraPermissions[g.key]
			moderation := g.key == "EDIT_ALL_COMMENTS" || g.key == "DELETE_ALL_COMMENTS"
			if !mapped && !moderation {
				continue
			}
			holders, ok := r.holder(g.kind, g.param)
			if !ok {
				unsupported[g.kind.String]++
				continue
			}
			for _, h := range holders {
				switch {
				case g.key == "EDIT_ALL_COMMENTS":
					editAll[h] = true
				case g.key == "DELETE_ALL_COMMENTS":
					deleteAll[h] = true
				default:
					if err := add(sp, h); err != nil {
						return err
					}
				}
			}
		}
		for h := range editAll {
			if deleteAll[h] {
				if err := add(perm.Moderate, h); err != nil {
					return err
				}
			}
		}
	}
	if err := ins.flush(); err != nil {
		return err
	}
	skipped := 0
	for kind, n := range unsupported {
		skipped += n
		r.rep.warn("%s of type \"%s\" not imported (access removed, to be reconfigured if needed)",
			plural(n, "Jira permission", "Jira permissions"), kind)
	}
	r.rep.step("project permissions", ins.total, skipped)
	return nil
}

// importSecurityLevels copies each project's issue security scheme into the
// project's own levels. A Jira level shared by several projects becomes one
// level per project.
func (r *run) importSecurityLevels() error {
	type lvl struct {
		id         int64
		name, desc sql.NullString
	}
	levelsOf := map[int64][]lvl{}
	if err := r.each(`SELECT id, name, description, scheme FROM schemeissuesecuritylevels`, func(rows *sql.Rows) error {
		var l lvl
		var scheme sql.NullInt64
		if err := rows.Scan(&l.id, &l.name, &l.desc, &scheme); err != nil {
			return err
		}
		levelsOf[scheme.Int64] = append(levelsOf[scheme.Int64], l)
		return nil
	}); err != nil {
		return err
	}
	type member struct{ kind, param sql.NullString }
	membersOf := map[int64][]member{}
	if err := r.each(`SELECT security, sec_type, sec_parameter FROM schemeissuesecurities`, func(rows *sql.Rows) error {
		var level sql.NullInt64
		var m member
		if err := rows.Scan(&level, &m.kind, &m.param); err != nil {
			return err
		}
		membersOf[level.Int64] = append(membersOf[level.Int64], m)
		return nil
	}); err != nil {
		return err
	}
	schemeOf, err := r.projectAssociations("IssueSecurityScheme")
	if err != nil {
		return err
	}

	r.levels = map[[2]int64]int64{}
	lins := r.inserter("security_levels", "id", "project_id", "name", "description")
	mins := r.inserter("security_level_members", "level_id", "holder", "user_id", "group_id", "role_id")
	var next int64
	skipped := 0
	for _, pid := range r.projectIDs() {
		scheme, ok := schemeOf[pid]
		if !ok {
			continue
		}
		for _, l := range levelsOf[scheme] {
			next++
			r.levels[[2]int64{l.id, pid}] = next
			if err := lins.add(next, pid, truncate(orDefault(l.name, "Level "+strconv.FormatInt(l.id, 10)), 255), sqlq.Value(l.desc)); err != nil {
				return err
			}
			for _, m := range membersOf[l.id] {
				holders, ok := r.holder(m.kind, m.param)
				if !ok {
					skipped++
					continue
				}
				for _, h := range holders {
					u, gr, ro := grantArgs(h)
					if err := mins.add(next, h.Holder, u, gr, ro); err != nil {
						return err
					}
				}
			}
		}
	}
	r.nextLevelID = next
	if err := lins.flush(); err != nil {
		return err
	}
	if err := mins.flush(); err != nil {
		return err
	}
	r.rep.step("security levels", lins.total, 0)
	r.rep.step("level members", mins.total, skipped)
	if skipped > 0 {
		r.rep.warn("%s of security levels not imported (unsupported type): these issues are "+
			"visible by fewer people than before, never by more", plural(skipped, "member", "members"))
	}
	return nil
}

// securityLevel returns the Solstice level of a secured issue. A level Jira no
// longer knows becomes a project level without members: nobody sees the
// issue until an administrator decides, rather than everybody.
func (r *run) securityLevel(jiraLevel sql.NullInt64, pid int64) (any, error) {
	if !jiraLevel.Valid || jiraLevel.Int64 == 0 {
		return nil, nil
	}
	k := [2]int64{jiraLevel.Int64, pid}
	if id, ok := r.levels[k]; ok {
		return id, nil
	}
	r.nextLevelID++
	id := r.nextLevelID
	if _, err := r.dst.ExecContext(r.ctx, `INSERT INTO security_levels (id, project_id, name, description)
		VALUES (?, ?, ?, ?)`, id, pid, "Restricted (import)",
		"Security level not found in Jira: no member."); err != nil {
		return nil, err
	}
	r.levels[k] = id
	r.rep.warn("project %s: a security level not found in Jira was replaced by \"Restricted (import)\", with no member",
		r.projects[pid].key)
	return id, nil
}

func (r *run) importComponentsAndVersions() error {
	archived := "NULL"
	if r.hasColumn("component", "archived") {
		archived = "archived"
	}
	deleted := "NULL"
	if r.hasColumn("component", "deleted") {
		deleted = "deleted"
	}
	r.components = map[int64]bool{}
	ins := r.inserter("components", "id", "project_id", "name", "description", "lead_id", "archived")
	skipped := 0
	if err := r.each(`SELECT id, project, cname, description, `+r.quote("lead")+`, `+archived+`, `+deleted+`
		FROM component`, func(rows *sql.Rows) error {
		var id int64
		var pid sql.NullInt64
		var name, desc, lead, arch, del sql.NullString
		if err := rows.Scan(&id, &pid, &name, &desc, &lead, &arch, &del); err != nil {
			return err
		}
		if r.projects[pid.Int64] == nil {
			skipped++
			return nil
		}
		leadID, err := r.user(lead)
		if err != nil {
			return err
		}
		r.components[id] = true
		return ins.add(id, pid.Int64, truncate(name.String, 255), sqlq.Value(desc), leadID, truthy(arch) || truthy(del))
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("components", ins.total, skipped)

	startDate := "NULL"
	if r.hasColumn("projectversion", "startdate") {
		startDate = "startdate"
	}
	r.versions = map[int64]bool{}
	ins = r.inserter("versions", "id", "project_id", "name", "description", "position", "released", "archived",
		"start_date", "release_date")
	skipped = 0
	if err := r.each(`SELECT id, project, vname, description, sequence, released, archived, `+startDate+`, releasedate
		FROM projectversion`, func(rows *sql.Rows) error {
		var id int64
		var pid, seq sql.NullInt64
		var name, desc, released, arch sql.NullString
		var start, release sql.NullTime
		if err := rows.Scan(&id, &pid, &name, &desc, &seq, &released, &arch, &start, &release); err != nil {
			return err
		}
		if r.projects[pid.Int64] == nil {
			skipped++
			return nil
		}
		r.versions[id] = true
		return ins.add(id, pid.Int64, truncate(name.String, 255), sqlq.Value(desc), seq.Int64, truthy(released), truthy(arch),
			r.ndate(start), r.ndate(release))
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("versions", ins.total, skipped)
	return nil
}

func (r *run) importCustomFields() error {
	r.fields = map[int64]string{}
	ins := r.inserter("custom_fields", "id", "type_key", "searcher_key", "name", "description")
	if err := r.each(`SELECT id, customfieldtypekey, customfieldsearcherkey, cfname, description FROM customfield`, func(rows *sql.Rows) error {
		var id int64
		var typeKey, searcher, name, desc sql.NullString
		if err := rows.Scan(&id, &typeKey, &searcher, &name, &desc); err != nil {
			return err
		}
		r.fields[id] = typeKey.String
		return ins.add(id, truncate(typeKey.String, 255), sqlq.Value(searcher), truncate(orDefault(name, "customfield_"+strconv.FormatInt(id, 10)), 255), sqlq.Value(desc))
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("custom fields", ins.total, 0)

	type opt struct {
		id, field, parent, seq int64
		value                  string
		disabled               bool
	}
	var opts []opt
	r.options = map[int64]bool{}
	skipped := 0
	if err := r.each(`SELECT id, customfield, parentoptionid, sequence, customvalue, disabled FROM customfieldoption`, func(rows *sql.Rows) error {
		var o opt
		var field, parent, seq sql.NullInt64
		var value, disabled sql.NullString
		if err := rows.Scan(&o.id, &field, &parent, &seq, &value, &disabled); err != nil {
			return err
		}
		if _, ok := r.fields[field.Int64]; !ok {
			skipped++
			return nil
		}
		o.field, o.parent, o.seq, o.value, o.disabled = field.Int64, parent.Int64, seq.Int64, value.String, truthy(disabled)
		r.options[o.id] = true
		opts = append(opts, o)
		return nil
	}); err != nil {
		return err
	}
	ins = r.inserter("custom_field_options", "id", "field_id", "parent_id", "position", "value", "disabled")
	for _, o := range opts {
		var parent any
		if o.parent != 0 && r.options[o.parent] {
			parent = o.parent
		}
		if err := ins.add(o.id, o.field, parent, o.seq, truncate(o.value, 255), o.disabled); err != nil {
			return err
		}
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("field options", ins.total, skipped)
	return nil
}
