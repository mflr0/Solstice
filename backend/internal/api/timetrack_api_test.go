package api

// Integration tests of time estimates, work logs and sprint editing. Like
// api_test.go they need the synthetic database (SOLSTICE_TEST_DB) and skip
// without it. Each test makes the issues and sprints it needs.

import (
	"encoding/json"
	"net/http"
	"testing"
)

// newTestIssue creates a task in DEMO as the user and returns its key.
func newTestIssue(t *testing.T, c *client, summary string, extra map[string]any) string {
	t.Helper()
	var project struct {
		IssueTypes []typeRef `json:"issueTypes"`
	}
	c.get("/api/projects/DEMO", &project)
	body := map[string]any{"project": "DEMO", "summary": summary}
	for _, it := range project.IssueTypes {
		if !it.Subtask {
			body["type"] = it.ID
			break
		}
	}
	for k, v := range extra {
		body[k] = v
	}
	status, out := c.do("POST", "/api/issues", body)
	if status != 201 {
		t.Fatalf("create %q: %d %s", summary, status, out)
	}
	var created struct{ Key string }
	json.Unmarshal(out, &created)
	return created.Key
}

func issueTime(t *testing.T, c *client, key string) timeTracking {
	t.Helper()
	var d issueDetail
	if status := c.get("/api/issues/"+key, &d); status != 200 {
		t.Fatalf("GET %s: %d", key, status)
	}
	return d.Time
}

func secs(p *int64) int64 {
	if p == nil {
		return -1
	}
	return *p
}

func wantTime(t *testing.T, what string, got timeTracking, original, remaining, spent int64) {
	t.Helper()
	if secs(got.Original) != original || secs(got.Remaining) != remaining || secs(got.Spent) != spent {
		t.Errorf("%s: original %d, remaining %d, spent %d; want %d, %d, %d (-1 is unset)", what,
			secs(got.Original), secs(got.Remaining), secs(got.Spent), original, remaining, spent)
	}
}

func historyFields(t *testing.T, c *client, key string) map[string]int {
	t.Helper()
	var history []historyEntry
	c.get("/api/issues/"+key+"/history", &history)
	out := map[string]int{}
	for _, h := range history {
		for _, it := range h.Items {
			out[it.Field]++
		}
	}
	return out
}

func TestEstimatesOnIssues(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	const day = 8 * 3600

	// Created with an original estimate: the remaining one starts equal.
	key := newTestIssue(t, paul, "Estimer la migration", map[string]any{"originalEstimate": "1d 2h"})
	wantTime(t, "created", issueTime(t, paul, key), day+2*3600, day+2*3600, -1)

	// An explicit remaining estimate on creation.
	other := newTestIssue(t, paul, "Estimer le correctif", map[string]any{"originalEstimate": "2h", "remainingEstimate": "30m"})
	wantTime(t, "created with both", issueTime(t, paul, other), 2*3600, 1800, -1)

	// Anything not in Jira's syntax is refused, on creation and on edit.
	if status, _ := paul.do("POST", "/api/issues", map[string]any{"project": "DEMO", "type": "x", "summary": "x",
		"originalEstimate": "bientôt"}); status != 400 {
		t.Errorf("create with a bad estimate: %d", status)
	}
	for _, bad := range []string{"soon", "1h 30", "-2h", "1x"} {
		if status, _ := paul.do("PATCH", "/api/issues/"+key, map[string]any{"originalEstimate": bad}); status != 400 {
			t.Errorf("estimate %q: %d, want 400", bad, status)
		}
	}
	wantTime(t, "after refusals", issueTime(t, paul, key), day+2*3600, day+2*3600, -1)

	// Editing the original estimate before any work moves the remaining one too, and is in the history.
	if status, body := paul.do("PATCH", "/api/issues/"+key, map[string]any{"originalEstimate": "2d"}); status != 200 {
		t.Fatalf("edit: %d %s", status, body)
	}
	wantTime(t, "edited", issueTime(t, paul, key), 2*day, 2*day, -1)
	if h := historyFields(t, paul, key); h["timeoriginalestimate"] != 1 || h["timeestimate"] != 1 {
		t.Errorf("history after the edit: %v", h)
	}

	// Editing the remaining estimate alone; null clears the original one.
	paul.do("PATCH", "/api/issues/"+key, map[string]any{"remainingEstimate": "1d"})
	wantTime(t, "remaining only", issueTime(t, paul, key), 2*day, day, -1)
	paul.do("PATCH", "/api/issues/"+key, map[string]any{"originalEstimate": nil})
	if got := issueTime(t, paul, key); got.Original != nil || secs(got.Remaining) != day {
		t.Errorf("cleared original estimate: %+v", got)
	}

	// The REST layer: timetracking on creation and on edit.
	rest := func(method, path string, body any) (int, map[string]any) {
		status, _, out := call(t, paul, testURL, method, path, body, func(r *http.Request) { r.Header.Set(csrfHeader, "1") })
		var v map[string]any
		json.Unmarshal(out, &v)
		return status, v
	}
	var project struct {
		IssueTypes []typeRef `json:"issueTypes"`
	}
	paul.get("/api/projects/DEMO", &project)
	var task string
	for _, it := range project.IssueTypes {
		if !it.Subtask {
			task = it.ID
			break
		}
	}
	status, created := rest("POST", "/rest/api/2/issue", map[string]any{"fields": map[string]any{
		"project": map[string]string{"key": "DEMO"}, "issuetype": map[string]string{"id": task}, "summary": "Estimation par l'API REST",
		"timetracking": map[string]string{"originalEstimate": "1w 1d"}}})
	if status != 201 {
		t.Fatalf("REST create: %d %v", status, created)
	}
	rkey := str(created, "key")
	_, issue := rest("GET", "/rest/api/2/issue/"+rkey+"?fields=timetracking,timeoriginalestimate", nil)
	if str(issue, "fields", "timetracking", "originalEstimate") != "1w 1d" || str(issue, "fields", "timetracking", "remainingEstimate") != "1w 1d" ||
		num(issue, "fields", "timeoriginalestimate") != 6*day {
		t.Errorf("REST timetracking after creation: %v", at(issue, "fields"))
	}
	if status, _ := rest("PUT", "/rest/api/2/issue/"+rkey, map[string]any{"update": map[string]any{
		"timetracking": []any{map[string]any{"edit": map[string]string{"originalEstimate": "3d", "remainingEstimate": "1d"}}}}}); status != 204 {
		t.Errorf("REST edit of timetracking: %d", status)
	}
	wantTime(t, "REST edit", issueTime(t, paul, rkey), 3*day, day, -1)
	if status, e := rest("PUT", "/rest/api/2/issue/"+rkey, map[string]any{"fields": map[string]any{
		"timetracking": map[string]string{"originalEstimate": "tomorrow"}}}); status != 400 || at(e, "errors", "timetracking") == nil {
		t.Errorf("REST edit with a bad estimate: %d %v", status, e)
	}
	// Someone who cannot edit the issue cannot change its estimates.
	chloe := login(t, "chloe.simon", "motdepasse")
	var d issueDetail
	if chloe.get("/api/issues/"+rkey, &d) == 200 && !d.Can["edit"] {
		if status, _ := chloe.do("PATCH", "/api/issues/"+rkey, map[string]any{"originalEstimate": "1h"}); status != 403 {
			t.Errorf("estimate edited by someone who may not edit the issue: %d", status)
		}
		wantTime(t, "after the refusal", issueTime(t, paul, rkey), 3*day, day, -1)
	}
}
