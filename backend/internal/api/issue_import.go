package api

// Importing issues into a project from a CSV file, from its settings. Columns
// go to fields by the ids and headers the CSV export writes (in any
// language), so an exported search imports back. A check reports, per row,
// the issue it would create or its errors, and writes nothing; the import
// creates every row in one transaction, or none. Like the Jira import, it
// sends no notifications and no webhooks.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"solstice/internal/audit"
	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

const (
	maxImportRows  = 1000
	maxImportBytes = 4 << 20
)

// importSystem are the system fields an import sets, in the order the
// mapping offers them. The issue key only names a row for the parent and
// epic columns of others.
var importSystem = []string{"issuekey", "summary", "issuetype", "status", "priority", "resolution", "assignee",
	"reporter", "description", "environment", "labels", "components", "versions", "fixVersions", "duedate",
	"parent", "epic", "security", "timeoriginalestimate", "timeestimate"}

// importReadOnly are the export's columns an import cannot set.
var importReadOnly = []string{"project", "resolutiondate", "creator", "created", "updated", "issuelinks",
	"subtasks", "watches", "votes", "timespent", "attachment", "comment", "worklog", "sprint"}

func importableCustom(f customFieldDef) bool {
	return settableKind(f.Kind) && f.Kind != issue.KindSprint && f.Kind != issue.KindEpicLink
}

// importReport answers both the check and the import.
type importReport struct {
	Columns []importColumn `json:"columns"`
	Targets []importTarget `json:"targets"` // the fields a column may go to
	Rows    []*importRow   `json:"rows"`
	Created []string       `json:"created,omitempty"` // the new keys, in the file's order
}

// importColumn is a column of the file and the field it goes to.
type importColumn struct {
	Header   string `json:"header"`
	Field    string `json:"field"`              // "": not imported
	Known    bool   `json:"known"`              // recognised from its header
	ReadOnly bool   `json:"readOnly,omitempty"` // a field an import cannot set
}

type importTarget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// importRow is a row of the file: the issue it would create, or its errors.
type importRow struct {
	Line    int      `json:"line"` // in the file, the header being line 1
	Key     string   `json:"key,omitempty"`
	Summary string   `json:"summary"`
	Type    string   `json:"type,omitempty"`
	Status  string   `json:"status,omitempty"`
	Errors  []string `json:"errors,omitempty"`

	f                  fields
	opts               createOpts
	parentKey, epicKey string // issue keys of other rows
	parentRow, epicRow int    // their index, or -1
	plan               *issuePlan
	state              int // 0: not planned yet, 1: planning, 2: planned (or failed)
}

// csvImport reads the rows of a file for one project.
type csvImport struct {
	s          *Server
	ctx        context.Context
	sub        *perm.Subject
	loc        string
	tz         *time.Location
	cat        *catalog
	projectID  int64
	projectKey string
	custom     []customFieldDef
	parts      map[string]map[string]int64 // components, versions, levels: lower-case name → id
	users      map[string]importUser
	order      []*importRow // planned rows, each after the rows it refers to
}

type importUser struct {
	id       int64
	username string
	err      string
}

