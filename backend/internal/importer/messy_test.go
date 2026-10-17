package importer

import (
	"database/sql"
	"reflect"
	"testing"
	"time"
)

func TestLeftOut(t *testing.T) {
	r := &run{rep: &Report{}}
	r.leftOut(0, "comment", "comments", "of a missing issue")
	if len(r.rep.Warnings) != 0 {
		t.Fatalf("a warning for nothing: %v", r.rep.Warnings)
	}
	r.leftOut(1, "comment", "comments", "of a missing issue")
	r.leftOut(3, "work log", "work logs", "of a missing issue")
	want := []string{"1 comment of a missing issue, not imported", "3 work logs of a missing issue, not imported"}
	if !reflect.DeepEqual(r.rep.Warnings, want) {
		t.Errorf("warnings = %q, want %q", r.rep.Warnings, want)
	}
}

func TestFitsType(t *testing.T) {
	const prefix = "com.atlassian.jira.plugin.system.customfieldtypes:"
	str := sql.NullString{String: "CLI-0042", Valid: true}
	text := sql.NullString{String: "long", Valid: true}
	num := sql.NullFloat64{Float64: 3, Valid: true}
	date := sql.NullTime{Time: time.Now(), Valid: true}
	none := sql.NullString{}
	for _, c := range []struct {
		kind string
		str  sql.NullString
		text sql.NullString
		num  sql.NullFloat64
		date sql.NullTime
		want bool
	}{
		{"float", none, none, num, sql.NullTime{}, true},
		{"float", str, none, sql.NullFloat64{}, sql.NullTime{}, false}, // text entered before the type became a number
		{"datepicker", none, none, sql.NullFloat64{}, date, true},
		{"datepicker", str, none, num, sql.NullTime{}, false},
		{"datetime", str, none, sql.NullFloat64{}, sql.NullTime{}, false},
		{"datetime", none, none, sql.NullFloat64{}, sql.NullTime{}, true}, // every column empty: an empty value, not a misfit
		{"float", none, none, sql.NullFloat64{}, sql.NullTime{}, true},
		{"textfield", none, none, sql.NullFloat64{}, sql.NullTime{}, true},
		{"textfield", str, none, sql.NullFloat64{}, sql.NullTime{}, true},
		{"textfield", none, none, sql.NullFloat64{}, date, false}, // date entered before the type became text
		{"textarea", none, text, sql.NullFloat64{}, sql.NullTime{}, true},
		{"url", none, none, num, sql.NullTime{}, false},
		{"select", str, none, sql.NullFloat64{}, sql.NullTime{}, true},
		{"gh-sprint", none, none, num, sql.NullTime{}, true}, // types not listed are never refused
	} {
		if got := fitsType(prefix+c.kind, c.str, c.text, c.num, c.date); got != c.want {
			t.Errorf("%s with str=%v text=%v num=%v date=%v: got %v, want %v", c.kind, c.str.Valid, c.text.Valid,
				c.num.Valid, c.date.Valid, got, c.want)
		}
	}
}
