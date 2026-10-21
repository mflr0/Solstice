package auth

import (
	"reflect"
	"strings"
	"testing"
)

func TestGroupNames(t *testing.T) {
	got := groupNames([]string{" Developers", "developers", "", "  ", "QA", strings.Repeat("x", 256)})
	if want := []string{"developers", "qa"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
