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

func TestEscapeLike(t *testing.T) {
	for in, want := range map[string]string{
		"plain":      "plain",
		`back\slash`: `back\\slash`,
		"50%_off":    `50\%\_off`,
		`a\%_`:       `a\\\%\_`,
	} {
		if got := EscapeLike(in); got != want {
			t.Errorf("EscapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	for _, tc := range []struct {
		in   string
		n    int
		want string
	}{
		{"abc", 5, "abc"},
		{"abcdef", 3, "abc"},
		{"héllo", 4, "héll"},
		{"日本語", 2, "日本"},
		{"", 0, ""},
	} {
		if got := Truncate(tc.in, tc.n); got != tc.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestProjectKey(t *testing.T) {
	want := "(p.pkey IN (?, ?) OR p.id IN (SELECT project_id FROM project_keys WHERE pkey IN (?, ?)))"
	if got := ProjectKey("p", Marks(2)); got != want {
		t.Errorf("ProjectKey = %q, want %q", got, want)
	}
}
