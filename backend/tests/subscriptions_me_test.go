package tests

// Test of the list of one's own filter subscriptions (GET /api/me/subscriptions).
// It needs SOLSTICE_TEST_DB and is skipped without it.

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestMySubscriptions(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	paul := login(t, "paul.martin", "motdepasse")
	hugo := login(t, "hugo.robert", "motdepasse")

	status, body := marie.do("POST", "/api/filters", map[string]any{"name": "Abonnements de la liste", "query": "project = DEMO",
		"shares": []map[string]any{{"kind": "everyone"}}})
	if status != 201 {
		t.Fatalf("filter: %d %s", status, body)
	}
	var f filterOut
	json.Unmarshal(body, &f)
	t.Cleanup(func() { marie.do("DELETE", fmt.Sprintf("/api/filters/%d", f.ID), nil) })
	base := fmt.Sprintf("/api/filters/%d/subscriptions", f.ID)
	if status, body := paul.do("POST", base, map[string]any{"schedule": "weekly", "days": []int{1, 4}, "hour": 7}); status != 201 {
		t.Fatalf("paul.martin subscribes: %d %s", status, body)
	}
	if status, body := marie.do("POST", base, map[string]any{"schedule": "daily", "hour": 9}); status != 201 {
		t.Fatalf("marie.durand subscribes: %d %s", status, body)
	}

	var list []mySubscriptionOut
	if status := paul.get("/api/me/subscriptions", &list); status != 200 {
		t.Fatalf("list: %d", status)
	}
	var found []mySubscriptionOut
	for _, s := range list {
		if s.FilterID == f.ID {
			found = append(found, s)
		}
	}
	if len(found) != 1 || found[0].FilterName != "Abonnements de la liste" || found[0].Schedule != "weekly" ||
		found[0].User == nil || found[0].User.Username != "paul.martin" || !found[0].CanEdit {
		t.Errorf("paul.martin's subscriptions on the filter: %+v", found)
	}

	// Someone without a subscription sees none of theirs on it; the filter's
	// owner sees their own subscription only.
	list = nil
	hugo.get("/api/me/subscriptions", &list)
	for _, s := range list {
		if s.FilterID == f.ID {
			t.Errorf("hugo.robert sees a subscription of %+v", s.User)
		}
	}
	list = nil
	marie.get("/api/me/subscriptions", &list)
	n := 0
	for _, s := range list {
		if s.FilterID == f.ID {
			n++
			if s.User == nil || s.User.Username != "marie.durand" {
				t.Errorf("marie.durand sees %+v", s.User)
			}
		}
	}
	if n != 1 {
		t.Errorf("marie.durand's subscriptions on the filter: %d", n)
	}

	// An administrator sees their own, not everyone's.
	admin := login(t, "admin", "admin")
	list = nil
	admin.get("/api/me/subscriptions", &list)
	for _, s := range list {
		if s.FilterID == f.ID {
			t.Errorf("admin sees %+v", s.User)
		}
	}
}
