package jql

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	q, err := Parse(`project = DEMO AND (status in ("In Progress", Done) OR assignee = currentUser()) AND NOT labels is EMPTY ORDER BY priority DESC, key`)
	if err != nil {
		t.Fatal(err)
	}
	want := And{
		And{
			Clause{"project", "=", []Value{{Text: "DEMO"}}},
			Or{
				Clause{"status", "in", []Value{{Text: "In Progress"}, {Text: "Done"}}},
				Clause{"assignee", "=", []Value{{Func: "currentuser"}}},
			},
		},
		Not{Clause{"labels", "is", []Value{{Empty: true}}}},
	}
	if !reflect.DeepEqual(q.Where, want) {
		t.Errorf("where\n got %#v\nwant %#v", q.Where, want)
	}
	if !reflect.DeepEqual(q.Order, []Sort{{"priority", true}, {"key", false}}) {
		t.Errorf("order %v", q.Order)
	}
}

func TestParseOddities(t *testing.T) {
	for _, s := range []string{
		`"Epic Link" = DEMO-3`,
		`cf[10100] > 3`,
		`assignee in membersOf("developpeurs")`,
		`created >= -7d and updated < startOfDay("-1d")`,
		`ORDER BY created DESC`,
		`summary ~ "panier vide" or text ~ 'paiement'`,
		`resolution = Unresolved`,
		`priority not in (Low, Lowest)`,
		`assignee is not EMPTY`,
		`status != Done`,
	} {
		if _, err := Parse(s); err != nil {
			t.Errorf("Parse(%q): %v", s, err)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for s, want := range map[string]string{
		`project = "DEMO`:          "Guillemet",
		`project DEMO`:             "Opérateur attendu",
		`status in (Done`:          "Parenthèse",
		`(project = DEMO`:          "Parenthèse",
		`status was Done`:          "historique",
		`assignee is bob`:          "EMPTY",
		`project = DEMO ORDER key`: "BY",
		`project = DEMO extra`:     "inattendu",
		`status ! Done`:            "incomplet",
	} {
		_, err := Parse(s)
		var e *Error
		if !errors.As(err, &e) || !strings.Contains(e.Msg, want) {
			t.Errorf("Parse(%q) = %v, want an error mentioning %q", s, err, want)
		}
	}
}

func compile(t *testing.T, s string) (string, []any, string) {
	t.Helper()
	q, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	paris, _ := time.LoadLocation("Europe/Paris")
	where, args, order, err := Compile(q, Env{
		Me:  7,
		Now: time.Date(2026, 10, 14, 10, 30, 0, 0, paris), // a Wednesday
		Loc: paris,
		CustomField: func(name string) (Field, bool) {
			if strings.EqualFold(name, "Story Points") || name == "10100" {
				return Field{ID: 10100, Name: "Story Points", TypeKey: "x:float"}, true
			}
			return Field{}, false
		},
	})
	if err != nil {
		t.Fatalf("Compile(%q): %v", s, err)
	}
	return where, args, order
}

func TestCompileDates(t *testing.T) {
	_, args, _ := compile(t, `created >= -7d AND updated < startOfWeek() AND due <= endOfMonth()`)
	want := []any{
		time.Date(2026, 10, 7, 8, 30, 0, 0, time.UTC),  // 7 days before 10:30 Paris (UTC+2)
		time.Date(2026, 10, 11, 22, 0, 0, 0, time.UTC), // Monday 12 October, midnight in Paris
		"2026-10-31", // due dates are days
	}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("args\n got %v\nwant %v", args, want)
	}
}

func TestCompileUsersAndEmpty(t *testing.T) {
	where, args, _ := compile(t, `assignee = currentUser() OR assignee in (EMPTY, "marie.durand")`)
	if !strings.Contains(where, "i.assignee_id IS NULL") || args[0] != int64(7) || args[1] != "marie.durand" {
		t.Errorf("where %s args %v", where, args)
	}
	where, _, _ = compile(t, `assignee != marie`)
	if !strings.HasPrefix(where, "NOT (") {
		t.Errorf("!= must exclude empty values like Jira: %s", where)
	}
}

func TestCompileResolutionUnresolved(t *testing.T) {
	if where, _, _ := compile(t, `resolution = Unresolved`); where != "i.resolution_id IS NULL" {
		t.Errorf("got %s", where)
	}
	if where, _, _ := compile(t, `resolution != Unresolved`); where != "NOT i.resolution_id IS NULL" {
		t.Errorf("got %s", where)
	}
}

func TestCompileCustomFields(t *testing.T) {
	where, args, _ := compile(t, `"Story Points" > 3 AND cf[10100] < 8`)
	if !strings.Contains(where, "v.field_id = 10100 AND v.number_value > ?") || !reflect.DeepEqual(args, []any{3.0, 8.0}) {
		t.Errorf("where %s args %v", where, args)
	}
	q, _ := Parse(`"Inconnu" = x`)
	if _, _, _, err := Compile(q, Env{}); err == nil || !strings.Contains(err.Error(), "Champ inconnu") {
		t.Errorf("unknown field: %v", err)
	}
}

func TestCompileOrder(t *testing.T) {
	_, _, order := compile(t, `ORDER BY priority DESC, key`)
	if !strings.Contains(order, "FROM priorities WHERE id = i.priority_id) ASC") || !strings.HasSuffix(order, "p.pkey ASC, i.num ASC") {
		t.Errorf("order %s", order)
	}
	_, _, order = compile(t, `project = DEMO`)
	if order != "i.updated_at DESC" {
		t.Errorf("default order %s", order)
	}
}

func TestCompileNeverInlinesUserText(t *testing.T) {
	where, _, _ := compile(t, `summary ~ "'; DROP TABLE issues; --" AND labels = "x' OR 1=1"`)
	if strings.Contains(where, "DROP") || strings.Contains(where, "1=1") {
		t.Fatalf("user text reached the SQL: %s", where)
	}
}

func TestLooksLike(t *testing.T) {
	for s, want := range map[string]bool{
		`project = DEMO`:                  true,
		`assignee in (a, b)`:              true,
		`status != Done order by created`: true,
		`ORDER BY created`:                true,
		`"Epic Link" = DEMO-1`:            true,
		`panier vide @marie`:              false,
		`statut:"en cours"`:               false,
		`DEMO-12`:                         false,
		`est:ouvert tri:priorité`:         false,
	} {
		if got := LooksLike(s); got != want {
			t.Errorf("LooksLike(%q) = %v", s, got)
		}
	}
}
