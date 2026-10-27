package auth

import (
	"errors"
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

func TestApplyGroups(t *testing.T) {
	e := &DirectoryEntry{Groups: []string{"staff", "IT"}}
	if err := ApplyGroups(e, "Staff", "it"); err != nil || e.Admin == nil || !*e.Admin {
		t.Errorf("in both groups: %v, admin %v", err, e.Admin)
	}
	e = &DirectoryEntry{Groups: []string{"staff"}}
	if err := ApplyGroups(e, "Staff", "it"); err != nil || e.Admin == nil || *e.Admin {
		t.Errorf("staff only: %v, admin %v", err, e.Admin)
	}
	if err := ApplyGroups(&DirectoryEntry{Groups: []string{"it"}}, "Staff", "it"); !errors.Is(err, ErrNoSession) {
		t.Errorf("outside the required group: %v", err)
	}
	// Without an administrators' group, the right is left to Solstice.
	e = &DirectoryEntry{}
	if err := ApplyGroups(e, "", ""); err != nil || e.Admin != nil {
		t.Errorf("no groups set: %v, admin %v", err, e.Admin)
	}
}
