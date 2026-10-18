package auth

import "testing"

func TestCanonicalUsername(t *testing.T) {
	for typed, want := range map[string]string{
		"admin":                     "admin",
		"Admin":                     "admin",
		"admin ":                    "admin",
		"admin   ":                  "admin",
		" admin":                    "admin",
		"  Admin  ":                 "admin",
		"\tADMIN\n":                 "admin",
		"Marie.Dupont@Example.org ": "marie.dupont@example.org",
		"":                          "",
		"   ":                       "",
	} {
		if got := CanonicalUsername(typed); got != want {
			t.Errorf("CanonicalUsername(%q) = %q, want %q", typed, got, want)
		}
	}
}
