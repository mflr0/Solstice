package api

import (
	"testing"

	"solstice/internal/i18n"
)

// Every seeded text, in every language, names one default of its catalogue
// only, so the key a catalogue row gets is never a guess.
func TestSeededTextsAreUnambiguous(t *testing.T) {
	for name, keys := range map[string][]i18n.Key{
		"priorities": seededPriorities, "resolutions": seededResolutions, "issue types": seededIssueTypes,
		"link names": seededLinkNames, "link verbs": seededLinkVerbs,
	} {
		for _, k := range keys {
			for _, loc := range i18n.Locales {
				text := i18n.M(k).In(loc)
				if got := i18n.Match(text, keys...); got != k {
					t.Errorf("%s: %q (%s, %s) matches %s", name, text, k, loc, got)
				}
			}
		}
	}
}
