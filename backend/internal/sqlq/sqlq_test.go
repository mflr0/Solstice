package sqlq

import (
	"database/sql"
	"testing"
)

func TestMarks(t *testing.T) {
	for n, want := range map[int]string{0: "", 1: "?", 3: "?, ?, ?"} {
		if got := Marks(n); got != want {
			t.Errorf("Marks(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestValue(t *testing.T) {
	if v := Value(sql.NullString{}); v != nil {
		t.Errorf("Value(NULL string) = %#v, want nil", v)
	}
	if v := Value(sql.NullString{String: "a", Valid: true}); v != "a" {
		t.Errorf("Value(\"a\") = %#v", v)
	}
	if v := Value(sql.NullInt64{Int64: 7, Valid: true}); v != int64(7) {
		t.Errorf("Value(7) = %#v", v)
	}
}
