package notify

import (
	"testing"

	"solstice/internal/events"
)

// eligible owns the eligibility rule: an e-mail goes only to an active user
// with a usable address.
func TestEligible(t *testing.T) {
	for _, c := range []struct {
		name string
		u    *events.User
		want bool
	}{
		{"nobody", nil, false},
		{"inactive", &events.User{Active: false, Email: "paul.martin@exemple.fr"}, false},
		{"no address", &events.User{Active: true}, false},
		{"not an address", &events.User{Active: true, Email: "Paul Martin"}, false},
		{"active with address", &events.User{Active: true, Email: "paul.martin@exemple.fr"}, true},
	} {
		if got := eligible(c.u); got != c.want {
			t.Errorf("%s: eligible() = %v, want %v", c.name, got, c.want)
		}
	}
}
