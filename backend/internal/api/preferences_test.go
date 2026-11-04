package api

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"solstice/internal/i18n"
)

// A preference value is refused with a 400 and its own message when its key
// is unknown or its shape is wrong.
func TestPreferenceValueRefusals(t *testing.T) {
	five := `{"search": ["a", "b", "c", "d", "e"]}`
	for _, c := range []struct {
		name, value string
		want        i18n.Message
	}{
		{"theme", `"dark"`, i18n.M(i18n.PreferenceUnknown, "theme")},
		{"shortcutsEnabled", `"yes"`, i18n.M(i18n.PreferenceInvalid, "shortcutsEnabled")},
		{"shortcutsEnabled", `null`, i18n.M(i18n.PreferenceInvalid, "shortcutsEnabled")},
		{"shortcutsEnabled", `1`, i18n.M(i18n.PreferenceInvalid, "shortcutsEnabled")},
		{"shortcuts", `["search"]`, i18n.M(i18n.PreferenceInvalid, "shortcuts")},
		{"shortcuts", `null`, i18n.M(i18n.PreferenceInvalid, "shortcuts")},
		{"shortcuts", `{"search": null}`, i18n.M(i18n.PreferenceInvalid, "shortcuts")},
		{"shortcuts", `{"search": "/"}`, i18n.M(i18n.PreferenceInvalid, "shortcuts")},
		{"shortcuts", `{"search": [1]}`, i18n.M(i18n.PreferenceInvalid, "shortcuts")},
		{"shortcuts", `{"fly": ["f"]}`, i18n.M(i18n.PreferenceShortcutActionUnknown, "fly")},
		{"shortcuts", five, i18n.M(i18n.PreferenceShortcutBindingsTooMany, maxShortcutBindings)},
		{"shortcuts", `{"search": [""]}`, i18n.M(i18n.PreferenceShortcutBindingInvalid, maxBindingLength)},
		{"shortcuts", `{"search": [null]}`, i18n.M(i18n.PreferenceShortcutBindingInvalid, maxBindingLength)},
		{"shortcuts", `{"search": ["Alt k"]}`, i18n.M(i18n.PreferenceShortcutBindingInvalid, maxBindingLength)},
		{"shortcuts", `{"search": ["\t"]}`, i18n.M(i18n.PreferenceShortcutBindingInvalid, maxBindingLength)},
		{"shortcuts", `{"search": ["` + strings.Repeat("é", maxBindingLength+1) + `"]}`,
			i18n.M(i18n.PreferenceShortcutBindingInvalid, maxBindingLength)},
	} {
		_, err := preferenceValue(c.name, json.RawMessage(c.value))
		var he *httpError
		if !errors.As(err, &he) || he.status != 400 || err.Error() != c.want.String() {
			t.Errorf("%s = %s: %v, want 400 %q", c.name, c.value, err, c.want)
		}
	}
}

// A valid value is stored compacted.
func TestPreferenceValueAccepted(t *testing.T) {
	for _, c := range []struct{ name, value, want string }{
		{"shortcutsEnabled", ` false `, `false`},
		{"shortcuts", `{}`, `{}`},
		{"shortcuts", `{"toggleScheme": ["d"]}`, `{"toggleScheme":["d"]}`},
		{"shortcuts", `{"search": ["Alt-k", "/"], "watch": []}`, `{"search":["Alt-k","/"],"watch":[]}`},
		{"shortcuts", `{"open": ["` + strings.Repeat("é", maxBindingLength) + `", "a", "b", "c"]}`,
			`{"open":["` + strings.Repeat("é", maxBindingLength) + `","a","b","c"]}`},
	} {
		got, err := preferenceValue(c.name, json.RawMessage(c.value))
		if err != nil || string(got) != c.want {
			t.Errorf("%s = %s: %s, %v, want %s", c.name, c.value, got, err, c.want)
		}
	}
}
