package sqlq

import (
	"database/sql"
	"reflect"
	"testing"
	"time"
)

func TestMarks(t *testing.T) {
	for n, want := range map[int]string{0: "", 1: "?", 3: "?, ?, ?"} {
		if got := Marks(n); got != want {
			t.Errorf("Marks(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestIDs(t *testing.T) {
	for _, tc := range []struct {
		in   []int64
		want []any
	}{
		{nil, nil},
		{[]int64{0, 0}, nil},
		{[]int64{3, 0, 1, 3, 1, 2}, []any{int64(3), int64(1), int64(2)}},
	} {
		if got := IDs(tc.in...); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("IDs(%v) = %#v, want %#v", tc.in, got, tc.want)
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

func TestTime(t *testing.T) {
	if v := Time(sql.NullTime{}); v != nil {
		t.Errorf("Time(NULL) = %#v, want nil", v)
	}
	at := time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC)
	v := Time(sql.NullTime{Time: at, Valid: true})
	if v == nil || !v.Equal(at) {
		t.Errorf("Time(valid) = %v, want %v", v, at)
	}
}

func TestNullIf(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  any
		want any
	}{
		{"empty", NullIf(""), nil},
		{"text", NullIf("x"), "x"},
		{"spaces", NullIf("  "), "  "},
		{"zero id", NullIf(int64(0)), nil},
		{"id", NullIf(int64(7)), int64(7)},
		{"zero count", NullIf(0), nil},
		{"count", NullIf(3), 3},
	} {
		if tc.got != tc.want {
			t.Errorf("NullIf %s = %#v, want %#v", tc.name, tc.got, tc.want)
		}
	}
}

func TestNullIfBlank(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want any
	}{
		{"", nil},
		{"  ", nil},
		{"\t\n ", nil},
		{"x", "x"},
		{" padded ", " padded "},
	} {
		if got := NullIfBlank(tc.in); got != tc.want {
			t.Errorf("NullIfBlank(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}

func TestEscapeLike(t *testing.T) {
	for in, want := range map[string]string{
		"plain":      "plain",
		`back\slash`: `back\\slash`,
		"50%_off":    `50\%\_off`,
		"%%":         `\%\%`,
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
		{"abc", 3, "abc"},
		{"abc", 0, ""},
		{"abc", -1, ""},
		{"éx", 1, "é"},
		{"a\xffbc", 2, "a\xff"}, // an invalid byte counts as one and is kept
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
