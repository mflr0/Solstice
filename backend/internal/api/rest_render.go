package api

// Issues and what they hold, in the shapes of Jira's REST API.

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/jql"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// sysField is one of Jira's system fields, as Solstice keeps them.
type sysField struct {
	id        string
	name      i18n.Message
	typ       string // schema type: string, user, datetime, array…
	items     string // for arrays: the type of the items
	navigable bool   // part of "*navigable", the default of searches
	clauses   []string
}

// orderable reports whether ORDER BY accepts the field, as Jira's
// "orderable" means.
func (f sysField) orderable() bool {
	for _, c := range f.clauses {
		if jql.Sortable(c) {
			return true
		}
	}
	return false
}

var systemFields = []sysField{
	{"issuekey", i18n.M(i18n.FieldNameKey), "", "", true, []string{"id", "issue", "issuekey", "key"}},
	{"summary", i18n.M(i18n.FieldNameSummary), "string", "", true, []string{"summary"}},
	{"issuetype", i18n.M(i18n.FieldNameIssueType), "issuetype", "", true, []string{"issuetype", "type"}},
	{"project", i18n.M(i18n.FieldNameProject), "project", "", true, []string{"project"}},
	{"status", i18n.M(i18n.FieldNameStatus), "status", "", true, []string{"status"}},
	{"priority", i18n.M(i18n.FieldNamePriority), "priority", "", true, []string{"priority"}},
	{"resolution", i18n.M(i18n.FieldNameResolution), "resolution", "", true, []string{"resolution"}},
	{"resolutiondate", i18n.M(i18n.FieldNameResolved), "datetime", "", true, []string{"resolutiondate", "resolved"}},
	{"assignee", i18n.M(i18n.FieldNameAssignee), "user", "", true, []string{"assignee"}},
	{"reporter", i18n.M(i18n.FieldNameReporter), "user", "", true, []string{"reporter"}},
	{"creator", i18n.M(i18n.FieldNameCreator), "user", "", true, []string{"creator"}},
	{"created", i18n.M(i18n.FieldNameCreated), "datetime", "", true, []string{"created", "createdDate"}},
	{"updated", i18n.M(i18n.FieldNameUpdated), "datetime", "", true, []string{"updated", "updatedDate"}},
	{"duedate", i18n.M(i18n.FieldNameDueDate), "date", "", true, []string{"due", "duedate"}},
	{"description", i18n.M(i18n.FieldNameDescription), "string", "", true, []string{"description"}},
	{"environment", i18n.M(i18n.FieldNameEnvironment), "string", "", true, []string{"environment"}},
	{"labels", i18n.M(i18n.FieldNameLabels), "array", "string", true, []string{"labels"}},
	{"components", i18n.M(i18n.FieldNameComponents), "array", "component", true, []string{"component"}},
	{"fixVersions", i18n.M(i18n.FieldNameFixVersions), "array", "version", true, []string{"fixVersion"}},
	{"versions", i18n.M(i18n.FieldNameAffectsVersions), "array", "version", true, []string{"affectedVersion"}},
	{"issuelinks", i18n.M(i18n.FieldNameLinkedIssues), "array", "issuelinks", true, []string{"issueLink"}},
	{"subtasks", i18n.M(i18n.FieldNameSubtasks), "array", "issuelinks", true, []string{"subtasks"}},
	{"parent", i18n.M(i18n.FieldNameParent), "issuelink", "", true, []string{"parent"}},
	{"security", i18n.M(i18n.FieldNameSecurityLevel), "securitylevel", "", true, []string{"level"}},
	{"watches", i18n.M(i18n.FieldNameWatchers), "watches", "", true, []string{"watchers"}},
	{"votes", i18n.M(i18n.FieldNameVotes), "votes", "", true, []string{"votes"}},
	{"timeoriginalestimate", i18n.M(i18n.FieldNameOriginalEstimate), "number", "", true, []string{"originalEstimate"}},
	{"timeestimate", i18n.M(i18n.FieldNameRemainingEstimate), "number", "", true, []string{"remainingEstimate"}},
	{"timespent", i18n.M(i18n.FieldNameTimeSpent), "number", "", true, []string{"timeSpent"}},
	{"timetracking", i18n.M(i18n.FieldNameTimeTracking), "timetracking", "", false, nil},
	{"attachment", i18n.M(i18n.FieldNameAttachments), "array", "attachment", false, []string{"attachments"}},
	{"comment", i18n.M(i18n.FieldNameComments), "comments-page", "", false, []string{"comment"}},
	{"worklog", i18n.M(i18n.FieldNameWorkLog), "array", "worklog", false, nil},
}

