package api

// Exporting a search as CSV, for spreadsheets: RFC 4180, UTF-8 with a byte
// order mark (so that Excel reads the accents), headers in the user's
// language, dates in their time zone. Only issues the user sees, 10 000 at
// most.

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"solstice/internal/i18n"
)

const maxExportRows = 10000

// defaultExportColumns are exported when the request names none.
var defaultExportColumns = []string{"issuekey", "issuetype", "summary", "status", "priority", "resolution",
	"assignee", "reporter", "created", "updated", "duedate"}

// exportAliases accept the interface's names and a few usual ones.
var exportAliases = map[string]string{
	"key": "issuekey", "type": "issuetype", "due": "duedate", "resolved": "resolutiondate",
	"affectsVersions": "versions", "affectedVersions": "versions", "originalEstimate": "timeoriginalestimate",
	"remainingEstimate": "timeestimate", "timeSpent": "timespent", "level": "security", "securityLevel": "security",
	"comments": "comment", "attachments": "attachment", "watchers": "watches", "links": "issuelinks",
}

// exportColumn is a column of the export.
type exportColumn struct {
	id      string
	header  string
	number  bool // written as a number, not guarded against formulas
	custom  *customFieldDef
	special string // epic, sprint
}

// exportColumns reads the columns parameter: system field ids (or aliases),
// customfield_<id>, epic and sprint.
func (s *Server) exportColumns(r *http.Request, fields []customFieldDef) ([]exportColumn, error) {
	loc := locale(r)
	ids := defaultExportColumns
	if v := strings.TrimSpace(r.URL.Query().Get("columns")); v != "" {
		ids = nil
		for part := range strings.SplitSeq(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				ids = append(ids, part)
			}
		}
	}
	var out []exportColumn
	for _, id := range ids {
		if alias, ok := exportAliases[id]; ok {
			id = alias
		}
		switch {
		case id == "epic":
			out = append(out, exportColumn{id: id, header: i18n.M("Epic").In(loc), special: id})
		case id == "sprint":
			out = append(out, exportColumn{id: id, header: i18n.M("Sprint").In(loc), special: id})
		case strings.HasPrefix(id, "customfield_"):
			found := false
			for i, f := range fields {
				if customID(f.ID) == id {
					out = append(out, exportColumn{id: id, header: f.Name, custom: &fields[i], number: f.Kind == "float"})
					found = true
				}
			}
			if !found {
				return nil, failf(http.StatusBadRequest, "Unknown column: %s.", id)
			}
		default:
			f, ok := systemField(id)
			if !ok {
				return nil, failf(http.StatusBadRequest, "Unknown column: %s.", id)
			}
			out = append(out, exportColumn{id: id, header: f.name.In(loc),
				number: id == "timeoriginalestimate" || id == "timeestimate" || id == "timespent"})
		}
	}
	return out, nil
}

