package jql

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// fuzzEnv is a compile environment that needs no database.
func fuzzEnv() Env {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		paris = time.UTC
	}
	return Env{
		Me:  7,
		Now: time.Date(2026, 10, 14, 10, 30, 0, 0, paris),
		Loc: paris,
		CustomField: func(name string) (Field, bool) {
			if strings.EqualFold(name, "Story Points") || name == "10100" {
				return Field{ID: 10100, Name: "Story Points", TypeKey: "x:float"}, true
			}
			return Field{}, false
		},
	}
}

// FuzzParse checks that any text either parses, or fails with an *Error whose
// position lies inside the query; and that a parsed query compiles without
// panicking.
func FuzzParse(f *testing.F) {
	for _, s := range []string{
		`project = DEMO`,
		`project = DEMO AND status in (Open, "In Progress") ORDER BY created DESC`,
		`assignee = currentUser() OR reporter is EMPTY`,
		`status WAS Done DURING ("2026-01-01", "2026-02-01")`,
		`status CHANGED FROM Open TO Done AFTER -2w BY bob`,
		`summary ~ "panne réseau" AND labels not in (a, b)`,
		`"Story Points" >= 3 AND created >= startOfMonth(-1)`,
		`NOT (priority = High OR priority = Highest)`,
		`fixVersion = earliestUnreleasedVersion(DEMO)`,
		`sprint in openSprints() AND issuetype = Bug`,
		`project = "DEMO`,
		`(((`,
		`ORDER BY`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		q, err := Parse(s)
		if err != nil {
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("Parse(%q) returned %T, want *Error: %v", s, err, err)
			}
			if n := utf8.RuneCountInString(s); e.Pos < 0 || e.Pos > n {
				t.Fatalf("Parse(%q): error position %d outside the query (%d characters)", s, e.Pos, n)
			}
			if e.Key == "" {
				t.Fatalf("Parse(%q): error without a message", s)
			}
			return
		}
		if q == nil {
			t.Fatalf("Parse(%q) returned neither a query nor an error", s)
		}
		_, _, _, _ = Compile(q, fuzzEnv())
	})
}

// FuzzCompileKeepsValuesOut checks that a value typed in a query never
// reaches the SQL text: values must travel as arguments.
func FuzzCompileKeepsValuesOut(f *testing.F) {
	for _, s := range []string{"x", "'; DROP TABLE issues; --", "a b", "été", ")", "1=1", "%", "_"} {
		f.Add(s)
	}
	templates := []string{
		`summary ~ "VALUE"`,
		`description ~ "VALUE"`,
		`text ~ "VALUE"`,
		`comment ~ "VALUE"`,
		`labels = "VALUE"`,
		`labels in ("VALUE", "other")`,
		`project = "VALUE"`,
		`status = "VALUE"`,
		`status was "VALUE"`,
		`status changed to "VALUE"`,
		`assignee = "VALUE"`,
		`reporter in ("VALUE")`,
		`component = "VALUE"`,
		`fixVersion = "VALUE"`,
		`priority = "VALUE"`,
		`resolution = "VALUE"`,
		`issuetype = "VALUE"`,
		`sprint = "VALUE"`,
		`"Story Points" = "VALUE"`,
		`issuekey = "VALUE"`,
	}
	f.Fuzz(func(t *testing.T, s string) {
		// The marker can only appear in the SQL if the compiler copied the value.
		value := "ZQXMARK" + strings.NewReplacer(`"`, "", `\`, "").Replace(s)
		for _, tpl := range templates {
			query := strings.Replace(tpl, "VALUE", value, 1)
			q, err := Parse(query)
			if err != nil {
				continue
			}
			where, _, order, err := Compile(q, fuzzEnv())
			if err != nil {
				continue
			}
			if strings.Contains(strings.ToUpper(where+order), "ZQXMARK") {
				t.Fatalf("Compile(%q) copied the value into the SQL:\n%s %s", query, where, order)
			}
		}
	})
}
