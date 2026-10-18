package search

import (
	"reflect"
	"strings"
	"testing"

	"solstice/internal/sqlq"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Query
	}{
		{"", Query{Sort: defaultSort}},
		{"panier vide", Query{Words: []string{"panier", "vide"}, Sort: defaultSort}},
		{`"panier vide" paiement`, Query{Words: []string{"panier vide", "paiement"}, Sort: defaultSort}},
		{"demo-12", Query{Keys: []string{"DEMO-12"}, Sort: defaultSort}},
		{"@marie @moi", Query{Filters: []Filter{{Assignee, "marie", false}, {Assignee, "moi", false}}, Sort: defaultSort}},
		{`statut:"en cours" -#bug`, Query{Filters: []Filter{{Status, "en cours", false}, {Label, "bug", true}}, Sort: defaultSort}},
		{"par:@paul Priorité:haute", Query{Filters: []Filter{{Reporter, "paul", false}, {Priority, "haute", false}}, Sort: defaultSort}},
		{"est:ouvert tri:création", Query{Filters: []Filter{{Category, "ouvert", false}}, Sort: sorts["création"]}},
		// Unknown prefixes and URLs are text, not filters.
		{"foo:bar https://x.y", Query{Words: []string{"foo:bar", "https://x.y"}, Sort: defaultSort}},
		{"-erreur", Query{Words: []string{"-erreur"}, Sort: defaultSort}},
		{"tri:nimporte", Query{Sort: defaultSort}},
	}
	for _, c := range cases {
		got := Parse(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Parse(%q)\n got %+v\nwant %+v", c.in, got, c.want)
		}
	}
}

func TestSQLGroupsAlternativesAndNegations(t *testing.T) {
	q := Parse(`statut:Ouvert statut:"In progress" -@personne`)
	sql, args := q.SQL(7)
	want := "(i.status_id IN (SELECT id FROM statuses WHERE name = ?) OR i.status_id IN (SELECT id FROM statuses WHERE name = ?)) AND NOT COALESCE((i.assignee_id IS NULL), FALSE)"
	if sql != want {
		t.Fatalf("got  %s\nwant %s", sql, want)
	}
	if !reflect.DeepEqual(args, []any{"Ouvert", "In progress"}) {
		t.Fatalf("args %v", args)
	}
}

func TestSQLText(t *testing.T) {
	sql, args := Parse(`panier "carte bleue" -erreur ok`).SQL(1)
	if !strings.HasPrefix(sql, "MATCH(i.summary, i.description) AGAINST (? IN BOOLEAN MODE)") {
		t.Fatalf("full-text condition missing: %s", sql)
	}
	if args[0] != `+panier* +"carte bleue" -erreur*` {
		t.Fatalf("boolean query %q", args[0])
	}
	// "ok" is too short for the full-text index.
	if !strings.Contains(sql, "i.summary LIKE ?") || args[1] != "%ok%" {
		t.Fatalf("short word fallback missing: %s %v", sql, args)
	}
}

func TestSQLStripsFullTextOperators(t *testing.T) {
	_, args := Parse(`a+b*(c) "x" @`).SQL(1)
	for _, a := range args {
		s, _ := a.(string)
		if strings.ContainsAny(strings.Trim(s, "%"), "()~<>") {
			t.Fatalf("operator leaked into %q", s)
		}
	}
}

func TestSQLMe(t *testing.T) {
	sql, args := Parse("@moi par:moi").SQL(42)
	if sql != "(i.assignee_id = ?) AND (i.reporter_id = ?)" || !reflect.DeepEqual(args, []any{int64(42), int64(42)}) {
		t.Fatalf("got %s %v", sql, args)
	}
}

func TestSQLUnknownCategoryMatchesNothing(t *testing.T) {
	if sql, _ := Parse("est:bizarre").SQL(1); sql != "(FALSE)" {
		t.Fatalf("got %s", sql)
	}
}

func TestSQLKeys(t *testing.T) {
	sql, args := Parse("DEMO-12 web-3").SQL(1)
	one := "((" + sqlq.ProjectKey("p", "?") + " AND i.num = ?) OR EXISTS (SELECT 1 FROM moved_issue_keys mk WHERE mk.issue_id = i.id AND mk.old_key = ?))"
	if sql != "("+one+" OR "+one+")" || !strings.Contains(one, "project_keys") {
		t.Fatalf("got %s", sql)
	}
	if !reflect.DeepEqual(args, []any{"DEMO", "DEMO", int64(12), "DEMO-12", "WEB", "WEB", int64(3), "WEB-3"}) {
		t.Fatalf("args %v", args)
	}
}
