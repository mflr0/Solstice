package tests

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// sprintsOf lists the open sprints of a board, by state.
func sprintsOf(t *testing.T, c *client, board string) (active, future []int64) {
	t.Helper()
	var b boardDetail
	if status := c.get("/api/boards/"+board, &b); status != 200 {
		t.Fatalf("GET board: %d", status)
	}
	for _, sp := range b.Sprints {
		switch sp.State {
		case "active":
			active = append(active, sp.ID)
		case "future":
			future = append(future, sp.ID)
		}
	}
	return active, future
}

func TestSprintEditAndDelete(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	board := boardID(t, "DEMO — Scrum")
	status, body := marie.do("POST", "/api/boards/"+board+"/sprints", map[string]any{"name": "À renommer"})
	if status != 201 {
		t.Fatalf("create sprint: %d %s", status, body)
	}
	var created struct{ ID int64 }
	json.Unmarshal(body, &created)
	path := "/api/sprints/" + strconv.FormatInt(created.ID, 10)

	// Rename, goal and planned dates of a future sprint.
	status, body = marie.do("PATCH", path, map[string]any{"name": "  Sprint 12 ", "goal": "Paiement en un clic",
		"start": "2026-11-02T08:00:00Z", "end": "2026-11-13T17:00:00Z"})
	var sp sprintOut
	json.Unmarshal(body, &sp)
	if status != 200 || sp.Name != "Sprint 12" || sp.Goal == nil || *sp.Goal != "Paiement en un clic" || sp.Start == nil ||
		sp.End == nil || sp.State != "future" {
		t.Fatalf("edit: %d %s", status, body)
	}
	// What is not given stays; null clears.
	status, body = marie.do("PATCH", path, map[string]any{"goal": nil, "end": nil})
	json.Unmarshal(body, &sp)
	if status != 200 || sp.Name != "Sprint 12" || sp.Goal != nil || sp.End != nil || sp.Start == nil {
		t.Errorf("partial edit: %d %s", status, body)
	}
	for name, bad := range map[string]map[string]any{
		"empty name":    {"name": "  "},
		"null name":     {"name": nil},
		"end <= start":  {"start": "2026-11-13T00:00:00Z", "end": "2026-11-02T00:00:00Z"},
		"unknown field": {"state": "active"},
		"bad date":      {"start": "demain"},
	} {
		if status, _ := marie.do("PATCH", path, bad); status != 400 {
			t.Errorf("%s: %d, want 400", name, status)
		}
	}
	// Board managers only.
	paul := login(t, "paul.martin", "motdepasse")
	if status, _ := paul.do("PATCH", path, map[string]any{"name": "Piraté"}); status != 403 {
		t.Errorf("a developer edited the sprint: %d", status)
	}
	if status, _ := paul.do("DELETE", path, nil); status != 403 {
		t.Errorf("a developer deleted the sprint: %d", status)
	}
	if status, _ := marie.do("PATCH", "/api/sprints/999999999", map[string]any{"name": "x"}); status != 404 {
		t.Errorf("unknown sprint: %d", status)
	}

	// The active sprint may be edited but not deleted.
	active, _ := sprintsOf(t, marie, board)
	for _, id := range active {
		apath := "/api/sprints/" + strconv.FormatInt(id, 10)
		if status, _ := marie.do("DELETE", apath, nil); status != 409 {
			t.Errorf("deleted the active sprint: %d", status)
		}
		if status, body := marie.do("PATCH", apath, map[string]any{"goal": "Objectif révisé"}); status != 200 {
			t.Errorf("edit of the active sprint: %d %s", status, body)
		}
		if status, _ := marie.do("PATCH", apath, map[string]any{"start": nil}); status != 400 {
			t.Errorf("the active sprint lost its start: %d", status)
		}
		if status, _ := marie.do("PATCH", apath, map[string]any{"start": "2999-01-01T00:00:00Z"}); status != 400 {
			t.Errorf("the active sprint starts in the future: %d", status)
		}
	}
	// A completed sprint is history.
	if closed := pickOptional(`SELECT CAST(id AS CHAR) FROM sprints WHERE state = 'closed' AND board_id = ? LIMIT 1`, board); closed != "" {
		cpath := "/api/sprints/" + closed
		if status, _ := marie.do("PATCH", cpath, map[string]any{"name": "Réécrit"}); status != 409 {
			t.Errorf("edited a completed sprint: %d", status)
		}
		if status, _ := marie.do("DELETE", cpath, nil); status != 409 {
			t.Errorf("deleted a completed sprint: %d", status)
		}
	}

	// Deleting a future sprint sends its issues back to the backlog.
	key := newTestIssue(t, paul, "Prévu pour un sprint supprimé", nil)
	if status, body := marie.do("POST", "/api/boards/"+board+"/move", map[string]any{"issue": key, "sprint": created.ID}); status != 204 {
		t.Fatalf("plan the issue: %d %s", status, body)
	}
	if status, body := marie.do("DELETE", path, nil); status != 204 {
		t.Fatalf("delete: %d %s", status, body)
	}
	if n := pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM sprints WHERE id = ?`, created.ID); n != "0" {
		t.Errorf("the sprint is still there")
	}
	var d issueDetail
	marie.get("/api/issues/"+key, &d)
	if len(d.Sprints) != 0 {
		t.Errorf("the issue is still in %+v", d.Sprints)
	}
	var backlog struct{ Backlog []boardCard }
	marie.get("/api/boards/"+board+"/backlog", &backlog)
	inBacklog := false
	for _, c := range backlog.Backlog {
		inBacklog = inBacklog || c.Key == key
	}
	if !inBacklog {
		t.Errorf("%s is not back in the backlog", key)
	}
	if h := historyFields(t, marie, key); h["Sprint"] != 2 {
		t.Errorf("history of the planned and unplanned issue: %v", h)
	}
	if status, _ := marie.do("DELETE", path, nil); status != 404 {
		t.Errorf("deleting twice: %d", status)
	}

	// The Agile REST API.
	rest := func(method, path string, body any) (int, map[string]any) {
		status, _, out := call(t, marie, testURL, method, path, body, func(r *http.Request) { r.Header.Set(csrfHeader, "1") })
		var v map[string]any
		json.Unmarshal(out, &v)
		return status, v
	}
	_, body = marie.do("POST", "/api/boards/"+board+"/sprints", map[string]any{"name": "Via REST"})
	json.Unmarshal(body, &created)
	rpath := "/rest/agile/1.0/sprint/" + strconv.FormatInt(created.ID, 10)
	status, v := rest("POST", rpath, map[string]any{"name": "Sprint REST", "goal": "Tout est configurable",
		"startDate": "2026-11-16T09:00:00.000+0100", "endDate": "2026-11-27T18:00:00.000+0100"})
	if status != 200 || str(v, "name") != "Sprint REST" || str(v, "goal") != "Tout est configurable" || str(v, "state") != "future" ||
		!strings.HasPrefix(str(v, "startDate"), "2026-11-16T08:00:00.000") || !strings.HasPrefix(str(v, "endDate"), "2026-11-27T17:00:00.000") {
		t.Errorf("REST update: %d %v", status, v)
	}
	if status, v := rest("PUT", rpath, map[string]any{"name": "Sprint REST 2", "goal": nil}); status != 200 ||
		str(v, "name") != "Sprint REST 2" || at(v, "goal") != nil {
		t.Errorf("REST PUT: %d %v", status, v)
	}
	if status, _ := rest("POST", rpath, map[string]any{"endDate": "demain"}); status != 400 {
		t.Errorf("REST update with a bad date: %d", status)
	}
	if status, _ := rest("POST", rpath, map[string]any{"state": "closed"}); status != 400 {
		t.Errorf("REST: completing a future sprint: %d", status)
	}
	if status, _, _ := call(t, login(t, "paul.martin", "motdepasse"), testURL, "DELETE", rpath, nil,
		func(r *http.Request) { r.Header.Set(csrfHeader, "1") }); status != 403 {
		t.Errorf("REST delete by a developer: %d", status)
	}
	if status, _ := rest("DELETE", rpath, nil); status != 204 {
		t.Errorf("REST delete: %d", status)
	}
	// Starting and completing through the state, when the board has no sprint running.
	if active, _ := sprintsOf(t, marie, board); len(active) == 0 {
		_, body = marie.do("POST", "/api/boards/"+board+"/sprints", map[string]any{"name": "Cycle de vie"})
		json.Unmarshal(body, &created)
		lpath := "/rest/agile/1.0/sprint/" + strconv.FormatInt(created.ID, 10)
		if status, v := rest("POST", lpath, map[string]any{"state": "active", "endDate": "2999-01-01T00:00:00.000+0000"}); status != 200 ||
			str(v, "state") != "active" || str(v, "startDate") == "" {
			t.Errorf("REST start: %d %v", status, v)
		}
		if status, v := rest("POST", lpath, map[string]any{"name": "Cycle de vie, mois 2", "goal": "En cours"}); status != 200 ||
			str(v, "state") != "active" || str(v, "name") != "Cycle de vie, mois 2" {
			t.Errorf("REST edit of an active sprint: %d %v", status, v)
		}
		if status, _ := rest("DELETE", lpath, nil); status != 409 {
			t.Errorf("REST delete of an active sprint: %d", status)
		}
		if status, v := rest("POST", lpath, map[string]any{"state": "closed"}); status != 200 || str(v, "state") != "closed" ||
			str(v, "completeDate") == "" {
			t.Errorf("REST complete: %d %v", status, v)
		}
		if status, _ := rest("POST", lpath, map[string]any{"name": "Trop tard"}); status != 409 {
			t.Errorf("REST edit of a completed sprint: %d", status)
		}
	}
	if status, _ := rest("GET", rpath, nil); status != 404 {
		t.Errorf("REST get after the deletion: %d", status)
	}
}

// pickOptional is pick for a query that may find nothing: it answers "".
func pickOptional(query string, args ...any) string {
	var s string
	if err := testDB.QueryRow(query, args...).Scan(&s); err != nil {
		return ""
	}
	return s
}
