package importer

import (
	"database/sql"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"solstice/internal/issue"
	"solstice/internal/sqlq"
)

func (r *run) importIssues() error {
	// Jira stores "B is a sub-task of A" as a link of a hidden type.
	parentOf := map[int64]int64{}
	if len(r.subtaskLink) > 0 {
		var ids []string
		for id := range r.subtaskLink {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
		if err := r.each(`SELECT source, destination FROM issuelink WHERE linktype IN (`+strings.Join(ids, ", ")+`)`, func(rows *sql.Rows) error {
			var parent, child sql.NullInt64
			if err := rows.Scan(&parent, &child); err != nil {
				return err
			}
			parentOf[child.Int64] = parent.Int64
			return nil
		}); err != nil {
			return err
		}
	}

	archived := "NULL"
	if r.hasColumn("jiraissue", "archived") {
		archived = "archived"
	}
	missing := map[string]map[string]bool{
		"issue_types": {}, "statuses": {}, "priorities": {}, "resolutions": {},
	}
	known := map[string]map[string]bool{
		"issue_types": r.issueTypes, "statuses": r.statuses, "priorities": r.priorities, "resolutions": r.resolutions,
	}
	ref := func(table string, id sql.NullString) any {
		if !id.Valid || id.String == "" {
			return nil
		}
		if !known[table][id.String] {
			missing[table][id.String] = true
		}
		return id.String
	}

	r.issues = map[int64]issueRef{}
	ins := r.inserter("issues", "id", "project_id", "num", "issue_type_id", "parent_id", "summary", "description",
		"environment", "status_id", "priority_id", "resolution_id", "reporter_id", "assignee_id", "creator_id",
		"security_level_id", "created_at", "updated_at", "resolved_at", "due_date", "original_estimate",
		"remaining_estimate", "time_spent", "archived")
	skipped := 0
	now := time.Now().UTC()
	if err := r.each(`SELECT id, project, issuenum, issuetype, summary, description, environment, issuestatus,
		priority, resolution, reporter, assignee, creator, security, created, updated, resolutiondate, duedate,
		timeoriginalestimate, timeestimate, timespent, `+archived+` FROM jiraissue ORDER BY id`, func(rows *sql.Rows) error {
		var id int64
		var pid, num, security, estimate, remaining, spent sql.NullInt64
		var issueType, summary, desc, env, status, priority, resolution, reporter, assignee, creator, arch sql.NullString
		var created, updated, resolved, due sql.NullTime
		if err := rows.Scan(&id, &pid, &num, &issueType, &summary, &desc, &env, &status, &priority, &resolution,
			&reporter, &assignee, &creator, &security, &created, &updated, &resolved, &due,
			&estimate, &remaining, &spent, &arch); err != nil {
			return err
		}
		p := r.projects[pid.Int64]
		if p == nil || !num.Valid {
			skipped++
			return nil
		}
		reporterID, err := r.user(reporter)
		if err != nil {
			return err
		}
		assigneeID, err := r.user(assignee)
		if err != nil {
			return err
		}
		creatorID, err := r.user(creator)
		if err != nil {
			return err
		}
		level, err := r.securityLevel(security, pid.Int64)
		if err != nil {
			return err
		}
		createdAt := now
		if created.Valid {
			createdAt = created.Time.UTC()
		} else if updated.Valid {
			createdAt = updated.Time.UTC()
		}
		updatedAt := createdAt
		if updated.Valid {
			updatedAt = updated.Time.UTC()
		}
		var parent any
		if pa, ok := parentOf[id]; ok {
			parent = pa
		}
		if !status.Valid || status.String == "" {
			status = sql.NullString{String: "unknown", Valid: true}
		}
		if !issueType.Valid || issueType.String == "" {
			issueType = sql.NullString{String: "unknown", Valid: true}
		}
		r.issues[id] = issueRef{project: pid.Int64, num: num.Int64}
		p.maxNum = max(p.maxNum, num.Int64)
		return ins.add(id, pid.Int64, num.Int64, ref("issue_types", issueType), parent,
			truncate(summary.String, 255), sqlq.Value(desc), sqlq.Value(env), ref("statuses", status),
			ref("priorities", priority), ref("resolutions", resolution), reporterID, assigneeID, creatorID,
			level, createdAt, updatedAt, ntime(resolved), r.ndate(due), sqlq.Value(estimate), sqlq.Value(remaining),
			sqlq.Value(spent), truthy(arch))
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("issues", ins.total, skipped)

	// Values used by issues but missing from Jira's catalogues.
	for _, table := range []string{"issue_types", "statuses", "priorities", "resolutions"} {
		ids := make([]string, 0, len(missing[table]))
		for id := range missing[table] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			q := "INSERT INTO " + table + " (id, name) VALUES (?, ?)"
			if table == "statuses" {
				q = "INSERT INTO statuses (id, name, category) VALUES (?, ?, 'todo')"
			}
			if _, err := r.dst.ExecContext(r.ctx, q, id, "Unknown ("+id+")"); err != nil {
				return err
			}
		}
		if len(ids) > 0 {
			r.rep.warn("%s from Jira in %s, recreated under the name \"Unknown (id)\"",
				plural(len(ids), "unknown value", "unknown values"), table)
		}
	}

	// A sub-task whose parent was not imported becomes a standalone issue.
	res, err := r.dst.ExecContext(r.ctx, `UPDATE issues c LEFT JOIN issues p ON p.id = c.parent_id
		SET c.parent_id = NULL WHERE c.parent_id IS NOT NULL AND p.id IS NULL`)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if r.check != nil { // nothing was updated: count the sub-tasks that would be
		for child, parent := range parentOf {
			if _, ok := r.issues[child]; ok {
				if _, ok := r.issues[parent]; !ok {
					n++
				}
			}
		}
	}
	if n > 0 {
		r.rep.warn("%s whose parent was not imported, now standalone",
			plural(int(n), "sub-task", "sub-tasks"))
	}
	for pid, p := range r.projects {
		if _, err := r.dst.ExecContext(r.ctx, `UPDATE projects SET issue_counter = GREATEST(issue_counter, ?)
			WHERE id = ?`, p.maxNum, pid); err != nil {
			return err
		}
	}

	moved := r.inserter("moved_issue_keys", "old_key", "issue_id")
	seen := map[string]bool{}
	skipped, gone, reused := 0, 0, 0
	// In order, so that a key given to two issues stays with the first.
	if err := r.each(`SELECT old_issue_key, issue_id FROM moved_issue_key ORDER BY id`, func(rows *sql.Rows) error {
		var key sql.NullString
		var issue sql.NullInt64
		if err := rows.Scan(&key, &issue); err != nil {
			return err
		}
		if _, ok := r.issues[issue.Int64]; !ok {
			gone++
		} else if seen[key.String] {
			reused++
		}
		if _, ok := r.issues[issue.Int64]; !ok || key.String == "" || seen[key.String] {
			skipped++
			return nil
		}
		seen[key.String] = true
		return moved.add(key.String, issue.Int64)
	}); err != nil {
		return err
	}
	if err := moved.flush(); err != nil {
		return err
	}
	r.rep.step("old keys of moved issues", moved.total, skipped)
	r.leftOut(gone, "old issue key", "old issue keys", "of a missing issue")
	r.leftOut(reused, "old issue key", "old issue keys", "already given to another issue (it keeps the first)")
	return nil
}

func (r *run) hasIssue(id sql.NullInt64) bool {
	_, ok := r.issues[id.Int64]
	return id.Valid && ok
}

// importIssueDetails copies labels, components, versions, links and watchers.
func (r *run) importIssueDetails() error {
	ins := r.inserter("labels", "issue_id", "field_id", "label")
	skipped := 0
	if err := r.each(`SELECT issue, fieldid, label FROM label`, func(rows *sql.Rows) error {
		var issue, field sql.NullInt64
		var label sql.NullString
		if err := rows.Scan(&issue, &field, &label); err != nil {
			return err
		}
		if !r.hasIssue(issue) || strings.TrimSpace(label.String) == "" {
			skipped++
			return nil
		}
		if field.Valid {
			if _, ok := r.fields[field.Int64]; !ok {
				skipped++
				return nil
			}
		}
		return ins.add(issue.Int64, sqlq.Value(field), truncate(label.String, 255))
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("labels", ins.total, skipped)

	comps := r.inserter("issue_components", "issue_id", "component_id")
	vers := r.inserter("issue_versions", "issue_id", "version_id", "kind")
	skipped, noComponent, noIssue := 0, 0, 0
	if err := r.each(`SELECT source_node_id, sink_node_id, association_type FROM nodeassociation
		WHERE source_node_entity = 'Issue' AND association_type IN ('IssueComponent', 'IssueVersion', 'IssueFixVersion')`,
		func(rows *sql.Rows) error {
			var issue, sink sql.NullInt64
			var kind sql.NullString
			if err := rows.Scan(&issue, &sink, &kind); err != nil {
				return err
			}
			if !r.hasIssue(issue) {
				skipped++
				noIssue++
				return nil
			}
			switch kind.String {
			case "IssueComponent":
				if r.components[sink.Int64] {
					return comps.add(issue.Int64, sink.Int64)
				}
				noComponent++
				return nil
			case "IssueVersion":
				if r.versions[sink.Int64] {
					return vers.add(issue.Int64, sink.Int64, "affects")
				}
			case "IssueFixVersion":
				if r.versions[sink.Int64] {
					return vers.add(issue.Int64, sink.Int64, "fix")
				}
			}
			skipped++
			return nil
		}); err != nil {
		return err
	}
	if err := comps.flush(); err != nil {
		return err
	}
	if err := vers.flush(); err != nil {
		return err
	}
	r.rep.step("issue components", comps.total, noComponent)
	r.rep.step("issue versions", vers.total, skipped)
	r.leftOut(noComponent, "component link", "component links", "to a missing component")
	r.leftOut(noIssue, "component or version link", "component or version links", "of a missing issue")

	r.epicOf = map[int64]int64{}
	links := r.inserter("issue_links", "id", "type_id", "source_id", "target_id")
	skipped = 0
	if err := r.each(`SELECT id, linktype, source, destination FROM issuelink`, func(rows *sql.Rows) error {
		var id int64
		var kind, source, target sql.NullInt64
		if err := rows.Scan(&id, &kind, &source, &target); err != nil {
			return err
		}
		if r.subtaskLink[kind.Int64] {
			return nil // imported as the parent column
		}
		if r.epicLink[kind.Int64] {
			if r.hasIssue(source) && r.hasIssue(target) {
				r.epicOf[target.Int64] = source.Int64
			}
			return nil // imported as the epic column
		}
		if !r.linkTypes[kind.Int64] || !r.hasIssue(source) || !r.hasIssue(target) {
			skipped++
			return nil
		}
		return links.add(id, kind.Int64, source.Int64, target.Int64)
	}); err != nil {
		return err
	}
	if err := links.flush(); err != nil {
		return err
	}
	r.rep.step("issue links", links.total, skipped)
	r.leftOut(skipped, "issue link", "issue links", "of a missing issue or link type")

	remote := r.inserter("remote_links", "id", "issue_id", "url", "title", "relationship", "global_id")
	globals := globalIDs{}
	skipped, noURL := 0, 0
	if err := r.each(`SELECT id, issueid, url, title, relationship, globalid FROM remotelink ORDER BY id`, func(rows *sql.Rows) error {
		var id int64
		var issue sql.NullInt64
		var url, title, rel, global sql.NullString
		if err := rows.Scan(&id, &issue, &url, &title, &rel, &global); err != nil {
			return err
		}
		if !r.hasIssue(issue) {
			skipped++
			return nil
		}
		if url.String == "" {
			skipped++
			noURL++
			return nil
		}
		return remote.add(id, issue.Int64, url.String, truncate(orDefault(title, url.String), 255), sqlq.Value(rel),
			globals.take(issue.Int64, global))
	}); err != nil {
		return err
	}
	if err := remote.flush(); err != nil {
		return err
	}
	r.rep.step("remote links", remote.total, skipped)
	r.leftOut(noURL, "remote link", "remote links", "without a URL")

	watch := r.inserter("watchers", "issue_id", "user_id", "created_at")
	vote := r.inserter("votes", "issue_id", "user_id", "created_at")
	skipped = 0
	if err := r.each(`SELECT source_name, sink_node_id, association_type, created FROM userassociation
		WHERE sink_node_entity = 'Issue' AND association_type IN ('WatchIssue', 'VoteIssue')`, func(rows *sql.Rows) error {
		var key, kind sql.NullString
		var issue sql.NullInt64
		var created sql.NullTime
		if err := rows.Scan(&key, &issue, &kind, &created); err != nil {
			return err
		}
		uid, ok := r.users[key.String]
		if !ok || !r.hasIssue(issue) {
			skipped++
			return nil
		}
		if kind.String == "WatchIssue" {
			return watch.add(issue.Int64, uid, ntime(created))
		}
		return vote.add(issue.Int64, uid, ntime(created))
	}); err != nil {
		return err
	}
	if err := watch.flush(); err != nil {
		return err
	}
	if err := vote.flush(); err != nil {
		return err
	}
	r.rep.step("watchers", watch.total, 0)
	r.rep.step("votes", vote.total, skipped)
	r.leftOut(skipped, "watch or vote", "watches or votes", "of an unknown user or a missing issue")
	return nil
}

// actionRefs are the Solstice values a comment or work log points to.
type actionRefs struct {
	author, updater, group, role any
}

// refsOf resolves the author, last editor and visibility of a comment or
// work log.
func (r *run) refsOf(author, updater, group sql.NullString, role sql.NullInt64) (actionRefs, error) {
	var ref actionRefs
	var err error
	if ref.author, err = r.user(author); err != nil {
		return ref, err
	}
	if ref.updater, err = r.user(updater); err != nil {
		return ref, err
	}
	if ref.role, err = r.visibilityRole(role); err != nil {
		return ref, err
	}
	if group.String != "" {
		ref.group = strings.ToLower(group.String)
	}
	return ref, nil
}

// visibilityRole returns the role a comment or work log is restricted to. A
// role deleted from Jira is recreated without members, so the restricted
// content stays hidden instead of becoming public.
func (r *run) visibilityRole(role sql.NullInt64) (any, error) {
	if !role.Valid || role.Int64 == 0 {
		return nil, nil
	}
	if !r.roles[role.Int64] {
		if _, err := r.dst.ExecContext(r.ctx, `INSERT INTO project_roles (id, name) VALUES (?, ?)`,
			role.Int64, fmt.Sprintf("Deleted role (%d)", role.Int64)); err != nil {
			return nil, err
		}
		r.roles[role.Int64] = true
	}
	return role.Int64, nil
}

func (r *run) importCommentsAndWorklogs() error {
	now := time.Now().UTC()
	stamp := func(primary, fallback sql.NullTime) time.Time {
		if primary.Valid {
			return primary.Time.UTC()
		}
		if fallback.Valid {
			return fallback.Time.UTC()
		}
		return now
	}

	ins := r.inserter("comments", "id", "issue_id", "author_id", "body", "created_at", "updated_at", "updated_by_id",
		"visibility_group", "visibility_role_id")
	skipped, restricted := 0, 0
	if err := r.each(`SELECT id, issueid, author, actionlevel, rolelevel, actionbody, created, updateauthor, updated
		FROM jiraaction WHERE actiontype = 'comment'`, func(rows *sql.Rows) error {
		var id int64
		var issue, role sql.NullInt64
		var author, group, body, updater sql.NullString
		var created, updated sql.NullTime
		if err := rows.Scan(&id, &issue, &author, &group, &role, &body, &created, &updater, &updated); err != nil {
			return err
		}
		if !r.hasIssue(issue) {
			skipped++
			return nil
		}
		ref, err := r.refsOf(author, updater, group, role)
		if err != nil {
			return err
		}
		if ref.group != nil || ref.role != nil {
			restricted++
		}
		c := stamp(created, updated)
		return ins.add(id, issue.Int64, ref.author, body.String, c, stamp(updated, created), ref.updater, ref.group, ref.role)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("comments", ins.total, skipped)
	r.leftOut(skipped, "comment", "comments", "of a missing issue")
	r.rep.step("of which restricted comments", restricted, 0)

	ins = r.inserter("worklogs", "id", "issue_id", "author_id", "body", "started_at", "seconds", "created_at",
		"updated_at", "updated_by_id", "visibility_group", "visibility_role_id")
	skipped = 0
	if err := r.each(`SELECT id, issueid, author, grouplevel, rolelevel, worklogbody, created, updateauthor, updated,
		startdate, timeworked FROM worklog`, func(rows *sql.Rows) error {
		var id int64
		var issue, role, seconds sql.NullInt64
		var author, group, body, updater sql.NullString
		var created, updated, started sql.NullTime
		if err := rows.Scan(&id, &issue, &author, &group, &role, &body, &created, &updater, &updated, &started, &seconds); err != nil {
			return err
		}
		if !r.hasIssue(issue) {
			skipped++
			return nil
		}
		ref, err := r.refsOf(author, updater, group, role)
		if err != nil {
			return err
		}
		c := stamp(created, updated)
		return ins.add(id, issue.Int64, ref.author, sqlq.Value(body), stamp(started, created), seconds.Int64, c,
			stamp(updated, created), ref.updater, ref.group, ref.role)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("worklogs", ins.total, skipped)
	r.leftOut(skipped, "work log", "work logs", "of a missing issue")
	return nil
}

func (r *run) importAttachments() error {
	dir := filepath.Join(r.opt.DataDir, "attachments")
	if r.check == nil {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}
	ins := r.inserter("attachments", "id", "issue_id", "author_id", "filename", "mime_type", "size", "created_at",
		"file_missing")
	skipped, missingFiles := 0, 0
	now := time.Now().UTC()
	if err := r.each(`SELECT id, issueid, mimetype, filename, created, filesize, author FROM fileattachment`, func(rows *sql.Rows) error {
		var id int64
		var issue, size sql.NullInt64
		var mime, name, author sql.NullString
		var created sql.NullTime
		if err := rows.Scan(&id, &issue, &mime, &name, &created, &size, &author); err != nil {
			return err
		}
		if !r.hasIssue(issue) {
			skipped++
			return nil
		}
		authorID, err := r.user(author)
		if err != nil {
			return err
		}
		found, err := r.copyAttachment(id, r.issues[issue.Int64], dir)
		if err != nil {
			return err
		}
		if !found {
			missingFiles++
		}
		r.check.attachment(found)
		at := now
		if created.Valid {
			at = created.Time.UTC()
		}
		return ins.add(id, issue.Int64, authorID, truncate(orDefault(name, "file"), 255),
			orDefault(mime, "application/octet-stream"), size.Int64, at, !found)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("attachments", ins.total, skipped)
	switch {
	case r.opt.JiraHome == "" && ins.total > 0:
		r.rep.warn("attachments: only the metadata is imported; run again with -jira-home to copy the files")
	case missingFiles > 0:
		r.rep.warn("%s in the Jira home (already missing on Jira's side?)",
			plural(missingFiles, "attachment without a file", "attachments without a file"))
	}
	return nil
}

// copyAttachment looks for an attachment in the Jira home and copies it to
// dir/<id>. Jira 7+ stores attachments under
// data/attachments/<project key>/<bucket of 10000>/<issue key>/<id>; older
// versions had no bucket. Renamed projects keep their original key on disk.
func (r *run) copyAttachment(id int64, issue issueRef, dir string) (bool, error) {
	if r.opt.JiraHome == "" {
		return false, nil
	}
	p := r.projects[issue.project]
	keys := []string{p.originalKey}
	if p.key != p.originalKey {
		keys = append(keys, p.key)
	}
	for _, k := range keys {
		if !safePathPart(k) {
			if !r.unsafeKeys[k] {
				if r.unsafeKeys == nil {
					r.unsafeKeys = map[string]bool{}
				}
				r.unsafeKeys[k] = true
				r.rep.warn("attachments: the project key %q cannot be used in a file path, its attachment files are not copied", k)
			}
			return false, nil
		}
	}
	// Symlinks are resolved on both sides: a link inside the home can point out of it.
	root, err := filepath.EvalSymlinks(r.opt.JiraHome)
	if err != nil {
		return false, nil
	}
	base := filepath.Join(root, "data", "attachments")
	bucket := strconv.FormatInt(((issue.num-1)/10000+1)*10000, 10)
	name := strconv.FormatInt(id, 10)
	for _, pk := range keys {
		for _, ik := range keys {
			issueKey := ik + "-" + strconv.FormatInt(issue.num, 10)
			for _, path := range []string{
				filepath.Join(base, pk, bucket, issueKey, name),
				filepath.Join(base, pk, issueKey, name),
			} {
				resolved, err := filepath.EvalSymlinks(path)
				if err != nil || !within(root, resolved) {
					continue
				}
				ok, err := r.fetchFile(resolved, filepath.Join(dir, name))
				if ok || err != nil {
					return ok, err
				}
			}
		}
	}
	return false, nil
}

// fetchFile copies src to dst. A check copies nothing: it only looks, and a
// file it cannot look at counts as missing instead of stopping it.
func (r *run) fetchFile(src, dst string) (bool, error) {
	if r.check == nil {
		return copyFile(src, dst)
	}
	info, err := os.Stat(src)
	if err != nil {
		if !os.IsNotExist(err) {
			r.check.unreadable(err)
		}
		return false, nil
	}
	return info.Mode().IsRegular(), nil
}

func safePathPart(s string) bool {
	return s != "" && s != "." && !strings.ContainsAny(s, "/\\\x00") && !strings.Contains(s, "..")
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// copyFile copies src to dst through a temporary file. A missing src is not
// an error: it returns false.
func copyFile(src, dst string) (bool, error) {
	in, err := os.Open(src) // #nosec G304 -- path built by the server, not from user input
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer in.Close()
	tmp := dst + ".part"
	out, err := os.Create(tmp) // #nosec G304 -- path built by the server, not from user input
	if err != nil {
		return false, err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return false, err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return false, err
	}
	return true, os.Rename(tmp, dst)
}

func (r *run) importCustomFieldValues() error {
	ins := r.inserter("custom_field_values", "id", "issue_id", "field_id", "parent_key", "string_value",
		"number_value", "text_value", "date_value")
	skipped, orphans, unknownUsers := 0, 0, 0
	misfit := map[int64]int{}
	if err := r.each(`SELECT id, issue, customfield, parentkey, stringvalue, numbervalue, textvalue, datevalue
		FROM customfieldvalue`, func(rows *sql.Rows) error {
		var id int64
		var issueID, field sql.NullInt64
		var parentKey, str, text sql.NullString
		var num sql.NullFloat64
		var date sql.NullTime
		if err := rows.Scan(&id, &issueID, &field, &parentKey, &str, &num, &text, &date); err != nil {
			return err
		}
		typeKey, ok := r.fields[field.Int64]
		if !ok || !r.hasIssue(issueID) {
			skipped++
			orphans++
			return nil
		}
		if !fitsType(typeKey, str, text, num, date) {
			skipped++
			misfit[field.Int64]++
			return nil
		}
		var strVal, dateVal, numVal any = sqlq.Value(str), ntime(date), nil
		if num.Valid {
			numVal = num.Float64
		}
		switch issue.FieldKind(typeKey) {
		case "gh-epic-link":
			// Older Jira Software kept the epic in this field rather than a link.
			if num.Valid && r.hasIssue(sql.NullInt64{Int64: int64(num.Float64), Valid: true}) {
				r.epicOf[issueID.Int64] = int64(num.Float64)
			}
		case "gh-sprint":
			r.check.sprintMember(issueID.Int64, str, num)
		case "userpicker", "multiuserpicker":
			uid, ok := r.users[str.String]
			if !ok {
				skipped++
				unknownUsers++
				return nil
			}
			strVal = strconv.FormatInt(uid, 10)
		case "datepicker":
			if date.Valid {
				y, m, d := date.Time.In(r.tz).Date()
				dateVal = time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
			}
		}
		if s, ok := strVal.(string); ok {
			strVal = truncate(s, 255)
		}
		return ins.add(id, issueID.Int64, field.Int64, sqlq.Value(parentKey), strVal, numVal, sqlq.Value(text), dateVal)
	}); err != nil {
		return err
	}
	if err := ins.flush(); err != nil {
		return err
	}
	r.rep.step("custom field values", ins.total, skipped)
	r.leftOut(orphans, "custom field value", "custom field values", "of a missing issue or field")
	r.leftOut(unknownUsers, "custom field value", "custom field values", "naming an unknown user")
	if len(misfit) > 0 {
		n := 0
		for _, c := range misfit {
			n += c
		}
		ids := make([]string, 0, len(misfit))
		for _, id := range slices.Sorted(maps.Keys(misfit)) {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
		r.rep.warn("%s stored for a former type of their custom field, not imported (fields %s)",
			plural(n, "value", "values"), strings.Join(ids, ", "))
	}
	return nil
}

// When the type of a field changes, Jira leaves the existing values in the
// column of the former type, which the current type never reads.
func fitsType(typeKey string, str, text sql.NullString, num sql.NullFloat64, date sql.NullTime) bool {
	if !str.Valid && !text.Valid && !num.Valid && !date.Valid {
		return true
	}
	switch issue.FieldKind(typeKey) {
	case "float":
		return num.Valid
	case "datepicker", "datetime":
		return date.Valid
	case "textfield", "url", "textarea":
		return str.Valid || text.Valid
	}
	return true
}

func (r *run) importHistory() error {
	groups := r.inserter("change_groups", "id", "issue_id", "author_id", "created_at")
	skipped := 0
	now := time.Now().UTC()
	if err := r.each(`SELECT id, issueid, author, created FROM changegroup`, func(rows *sql.Rows) error {
		var id int64
		var issue sql.NullInt64
		var author sql.NullString
		var created sql.NullTime
		if err := rows.Scan(&id, &issue, &author, &created); err != nil {
			return err
		}
		if !r.hasIssue(issue) {
			skipped++
			return nil
		}
		authorID, err := r.user(author)
		if err != nil {
			return err
		}
		at := now
		if created.Valid {
			at = created.Time.UTC()
		}
		r.check.changeGroup(id)
		return groups.add(id, issue.Int64, authorID, at)
	}); err != nil {
		return err
	}
	if err := groups.flush(); err != nil {
		return err
	}
	r.rep.step("changes (history)", groups.total, skipped)
	r.leftOut(skipped, "change", "changes", "of a missing issue")

	items := r.inserter("change_items", "id", "group_id", "field_type", "field", "old_value", "old_string",
		"new_value", "new_string")
	r.check.sortChangeGroups()
	if err := r.each(`SELECT id, groupid, fieldtype, field, oldvalue, oldstring, newvalue, newstring FROM changeitem`, func(rows *sql.Rows) error {
		var id int64
		var group sql.NullInt64
		var fieldType, field, oldValue, oldString, newValue, newString sql.NullString
		if err := rows.Scan(&id, &group, &fieldType, &field, &oldValue, &oldString, &newValue, &newString); err != nil {
			return err
		}
		r.check.changeItem(group.Int64)
		return items.add(id, group.Int64, truncate(orDefault(fieldType, "jira"), 255), truncate(field.String, 255),
			sqlq.Value(oldValue), sqlq.Value(oldString), sqlq.Value(newValue), sqlq.Value(newString))
	}); err != nil {
		return err
	}
	if err := items.flush(); err != nil {
		return err
	}
	// Change groups can number in the millions: rather than keeping their ids
	// in memory, items whose group was not imported are removed afterwards.
	res, err := r.dst.ExecContext(r.ctx, `DELETE ci FROM change_items ci
		LEFT JOIN change_groups g ON g.id = ci.group_id WHERE g.id IS NULL`)
	if err != nil {
		return err
	}
	orphans, _ := res.RowsAffected()
	if r.check != nil {
		orphans = r.check.orphanItems
	}
	r.rep.step("change details", items.total-int(orphans), int(orphans))
	r.leftOut(int(orphans), "change detail", "change details", "of a missing change")
	return nil
}
