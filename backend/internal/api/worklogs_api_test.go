package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestLoggedWorkAdjustsEstimate(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	const day = 8 * 3600
	key := newTestIssue(t, paul, "Le temps saisi réduit le reste à faire", map[string]any{"originalEstimate": "2d"})
	paul.do("PATCH", "/api/issues/"+key, map[string]any{"remainingEstimate": "2h"})
	if status, body := paul.do("POST", "/api/issues/"+key+"/worklogs", map[string]any{"seconds": 3600}); status != 201 {
		t.Fatalf("log work: %d %s", status, body)
	}
	wantTime(t, "after 1h", issueTime(t, paul, key), 2*day, 3600, 3600)
	paul.do("POST", "/api/issues/"+key+"/worklogs", map[string]any{"seconds": 3 * 3600})
	wantTime(t, "after 4h", issueTime(t, paul, key), 2*day, 0, 4*3600)
	if h := historyFields(t, paul, key); h["timespent"] != 2 {
		t.Errorf("history after two work logs: %v", h)
	}
	// Once work is logged, the original estimate no longer drags the remaining one along.
	paul.do("PATCH", "/api/issues/"+key, map[string]any{"originalEstimate": "3d"})
	wantTime(t, "original after work", issueTime(t, paul, key), 3*day, 0, 4*3600)
}