// exportCSV answers GET /api/issues/export.csv: the search given as for
// /api/issues (jql, or q in either syntax, and project), in the columns
// asked for. tz may name the user's time zone (default: the server's).
func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	fields, err := s.customFields(ctx)
	if err != nil {
		return err
	}
	columns, err := s.exportColumns(r, fields)
	if err != nil {
		return err
	}
	tz := s.zone()
	if name := r.URL.Query().Get("tz"); name != "" {
		if tz, err = time.LoadLocation(name); err != nil {
			return failf(http.StatusBadRequest, "Unknown time zone: %s.", name)
		}
	}
	where, args, order, _, err := s.searchCondition(ctx, sub, r.URL.Query())
	if err != nil {
		return err
	}
	rows, total, err := s.searchRows(ctx, where, args, order, 0, maxExportRows)
	if err != nil {
		return err
	}
	cat, err := s.catalogue(ctx)
	if err != nil {
		return err
	}
	// Load what the columns need.
	sel := &fieldSelection{include: map[string]bool{}, exclude: map[string]bool{}}
	for _, c := range columns {
		sel.include[c.id] = true
		if c.special == "sprint" {
			if f, ok := fieldOfKind(fields, "gh-sprint"); ok {
				sel.include[customID(f.ID)] = true
			}
		}
	}
	want := sel.loads(fields)
	set, err := s.loadIssues(ctx, sub, rows, func(k string) bool {
		return want(k) || (k == loadEpic && sel.include["epic"]) || (k == loadSprints && sel.include["sprint"])
	})
	if err != nil {
		return err
	}
	o := s.restOut(r)
	o.tz = tz
	users, err := s.jiraUsers(ctx, o, set.userIDs()...)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="solstice-`+time.Now().In(tz).Format("2006-01-02")+`.csv"`)
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	if total > maxExportRows {
		w.Header().Set("X-Truncated", "true")
	}
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("\uFEFF")); err != nil {
		return nil
	}
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	header := make([]string, len(columns))
	for i, c := range columns {
		header[i] = guardFormula(c.header)
	}
	cw.Write(header)
	for _, d := range set.list {
		record := make([]string, len(columns))
		for i, c := range columns {
			v := o.exportCell(cat, set, users, d, c)
			if !c.number {
				v = guardFormula(v)
			}
			record[i] = v
		}
		if err := cw.Write(record); err != nil {
			return nil // the client went away: nothing more to say
		}
	}
	cw.Flush()
	return nil
}

// guardFormula keeps a spreadsheet from running a cell as a formula
// (CSV injection): text starting with = + - @ or a control character gets
// a leading apostrophe, which spreadsheets show as text.
func guardFormula(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

// exportCell writes one value of an issue as text.
func (o *restOut) exportCell(cat *catalog, set *issueSet, users map[int64]*jiraUser, d *issueData, c exportColumn) string {
	row := d.row
	when := func(t time.Time) string { return t.In(o.tz).Format("2006-01-02 15:04") }
	who := func(id int64) string {
		if u, ok := users[id]; ok && id != 0 {
			return u.DisplayName
		}
		return ""
	}
	join := func(items []string) string { return strings.Join(items, ", ") }
	if c.custom != nil {
		return o.customText(set, users, *c.custom, d)
	}
	switch c.id {
	case "issuekey":
		return row.Key()
	case "summary":
		return row.Summary
	case "issuetype":
		return cat.typeOf(row.TypeID).Name
	case "project":
		return row.ProjectKey
	case "status":
		return cat.statusOf(row.StatusID).Name
	case "priority":
		if p := cat.priorityOf(row.PriorityID); p != nil {
			return p.Name
		}
	case "resolution":
		if v := cat.resolutionOf(row.ResolutionID); v != nil {
			return v.Name
		}
	case "resolutiondate":
		if row.Resolved.Valid {
			return when(row.Resolved.Time)
		}
	case "assignee":
		return who(row.AssigneeID.Int64)
	case "reporter":
		return who(row.ReporterID.Int64)
	case "creator":
		return who(row.CreatorID.Int64)
	case "created":
		return when(row.Created)
	case "updated":
		return when(row.Updated)
	case "duedate":
		if d := dateOnly(row.Due); d != nil {
			return *d
		}
	case "description":
		return row.Description.String
	case "environment":
		return row.Environment.String
	case "labels":
		return join(d.labels)
	case "components":
		var names []string
		for _, c := range d.components {
			names = append(names, c.Name)
		}
		return join(names)
	case "fixVersions", "versions":
		list := d.fixVersions
		if c.id == "versions" {
			list = d.versions
		}
		var names []string
		for _, v := range list {
			names = append(names, v.Name)
		}
		return join(names)
	case "issuelinks":
		var keys []string
		for _, l := range d.links {
			lt := cat.linkType[l.typeID]
			label := lt.Outward
			if !l.outward {
				label = lt.Inward
			}
			keys = append(keys, label+" "+l.other.Key())
		}
		return join(keys)
	case "subtasks":
		var keys []string
		for _, st := range d.subtasks {
			keys = append(keys, st.Key())
		}
		return join(keys)
	case "parent":
		if d.parent != nil {
			return d.parent.Key()
		}
	case "security":
		if row.LevelID.Valid {
			return set.levels[row.LevelID.Int64]
		}
	case "watches":
		return strconv.Itoa(d.watches.Count)
	case "votes":
		return strconv.Itoa(d.votes.Count)
	case "timeoriginalestimate", "timeestimate", "timespent":
		v := map[string]sql.NullInt64{"timeoriginalestimate": row.Original, "timeestimate": row.Remaining,
			"timespent": row.Spent}[c.id]
		if v.Valid {
			return strconv.FormatFloat(float64(v.Int64)/3600, 'f', -1, 64) // hours
		}
	case "attachment":
		var names []string
		for _, a := range d.attachments {
			names = append(names, a.name)
		}
		return join(names)
	case "comment":
		var bodies []string
		for _, cm := range d.comments {
			bodies = append(bodies, fmt.Sprintf("%s; %s; %s", when(cm.created), who(cm.author.Int64), cm.body))
		}
		return strings.Join(bodies, "\n\n")
	case "worklog":
		var logs []string
		for _, wl := range d.worklogs {
			logs = append(logs, fmt.Sprintf("%s; %s; %s", when(wl.started), who(wl.author.Int64), jiraDuration(wl.seconds)))
		}
		return strings.Join(logs, "\n")
	case "timetracking":
		return ""
	case "epic":
		if d.epic != nil {
			return d.epic.Key()
		}
	case "sprint":
		var names []string
		for _, sp := range d.sprints {
			names = append(names, sp.Name)
		}
		return join(names)
	}
	return ""
}

// customText writes a custom field's value as text (exports, history).
func (o *restOut) customText(set *issueSet, users map[int64]*jiraUser, f customFieldDef, d *issueData) string {
	var text func(v any) string
	text = func(v any) string {
		switch v := v.(type) {
		case nil:
			return ""
		case string:
			return v
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case *jiraUser:
			return v.DisplayName
		case []string:
			return strings.Join(v, ", ")
		case []any:
			var parts []string
			for _, item := range v {
				parts = append(parts, text(item))
			}
			return strings.Join(parts, ", ")
		case map[string]any:
			out := text(v["value"])
			if child, ok := v["child"]; ok {
				out += " › " + text(child)
			}
			return out
		}
		return fmt.Sprint(v)
	}
	if f.Kind == "gh-sprint" {
		var names []string
		for _, sp := range d.sprints {
			names = append(names, sp.Name)
		}
		return strings.Join(names, ", ")
	}
	if f.Kind == "datetime" {
		if values := d.custom[f.ID]; len(values) > 0 && values[0].date.Valid {
			return values[0].date.Time.In(o.tz).Format("2006-01-02 15:04")
		}
	}
	return text(o.customValue(set, users, f, d))
}
