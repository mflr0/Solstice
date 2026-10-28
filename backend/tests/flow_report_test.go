package tests

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"solstice/internal/jirafixture"
)

type flowReport struct {
	Columns []struct{ Name string } `json:"columns"`
	Days    []struct {
		Date   string `json:"date"`
		Counts []int  `json:"counts"`
	} `json:"days"`
}

// The cumulative flow counts, per day, the issues the board's filter and
// sub-filter match today in each column, and only those the viewer sees.
func TestFlowReport(t *testing.T) {
	marie := login(t, "marie.durand", jirafixture.Password)
	paul := login(t, "paul.martin", jirafixture.Password)
	var created struct{ ID int64 }
	marie.call("POST", "/api/boards", map[string]any{"name": unique("Flux "), "kind": "kanban", "project": "DEMO"},
		201, &created)
	path := fmt.Sprintf("/api/boards/%d", created.ID)
	filter := pick(t, `SELECT CAST(filter_id AS CHAR) FROM boards WHERE id = ?`, created.ID)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM boards WHERE id = ?`, created.ID)
		testDB.Exec(`DELETE FROM filters WHERE id = ?`, filter)
	})

	token := unique("flux")
	a := createIssue(t, marie, "DEMO", "Avance "+token, "")
	createIssue(t, marie, "DEMO", "Reste "+token, "")
	hidden := createIssue(t, marie, "DEMO", "Restreint "+token, "")
	createIssue(t, marie, "DEMO", "Hors du sous-filtre", "")
	level := pick(t, `SELECT CAST(id AS CHAR) FROM security_levels WHERE name = 'Restreint'`)
	if _, err := testDB.Exec(`UPDATE issues SET security_level_id = ? WHERE id = ?`, level, issueID(t, hidden)); err != nil {
		t.Fatal(err)
	}
	var d struct {
		Status      statusRef
		Transitions []struct{ To statusRef }
	}
	marie.call("GET", "/api/issues/"+a, nil, 200, &d)
	if len(d.Transitions) == 0 {
		t.Fatalf("%s has no transition", a)
	}
	marie.call("PUT", path, map[string]any{"subQuery": `summary ~ "` + token + `"`, "columns": []map[string]any{
		{"name": "Départ", "statuses": []string{d.Status.ID}},
		{"name": "Arrivée", "statuses": []string{d.Transitions[0].To.ID}},
	}}, 200, nil)
	doneCol := readBoard(t, marie, path).Columns[1].ID
	if status, out := marie.do("POST", path+"/move", map[string]any{"issue": a, "column": doneCol}); status != 204 {
		t.Fatalf("move %s: %d %s", a, status, out)
	}
	marie.call("PATCH", "/api/filters/"+filter, map[string]any{"shares": []map[string]any{{"kind": "everyone"}}}, 200, nil)

	read := func(c *client, query string) flowReport {
		t.Helper()
		var r flowReport
		c.call("GET", path+"/reports/flow"+query, nil, 200, &r)
		return r
	}
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	r := read(marie, "?days=7&tz=Europe/Paris")
	if names := []string{r.Columns[0].Name, r.Columns[1].Name}; len(r.Columns) != 2 || names[0] != "Départ" || names[1] != "Arrivée" {
		t.Errorf("columns %+v", r.Columns)
	}
	if len(r.Days) != 7 {
		t.Fatalf("%d days, want 7", len(r.Days))
	}
	if today := time.Now().In(paris).Format(time.DateOnly); r.Days[6].Date != today {
		t.Errorf("last day %s, want today %s", r.Days[6].Date, today)
	}
	// The issues were created today: nothing before.
	if got := r.Days[5].Counts; !slices.Equal(got, []int{0, 0}) {
		t.Errorf("yesterday: %v, want [0 0]", got)
	}
	if got := r.Days[6].Counts; !slices.Equal(got, []int{2, 1}) {
		t.Errorf("marie today: %v, want [2 1]", got)
	}
	// paul.martin doesn't see the restricted issue.
	if got := read(paul, "").Days; len(got) != 30 || !slices.Equal(got[29].Counts, []int{1, 1}) {
		t.Errorf("paul.martin, default period: %d days, today %v, want 30 days, [1 1]", len(got), got[len(got)-1].Counts)
	}
	if status := marie.get(path+"/reports/flow?tz=Mars/Olympus", nil); status != 400 {
		t.Errorf("unknown time zone: %d, want 400", status)
	}

	// Someone who doesn't see the board doesn't see its report.
	marie.call("PATCH", "/api/filters/"+filter, map[string]any{"shares": []any{}}, 200, nil)
	if status := paul.get(path+"/reports/flow", nil); status != 404 {
		t.Errorf("paul.martin, without the board: %d, want 404", status)
	}
}