// importIssues answers POST /api/projects/{key}/import.csv: a multipart body
// with the file, optionally the mapping (a JSON list of field ids, one per
// column, "" to skip one) and tz, the time zone of the file's dates. With
// check=1 it only reports.
func (s *Server) importIssues(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	projectID, err := s.configurableProject(r)
	if err != nil {
		return err
	}
	sub, err := s.subject(r)
	if err != nil {
		return err
	}
	data, mapping, tzName, err := readImportUpload(w, r)
	if err != nil {
		return err
	}
	tz := s.zone()
	if tzName != "" {
		if tz, err = time.LoadLocation(tzName); err != nil {
			return failf(http.StatusBadRequest, i18n.CommonTimeZoneUnknown, tzName)
		}
	}
	header, records, lines, err := readImportCSV(data)
	if err != nil {
		return err
	}
	im, err := s.newImporter(ctx, sub, projectID, locale(r), tz)
	if err != nil {
		return err
	}
	report, err := im.check(header, records, lines, mapping)
	if err != nil {
		return err
	}
	if r.URL.Query().Get("check") != "" {
		return writeJSON(w, http.StatusOK, report)
	}
	failed := 0
	for _, row := range report.Rows {
		if len(row.Errors) > 0 {
			failed++
		}
	}
	if failed > 0 {
		return &httpError{status: http.StatusBadRequest, msg: i18n.M(i18n.CSVImportRowsInvalid, failed),
			extra: map[string]any{"import": report}}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, row := range im.order {
		if err := s.insertIssue(ctx, tx, sub, row.plan); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	keys := make([]string, 0, len(report.Rows))
	for _, row := range report.Rows {
		keys = append(keys, im.projectKey+"-"+strconv.FormatInt(row.plan.num, 10))
	}
	report.Created = keys
	s.log.Info("issues imported from CSV", "project", im.projectKey, "rows", len(keys))
	s.auditLog(r, audit.IssueImport, target(audit.TargetProject, projectID, im.projectKey), map[string]any{"rows": len(keys)})
	return writeJSON(w, http.StatusOK, report)
}

// readImportUpload reads the parts of an import request.
func readImportUpload(w http.ResponseWriter, r *http.Request) (data []byte, mapping []string, tz string, err error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes+1<<20)
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, nil, "", fail(http.StatusBadRequest, i18n.CSVImportFileMissing)
	}
	tooLarge := failf(http.StatusRequestEntityTooLarge, i18n.CSVImportFileTooLarge, maxImportBytes>>20)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, "", tooLarge
		}
		switch part.FormName() {
		case "file":
			if data, err = io.ReadAll(io.LimitReader(part, maxImportBytes+1)); err != nil {
				return nil, nil, "", tooLarge
			}
			if len(data) > maxImportBytes {
				return nil, nil, "", tooLarge
			}
		case "mapping":
			raw, err := io.ReadAll(io.LimitReader(part, 1<<20))
			if err != nil || json.Unmarshal(raw, &mapping) != nil || mapping == nil {
				return nil, nil, "", fail(http.StatusBadRequest, i18n.CSVImportMappingInvalid)
			}
		case "tz":
			raw, err := io.ReadAll(io.LimitReader(part, 256))
			if err != nil {
				return nil, nil, "", tooLarge
			}
			tz = strings.TrimSpace(string(raw))
		}
	}
	if data == nil {
		return nil, nil, "", fail(http.StatusBadRequest, i18n.CSVImportFileMissing)
	}
	return data, mapping, tz, nil
}

// readImportCSV reads the header and the rows of a file, with the line each
// row starts on. Blank rows are left out. A header with more semicolons than
// commas is read as semicolon-separated, as spreadsheets in some languages
// write it.
func readImportCSV(data []byte) (header []string, records [][]string, lines []int, err error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // the export's byte order mark
	cr := csv.NewReader(bytes.NewReader(data))
	first, _, _ := bytes.Cut(data, []byte("\n"))
	if bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		cr.Comma = ';'
	}
	cr.FieldsPerRecord = -1
	invalid := func(err error) error {
		var pe *csv.ParseError
		line := 1
		if errors.As(err, &pe) {
			line = pe.StartLine
		}
		return failf(http.StatusBadRequest, i18n.CSVImportFileInvalid, line)
	}
	header, err = cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil, nil, fail(http.StatusBadRequest, i18n.CSVImportFileEmpty)
	}
	if err != nil {
		return nil, nil, nil, invalid(err)
	}
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, nil, invalid(err)
		}
		if !slices.ContainsFunc(rec, func(v string) bool { return strings.TrimSpace(v) != "" }) {
			continue
		}
		if len(records) == maxImportRows {
			return nil, nil, nil, failf(http.StatusBadRequest, i18n.CSVImportTooManyRows, maxImportRows)
		}
		line, _ := cr.FieldPos(0)
		records = append(records, rec)
		lines = append(lines, line)
	}
	return header, records, lines, nil
}

