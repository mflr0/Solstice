package api

import (
	"strings"
	"testing"

	"solstice/internal/i18n"
)

// An empty option and a too long one are refused with their own message.
func TestCleanOptionRefusals(t *testing.T) {
	for _, c := range []struct {
		in   string
		want i18n.Key
	}{
		{"", i18n.CustomFieldOptionEmpty},
		{"  ", i18n.CustomFieldOptionEmpty},
		{strings.Repeat("é", 256), i18n.CustomFieldOptionTooLong},
	} {
		_, err := cleanOption(c.in)
		if want := i18n.M(c.want).String(); err == nil || err.Error() != want {
			t.Errorf("cleanOption(%q) = %v, want %q", c.in, err, want)
		}
	}
	if v, err := cleanOption(" " + strings.Repeat("é", 255) + " "); err != nil || v != strings.Repeat("é", 255) {
		t.Errorf("255 characters: %q, %v", v, err)
	}
}
