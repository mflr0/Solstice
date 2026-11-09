package tests

// A board's cards and backlog narrowed by the filter row's JQL: on top of
// the quick filters, never past what the user may see, and a bad query is
// refused like on the search.

import (
	"fmt"
	"net/url"
	"slices"
	"testing"

	"solstice/internal/jirafixture"
)

func TestBoardJQL(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", jirafixture.Password) // developpeurs, sees the project
	key := newProject(t, admin, nil)
	fruit, red := unique("pomme"), unique("rouge")

	redFruit := createIssue(t, admin, key, fruit+" "+red, "")
	greenFruit := createIssue(t, admin, key, fruit+" verte", "")
	createIssue(t, admin, key, "Poire "+red, "")
	hidden := createIssue(t, admin, key, fruit+" secrète", "")
	var level securityLevelAdmin
	admin.call("POST", "/api/projects/"+key+"/security-levels", map[string]any{"name": "Personne"}, 201, &level)
	if _, err := testDB.Exec(`UPDATE issues SET security_level_id = ? WHERE id = ?`, level.ID, issueID(t, hidden)); err != nil {
		t.Fatal(err)
	}

	for _, kind := range []string{"kanban", "scrum"} {
		var created struct{ ID int64 }
		admin.call("POST", "/api/boards", map[string]any{"kind": kind, "project": key}, 201, &created)
		path := fmt.Sprintf("/api/boards/%d", created.ID)
		t.Cleanup(func() { admin.do("DELETE", path, nil) })
		admin.call("PUT", path, map[string]any{"quickFilters": []map[string]any{
			{"name": "Rouges", "query": `summary ~ "` + red + `"`},
		}}, 200, nil)
		quick := readBoard(t, admin, path).QuickFilters[0].ID

		list := path + "/cards"
		if kind == "scrum" {
			list = path + "/backlog"
		}
		keys := func(c *client, params url.Values) []string {
			t.Helper()
			var out struct{ Cards, Backlog []struct{ Key string } }
			c.call("GET", list+"?"+params.Encode(), nil, 200, &out)
			ks := []string{}
			for _, c := range append(out.Cards, out.Backlog...) {
				ks = append(ks, c.Key)
			}
			slices.Sort(ks)
			return ks
		}
		byFruit := `summary ~ "` + fruit + `" ORDER BY created DESC`
		want := []string{greenFruit, redFruit}
		slices.Sort(want)
		if got := keys(paul, url.Values{"jql": {byFruit}}); !slices.Equal(got, want) {
			t.Errorf("%s jql: %v, want %v", kind, got, want)
		}
		if got := keys(paul, url.Values{"jql": {byFruit}, "quick": {fmt.Sprint(quick)}}); !slices.Equal(got, []string{redFruit}) {
			t.Errorf("%s jql and quick filter: %v, want %v", kind, got, []string{redFruit})
		}
		if got := keys(paul, url.Values{"jql": {`key = ` + hidden}}); len(got) != 0 {
			t.Errorf("%s jql naming a hidden issue: %v, want none", kind, got)
		}
		if got := keys(paul, url.Values{"jql": {"  "}}); len(got) != 3 {
			t.Errorf("%s blank jql: %v, want the 3 issues paul sees", kind, got)
		}
		if status, msg, _ := paul.errorIn("en", "GET", list+"?"+url.Values{"jql": {"summary ~"}}.Encode(), nil); status != 400 || msg == "" {
			t.Errorf("%s bad jql: %d %q, want 400 with a message", kind, status, msg)
		}
	}
}
