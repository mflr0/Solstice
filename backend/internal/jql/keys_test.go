package jql

import (
	"slices"
	"strings"
	"testing"
)

func TestKeysInFindsWhatTheParserAccepts(t *testing.T) {
	for _, key := range []string{"X-1", "WEB-12", "A1-7", "AB_C-30", "Q9_-4", "RH-0012"} {
		if !keyPattern.MatchString(key) {
			t.Fatalf("%s is not a key for the parser", key)
		}
		for _, text := range []string{key, "This issue blocks " + key, "(" + key + ")", key + ", WEB-1"} {
			if got := KeysIn(text); len(got) == 0 || got[0] != key {
				t.Errorf("KeysIn(%q) = %q, want %s first", text, got, key)
			}
		}
	}
}

func TestKeysInFindsNothingTheParserRejects(t *testing.T) {
	for _, text := range []string{"WEB-", "-12", "1A-3", "_WEB-3", "WEB-3x", "WEB 12", "WÉB-3"} {
		for _, key := range KeysIn(text) {
			if !keyPattern.MatchString(key) {
				t.Errorf("KeysIn(%q) found %q, which the parser rejects", text, key)
			}
		}
	}
	if got := KeysIn("blocks WEB-1 and is blocked by X-2"); !slices.Equal(got, []string{"WEB-1", "X-2"}) {
		t.Errorf("two keys: %q", got)
	}
	if got := KeysIn(strings.ToLower("blocks WEB-1")); len(got) != 0 {
		t.Errorf("lower case: %q", got)
	}
}
