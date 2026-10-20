package issue

import (
	"regexp"
	"strings"
)

var keyPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)-(\d+)$`)

// keyInText is keyPattern unanchored, upper case only: stored text writes
// keys the way projects store them.
var keyInText = regexp.MustCompile(`\b[A-Z][A-Z0-9_]*-\d+\b`)

// ParseKey splits an issue key such as "web-12" into its project key, in
// upper case, and its number, as written. ok is false when s is not a key.
func ParseKey(s string) (project, num string, ok bool) {
	m := keyPattern.FindStringSubmatch(s)
	if m == nil {
		return "", "", false
	}
	return strings.ToUpper(m[1]), m[2], true
}

// KeysIn returns the issue keys written in text, such as "WEB-12" in "This
// issue blocks WEB-12".
func KeysIn(text string) []string {
	return keyInText.FindAllString(text, -1)
}