func systemField(id string) (sysField, bool) {
	for _, f := range systemFields {
		if f.id == id {
			return f, true
		}
	}
	return sysField{}, false
}

// fieldSelection is Jira's "fields" parameter: *all, *navigable, ids, and
// -id to leave one out.
type fieldSelection struct {
	all, navigable   bool
	include, exclude map[string]bool
}

func parseFieldSelection(values []string, def string) *fieldSelection {
	sel := &fieldSelection{include: map[string]bool{}, exclude: map[string]bool{}}
	positive := false
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			part = strings.TrimSpace(part)
			switch {
			case part == "":
			case part == "*all":
				sel.all, positive = true, true
			case part == "*navigable":
				sel.navigable, positive = true, true
			case strings.HasPrefix(part, "-"):
				sel.exclude[part[1:]] = true
			default:
				sel.include[part] = true
				positive = true
			}
		}
	}
	if !positive {
		sel.all, sel.navigable = def == "*all", def == "*navigable"
	}
	return sel
}

func (sel *fieldSelection) has(id string, navigable bool) bool {
	if sel.exclude[id] {
		return false
	}
	return sel.include[id] || sel.all || (sel.navigable && navigable)
}

// hasSystem tells whether a system field is selected.
func (sel *fieldSelection) hasSystem(id string) bool {
	f, _ := systemField(id)
	return sel.has(id, f.navigable)
}

func customID(id int64) string { return "customfield_" + strconv.FormatInt(id, 10) }

// loads tells loadIssues what the selected fields need.
func (sel *fieldSelection) loads(defs []customFieldDef) func(string) bool {
	custom, epic, sprint := false, false, false
	for _, f := range defs {
		if sel.has(customID(f.ID), true) {
			custom = true
			epic = epic || f.Kind == issue.KindEpicLink
			sprint = sprint || f.Kind == issue.KindSprint
		}
	}
	need := map[string]bool{
		loadProjectKinds: sel.hasSystem("project"), loadLevels: sel.hasSystem("security"),
		loadLabels: sel.hasSystem("labels"), loadComponents: sel.hasSystem("components"),
		loadVersions: sel.hasSystem("fixVersions") || sel.hasSystem("versions"),
		loadLinks:    sel.hasSystem("issuelinks"), loadSubtasks: sel.hasSystem("subtasks"),
		loadParent: sel.hasSystem("parent"), loadEpic: epic, loadAttachments: sel.hasSystem("attachment"),
		loadCustom: custom, loadSprints: sprint, loadWatches: sel.hasSystem("watches"),
		loadVotes: sel.hasSystem("votes"), loadComments: sel.hasSystem("comment"),
		loadWorklogs: sel.hasSystem("worklog"),
	}
	return func(kind string) bool { return need[kind] }
}

// issueExpand lists the expansions Jira announces on every issue.
const issueExpand = "renderedFields,names,schema,operations,editmeta,changelog,versionedRepresentations"

