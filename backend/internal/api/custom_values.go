package api

// Custom field values: translateCustom reads a new value from a request in
// Jira's form, applyCustom and writeCustom store it with its history line.
// The REST API and the interface API both write custom values through here.

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"solstice/internal/i18n"
	"solstice/internal/perm"
	"solstice/internal/sqlq"
)

// customWrite replaces the value of a custom field.
type customWrite struct {
	field   customFieldDef
	rows    []customRaw
	labels  []string
	display string // the new value, for the history
}

// editProject is what translating a request needs to know of the project.
type editProject struct {
	id        int64
	key       string
	lead      sql.NullInt64
	assignTo  string // default assignee: lead or unassigned
	customs   []customFieldDef
	issueType string
}

// safeURL reports whether a url custom field value may become a link: only
// the http, https and mailto schemes are accepted.
func safeURL(v string) bool {
	u, err := url.Parse(strings.TrimSpace(v))
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto":
		return true
	}
	return false
}

// The kinds translateCustom takes values for; a test checks the list.
var settableKinds = []string{"gh-epic-link", "gh-sprint", "url", "textfield", "gh-epic-label", "textarea", "float",
	"datepicker", "datetime", "select", "radiobuttons", "gh-epic-status", "multiselect", "multicheckboxes", "cascadingselect",
	"userpicker", "multiuserpicker", "labels"}

func settableKind(kind string) bool { return slices.Contains(settableKinds, kind) }

