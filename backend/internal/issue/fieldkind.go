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
