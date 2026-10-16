package tests

import (
	"encoding/json"
	"testing"
)

// TestStatusUsage checks where the administration says a status is used, and
// that it agrees with the counts of the list and with what deletion refuses.
func TestStatusUsage(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")

	var list []struct {
		ID    string         `json:"id"`
		Usage map[string]int `json:"usage"`
	}
	if status := admin.get("/api/admin/statuses", &list); status != 200 {
		t.Fatalf("list: %d", status)
	}
	var used string
	var counts map[string]int
	for _, st := range list {
		if st.Usage["issues"] > 0 && st.Usage["workflows"] > 0 {
			used, counts = st.ID, st.Usage
			break
		}
	}
	if used == "" {
		t.Skip("no status used by issues and workflows in the fixture")
	}
	var usage statusUsageOut
	if status := admin.get("/api/admin/statuses/"+used+"/usage", &usage); status != 200 {
		t.Fatalf("usage: %d", status)
	}
	issues := 0
	for _, p := range usage.Projects {
		issues += p.Issues
	}
	if issues != counts["issues"] || len(usage.Workflows) != counts["workflows"] || len(usage.Boards) == 0 && counts["boardColumns"] > 0 {
		t.Errorf("usage %+v disagrees with counts %v", usage, counts)
	}
	if status, _ := admin.do("DELETE", "/api/statuses/"+used, nil); status != 409 {
		t.Errorf("delete a used status: %d", status)
	}

	// A new status is used nowhere.
	status, body := admin.do("POST", "/api/statuses", map[string]any{"name": "Usage test", "category": "todo"})
	if status != 201 {
		t.Fatalf("create: %d %s", status, body)
	}
	var created struct{ ID string }
	json.Unmarshal(body, &created)
	t.Cleanup(func() { admin.do("DELETE", "/api/statuses/"+created.ID, nil) })
	usage = statusUsageOut{}
	admin.get("/api/admin/statuses/"+created.ID+"/usage", &usage)
	if len(usage.Workflows)+len(usage.Boards)+len(usage.Projects) != 0 {
		t.Errorf("a new status is used: %+v", usage)
	}

	if status := admin.get("/api/admin/statuses/no-such-status/usage", nil); status != 404 {
		t.Errorf("unknown status: %d", status)
	}
	if status := paul.get("/api/admin/statuses/"+used+"/usage", nil); status != 403 {
		t.Errorf("non-admin: %d", status)
	}
}
