package api

// Tests of the CSV export's formula guard.

import (
	"testing"
)

func TestGuardFormula(t *testing.T) {
	for in, want := range map[string]string{"=1+1": "'=1+1", "+33 6": "'+33 6", "-2+3": "'-2+3", "@SUM(A1)": "'@SUM(A1)",
		"\tx": "'\tx", "Panier": "Panier", "": "", "2026-10-15": "2026-10-15"} {
		if got := guardFormula(in); got != want {
			t.Errorf("guardFormula(%q) = %q, want %q", in, got, want)
		}
	}
}