// jiraIssue renders an issue with the selected fields.
func (s *Server) jiraIssue(o *restOut, cat *catalog, set *issueSet, users map[int64]*jiraUser, d *issueData,
	sel *fieldSelection) map[string]any {
	row := d.row
	f := map[string]any{}
	put := func(id string, v func() any) {
		if sel.hasSystem(id) {
			f[id] = v()
		}
	}
	put("summary", func() any { return row.Summary })
	put("issuetype", func() any { return o.issueType(cat.typeOf(row.TypeID)) })
	put("project", func() any {
		return o.projectRef(row.ProjectID, row.ProjectKey, row.ProjectName, set.projectKinds[row.ProjectID])
	})
	put("status", func() any { return o.status(cat.statusOf(row.StatusID)) })
	put("priority", func() any { return o.priority(cat.priorityOf(row.PriorityID)) })
	put("resolution", func() any { return o.resolution(cat.resolutionOf(row.ResolutionID)) })
	put("resolutiondate", func() any { return o.timePtr(sqlq.Time(row.Resolved)) })
	put("assignee", func() any { return userOf(users, row.AssigneeID.Int64) })
	put("reporter", func() any { return userOf(users, row.ReporterID.Int64) })
	put("creator", func() any { return userOf(users, row.CreatorID.Int64) })
	put("created", func() any { return o.time(row.Created) })
	put("updated", func() any { return o.time(row.Updated) })
	put("duedate", func() any {
		if d := dateOnly(row.Due); d != nil {
			return *d
		}
		return nil
	})
	put("description", func() any { return sqlq.Value(row.Description) })
	put("environment", func() any { return sqlq.Value(row.Environment) })
	put("labels", func() any { return d.labels })
	put("components", func() any {
		out := []any{}
		for _, c := range d.components {
			out = append(out, map[string]any{"self": o.api + "/component/" + o.id(c.ID), "id": o.id(c.ID), "name": c.Name})
		}
		return out
	})
	versions := func(list []versionData) any {
		out := []any{}
		for _, v := range list {
			out = append(out, o.version(v, row.ProjectID))
		}
		return out
	}
	put("fixVersions", func() any { return versions(d.fixVersions) })
	put("versions", func() any { return versions(d.versions) })
	put("issuelinks", func() any {
		out := []any{}
		for _, l := range d.links {
			lt := cat.linkType[l.typeID]
			link := map[string]any{"id": o.id(l.id), "self": o.api + "/issueLink/" + o.id(l.id), "type": o.linkType(lt)}
			if l.outward {
				link["outwardIssue"] = o.issueBrief(cat, l.other)
			} else {
				link["inwardIssue"] = o.issueBrief(cat, l.other)
			}
			out = append(out, link)
		}
		return out
	})
	put("subtasks", func() any {
		out := []any{}
		for _, st := range d.subtasks {
			out = append(out, o.issueBrief(cat, st))
		}
		return out
	})
	if d.parent != nil {
		put("parent", func() any { return o.issueBrief(cat, d.parent) })
	}
	put("security", func() any {
		if !row.LevelID.Valid {
			return nil
		}
		return map[string]any{"self": o.api + "/securitylevel/" + o.id(row.LevelID.Int64), "id": o.id(row.LevelID.Int64),
			"name": set.levels[row.LevelID.Int64], "description": ""}
	})
	put("watches", func() any {
		return map[string]any{"self": o.api + "/issue/" + row.Key() + "/watchers", "watchCount": d.watches.Count,
			"isWatching": d.watches.Me}
	})
	put("votes", func() any {
		return map[string]any{"self": o.api + "/issue/" + row.Key() + "/votes", "votes": d.votes.Count,
			"hasVoted": d.votes.Me}
	})
	put("timeoriginalestimate", func() any { return sqlq.Value(row.Original) })
	put("timeestimate", func() any { return sqlq.Value(row.Remaining) })
	put("timespent", func() any { return sqlq.Value(row.Spent) })
	put("timetracking", func() any {
		out := map[string]any{}
		for _, t := range []struct {
			name string
			v    sql.NullInt64
		}{{"originalEstimate", row.Original}, {"remainingEstimate", row.Remaining}, {"timeSpent", row.Spent}} {
			if t.v.Valid {
				out[t.name] = jiraDuration(t.v.Int64)
				out[t.name+"Seconds"] = t.v.Int64
			}
		}
		return out
	})
	put("attachment", func() any {
		out := []any{}
		for _, a := range d.attachments {
			out = append(out, o.attachment(a, users))
		}
		return out
	})
	put("comment", func() any {
		list := []any{}
		for _, c := range d.comments {
			list = append(list, o.comment(row, c, users))
		}
		return map[string]any{"comments": list, "maxResults": len(list), "total": len(list), "startAt": 0}
	})
	put("worklog", func() any {
		list := []any{}
		for _, w := range d.worklogs {
			list = append(list, o.worklog(row, w, users))
		}
		return map[string]any{"startAt": 0, "maxResults": len(list), "total": len(list), "worklogs": list}
	})
	for _, cf := range set.fields {
		id := customID(cf.ID)
		if sel.has(id, true) {
			f[id] = o.customValue(set, users, cf, d)
		}
	}
	return map[string]any{"expand": issueExpand, "id": o.id(row.ID), "self": o.api + "/issue/" + o.id(row.ID),
		"key": row.Key(), "fields": f}
}

