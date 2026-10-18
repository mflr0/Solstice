package api

import (
	"math/rand/v2"
	"sort"
	"testing"
)

func TestRankBetweenBasics(t *testing.T) {
	for _, c := range [][2]string{
		{"", ""},
		{"", "0|i0000f:"},
		{"0|i0000f:", ""},
		{"0|i0000f:", "0|i0000g:"},
		{"0|i0000f:", "0|i0000f:a"},
		{"a", "b"},
		{"a", "a1"},
		{"0|hzzzzz:", "0|i00000:"},
		{"1", "2"},
	} {
		got := rankBetween(c[0], c[1])
		if !(c[0] == "" || c[0] < got) || !(c[1] == "" || got < c[1]) {
			t.Errorf("rankBetween(%q, %q) = %q: not in between", c[0], c[1], got)
		}
		if got[len(got)-1] == rankMin {
			t.Errorf("rankBetween(%q, %q) = %q ends on the smallest digit", c[0], c[1], got)
		}
	}
}

// Repeatedly inserting into random gaps must keep a strict order: this is
// what dragging cards around a board does.
func TestRankBetweenKeepsOrder(t *testing.T) {
	rnd := rand.New(rand.NewPCG(1, 2))
	ranks := []string{rankBetween("", "")}
	for range 5000 {
		i := rnd.IntN(len(ranks) + 1)
		var a, b string
		if i > 0 {
			a = ranks[i-1]
		}
		if i < len(ranks) {
			b = ranks[i]
		}
		r := rankBetween(a, b)
		if (a != "" && r <= a) || (b != "" && r >= b) {
			t.Fatalf("rankBetween(%q, %q) = %q", a, b, r)
		}
		ranks = append(ranks[:i], append([]string{r}, ranks[i:]...)...)
	}
	if !sort.StringsAreSorted(ranks) {
		t.Fatal("ranks are out of order")
	}
	for _, r := range ranks {
		if len(r) > 40 {
			t.Fatalf("rank grew too long: %q", r)
		}
	}
}