func TestWorklogEditAndDelete(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	key := newTestIssue(t, paul, "Corriger le temps saisi", map[string]any{"originalEstimate": "4h"})
	paul.do("POST", "/api/issues/"+key+"/worklogs", map[string]any{"seconds": 3600, "body": "Analyse"})
	var logs []worklogOut
	paul.get("/api/issues/"+key+"/worklogs", &logs)
	if len(logs) != 1 || !logs[0].CanEdit {
		t.Fatalf("work logs: %+v", logs)
	}
	path := fmt.Sprintf("/api/worklogs/%d", logs[0].ID)
	wantTime(t, "logged", issueTime(t, paul, key), 4*3600, 3*3600, 3600)

	// The author edits the time: spent and remaining follow by the difference.
	status, body := paul.do("PUT", path, map[string]any{"seconds": 7200})
	if status != 200 {
		t.Fatalf("edit: %d %s", status, body)
	}
	var edited worklogOut
	json.Unmarshal(body, &edited)
	if edited.Seconds != 7200 || edited.Body == nil || *edited.Body != "Analyse" || !edited.CanEdit {
		t.Errorf("edited work log: %+v", edited)
	}
	wantTime(t, "edited to 2h", issueTime(t, paul, key), 4*3600, 2*3600, 7200)
	// And shortens it: time comes back to the remaining estimate.
	paul.do("PUT", path, map[string]any{"seconds": 1800, "body": "Analyse rapide"})
	wantTime(t, "edited to 30m", issueTime(t, paul, key), 4*3600, 3*3600+1800, 1800)
	paul.get("/api/issues/"+key+"/worklogs", &logs)
	if logs[0].Seconds != 1800 || logs[0].Body == nil || *logs[0].Body != "Analyse rapide" {
		t.Errorf("work log after the edit: %+v", logs[0])
	}
	if h := historyFields(t, paul, key); h["timespent"] < 3 {
		t.Errorf("history of the edits: %v", h)
	}

	for _, bad := range []map[string]any{{"seconds": 0}, {"seconds": -5}, {"seconds": 8 * 24 * 3600}} {
		if status, _ := paul.do("PUT", path, bad); status != 400 {
			t.Errorf("edit with %v: %d, want 400", bad, status)
		}
	}
	if status, _ := paul.do("PUT", "/api/worklogs/999999999", map[string]any{"seconds": 60}); status != 404 {
		t.Errorf("unknown work log: %d", status)
	}

	// Someone else may not, unless they moderate the project.
	hugo := login(t, "hugo.robert", "motdepasse")
	if status, _ := hugo.do("PUT", path, map[string]any{"seconds": 60}); status != 403 {
		t.Errorf("another user edited the work log: %d", status)
	}
	if status, _ := hugo.do("DELETE", path, nil); status != 403 {
		t.Errorf("another user deleted the work log: %d", status)
	}
	var hugoLogs []worklogOut
	hugo.get("/api/issues/"+key+"/worklogs", &hugoLogs)
	if len(hugoLogs) != 1 || hugoLogs[0].CanEdit {
		t.Errorf("what another user may edit: %+v", hugoLogs)
	}
	marie := login(t, "marie.durand", "motdepasse")
	var d issueDetail
	marie.get("/api/issues/"+key, &d)
	if d.Can["moderate"] {
		if status, body := marie.do("PUT", path, map[string]any{"seconds": 900}); status != 200 {
			t.Errorf("a moderator could not edit: %d %s", status, body)
		}
		wantTime(t, "edited by a moderator", issueTime(t, paul, key), 4*3600, 4*3600-900, 900)
	}

	// Deleting gives the time back.
	if status, body := paul.do("DELETE", path, nil); status != 204 {
		t.Fatalf("delete: %d %s", status, body)
	}
	paul.get("/api/issues/"+key+"/worklogs", &logs)
	if len(logs) != 0 {
		t.Errorf("work logs after the deletion: %+v", logs)
	}
	wantTime(t, "deleted", issueTime(t, paul, key), 4*3600, 4*3600, -1)
	if status, _ := paul.do("DELETE", path, nil); status != 404 {
		t.Errorf("deleting twice: %d", status)
	}

	// The REST layer.
	rest := func(c *client, method, path string, body any) (int, map[string]any) {
		status, _, out := call(t, c, testURL, method, path, body, func(r *http.Request) { r.Header.Set(csrfHeader, "1") })
		var v map[string]any
		json.Unmarshal(out, &v)
		return status, v
	}
	status, wl := rest(paul, "POST", "/rest/api/2/issue/"+key+"/worklog", map[string]any{"timeSpent": "1h", "comment": "REST"})
	if status != 201 {
		t.Fatalf("REST add: %d %v", status, wl)
	}
	id := str(wl, "id")
	rpath := "/rest/api/2/issue/" + key + "/worklog/" + id
	if status, v := rest(paul, "GET", rpath, nil); status != 200 || str(v, "comment") != "REST" {
		t.Errorf("REST get: %d %v", status, v)
	}
	status, wl = rest(paul, "PUT", rpath, map[string]any{"timeSpent": "2h 30m", "comment": "REST, revu"})
	if status != 200 || num(wl, "timeSpentSeconds") != 9000 || str(wl, "comment") != "REST, revu" || str(wl, "timeSpent") != "2h 30m" {
		t.Errorf("REST edit: %d %v", status, wl)
	}
	wantTime(t, "REST edit", issueTime(t, paul, key), 4*3600, 4*3600-9000, 9000)
	if status, _ := rest(paul, "PUT", rpath, map[string]any{"timeSpent": "lots"}); status != 400 {
		t.Errorf("REST edit with a bad duration: %d", status)
	}
	if status, _ := rest(hugo, "DELETE", rpath, nil); status != 403 {
		t.Errorf("REST delete by another user: %d", status)
	}
	// A work log is only reachable through its own issue.
	other := newTestIssue(t, paul, "Autre ticket", nil)
	if status, _ := rest(paul, "DELETE", "/rest/api/2/issue/"+other+"/worklog/"+id, nil); status != 404 {
		t.Errorf("work log through another issue: %d", status)
	}
	if status, _ := rest(paul, "DELETE", rpath, nil); status != 204 {
		t.Errorf("REST delete: %d", status)
	}
	wantTime(t, "REST delete", issueTime(t, paul, key), 4*3600, 4*3600, -1)
}