func (o *restOut) projectRef(id int64, key, name, kind string) map[string]any {
	if kind == "" {
		kind = "software"
	}
	return map[string]any{"self": o.api + "/project/" + o.id(id), "id": o.id(id), "key": key, "name": name,
		"projectTypeKey": kind, "avatarUrls": o.avatars("projectavatar", key)}
}

func (o *restOut) version(v versionData, projectID int64) map[string]any {
	out := map[string]any{"self": o.api + "/version/" + o.id(v.ID), "id": o.id(v.ID), "name": v.Name,
		"archived": v.Archived, "released": v.Released, "projectId": projectID}
	if v.Release.Valid {
		out["releaseDate"] = v.Release.Time.Format(time.DateOnly)
	}
	return out
}

func (o *restOut) linkType(lt linkTypeRef) map[string]any {
	return map[string]any{"id": o.id(lt.ID), "name": lt.Name, "inward": lt.Inward, "outward": lt.Outward,
		"self": o.api + "/issueLinkType/" + o.id(lt.ID)}
}

// issueBrief is how Jira shows an issue inside another: links, sub-tasks,
// parent.
func (o *restOut) issueBrief(cat *catalog, iss *issueRow) map[string]any {
	return map[string]any{"id": o.id(iss.ID), "key": iss.Key(), "self": o.api + "/issue/" + o.id(iss.ID),
		"fields": map[string]any{
			"summary": iss.Summary, "status": o.status(cat.statusOf(iss.StatusID)),
			"priority": o.priority(cat.priorityOf(iss.PriorityID)), "issuetype": o.issueType(cat.typeOf(iss.TypeID)),
		}}
}

func (o *restOut) attachment(a attachmentData, users map[int64]*jiraUser) map[string]any {
	return map[string]any{"self": o.api + "/attachment/" + o.id(a.id), "id": o.id(a.id), "filename": a.name,
		"author": userOf(users, a.author.Int64), "created": o.time(a.created), "size": a.size, "mimeType": a.mime,
		"content": o.base + "/secure/attachment/" + o.id(a.id) + "/" + url.PathEscape(a.name)}
}

func (o *restOut) comment(iss *issueRow, c commentRow, users map[int64]*jiraUser) map[string]any {
	updatedBy := c.updatedBy
	if !updatedBy.Valid {
		updatedBy = c.author
	}
	out := map[string]any{"self": o.api + "/issue/" + o.id(iss.ID) + "/comment/" + o.id(c.id), "id": o.id(c.id),
		"author": userOf(users, c.author.Int64), "body": c.body, "updateAuthor": userOf(users, updatedBy.Int64),
		"created": o.time(c.created), "updated": o.time(c.updated)}
	switch {
	case c.group.Valid:
		out["visibility"] = map[string]any{"type": "group", "value": c.group.String}
	case c.roleID.Valid:
		out["visibility"] = map[string]any{"type": "role", "value": c.roleName.String}
	}
	return out
}

