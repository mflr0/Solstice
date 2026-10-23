package api

import (
	"slices"
	"testing"
)

func TestBulkSetApply(t *testing.T) {
	have := []string{"a", "b"}
	for _, c := range []struct {
		mode   string
		values []string
		want   []string
	}{
		{"add", []string{"b", "c"}, []string{"a", "b", "c"}},
		{"remove", []string{"a", "z"}, []string{"b"}},
		{"replace", []string{"c"}, []string{"c"}},
		{"replace", nil, []string{}},
	} {
		b := &bulkSet[string]{Mode: c.mode, Values: c.values}
		if got := b.apply(have); !slices.Equal(got, c.want) {
			t.Errorf("%s %v on %v: %v, want %v", c.mode, c.values, have, got, c.want)
		}
	}
	if !slices.Equal(have, []string{"a", "b"}) {
		t.Errorf("apply changed its input: %v", have)
	}
}
