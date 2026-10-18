package jirafixture

import (
	"os"
	"regexp"
	"testing"
)

// The committed seed of the integration tests is a public file: its accounts
// must be the fixture's.
func TestCommittedSeedAccounts(t *testing.T) {
	seed, err := os.ReadFile("../../tests/testdata/seed.sql")
	if err != nil {
		t.Fatal(err)
	}
	display := map[string]string{"ancien.collegue": "ancien.collegue"}
	for _, u := range append(internalUsers, ldapOnlyUsers...) {
		display[u.name] = u.first + " " + u.last
	}
	rows := regexp.MustCompile("(?m)^INSERT INTO `users` VALUES .*$").FindAll(seed, -1)
	account := regexp.MustCompile(`\(\d+,'([^']*)','[^']*','([^']*)'`)
	n := 0
	for _, row := range rows {
		for _, m := range account.FindAllSubmatch(row, -1) {
			n++
			want, ok := display[string(m[1])]
			if !ok {
				t.Errorf("user name %q is not one of the fixture's", m[1])
			} else if string(m[2]) != want {
				t.Errorf("display name of %s is %q, the fixture gives %q", m[1], m[2], want)
			}
		}
	}
	if n == 0 {
		t.Fatal("no user found in the seed")
	}
}