func (o *restOut) worklog(iss *issueRow, w worklogData, users map[int64]*jiraUser) map[string]any {
	updatedBy := w.updateAuthor
	if !updatedBy.Valid {
		updatedBy = w.author
	}
	out := map[string]any{"self": o.api + "/issue/" + o.id(iss.ID) + "/worklog/" + o.id(w.id), "id": o.id(w.id),
		"issueId": o.id(iss.ID), "author": userOf(users, w.author.Int64), "updateAuthor": userOf(users, updatedBy.Int64),
		"comment": w.body.String, "created": o.time(w.created), "updated": o.time(w.updated),
		"started": o.time(w.started), "timeSpent": jiraDuration(w.seconds), "timeSpentSeconds": w.seconds}
	switch {
	case w.group.Valid:
		out["visibility"] = map[string]any{"type": "group", "value": w.group.String}
	case w.roleName.Valid:
		out["visibility"] = map[string]any{"type": "role", "value": w.roleName.String}
	}
	return out
}

// jiraDuration writes seconds as Jira does: "1w 2d 3h 20m" (a day is 8
// hours, a week 5 days).
func jiraDuration(secs int64) string {
	var parts []string
	for _, u := range []struct {
		n    int64
		unit string
	}{{issue.WorkWeekSeconds, "w"}, {issue.WorkDaySeconds, "d"}, {3600, "h"}, {60, "m"}} {
		if secs >= u.n {
			parts = append(parts, strconv.FormatInt(secs/u.n, 10)+u.unit)
			secs %= u.n
		}
	}
	if len(parts) == 0 {
		return "0m"
	}
	return strings.Join(parts, " ")
}

// staleKind tells whether REST renders a field of this kind as null: one of
// hiddenKinds, which Solstice does not keep up to date. REST shows the others
// the way Jira does: gh-lexo-rank and gh-sprint from Solstice's own data,
// gh-epic-color as imported.
func staleKind(kind string) bool {
	switch kind {
	case issue.KindLexoRank, issue.KindSprint, issue.KindEpicColor:
		return false
	}
	return hiddenKinds[kind]
}

// customValue renders a custom field's value the way Jira does for its type.
func (o *restOut) customValue(set *issueSet, users map[int64]*jiraUser, f customFieldDef, d *issueData) any {
	switch f.Kind {
	case issue.KindEpicLink:
		if d.epic != nil {
			return d.epic.Key()
		}
		return nil
	case issue.KindSprint:
		if len(d.sprints) == 0 {
			return nil
		}
		out := []string{}
		for _, sp := range d.sprints {
			out = append(out, o.legacySprint(sp))
		}
		return out
	case issue.KindLexoRank:
		return sqlq.Value(d.row.Rank)
	case issue.KindLabels:
		if l := d.customLabels[f.ID]; len(l) > 0 {
			return l
		}
		return nil
	}
	values := d.custom[f.ID]
	if len(values) == 0 || staleKind(f.Kind) {
		return nil
	}
	switch f.Kind {
	case issue.KindSelect, issue.KindRadioButtons, issue.KindEpicStatus:
		return o.option(set, values[0])
	case issue.KindMultiSelect, issue.KindMultiCheckboxes:
		out := []any{}
		for _, v := range values {
			out = append(out, o.option(set, v))
		}
		return out
	case issue.KindCascadingSelect:
		var parent map[string]any
		for _, v := range values {
			if !v.parentKey.Valid {
				parent = o.option(set, v)
			} else if parent != nil {
				parent["child"] = o.option(set, v)
			}
		}
		if parent == nil {
			return nil
		}
		return parent
	case issue.KindUserPicker:
		id, _ := strconv.ParseInt(values[0].str.String, 10, 64)
		return userOf(users, id)
	case issue.KindMultiUserPicker:
		out := []any{}
		for _, v := range values {
			id, _ := strconv.ParseInt(v.str.String, 10, 64)
			if u := userOf(users, id); u != nil {
				out = append(out, u)
			}
		}
		return out
	case issue.KindDatePicker:
		if values[0].date.Valid {
			return values[0].date.Time.UTC().Format(time.DateOnly)
		}
	case issue.KindDateTime:
		if values[0].date.Valid {
			return o.time(values[0].date.Time)
		}
	case issue.KindFloat:
		if values[0].num.Valid {
			return values[0].num.Float64
		}
	}
	v := values[0]
	switch {
	case v.str.Valid:
		return v.str.String
	case v.text.Valid:
		return v.text.String
	case v.num.Valid:
		return v.num.Float64
	case v.date.Valid:
		return o.time(v.date.Time)
	}
	return nil
}

