package tests

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"solstice/internal/jirafixture"
)

type sprintReport struct {
	Estimate       string             `json:"estimate"`
	Totals         map[string]float64 `json:"totals"`
	StartRemaining float64            `json:"startRemaining"`
	Steps          []struct {
		Issue     string  `json:"issue"`
		Remaining float64 `json:"remaining"`
	} `json:"steps"`
	Issues []struct {
		Key       string   `json:"key"`
		Committed bool     `json:"committed"`
		Outcome   string   `json:"outcome"`
		Start     *float64 `json:"startEstimate"`
		End       *float64 `json:"endEstimate"`
	} `json:"issues"`
}

// The sprint report rebuilds the sprint from the change log, and leaves out
// the issues the viewer can't browse.
func TestSprintReport(t *testing.T) {
	marie := login(t, "marie.durand", jirafixture.Password)
	board, sprint := scopeBoard(t, marie, "WEB")
	path := "/api/boards/" + board
	reportPath := fmt.Sprintf("%s/sprints/%d/report", path, sprint)

	a := createIssue(t, marie, "WEB", "Rapport engagé fini", "")
	b := createIssue(t, marie, "WEB", "Rapport engagé retiré", "")
	c := createIssue(t, marie, "WEB", "Rapport ajouté", "")
	var d struct {
		Status      statusRef
		Transitions []struct{ To statusRef }
	}
	marie.call("GET", "/api/issues/"+a, nil, 200, &d)
	if len(d.Transitions) == 0 {
		t.Fatalf("%s has no transition", a)
	}
	marie.call("PUT", path, map[string]any{"columns": []map[string]any{
		{"name": "Départ", "statuses": []string{d.Status.ID}},
		{"name": "Fini", "statuses": []string{d.Transitions[0].To.ID}},
	}}, 200, nil)
	doneCol := readBoard(t, marie, path).Columns[1].ID
	for key, est := range map[string]string{a: "2h", b: "3h", c: "1h"} {
		marie.call("PATCH", "/api/issues/"+key, map[string]any{"originalEstimate": est}, 200, nil)
	}
	move := func(body map[string]any) {
		t.Helper()
		if status, out := marie.do("POST", path+"/move", body); status != 204 {
			t.Fatalf("move %v: %d %s", body, status, out)
		}
	}

	// The report needs a started sprint.
	if status := marie.get(reportPath, nil); status != 404 {
		t.Errorf("report of a future sprint: %d, want 404", status)
	}

	move(map[string]any{"issue": a, "sprint": sprint})
	move(map[string]any{"issue": b, "sprint": sprint})
	// An RH issue in the sprint: marie can't browse it.
	hidden := pick(t, `SELECT CAST(i.id AS CHAR) FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' ORDER BY i.id LIMIT 1`)
	if _, err := testDB.Exec(`INSERT INTO sprint_issues (sprint_id, issue_id) VALUES (?, ?)`, sprint, hidden); err != nil {
		t.Fatal(err)
	}
	marie.call("POST", fmt.Sprintf("/api/sprints/%d/start", sprint), map[string]any{}, 204, nil)
	time.Sleep(10 * time.Millisecond) // later changes land after the start, at the log's millisecond precision

	move(map[string]any{"issue": c, "sprint": sprint})
	move(map[string]any{"issue": b, "sprint": 0})
	move(map[string]any{"issue": a, "column": doneCol})

	check := func(what string, r sprintReport, totals map[string]float64, start float64, steps []float64) {
		t.Helper()
		for k, v := range totals {
			if r.Totals[k] != v {
				t.Errorf("%s: %s = %v, want %v (totals %v)", what, k, r.Totals[k], v, r.Totals)
			}
		}
		var got []float64
		for _, s := range r.Steps {
			got = append(got, s.Remaining)
		}
		if r.StartRemaining != start || !slices.Equal(got, steps) {
			t.Errorf("%s: burndown %v then %v, want %v then %v", what, r.StartRemaining, got, start, steps)
		}
		outcomes := map[string]string{}
		for _, is := range r.Issues {
			outcomes[is.Key] = fmt.Sprint(is.Committed, " ", is.Outcome)
		}
		want := map[string]string{a: "true completed", b: "true removed", c: "false notCompleted"}
		if len(outcomes) != len(want) {
			t.Errorf("%s: issues %v, want %v (and no RH issue)", what, outcomes, want)
		}
		for k, v := range want {
			if outcomes[k] != v {
				t.Errorf("%s: %s is %q, want %q", what, k, outcomes[k], v)
			}
		}
	}

	// No estimate on the board: issues are counted.
	var r sprintReport
	marie.call("GET", reportPath, nil, 200, &r)
	if r.Estimate != "" {
		t.Errorf("estimate %q, want none", r.Estimate)
	}
	check("counted", r, map[string]float64{"committed": 2, "added": 1, "removed": 1, "completed": 1, "notCompleted": 1},
		2, []float64{3, 2, 1})

	// Estimated in hours of original estimate.
	marie.call("PUT", path, map[string]any{"estimate": "timeoriginalestimate"}, 200, nil)
	r = sprintReport{}
	marie.call("GET", reportPath, nil, 200, &r)
	check("estimated", r, map[string]float64{"committed": 5, "added": 1, "removed": 3, "completed": 2, "notCompleted": 1},
		5, []float64{6, 3, 1})

	var listed []struct {
		ID    int64
		State string
	}
	marie.call("GET", path+"/reports/sprints", nil, 200, &listed)
	if len(listed) != 1 || listed[0].ID != sprint || listed[0].State != "active" {
		t.Errorf("sprints with a report: %+v, want the active one", listed)
	}

	// Once closed, later changes leave the report as it was.
	marie.call("POST", fmt.Sprintf("/api/sprints/%d/complete", sprint), map[string]any{}, 200, nil)
	time.Sleep(10 * time.Millisecond)
	move(map[string]any{"issue": c, "column": doneCol})
	r = sprintReport{}
	marie.call("GET", reportPath, nil, 200, &r)
	check("closed", r, map[string]float64{"committed": 5, "added": 1, "removed": 3, "completed": 2, "notCompleted": 1},
		5, []float64{6, 3, 1})

	// Someone who doesn't see the board doesn't see its report.
	filter := pick(t, `SELECT CAST(filter_id AS CHAR) FROM boards WHERE id = ?`, board)
	marie.call("PATCH", "/api/filters/"+filter, map[string]any{"shares": []map[string]any{}}, 200, nil)
	if status := login(t, "paul.martin", jirafixture.Password).get(reportPath, nil); status != 404 {
		t.Errorf("paul.martin, without the board: %d, want 404", status)
	}
}
