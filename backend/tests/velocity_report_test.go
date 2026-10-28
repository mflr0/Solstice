package tests

import (
	"fmt"
	"testing"
	"time"

	"solstice/internal/jirafixture"
)

type velocityReport struct {
	Estimate string `json:"estimate"`
	Sprints  []struct {
		Sprint struct {
			ID int64 `json:"id"`
		} `json:"sprint"`
		Committed float64 `json:"committed"`
		Completed float64 `json:"completed"`
	} `json:"sprints"`
}

// The velocity report gives the committed and completed work of the board's
// last closed sprints, oldest first, without the issues the viewer can't
// browse.
func TestVelocityReport(t *testing.T) {
	marie := login(t, "marie.durand", jirafixture.Password)
	board, first := scopeBoard(t, marie, "WEB")
	second := scopeSprint(t, marie, board)
	path := "/api/boards/" + board
	velocityPath := path + "/reports/velocity"

	carried := createIssue(t, marie, "WEB", "Vélocité reportée", "")
	removed := createIssue(t, marie, "WEB", "Vélocité retirée", "")
	finished := createIssue(t, marie, "WEB", "Vélocité finie", "")
	var d struct {
		Status      statusRef
		Transitions []struct{ To statusRef }
	}
	marie.call("GET", "/api/issues/"+carried, nil, 200, &d)
	// Completing a sprint carries the issues whose status isn't in the done
	// category: the last column must hold one that is.
	done := ""
	for _, tr := range d.Transitions {
		if tr.To.Category == "done" {
			done = tr.To.ID
		}
	}
	if done == "" {
		t.Fatalf("%s has no transition to a done status: %+v", carried, d.Transitions)
	}
	marie.call("PUT", path, map[string]any{"columns": []map[string]any{
		{"name": "Départ", "statuses": []string{d.Status.ID}},
		{"name": "Fini", "statuses": []string{done}},
	}}, 200, nil)
	doneCol := readBoard(t, marie, path).Columns[1].ID
	for key, est := range map[string]string{carried: "5h", removed: "3h", finished: "2h"} {
		marie.call("PATCH", "/api/issues/"+key, map[string]any{"originalEstimate": est}, 200, nil)
	}
	move := func(body map[string]any) {
		t.Helper()
		if status, out := marie.do("POST", path+"/move", body); status != 204 {
			t.Fatalf("move %v: %d %s", body, status, out)
		}
	}
	velocity := func(query string) velocityReport {
		t.Helper()
		var v velocityReport
		marie.call("GET", velocityPath+query, nil, 200, &v)
		return v
	}
	check := func(what string, v velocityReport, want ...[3]float64) {
		t.Helper()
		if len(v.Sprints) != len(want) {
			t.Fatalf("%s: sprints %+v, want %v", what, v.Sprints, want)
		}
		for i, w := range want {
			if s := v.Sprints[i]; s.Sprint.ID != int64(w[0]) || s.Committed != w[1] || s.Completed != w[2] {
				t.Errorf("%s: sprint %d = %+v, want id %v, committed %v, completed %v", what, i, s, w[0], w[1], w[2])
			}
		}
	}

	if v := velocity(""); len(v.Sprints) != 0 {
		t.Errorf("no closed sprint: %+v, want none", v.Sprints)
	}

	for _, key := range []string{carried, removed, finished} {
		move(map[string]any{"issue": key, "sprint": first})
	}
	// An RH issue in the sprint: marie can't browse it.
	hidden := pick(t, `SELECT CAST(i.id AS CHAR) FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' ORDER BY i.id LIMIT 1`)
	if _, err := testDB.Exec(`INSERT INTO sprint_issues (sprint_id, issue_id) VALUES (?, ?)`, first, hidden); err != nil {
		t.Fatal(err)
	}
	marie.call("POST", fmt.Sprintf("/api/sprints/%d/start", first), map[string]any{}, 204, nil)
	time.Sleep(10 * time.Millisecond) // later changes land after the start, at the log's millisecond precision
	move(map[string]any{"issue": removed, "sprint": 0})
	move(map[string]any{"issue": finished, "column": doneCol})
	time.Sleep(10 * time.Millisecond)
	marie.call("POST", fmt.Sprintf("/api/sprints/%d/complete", first), map[string]any{"moveTo": second}, 200, nil)
	time.Sleep(10 * time.Millisecond)

	// The active sprint isn't in it: its numbers aren't final.
	marie.call("POST", fmt.Sprintf("/api/sprints/%d/start", second), map[string]any{}, 204, nil)
	time.Sleep(10 * time.Millisecond)
	check("one closed", velocity(""), [3]float64{float64(first), 3, 1})

	move(map[string]any{"issue": carried, "column": doneCol})
	time.Sleep(10 * time.Millisecond)
	marie.call("POST", fmt.Sprintf("/api/sprints/%d/complete", second), map[string]any{}, 200, nil)

	// No estimate on the board: issues are counted.
	v := velocity("")
	if v.Estimate != "" {
		t.Errorf("estimate %q, want none", v.Estimate)
	}
	check("counted", v, [3]float64{float64(first), 3, 1}, [3]float64{float64(second), 1, 1})

	marie.call("PUT", path, map[string]any{"estimate": "timeoriginalestimate"}, 200, nil)
	v = velocity("")
	if v.Estimate != "time" {
		t.Errorf("estimate %q, want time", v.Estimate)
	}
	check("estimated", v, [3]float64{float64(first), 10, 2}, [3]float64{float64(second), 5, 5})
	check("last one", velocity("?count=1"), [3]float64{float64(second), 5, 5})

	for _, bad := range []string{"0", "21", "x"} {
		if status := marie.get(velocityPath+"?count="+bad, nil); status != 400 {
			t.Errorf("count=%s: %d, want 400", bad, status)
		}
	}

	// Someone who doesn't see the board doesn't see its velocity.
	filter := pick(t, `SELECT CAST(filter_id AS CHAR) FROM boards WHERE id = ?`, board)
	marie.call("PATCH", "/api/filters/"+filter, map[string]any{"shares": []map[string]any{}}, 200, nil)
	if status := login(t, "paul.martin", jirafixture.Password).get(velocityPath, nil); status != 404 {
		t.Errorf("paul.martin, without the board: %d, want 404", status)
	}
}