func (o *restOut) option(set *issueSet, v customRaw) map[string]any {
	id, err := strconv.ParseInt(v.str.String, 10, 64)
	opt, ok := set.options[id]
	if err != nil || !ok {
		return map[string]any{"value": v.str.String}
	}
	return map[string]any{"self": o.api + "/customFieldOption/" + o.id(id), "value": opt.value, "id": o.id(id),
		"disabled": false}
}

// legacySprint is a sprint as Jira's REST API v2 writes it in the sprint
// field: the text of a Java object, which tools parse.
func (o *restOut) legacySprint(sp sprintData) string {
	date := func(t *time.Time) string {
		if t == nil {
			return "<null>"
		}
		return t.In(o.tz).Format("2006-01-02T15:04:05.000Z07:00")
	}
	goal := ""
	if sp.Goal != nil {
		goal = *sp.Goal
	}
	return fmt.Sprintf("com.atlassian.greenhopper.service.sprint.Sprint@%x[id=%d,rapidViewId=%s,state=%s,name=%s,"+
		"startDate=%s,endDate=%s,completeDate=%s,activatedDate=%s,sequence=%d,goal=%s,autoStartStop=false,synced=false]",
		sp.ID, sp.ID, nullIDText(sp.board.Int64), strings.ToUpper(string(sp.State)), sp.Name, date(sp.Start), date(sp.End),
		date(sp.Completed), date(sp.Start), sp.position, goal)
}

func nullIDText(id int64) string {
	if id == 0 {
		return "<null>"
	}
	return strconv.FormatInt(id, 10)
}

// sprintJSON is a sprint as Jira's Agile API shows one.
func (o *restOut) sprintJSON(sp sprintOut, board int64) map[string]any {
	out := map[string]any{"id": sp.ID, "self": o.base + issue.AgileAPIPath + "/sprint/" + o.id(sp.ID), "state": sp.State,
		"name": sp.Name}
	date := func(t *time.Time) string { return t.In(o.tz).Format("2006-01-02T15:04:05.000Z07:00") }
	if sp.Start != nil {
		out["startDate"], out["activatedDate"] = date(sp.Start), date(sp.Start)
	}
	if sp.End != nil {
		out["endDate"] = date(sp.End)
	}
	if sp.Completed != nil {
		out["completeDate"] = date(sp.Completed)
	}
	if board != 0 {
		out["originBoardId"] = board
	}
	if sp.Goal != nil {
		out["goal"] = *sp.Goal
	}
	return out
}

// fieldNames maps the ids of the given fields to their names (expand=names).
func (o *restOut) fieldNames(set *issueSet, sel *fieldSelection) map[string]string {
	out := map[string]string{}
	for _, f := range systemFields {
		if f.id != "issuekey" && sel.has(f.id, f.navigable) {
			out[f.id] = o.msg(f.name)
		}
	}
	for _, f := range set.fields {
		if sel.has(customID(f.ID), true) {
			out[customID(f.ID)] = f.Name
		}
	}
	return out
}

