package db

import (
	"regexp"
	"slices"
	"testing"

	"solstice/internal/i18n"
)

// The icons 0023 gives types still named as a seeded default, in every
// language, as i18n.Match tells them.
func TestIssueTypeIconsMatchEveryLanguage(t *testing.T) {
	icons := map[string]i18n.Key{"task": i18n.DefaultTypeTask, "bug": i18n.DefaultTypeBug, "story": i18n.DefaultTypeStory,
		"epic": i18n.DefaultTypeEpic, "subtask": i18n.DefaultTypeSubtask}
	body, err := migrations.ReadFile("migrations/0023_issue_type_style.sql")
	if err != nil {
		t.Fatal(err)
	}
	update := regexp.MustCompile(`(?s)^UPDATE issue_types SET icon = '(\w+)' WHERE icon IS NULL AND name COLLATE utf8mb4_bin IN \((.*)\)$`)
	quoted := regexp.MustCompile(`'([^']*)'`)
	got := map[string][]string{}
	for _, stmt := range splitStatements(string(body)) {
		m := update.FindStringSubmatch(stmt)
		if m == nil {
			continue
		}
		for _, q := range quoted.FindAllStringSubmatch(m[2], -1) {
			got[m[1]] = append(got[m[1]], q[1])
		}
	}
	for icon, key := range icons {
		var want []string
		for _, l := range i18n.Locales {
			if name := i18n.M(key).In(l); !slices.Contains(want, name) {
				want = append(want, name)
			}
		}
		slices.Sort(want)
		names := slices.Clone(got[icon])
		slices.Sort(names)
		if !slices.Equal(names, want) {
			t.Errorf("names given the icon %s: %q, want %q", icon, got[icon], want)
		}
		delete(got, icon)
	}
	if len(got) > 0 {
		t.Errorf("icons of no default type: %v", got)
	}
}