// translateCustom reads a custom field's new value. The epic link and the
// sprint, which Solstice keeps as properties of the issue, become those.
func (s *Server) translateCustom(ctx context.Context, p *editProject, edit *jiraEdit, field string,
	raw json.RawMessage) error {
	var f customFieldDef
	found := false
	for _, cf := range p.customs {
		if customID(cf.ID) == field {
			f, found = cf, true
		}
	}
	if !found {
		return fieldError(field, failf(http.StatusBadRequest, i18n.FieldNotSettable, field))
	}
	isNull := len(raw) == 0 || string(raw) == "null"
	if !isNull && !settableKind(f.Kind) {
		return fieldError(field, failf(http.StatusBadRequest, i18n.FieldNotSettable, field))
	}
	switch f.Kind {
	case "gh-epic-link":
		var key *string
		if json.Unmarshal(raw, &key) != nil {
			return invalidValue(field)
		}
		b, _ := json.Marshal(key)
		edit.ui["epic"] = b
		return nil
	case "gh-sprint":
		var id int64
		if !isNull {
			var n json.Number
			var list []json.Number
			switch {
			case json.Unmarshal(raw, &n) == nil:
			case json.Unmarshal(raw, &list) == nil && len(list) == 1:
				n = list[0]
			default:
				return invalidValue(field)
			}
			v, err := n.Int64()
			if err != nil {
				return invalidValue(field)
			}
			id = v
		}
		edit.sprint = &id
		return nil
	}
	w := customWrite{field: f}
	if isNull {
		edit.custom = append(edit.custom, w)
		return nil
	}
	var display []string
	addText := func(column string, v string) {
		row := customRaw{}
		switch column {
		case "text":
			row.text = sql.NullString{String: v, Valid: true}
		default:
			row.str = sql.NullString{String: v, Valid: true}
		}
		w.rows = append(w.rows, row)
		display = append(display, v)
	}
	switch f.Kind {
	case "url":
		var v string
		if json.Unmarshal(raw, &v) != nil || utf8.RuneCountInString(v) > 255 {
			return invalidValue(field)
		}
		if strings.TrimSpace(v) != "" {
			if !safeURL(v) {
				return invalidValue(field)
			}
			addText("string", v)
		}
	case "textfield", "gh-epic-label":
		var v string
		if json.Unmarshal(raw, &v) != nil || utf8.RuneCountInString(v) > 255 {
			return invalidValue(field)
		}
		if strings.TrimSpace(v) != "" {
			addText("string", v)
		}
	case "textarea":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		if strings.TrimSpace(v) != "" {
			addText("text", v)
		}
	case "float":
		var n json.Number
		var str string
		if json.Unmarshal(raw, &str) == nil {
			n = json.Number(strings.TrimSpace(str))
		} else if json.Unmarshal(raw, &n) != nil {
			return invalidValue(field)
		}
		v, err := n.Float64()
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return invalidValue(field)
		}
		w.rows = append(w.rows, customRaw{num: sql.NullFloat64{Float64: v, Valid: true}})
		display = append(display, strconv.FormatFloat(v, 'f', -1, 64))
	case "datepicker":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		d, err := time.Parse(time.DateOnly, v)
		if err != nil {
			return fieldError(field, fail(http.StatusBadRequest, i18n.IssueDueDateInvalid))
		}
		w.rows = append(w.rows, customRaw{date: sql.NullTime{Time: d, Valid: true}})
		display = append(display, v)
	case "datetime":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		t, ok := parseJiraTime(v)
		if !ok {
			return invalidValue(field)
		}
		w.rows = append(w.rows, customRaw{date: sql.NullTime{Time: t.UTC(), Valid: true}})
		display = append(display, t.UTC().Format(time.RFC3339))
	case "select", "radiobuttons", "gh-epic-status", "multiselect", "multicheckboxes", "cascadingselect":
		options, err := s.fieldOptions(ctx, f.ID)
		if err != nil {
			return err
		}
		var refs []ref
		if f.Kind == "multiselect" || f.Kind == "multicheckboxes" {
			var ok bool
			if refs, ok = parseRefs(raw); !ok {
				return invalidValue(field)
			}
		} else {
			r, ok := parseRef(raw)
			if !ok {
				return invalidValue(field)
			}
			refs = []ref{r}
		}
		for _, r := range refs {
			opt, ok := pickOption(options, r, 0)
			if !ok {
				return unknownValue(field, r.text())
			}
			w.rows = append(w.rows, customRaw{str: sql.NullString{String: strconv.FormatInt(opt.id, 10), Valid: true}})
			display = append(display, opt.value)
		}
		if f.Kind == "cascadingselect" {
			var m struct {
				Child json.RawMessage `json:"child"`
			}
			json.Unmarshal(raw, &m)
			if r, ok := parseRef(m.Child); ok && !r.null && len(w.rows) == 1 {
				parentID, _ := strconv.ParseInt(w.rows[0].str.String, 10, 64)
				opt, ok := pickOption(options, r, parentID)
				if !ok {
					return unknownValue(field, r.text())
				}
				w.rows = append(w.rows, customRaw{parentKey: sql.NullString{String: "1", Valid: true},
					str: sql.NullString{String: strconv.FormatInt(opt.id, 10), Valid: true}})
				display[0] += " › " + opt.value
			}
		}
	case "userpicker", "multiuserpicker":
		var refs []ref
		if f.Kind == "multiuserpicker" {
			var ok bool
			if refs, ok = parseRefs(raw); !ok {
				return invalidValue(field)
			}
		} else {
			r, ok := parseRef(raw)
			if !ok {
				return invalidValue(field)
			}
			refs = []ref{r}
		}
		for _, r := range refs {
			id, found := s.userByName(ctx, r.text())
			if !found {
				return unknownValue(field, r.text())
			}
			_, name := s.userName(ctx, id)
			w.rows = append(w.rows, customRaw{str: sql.NullString{String: strconv.FormatInt(id, 10), Valid: true}})
			display = append(display, name)
		}
	case "labels":
		var v []string
		if json.Unmarshal(raw, &v) != nil {
			return invalidValue(field)
		}
		seen := map[string]bool{}
		for _, l := range v {
			l = strings.Join(strings.Fields(l), "_")
			if l != "" && utf8.RuneCountInString(l) <= 255 && !seen[l] {
				seen[l] = true
				w.labels = append(w.labels, l)
			}
		}
		slices.Sort(w.labels)
		display = w.labels
	default:
		return fieldError(field, failf(http.StatusBadRequest, i18n.FieldNotSettable, field))
	}
	w.display = strings.Join(display, ", ")
	edit.custom = append(edit.custom, w)
	return nil
}

type fieldOption struct {
	id, parent int64
	value      string
	disabled   bool
}

func (s *Server) fieldOptions(ctx context.Context, fieldID int64) ([]fieldOption, error) {
	var out []fieldOption
	err := sqlq.Each(ctx, s.db, `SELECT id, COALESCE(parent_id, 0), value, disabled FROM custom_field_options
		WHERE field_id = ? ORDER BY position, id`, func(rows *sql.Rows) error {
		var o fieldOption
		if err := rows.Scan(&o.id, &o.parent, &o.value, &o.disabled); err != nil {
			return err
		}
		out = append(out, o)
		return nil
	}, fieldID)
	return out, err
}