// jiraTransitions lists the transitions the user may take on an issue: none
// without the permission to change its status.
func (s *Server) jiraTransitions(ctx context.Context, o *restOut, sub *perm.Subject, cat *catalog, iss *issueRow,
	withFields bool) ([]map[string]any, error) {
	out := []map[string]any{}
	pi := iss.perm()
	if !sub.Can(perm.Transition, iss.ProjectID, &pi) {
		return out, nil
	}
	list, _, err := s.issueTransitions(ctx, cat, iss)
	if err != nil {
		return nil, err
	}
	for _, t := range list {
		tr := map[string]any{"id": transitionID(t), "name": t.Name, "to": o.status(t.To), "hasScreen": false,
			"isGlobal": false, "isInitial": false, "isAvailable": true, "isConditional": false}
		if withFields {
			fields := map[string]any{}
			if t.To.Category == issue.Done {
				allowed := []any{}
				for _, r := range cat.Resolutions {
					allowed = append(allowed, o.resolution(&r))
				}
				fields["resolution"] = map[string]any{"required": false, "name": o.msg(i18n.M(i18n.FieldNameResolution)),
					"schema": map[string]any{"type": "resolution", "system": "resolution"}, "operations": []string{"set"},
					"allowedValues": allowed}
			}
			tr["fields"] = fields
		}
		out = append(out, tr)
	}
	return out, nil
}

// Transitions that are not workflow transitions (projects without a
// workflow, or an issue whose status left its workflow) take their target
// status's id offset by this, so that their id stays numeric as Jira's.
const directTransition = 100000000

func transitionID(t transitionOut) string {
	if t.ID != 0 {
		return strconv.FormatInt(t.ID, 10)
	}
	if n, err := strconv.ParseInt(t.To.ID, 10, 64); err == nil {
		return strconv.FormatInt(directTransition+n, 10)
	}
	return "status:" + t.To.ID
}

// jiraChangelog renders an issue's history as Jira's changelog.
func (s *Server) jiraChangelog(ctx context.Context, o *restOut, sub *perm.Subject, iss *issueRow) (map[string]any, error) {
	entries, err := s.issueHistory(ctx, sub, iss)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, e := range entries {
		ids = append(ids, e.authorID.Int64)
	}
	users, err := s.jiraUsers(ctx, o, ids...)
	if err != nil {
		return nil, err
	}
	histories := []any{}
	for _, e := range entries {
		items := []any{}
		for _, it := range e.Items {
			items = append(items, map[string]any{"field": it.Field, "fieldtype": it.FieldType, "from": it.fromValue,
				"fromString": it.fromString, "to": it.toValue, "toString": it.toString})
		}
		histories = append(histories, map[string]any{"id": o.id(e.ID), "author": userOf(users, e.authorID.Int64),
			"created": o.time(e.Created), "items": items})
	}
	return map[string]any{"startAt": 0, "maxResults": len(histories), "total": len(histories), "histories": histories}, nil
}

// expandSet reads Jira's "expand" parameter.
func expandSet(values ...string) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out[part] = true
			}
		}
	}
	return out
}

