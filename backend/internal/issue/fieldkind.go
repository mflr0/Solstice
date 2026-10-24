package issue

import "strings"

// FieldKind returns the last part of a Jira custom field type key, e.g.
// "userpicker" for com.atlassian.jira.plugin.system.customfieldtypes:userpicker.
func FieldKind(typeKey string) string {
	if i := strings.LastIndexByte(typeKey, ':'); i >= 0 {
		return typeKey[i+1:]
	}
	return typeKey
}

// Custom field kinds, as FieldKind returns them.
const (
	KindTextField       = "textfield"
	KindTextArea        = "textarea"
	KindURL             = "url"
	KindFloat           = "float"
	KindDatePicker      = "datepicker"
	KindDateTime        = "datetime"
	KindLabels          = "labels"
	KindSelect          = "select"
	KindMultiSelect     = "multiselect"
	KindRadioButtons    = "radiobuttons"
	KindMultiCheckboxes = "multicheckboxes"
	KindCascadingSelect = "cascadingselect"
	KindUserPicker      = "userpicker"
	KindMultiUserPicker = "multiuserpicker"
	KindSprint          = "gh-sprint"
	KindEpicLink        = "gh-epic-link"
	KindEpicLabel       = "gh-epic-label"
	KindEpicStatus      = "gh-epic-status"
	KindEpicColor       = "gh-epic-color"
	KindLexoRank        = "gh-lexo-rank"
	KindGlobalRank      = "gh-global-rank"
)