// pickOption finds an enabled option under parent (0: top level) by id or
// value (exact, then ignoring case).
func pickOption(options []fieldOption, r ref, parent int64) (fieldOption, bool) {
	for pass := range 3 {
		for _, o := range options {
			if o.parent != parent || o.disabled {
				continue
			}
			switch {
			case pass == 0 && r.id != "" && strconv.FormatInt(o.id, 10) == r.id:
				return o, true
			case pass == 1 && r.text() != "" && o.value == r.text():
				return o, true
			case pass == 2 && r.text() != "" && strings.EqualFold(o.value, r.text()):
				return o, true
			}
		}
	}
	return fieldOption{}, false
}

// applyCustom stores custom field values and records the change in the
// issue's history. Callers check the permission first.
func (s *Server) applyCustom(ctx context.Context, sub *perm.Subject, o *restOut, iss *issueRow, writes []customWrite) error {
	if len(writes) == 0 {
		return nil
	}
	before, err := s.customBefore(ctx, sub, o, iss, writes)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	changes, err := writeCustom(ctx, tx, iss.ID, writes, before)
	if err != nil || len(changes) == 0 {
		return err
	}
	now := time.Now().UTC()
	if err := recordHistory(ctx, tx, iss.ID, sub.UserID, now, nil, changes); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, now, iss.ID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.changed(iss, sub.UserID, "issue", changes) // notifications and webhooks too
	return nil
}

// customBefore returns, for each write, the value it replaces as the history
// shows it.
func (s *Server) customBefore(ctx context.Context, sub *perm.Subject, o *restOut, iss *issueRow, writes []customWrite) ([]string, error) {
	if len(writes) == 0 {
		return nil, nil
	}
	set, err := s.loadIssues(ctx, sub, []*issueRow{iss}, func(k string) bool { return k == loadCustom })
	if err != nil {
		return nil, err
	}
	users, err := s.jiraUsers(ctx, o, set.userIDs()...)
	if err != nil {
		return nil, err
	}
	before := make([]string, len(writes))
	for i, w := range writes {
		before[i] = o.customText(set, users, w.field, set.list[0])
	}
	return before, nil
}

// writeCustom stores custom field values in tx and returns the history lines
// of those that differ from before (nil: a new issue, which has none).
func writeCustom(ctx context.Context, tx *sql.Tx, issueID int64, writes []customWrite, before []string) ([]change, error) {
	var changes []change
	for i, w := range writes {
		var was string
		if before != nil {
			was = before[i]
		}
		if was == w.display {
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM custom_field_values WHERE issue_id = ? AND field_id = ?`,
			issueID, w.field.ID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM labels WHERE issue_id = ? AND field_id = ?`, issueID,
			w.field.ID); err != nil {
			return nil, err
		}
		for _, v := range w.rows {
			if _, err := tx.ExecContext(ctx, `INSERT INTO custom_field_values (issue_id, field_id, parent_key,
				string_value, number_value, text_value, date_value) VALUES (?, ?, ?, ?, ?, ?, ?)`, issueID, w.field.ID,
				v.parentKey, v.str, v.num, v.text, v.date); err != nil {
				return nil, err
			}
		}
		for _, l := range w.labels {
			if _, err := tx.ExecContext(ctx, `INSERT INTO labels (issue_id, field_id, label) VALUES (?, ?, ?)`,
				issueID, w.field.ID, l); err != nil {
				return nil, err
			}
		}
		changes = append(changes, change{field: w.field.Name, oldString: nullIfEmpty(was),
			newString: nullIfEmpty(w.display)})
	}
	return changes, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// editProjectOf loads what translating a request needs about a project.
func (s *Server) editProjectOf(ctx context.Context, projectID int64) (*editProject, error) {
	p := &editProject{id: projectID}
	if err := s.db.QueryRowContext(ctx, `SELECT pkey, lead_id, default_assignee FROM projects WHERE id = ?`, projectID).
		Scan(&p.key, &p.lead, &p.assignTo); err != nil {
		return nil, err
	}
	var err error
	p.customs, err = s.customFields(ctx)
	return p, err
}
