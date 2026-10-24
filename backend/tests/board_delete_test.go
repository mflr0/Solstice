package tests

import (
	"fmt"
	"testing"

	"solstice/internal/jirafixture"
)

// Deleting a board deletes its future sprints, completes its active one and
// keeps its closed ones as history.
func TestBoardDeleteSettlesItsSprints(t *testing.T) {
	marie := login(t, "marie.durand", jirafixture.Password)
	board, future := scopeBoard(t, marie, "WEB")
	active, closed := scopeSprint(t, marie, board), scopeSprint(t, marie, board)
	t.Cleanup(func() { testDB.Exec(`DELETE FROM sprints WHERE id IN (?, ?)`, active, closed) })
	plan := func(key string, sprint int64) {
		t.Helper()
		if status, body := marie.do("POST", "/api/boards/"+board+"/move", map[string]any{"issue": key, "sprint": sprint}); status != 204 {
			t.Fatalf("plan %s: %d %s", key, status, body)
		}
	}
	inClosed := createIssue(t, marie, "WEB", "Dans un sprint terminé", "")
	plan(inClosed, closed)
	marie.call("POST", fmt.Sprintf("/api/sprints/%d/start", closed), map[string]any{}, 204, nil)
	marie.call("POST", fmt.Sprintf("/api/sprints/%d/complete", closed), map[string]any{}, 200, nil)
	inActive := createIssue(t, marie, "WEB", "Dans le sprint en cours", "")
	plan(inActive, active)
	marie.call("POST", fmt.Sprintf("/api/sprints/%d/start", active), map[string]any{}, 204, nil)
	inFuture := createIssue(t, marie, "WEB", "Dans un sprint à venir", "")
	plan(inFuture, future)

	marie.call("DELETE", "/api/boards/"+board, nil, 204, nil)

	if n := pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM sprints WHERE id = ?`, future); n != "0" {
		t.Errorf("the future sprint is still there")
	}
	if h := historyFields(t, marie, inFuture); h["Sprint"] != 2 {
		t.Errorf("history of the issue of the future sprint: %v", h)
	}
	if state := pick(t, `SELECT CONCAT(state, ' ', completed_at IS NOT NULL) FROM sprints WHERE id = ?`, active); state != "closed 1" {
		t.Errorf("the active sprint: %q, want it completed", state)
	}
	if state := pick(t, `SELECT state FROM sprints WHERE id = ?`, closed); state != "closed" {
		t.Errorf("the closed sprint: %q", state)
	}
	for key, want := range map[string]string{inClosed: "1 closed", inActive: "1 closed", inFuture: "0 "} {
		got := pick(t, `SELECT CONCAT(COUNT(*), ' ', COALESCE(GROUP_CONCAT(DISTINCT s.state), '')) FROM sprint_issues si
			JOIN sprints s ON s.id = si.sprint_id JOIN issues i ON i.id = si.issue_id JOIN projects p ON p.id = i.project_id
			WHERE `+keyExpr+` = ?`, key)
		if got != want {
			t.Errorf("sprints of %s: %q, want %q", key, got, want)
		}
	}
}