// jiraIssues renders a page of issues with the selected fields and
// expansions (changelog, renderedFields, names, transitions). The Agile API
// (agile) adds the issue's sprint, closed sprints and epic as objects.
func (s *Server) jiraIssues(ctx context.Context, o *restOut, sub *perm.Subject, rows []*issueRow, sel *fieldSelection,
	expand map[string]bool, agile bool) ([]map[string]any, map[string]string, error) {
	cat, err := s.catalogue(ctx)
	if err != nil {
		return nil, nil, err
	}
	custom, err := s.customFields(ctx)
	if err != nil {
		return nil, nil, err
	}
	want := sel.loads(custom)
	inner := want
	want = func(k string) bool {
		return inner(k) || (k == loadComments && expand["renderedFields"] && sel.hasSystem("comment")) ||
			(agile && (k == loadSprints || k == loadEpic))
	}
	set, err := s.loadIssues(ctx, sub, custom, rows, want)
	if err != nil {
		return nil, nil, err
	}
	users, err := s.jiraUsers(ctx, o, set.userIDs()...)
	if err != nil {
		return nil, nil, err
	}
	out := make([]map[string]any, 0, len(set.list))
	for _, d := range set.list {
		item := s.jiraIssue(o, cat, set, users, d, sel)
		if agile {
			o.agileFields(cat, item["fields"].(map[string]any), d, sel)
		}
		if expand["changelog"] {
			if item["changelog"], err = s.jiraChangelog(ctx, o, sub, d.row); err != nil {
				return nil, nil, err
			}
		}
		if expand["renderedFields"] {
			item["renderedFields"] = o.renderedFields(d, sel)
		}
		if expand["transitions"] {
			if item["transitions"], err = s.jiraTransitions(ctx, o, sub, cat, d.row,
				expand["transitions.fields"]); err != nil {
				return nil, nil, err
			}
		}
		out = append(out, item)
	}
	var names map[string]string
	if expand["names"] {
		names = o.fieldNames(set, sel)
	}
	return out, names, nil
}

// renderedFields renders the wiki text of an issue as HTML: description,
// environment and comments.
func (o *restOut) renderedFields(d *issueData, sel *fieldSelection) map[string]any {
	out := map[string]any{}
	if sel.hasSystem("description") {
		out["description"] = renderWikiPtr(d.row.Description)
	}
	if sel.hasSystem("environment") {
		out["environment"] = renderWikiPtr(d.row.Environment)
	}
	if sel.hasSystem("comment") {
		list := []any{}
		for _, c := range d.comments {
			list = append(list, map[string]any{"id": o.id(c.id), "body": renderWiki(c.body),
				"created": o.time(c.created), "updated": o.time(c.updated)})
		}
		out["comment"] = map[string]any{"comments": list, "maxResults": len(list), "total": len(list), "startAt": 0}
	}
	return out
}

func renderWikiPtr(s sql.NullString) any {
	if !s.Valid {
		return nil
	}
	return renderWiki(s.String)
}

// sortComments orders comments for the comment endpoint's orderBy
// ("created", "-created").
func sortComments(list []commentRow, orderBy string) {
	if strings.TrimPrefix(orderBy, "+") == "-created" {
		sort.SliceStable(list, func(a, b int) bool { return list[a].created.After(list[b].created) })
	}
}

// agileFields adds what Jira's Agile API shows of an issue: its open sprint,
// its closed sprints and its epic.
func (o *restOut) agileFields(cat *catalog, f map[string]any, d *issueData, sel *fieldSelection) {
	if sel.has("sprint", true) || sel.has("closedSprints", true) {
		var open any
		closed := []any{}
		for _, sp := range d.sprints {
			if sp.State == issue.SprintClosed {
				closed = append(closed, o.sprintJSON(sp.sprintOut, sp.board.Int64))
			} else {
				open = o.sprintJSON(sp.sprintOut, sp.board.Int64)
			}
		}
		f["sprint"], f["closedSprints"] = open, closed
	}
	if sel.has("epic", true) {
		f["epic"] = nil
		if d.epic != nil {
			f["epic"] = o.epicJSON(cat, d.epic, "")
		}
	}
	f["flagged"] = false
}

// epicJSON is an epic as Jira's Agile API shows one.
func (o *restOut) epicJSON(cat *catalog, epic *issueRow, name string) map[string]any {
	if name == "" {
		name = epic.Summary
	}
	return map[string]any{"id": epic.ID, "key": epic.Key(), "self": o.base + issue.AgileAPIPath + "/epic/" + o.id(epic.ID),
		"name": name, "summary": epic.Summary, "color": map[string]string{"key": "color_1"},
		"done": cat.statusOf(epic.StatusID).Category == issue.Done}
}
