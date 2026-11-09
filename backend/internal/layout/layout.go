// Package layout shapes the create and view forms of an issue type: which
// fields they show, in which order, where, and which a creation needs.
// A layout comes from a project's override of the type, else the type's
// template, else the built-in default, which is what the forms showed before
// layouts existed.
package layout

import (
	"slices"
	"strconv"
	"strings"

	"solstice/internal/i18n"
)

// Form is a form a layout shapes.
type Form string

const (
	Create Form = "create"
	View   Form = "view"
)

// ParseForm reads a form's name.
func ParseForm(s string) (Form, bool) {
	f := Form(s)
	return f, f == Create || f == View
}

// Zone is where a field goes in the view.
type Zone string

const (
	Main Zone = "main"
	Side Zone = "side"
)

// customPrefix starts the id of a custom field, as in Jira's REST API.
const customPrefix = "customfield_"

// Field is one field of a layout.
type Field struct {
	ID       string `json:"field"` // a system field's Jira REST id, or customfield_<id>
	Zone     Zone   `json:"zone,omitempty"`
	Hidden   bool   `json:"hidden"`
	Required bool   `json:"required,omitempty"`
}

// CustomID gives a custom field's id, from its field id.
func CustomID(id int64) string { return customPrefix + strconv.FormatInt(id, 10) }

// Custom gives the id of the custom field f places, 0 for a system field.
func (f Field) Custom() int64 {
	n, ok := strings.CutPrefix(f.ID, customPrefix)
	if !ok {
		return 0
	}
	id, err := strconv.ParseInt(n, 10, 64)
	if err != nil || id <= 0 || CustomID(id) != f.ID {
		return 0
	}
	return id
}

// fixed are the fields every form shows in its own place.
var fixed = []string{"project", "issuetype", "summary"}

// systemFields are the system fields both forms place, in no order.
var systemFields = []string{"description", "environment", "priority", "assignee", "reporter", "components",
	"fixVersions", "versions", "labels", "duedate", "timetracking", "security"}

// viewOnly are the system fields only the view shows: they are not set.
var viewOnly = []string{"created", "updated", "resolutiondate", "epic", "sprint"}

// Placeable lists the system fields a form places.
func Placeable(form Form) []string {
	if form == View {
		return slices.Concat(systemFields, viewOnly)
	}
	return slices.Clone(systemFields)
}

// defaults are the built-in layouts: the fields each form shows, in the
// order it shows them, then those it doesn't, hidden.
var defaults = map[Form][]Field{
	Create: {
		{ID: "priority"}, {ID: "description"}, {ID: "assignee"}, {ID: "duedate"}, {ID: "timetracking"},
		{ID: "components"}, {ID: "fixVersions"}, {ID: "security"}, {ID: "labels"},
		{ID: "environment", Hidden: true}, {ID: "reporter", Hidden: true}, {ID: "versions", Hidden: true},
	},
	View: {
		{ID: "description", Zone: Main}, {ID: "environment", Zone: Main},
		{ID: "assignee", Zone: Side}, {ID: "reporter", Zone: Side}, {ID: "priority", Zone: Side},
		{ID: "components", Zone: Side}, {ID: "fixVersions", Zone: Side}, {ID: "versions", Zone: Side},
		{ID: "epic", Zone: Side}, {ID: "sprint", Zone: Side}, {ID: "labels", Zone: Side},
		{ID: "duedate", Zone: Side}, {ID: "security", Zone: Side}, {ID: "created", Zone: Side},
		{ID: "updated", Zone: Side}, {ID: "resolutiondate", Zone: Side}, {ID: "timetracking", Zone: Side},
	},
}

// Default gives a form's built-in layout.
func Default(form Form) []Field { return slices.Clone(defaults[form]) }

// Error is a layout a template may not hold: Key, about the field Field.
type Error struct {
	Key   i18n.Key
	Field string
}

func (e *Error) Error() string { return i18n.M(e.Key, e.Field).String() }

// Check refuses a template a form may not hold. It does not know which
// custom fields exist: it only checks their ids are well formed.
func Check(form Form, fields []Field) error {
	placeable := Placeable(form)
	seen := map[string]bool{}
	for _, f := range fields {
		switch {
		case slices.Contains(fixed, f.ID):
			return &Error{i18n.LayoutFieldFixed, f.ID}
		case f.Custom() == 0 && !slices.Contains(placeable, f.ID):
			return &Error{i18n.FieldUnknown, f.ID}
		case seen[f.ID]:
			return &Error{i18n.LayoutFieldTwice, f.ID}
		case form == Create && f.Zone != "":
			return &Error{i18n.LayoutZoneOnCreate, f.ID}
		case form == View && f.Zone != Main && f.Zone != Side:
			return &Error{i18n.LayoutZoneInvalid, f.ID}
		case form == View && f.Required:
			return &Error{i18n.LayoutRequiredOnView, f.ID}
		case f.Hidden && f.Required:
			return &Error{i18n.LayoutRequiredHidden, f.ID}
		}
		seen[f.ID] = true
	}
	return nil
}

// Resolve gives the layout a form shows: override if customised (not
// empty), else typ, else the default. It drops the custom fields offered
// leaves out, and appends, shown, those it holds that the layout doesn't
// place, in its order, then the system fields the layout misses, hidden.
func Resolve(form Form, override, typ []Field, offered []int64) []Field {
	base := override
	if len(base) == 0 {
		base = typ
	}
	if len(base) == 0 {
		base = defaults[form]
	}
	out := make([]Field, 0, len(base)+len(offered))
	placed := map[string]bool{}
	for _, f := range base {
		if id := f.Custom(); id != 0 && !slices.Contains(offered, id) {
			continue
		}
		out = append(out, f)
		placed[f.ID] = true
	}
	for _, id := range offered {
		if f := (Field{ID: CustomID(id)}); !placed[f.ID] {
			if form == View {
				f.Zone = Side
			}
			out = append(out, f)
		}
	}
	for _, f := range defaults[form] {
		if !placed[f.ID] {
			f.Hidden = true
			out = append(out, f)
		}
	}
	return out
}
