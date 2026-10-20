package jql

import "regexp"

// keyInText is keyPattern unanchored, upper case only: stored text writes
// keys the way projects store them.
var keyInText = regexp.MustCompile(`\b[A-Z][A-Z0-9_]*-\d+\b`)

// KeysIn returns the issue keys written in text, such as "WEB-12" in "This
// issue blocks WEB-12".
func KeysIn(text string) []string {
	return keyInText.FindAllString(text, -1)
}
