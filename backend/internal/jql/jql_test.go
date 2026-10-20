package jql

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"solstice/internal/i18n"
)

func TestParse(t *testing.T) {
	q, err := Parse(`project = DEMO AND (status in ("In Progress", Done) OR assignee = currentUser()) AND NOT labels is EMPTY ORDER BY priority DESC, key`)
	if err != nil {
		t.Fatal(err)
	}
	want := And{
		And{
			Clause{Field: "project", Op: "=", Values: []Value{{Text: "DEMO"}}},
			Or{
				Clause{Field: "status", Op: "in", Values: []Value{{Text: "In Progress"}, {Text: "Done"}}},
				Clause{Field: "assignee", Op: "=", Values: []Value{{Func: "currentuser"}}},
			},
		},
		Not{Clause{Field: "labels", Op: "is", Values: []Value{{Empty: true}}}},
	}
	if got := noPos(q.Where); !reflect.DeepEqual(got, want) {
		t.Errorf("where\n got %#v\nwant %#v", got, want)
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

func TestParseHistory(t *testing.T) {
	q, err := Parse(`status was not in (Open, "In Progress") by (bob, currentUser()) during ("2024-01-01", now()) AND assignee changed from EMPTY to bob after -2w`)
	if err != nil {
		t.Fatal(err)
	}
	want := And{
		Clause{Field: "status", Op: "was not in", Values: []Value{{Text: "Open"}, {Text: "In Progress"}}, Preds: []Predicate{
			{"by", []Value{{Text: "bob"}, {Func: "currentuser"}}},
			{"during", []Value{{Text: "2024-01-01"}, {Func: "now"}}},
		}},
		Clause{Field: "assignee", Op: "changed", Preds: []Predicate{
			{"from", []Value{{Empty: true}}},
			{"to", []Value{{Text: "bob"}}},
			{"after", []Value{{Text: "-2w"}}},
		}},
	}
	if got := noPos(q.Where); !reflect.DeepEqual(got, want) {
		t.Errorf("where\n got %#v\nwant %#v", got, want)
	}
	if !LooksLike(`status was "In Progress"`) || !LooksLike(`assignee changed`) {
		t.Error("history clauses not recognised as JQL")
	}
}

func TestParseErrors(t *testing.T) {
	for s, want := range map[string]string{
		`project = "DEMO`:             "quotation mark",
		`project DEMO`:                "Operator expected",
		`status in (Done`:             "parenthesis",
		`(project = DEMO`:             "parenthesis",
		`status was Done from Open`:   "CHANGED",
		`status changed during (-1d)`: "DURING",
		`assignee is bob`:             "EMPTY",
		`project = DEMO ORDER key`:    "BY",
		`project = DEMO extra`:        "Unexpected",
		`status ! Done`:               "Incomplete",
	} {
		_, err := Parse(s)
		var e *Error
		if !errors.As(err, &e) || !strings.Contains(e.Error(), want) {
			t.Errorf("Parse(%q) = %v, want an error mentioning %q", s, err, want)
		}
	}
}

func compile(t *testing.T, s string) (string, []any, string) {
	t.Helper()
	return compileIn(t, s, fuzzEnv())
}

func compileIn(t *testing.T, s string, env Env) (string, []any, string) {
	t.Helper()
	q, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	where, args, order, err := Compile(q, env)
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
	if _, _, _, err := Compile(q, Env{}); err == nil || !strings.Contains(err.Error(), "Unknown field") {
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
		`project = DEMO`:                                        true,
		`assignee in (a, b)`:                                    true,
		`status != Done order by created`:                       true,
		`ORDER BY created`:                                      true,
		`"Epic Link" = DEMO-1`:                                  true,
		`assignee = currentUser()`:                              true,
		`labels = backend`:                                      true,
		`participant = paul.martin`:                             true,
		`key in (DEMO-1)`:                                       true,
		`key >= DEMO-1 AND project = WEB`:                       true,
		`project in (OLD, DEMO) AND key = DEMO-1`:               true,
		`fixVersion in releasedVersions(OLD) AND project = NEW`: true,
		`panier vide @marie`:                                    false,
		`projet:DEMO @personne`:                                 false,
		`@moi`:                                                  false,
		`#backend`:                                              false,
		`statut:"en cours"`:                                     false,
		`DEMO-12`:                                               false,
		`est:ouvert tri:priorité`:                               false,
	} {
		if got := LooksLike(s); got != want {
			t.Errorf("LooksLike(%q) = %v", s, got)
		}
	}
}

func TestCompileEqualsEmpty(t *testing.T) {
	// Found in a real Jira database: a user field compared with EMPTY.
	where, args, _ := compile(t, `(assignee != currentUser() OR assignee = EMPTY) AND reporter = currentUser()`)
	if !strings.Contains(where, "i.assignee_id IS NULL") || len(args) != 2 {
		t.Errorf("where %s args %v", where, args)
	}
	for _, q := range []string{`assignee = EMPTY`, `assignee in (EMPTY)`, `priority = EMPTY`, `fixVersion = EMPTY`} {
		where, _, _ := compile(t, q)
		if strings.TrimSpace(where) == "" || strings.Contains(where, "()") {
			t.Errorf("%s compiled to %q", q, where)
		}
	}
}

func TestCompileEmptyOperands(t *testing.T) {
	for _, q := range []string{`timespent > EMPTY`, `originalEstimate <= null`, `remainingEstimate < EMPTY`, `timeestimate >= EMPTY`} {
		parsed, err := Parse(q)
		if err != nil {
			t.Fatal(err)
		}
		var je *Error
		if _, _, _, err := Compile(parsed, fuzzEnv()); !errors.As(err, &je) || je.Key != i18n.JQLDurationInvalid {
			t.Errorf("Compile(%q) = %v, want JQLDurationInvalid", q, err)
		}
	}

	env := fuzzEnv()
	fields := map[string]Field{
		"Points":   {ID: 1, Name: "Points", TypeKey: "x:float"},
		"Team":     {ID: 2, Name: "Team", TypeKey: "x:select"},
		"Reviewer": {ID: 3, Name: "Reviewer", TypeKey: "x:userpicker"},
		"Epic":     {ID: 4, Name: "Epic", TypeKey: "x:gh-epic-link"},
	}
	env.CustomField = func(name string) (Field, bool) {
		f, ok := fields[name]
		return f, ok
	}
	for q, want := range map[string]string{
		`statusCategory = EMPTY`:       "FALSE",
		`statusCategory in (EMPTY)`:    "FALSE",
		`statusCategory != EMPTY`:      "NOT FALSE",
		`statusCategory not in (null)`: "NOT FALSE",
		`Points = EMPTY`:               `Points is EMPTY`,
		`Team in (EMPTY)`:              `Team is EMPTY`,
		`Reviewer = null`:              `Reviewer is EMPTY`,
		`Epic = EMPTY`:                 `Epic is EMPTY`,
		`Points != EMPTY`:              `Points is not EMPTY`,
		`Team not in (EMPTY)`:          `Team is not EMPTY`,
		`Reviewer != EMPTY`:            `Reviewer is not EMPTY`,
		`Epic != null`:                 `Epic is not EMPTY`,
	} {
		where, args, _ := compileIn(t, q, env)
		if strings.Contains(want, " is ") {
			want, _, _ = compileIn(t, want, env)
		}
		if where != want || len(args) != 0 {
			t.Errorf("%s compiled to %q %v, want %q", q, where, args, want)
		}
	}
}

func TestCompileKeyRangeNeedsKey(t *testing.T) {
	for _, q := range []string{`key > EMPTY`, `issuekey <= null`, `key < 12`, `id >= "DEMO"`} {
		parsed, err := Parse(q)
		if err != nil {
			t.Fatal(err)
		}
		var je *Error
		if _, _, _, err := Compile(parsed, fuzzEnv()); !errors.As(err, &je) || je.Key != i18n.JQLIssueKeyInvalid {
			t.Errorf("Compile(%q) = %v, want JQLIssueKeyInvalid", q, err)
		}
	}
	where, args, _ := compile(t, `key > demo-5`)
	if !strings.Contains(where, "i.num > ?") || len(args) != 3 || args[2] != int64(5) {
		t.Errorf("where %s args %v", where, args)
	}
}

func TestProjects(t *testing.T) {
	for s, want := range map[string][]string{
		`project = DEMO ORDER BY Rank ASC`:                 {"DEMO"},
		`project in (WEB, "Site web") AND status != Done`:  {"WEB", "Site web"},
		`assignee = currentUser()`:                         nil,
		`project = DEMO OR project = WEB`:                  nil,
		`status = Done AND (project = OPS AND type = Bug)`: {"OPS"},
	} {
		q, err := Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		if got := Projects(q); !reflect.DeepEqual(got, want) {
			t.Errorf("Projects(%q) = %v, want %v", s, got, want)
		}
	}
}

// History clauses assemble SQL from many pieces: every placeholder must get
// its argument, in order, and no user text may reach the SQL.
func TestCompileHistoryArguments(t *testing.T) {
	for _, q := range []string{
		`status was Done`,
		`status was not in (Open, "In Progress") by (bob, currentUser()) during ("2024-01-01", now())`,
		`assignee was "x'; DROP TABLE issues; --" before -2w`,
		`assignee changed from EMPTY to membersOf("devs") after startOfMonth() by bob`,
		`priority was High on "2025-03-04" OR resolution was Unresolved after -1d`,
		`fixVersion was "1.0" during (-30d, -1d) AND fixVersion changed to "2.0"`,
		`reporter was EMPTY OR status changed`,
	} {
		for i, env := range envs() {
			where, args, _ := compileIn(t, q, env)
			if n := strings.Count(where, "?"); n != len(args) {
				t.Errorf("%s (env %d): %d placeholders, %d arguments", q, i, n, len(args))
			}
			if strings.Contains(where, "DROP") {
				t.Errorf("user text reached the SQL: %s", where)
			}
		}
	}
}

// noPos clears clause positions, to compare trees by structure.
func noPos(n Node) Node {
	switch n := n.(type) {
	case And:
		return And{noPos(n.Left), noPos(n.Right)}
	case Or:
		return Or{noPos(n.Left), noPos(n.Right)}
	case Not:
		return Not{noPos(n.Inner)}
	case Clause:
		n.Pos = 0
		return n
	}
	return n
}

func TestCompileParticipant(t *testing.T) {
	where, args, _ := compile(t, `participant = currentUser() AND commenter in (bob, "x'; DROP")`)
	if n := strings.Count(where, "?"); n != len(args) {
		t.Errorf("%d placeholders, %d arguments", n, len(args))
	}
	if strings.Contains(where, "DROP") {
		t.Errorf("user text reached the SQL: %s", where)
	}
	if !strings.Contains(where, "visibility_group IS NULL") {
		t.Errorf("restricted comments and worklogs must not count: %s", where)
	}
	if !LooksLike(`participant = bob`) || !LooksLike(`commenter in (bob)`) {
		t.Error("participant/commenter not recognised as JQL")
	}
}

func TestErrorPositions(t *testing.T) {
	_, err := Parse(`project = DEMO AND status in (Done`)
	var je *Error
	if !errors.As(err, &je) || je.Pos != 34 {
		t.Errorf("syntax error at %+v, want position 34", err)
	}
	q, err := Parse(`project = DEMO AND couleur = bleu`)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, err = Compile(q, Env{CustomField: func(string) (Field, bool) { return Field{}, false }})
	if !errors.As(err, &je) || je.Pos != 19 {
		t.Errorf("unknown field error at %+v, want position 19", err)
	}
}

// TestParseHostileNesting checks that a query nested to millions of levels,
// or longer than MaxQueryLen, is refused instead of overflowing the stack.
func TestParseHostileNesting(t *testing.T) {
	deep := strings.Repeat("(", maxDepth+1) + "x = 1" + strings.Repeat(")", maxDepth+1)
	for name, q := range map[string]string{
		"parentheses": deep,
		"not":         strings.Repeat("NOT ", maxDepth+1) + "x = 1",
		"huge":        "project = A AND " + strings.Repeat("(", 4_000_000) + "x = 1",
	} {
		_, err := Parse(q)
		var je *Error
		if !errors.As(err, &je) {
			t.Errorf("%s: want a *Error, got %v", name, err)
		}
	}
	if _, err := Parse(strings.Repeat("(", maxDepth-1) + "x = 1" + strings.Repeat(")", maxDepth-1)); err != nil {
		t.Errorf("nesting within the limit refused: %v", err)
	}
}