// unguard undoes the export's guard against formulas (guardFormula).
func unguard(v string) string {
	if len(v) > 1 && v[0] == '\'' && strings.ContainsRune("=+-@\t\r", rune(v[1])) {
		return v[1:]
	}
	return v
}

func (s *Server) newImporter(ctx context.Context, sub *perm.Subject, projectID int64, loc string,
	tz *time.Location) (*csvImport, error) {
	im := &csvImport{s: s, ctx: ctx, sub: sub, loc: loc, tz: tz, projectID: projectID,
		parts: map[string]map[string]int64{}, users: map[string]importUser{}}
	var err error
	if im.cat, err = s.catalogue(ctx); err != nil {
		return nil, err
	}
	if im.custom, err = s.customFields(ctx); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT pkey FROM projects WHERE id = ?`, projectID).Scan(&im.projectKey); err != nil {
		return nil, err
	}
	for kind, table := range map[string]string{"components": "components", "versions": "versions",
		"security": "security_levels"} {
		names := map[string]int64{}
		if err := sqlq.Each(ctx, s.db, `SELECT id, name FROM `+table+` WHERE project_id = ? ORDER BY id`,
			func(rows *sql.Rows) error {
				var id int64
				var name string
				if err := rows.Scan(&id, &name); err != nil {
					return err
				}
				if _, ok := names[strings.ToLower(name)]; !ok {
					names[strings.ToLower(name)] = id
				}
				return nil
			}, projectID); err != nil {
			return nil, err
		}
		im.parts[kind] = names
	}
	return im, nil
}

// targetName is a field's name in the user's language.
func (im *csvImport) targetName(id string) string {
	if id == "epic" {
		return i18n.M(i18n.FieldNameEpic).In(im.loc)
	}
	for _, f := range im.custom {
		if customID(f.ID) == id {
			return f.Name
		}
	}
	return systemFieldName(id).In(im.loc)
}

// targets lists the fields a column may go to.
func (im *csvImport) targets() []importTarget {
	var out []importTarget
	for _, id := range importSystem {
		out = append(out, importTarget{ID: id, Name: im.targetName(id)})
	}
	for _, f := range im.custom {
		if importableCustom(f) {
			out = append(out, importTarget{ID: customID(f.ID), Name: f.Name})
		}
	}
	return out
}

// headerIndex maps the headers a file may have, lower case, to field ids:
// ids and the export's aliases, names in every language, custom field names.
// A name two custom fields share maps to nothing.
func (im *csvImport) headerIndex() map[string]string {
	idx := map[string]string{}
	add := func(name, id string) {
		if k := strings.ToLower(strings.TrimSpace(name)); k != "" {
			if _, ok := idx[k]; !ok {
				idx[k] = id
			}
		}
	}
	system := append(slices.Clone(importSystem), importReadOnly...)
	for _, id := range system {
		add(id, id)
	}
	for alias, id := range exportAliases {
		add(alias, id)
	}
	for _, loc := range i18n.Locales {
		for _, id := range system {
			switch id {
			case "epic":
				add(i18n.M(i18n.FieldNameEpic).In(loc), id)
			case "sprint":
				add(i18n.M(i18n.FieldNameSprint).In(loc), id)
			default:
				add(systemFieldName(id).In(loc), id)
			}
		}
	}
	shared := map[string]bool{}
	for _, f := range im.custom {
		id := customID(f.ID)
		switch f.Kind {
		case issue.KindEpicLink:
			id = "epic"
		case issue.KindSprint:
			id = "sprint"
		}
		add(customID(f.ID), id)
		k := strings.ToLower(strings.TrimSpace(f.Name))
		if prev, ok := idx[k]; ok && strings.HasPrefix(prev, "customfield_") && prev != id {
			shared[k] = true
		}
		add(f.Name, id)
	}
	for k := range shared {
		delete(idx, k)
	}
	return idx
}

// columns maps the header: from the mapping when there is one, else from the
// headers, each field going to its first column only.
func (im *csvImport) columns(header, mapping []string) ([]importColumn, error) {
	idx := im.headerIndex()
	settable := map[string]bool{}
	for _, t := range im.targets() {
		settable[t.ID] = true
	}
	cols := make([]importColumn, len(header))
	used := map[string]bool{}
	for i, h := range header {
		h = strings.TrimSpace(unguard(h))
		id, known := idx[strings.ToLower(h)]
		cols[i] = importColumn{Header: h, Known: known}
		switch {
		case !known:
		case !settable[id]:
			cols[i].ReadOnly = true
		case !used[id]:
			cols[i].Field = id
			used[id] = true
		}
	}
	if mapping == nil {
		return cols, nil
	}
	if len(mapping) != len(header) {
		return nil, fail(http.StatusBadRequest, i18n.CSVImportMappingInvalid)
	}
	used = map[string]bool{}
	for i, id := range mapping {
		if id != "" && !settable[id] {
			return nil, fail(http.StatusBadRequest, i18n.CSVImportMappingInvalid)
		}
		if id != "" && used[id] {
			return nil, failf(http.StatusBadRequest, i18n.CSVImportColumnTwice, im.targetName(id))
		}
		used[id] = true
		cols[i].Field = id
	}
	return cols, nil
}

// check maps the columns and plans every row.
func (im *csvImport) check(header []string, records [][]string, lines []int, mapping []string) (*importReport, error) {
	cols, err := im.columns(header, mapping)
	if err != nil {
		return nil, err
	}
	report := &importReport{Columns: cols, Targets: im.targets(), Rows: make([]*importRow, len(records))}
	byKey := map[string]int{}
	for i, rec := range records {
		row := &importRow{Line: lines[i], f: fields{}, parentRow: -1, epicRow: -1}
		if err := im.read(row, cols, rec); err != nil {
			return nil, err
		}
		if row.Key != "" {
			if _, dup := byKey[row.Key]; dup {
				row.Errors = append(row.Errors, i18n.M(i18n.CSVImportKeyTwice, row.Key).In(im.loc))
			} else {
				byKey[row.Key] = i
			}
		}
		report.Rows[i] = row
	}
	// A parent or epic that is another row's key is that row's new issue;
	// any other key names an issue of the project.
	for _, row := range report.Rows {
		for _, ref := range []struct {
			key   string
			field string
			index *int
		}{{row.parentKey, "parent", &row.parentRow}, {row.epicKey, "epic", &row.epicRow}} {
			if ref.key == "" {
				continue
			}
			if i, ok := byKey[ref.key]; ok {
				*ref.index = i
			} else {
				row.f[ref.field], _ = json.Marshal(ref.key)
			}
		}
	}
	for _, row := range report.Rows {
		if _, err := im.plan(report.Rows, row); err != nil {
			return nil, err
		}
	}
	return report, nil
}

// plan plans a row after the rows it refers to, and tells whether it passed.
func (im *csvImport) plan(rows []*importRow, row *importRow) (bool, error) {
	if row.state != 0 {
		return row.plan != nil, nil
	}
	row.state = 1
	for _, ref := range []struct {
		index int
		key   string
		dst   **issuePlan
	}{{row.parentRow, row.parentKey, &row.opts.parent}, {row.epicRow, row.epicKey, &row.opts.epic}} {
		if ref.index < 0 {
			continue
		}
		dep := rows[ref.index]
		if dep.state == 1 {
			row.Errors = append(row.Errors, i18n.M(i18n.CSVImportReferenceCycle, ref.key).In(im.loc))
			continue
		}
		ok, err := im.plan(rows, dep)
		if err != nil {
			return false, err
		}
		if !ok {
			row.Errors = append(row.Errors, i18n.M(i18n.CSVImportReferenceFailed, ref.key).In(im.loc))
			continue
		}
		*ref.dst = dep.plan
	}
	row.opts.epicKey = row.epicKey
	row.state = 2
	if len(row.Errors) > 0 {
		return false, nil
	}
	p, err := im.s.planIssue(im.ctx, im.sub, row.f, row.opts)
	var he *httpError
	if errors.As(err, &he) {
		row.Errors = append(row.Errors, he.msg.In(im.loc))
		return false, nil
	}
	if err != nil {
		return false, err
	}
	row.plan = p
	row.Type = p.typ.Name
	row.Status = im.cat.statusOf(p.status).Name
	im.order = append(im.order, row)
	return true, nil
}

// read turns a record into the fields of a creation, noting what it cannot
// read in row.Errors.
func (im *csvImport) read(row *importRow, cols []importColumn, rec []string) error {
	set := func(k string, v any) { row.f[k], _ = json.Marshal(v) }
	set("project", im.projectKey)
	custom := map[string]any{}
	for i, col := range cols {
		if col.Field == "" || i >= len(rec) {
			continue
		}
		v := strings.TrimSpace(unguard(rec[i]))
		if v == "" {
			continue
		}
		unknown := func() { row.Errors = append(row.Errors, i18n.M(i18n.CSVImportValueUnknown, col.Header, v).In(im.loc)) }
		invalid := func() { row.Errors = append(row.Errors, i18n.M(i18n.CSVImportValueInvalid, col.Header, v).In(im.loc)) }
		switch col.Field {
		case "issuekey":
			row.Key = strings.ToUpper(v)
		case "summary":
			row.Summary = v
			set("summary", v)
		case "issuetype":
			if t, ok := findNamed(im.cat.IssueTypes, v, func(t typeRef) (string, string) { return t.ID, t.Name }); ok {
				set("type", t.ID)
				row.Type = t.Name
			} else {
				unknown()
			}
		case "status":
			if st, ok := findNamed(im.cat.Statuses, v, func(s statusRef) (string, string) { return s.ID, s.Name }); ok {
				row.opts.status = st.ID
				row.Status = st.Name
			} else {
				unknown()
			}
		case "resolution":
			if res, ok := findNamed(im.cat.Resolutions, v, func(r namedRef) (string, string) { return r.ID, r.Name }); ok {
				row.opts.resolution = res.ID
			} else {
				unknown()
			}
		case "priority":
			if pr, ok := findNamed(im.cat.Priorities, v, func(p priorityRef) (string, string) { return p.ID, p.Name }); ok {
				set("priority", pr.ID)
			} else {
				unknown()
			}
		case "assignee", "reporter":
			u, err := im.user(v)
			if err != nil {
				return err
			}
			switch {
			case u.err != "":
				row.Errors = append(row.Errors, i18n.M(i18n.CSVImportUserAmbiguous, col.Header, v).In(im.loc))
			case u.id == 0:
				unknown()
			case col.Field == "assignee":
				set("assignee", u.id)
			default:
				row.opts.reporter = u.id
			}
		case "description", "environment":
			set(col.Field, v)
		case "labels":
			set("labels", strings.FieldsFunc(v, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }))
		case "components", "versions", "fixVersions", "security":
			kind := map[string]string{"fixVersions": "versions"}[col.Field]
			if kind == "" {
				kind = col.Field
			}
			names := im.parts[kind]
			var ids []int64
			for _, name := range splitNames(v, func(n string) bool { _, ok := names[strings.ToLower(n)]; return ok }) {
				id, ok := names[strings.ToLower(name)]
				if !ok {
					row.Errors = append(row.Errors, i18n.M(i18n.CSVImportValueUnknown, col.Header, name).In(im.loc))
					continue
				}
				ids = append(ids, id)
			}
			switch col.Field {
			case "security":
				if len(ids) == 1 {
					set("securityLevel", ids[0])
				} else if len(ids) > 1 {
					invalid()
				}
			case "versions":
				set("affectsVersions", ids)
			default:
				set(col.Field, ids)
			}
		case "duedate":
			set("due", v)
		case "parent":
			row.parentKey = strings.ToUpper(v)
		case "epic":
			row.epicKey = strings.ToUpper(v)
		case "timeoriginalestimate", "timeestimate":
			// The export writes hours; anything else is read in Jira's syntax.
			text := v
			if h, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64); err == nil {
				text = strconv.FormatInt(int64(h*60+0.5), 10) + "m"
			}
			set(map[string]string{"timeoriginalestimate": "originalEstimate", "timeestimate": "remainingEstimate"}[col.Field], text)
		default:
			value, ok, err := im.customValue(col.Field, v)
			if err != nil {
				return err
			}
			if !ok {
				unknown()
				continue
			}
			custom[col.Field] = value
		}
	}
	if len(custom) > 0 {
		set("customFields", custom)
	}
	return nil
}

// customValue turns a custom field's text, as the export writes it, into
// the value the interface sends.
func (im *csvImport) customValue(id, v string) (any, bool, error) {
	var f customFieldDef
	for _, cf := range im.custom {
		if customID(cf.ID) == id {
			f = cf
		}
	}
	list := func() []string { return splitNames(v, func(string) bool { return false }) }
	user := func(name string) (any, bool, error) {
		u, err := im.user(name)
		if err != nil || u.id == 0 {
			return nil, false, err
		}
		return map[string]string{"name": u.username}, true, nil
	}
	switch f.Kind {
	case issue.KindSelect, issue.KindRadioButtons, issue.KindEpicStatus:
		return map[string]string{"value": v}, true, nil
	case issue.KindMultiSelect, issue.KindMultiCheckboxes:
		var out []map[string]string
		for _, item := range list() {
			out = append(out, map[string]string{"value": item})
		}
		return out, true, nil
	case issue.KindCascadingSelect:
		parent, child, ok := strings.Cut(v, "›")
		out := map[string]any{"value": strings.TrimSpace(parent)}
		if ok {
			out["child"] = map[string]string{"value": strings.TrimSpace(child)}
		}
		return out, true, nil
	case issue.KindUserPicker:
		return user(v)
	case issue.KindMultiUserPicker:
		var out []any
		for _, name := range list() {
			u, ok, err := user(name)
			if err != nil || !ok {
				return nil, false, err
			}
			out = append(out, u)
		}
		return out, true, nil
	case issue.KindLabels:
		return strings.FieldsFunc(v, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }), true, nil
	case issue.KindDateTime:
		t, err := time.ParseInLocation("2006-01-02 15:04", v, im.tz)
		if err != nil {
			var ok bool
			if t, ok = parseJiraTime(v); !ok {
				return v, true, nil // translateCustom says it is invalid
			}
		}
		return t.Format("2006-01-02T15:04:05.000-0700"), true, nil
	}
	return v, true, nil
}

// user finds a person by username, else e-mail, else display name.
func (im *csvImport) user(name string) (importUser, error) {
	if u, ok := im.users[name]; ok {
		return u, nil
	}
	var u importUser
	for _, where := range []string{`username = LOWER(?)`, `email <> '' AND LOWER(email) = LOWER(?)`, `display_name = ?`} {
		var found []importUser
		if err := sqlq.Each(im.ctx, im.s.db, `SELECT id, username FROM users WHERE `+where+` ORDER BY id LIMIT 2`,
			func(rows *sql.Rows) error {
				var m importUser
				err := rows.Scan(&m.id, &m.username)
				found = append(found, m)
				return err
			}, name); err != nil {
			return u, err
		}
		if len(found) == 1 {
			u = found[0]
			break
		}
		if len(found) > 1 {
			u.err = "ambiguous"
			break
		}
	}
	im.users[name] = u
	return u, nil
}

// findNamed finds an item of a catalogue by id or name, ignoring case.
func findNamed[T any](items []T, v string, idName func(T) (string, string)) (T, bool) {
	for _, it := range items {
		if id, _ := idName(it); id == v {
			return it, true
		}
	}
	for _, it := range items {
		if _, name := idName(it); strings.EqualFold(name, v) {
			return it, true
		}
	}
	var zero T
	return zero, false
}

// splitNames splits a list the export joined with commas, keeping together
// the parts that make up a name with a comma.
func splitNames(v string, isName func(string) bool) []string {
	var parts []string
	for part := range strings.SplitSeq(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	var out []string
	for i := 0; i < len(parts); {
		j := len(parts)
		for ; j > i+1 && !isName(strings.Join(parts[i:j], ", ")); j-- {
		}
		out = append(out, strings.Join(parts[i:j], ", "))
		i = j
	}
	return out
}
