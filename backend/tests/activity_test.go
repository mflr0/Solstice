package tests

// What people do on issues besides editing them: votes, watching, work
// logs, deleting comments and links; and the lookups the interface makes
// (people, mentions, assignable users), signing out, subscription edits.

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// rhKey is an RH issue paul.martin may not see.
func rhKey(t *testing.T) string {
	return pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'RH' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
}

// scratchIssue creates a DEMO issue as paul.martin, deleted when the test ends.
func scratchIssue(t *testing.T, paul *client, summary string) string {
	t.Helper()
	key := createIssue(t, paul, "DEMO", summary, "")
	t.Cleanup(func() { call(t, nil, testURL, "DELETE", "/rest/api/2/issue/"+key, nil, basic("admin", "admin")) })
	return key
}

func TestVotesAndWatchers(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	hugo := login(t, "hugo.robert", "motdepasse")
	key := scratchIssue(t, paul, "Votes et observateurs")

	var c counter
	hugo.call("PUT", "/api/issues/"+key+"/vote", nil, 200, &c)
	if c.Count != 1 || !c.Me {
		t.Errorf("after voting: %+v", c)
	}
	hugo.call("PUT", "/api/issues/"+key+"/vote", nil, 200, &c)
	if c.Count != 1 {
		t.Errorf("voting twice counts twice: %+v", c)
	}
	var d issueDetail
	paul.get("/api/issues/"+key, &d)
	if d.Votes.Count != 1 || d.Votes.Me || d.Watchers.Count != 1 || !d.Watchers.Me {
		t.Errorf("seen by the reporter: votes %+v, watchers %+v", d.Votes, d.Watchers)
	}
	hugo.call("DELETE", "/api/issues/"+key+"/vote", nil, 200, &c)
	if c.Count != 0 || c.Me {
		t.Errorf("after unvoting: %+v", c)
	}

	hugo.call("PUT", "/api/issues/"+key+"/watch", nil, 200, &c)
	if c.Count != 2 || !c.Me {
		t.Errorf("after watching: %+v", c)
	}
	paul.call("DELETE", "/api/issues/"+key+"/watch", nil, 200, &c)
	if c.Count != 1 || c.Me {
		t.Errorf("after the reporter stopped watching: %+v", c)
	}

	// Nothing on an issue one does not see.
	for _, path := range []string{"/vote", "/watch"} {
		if status, _ := paul.do("PUT", "/api/issues/"+rhKey(t)+path, nil); status != 404 {
			t.Errorf("PUT %s on an RH issue: %d", path, status)
		}
	}
}

func TestWorklogs(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	key := scratchIssue(t, paul, "Temps passé")
	for _, bad := range []map[string]any{{"seconds": 0}, {"seconds": -60}, {"seconds": 8 * 24 * 3600}} {
		if status, _ := paul.do("POST", "/api/issues/"+key+"/worklogs", bad); status != 400 {
			t.Errorf("work log %v: %d", bad, status)
		}
	}
	paul.call("POST", "/api/issues/"+key+"/worklogs", map[string]any{"seconds": 5400, "body": "Analyse",
		"started": "2026-10-14T09:00:00+02:00"}, 201, nil)
	paul.call("POST", "/api/issues/"+key+"/worklogs", map[string]any{"seconds": 600}, 201, nil)
	var list []struct {
		Author  *userRef
		Seconds int64
		Body    *string
		Started string
	}
	paul.call("GET", "/api/issues/"+key+"/worklogs", nil, 200, &list)
	if len(list) != 2 || list[0].Seconds != 5400 || list[0].Body == nil || *list[0].Body != "Analyse" ||
		list[0].Author == nil || list[0].Author.Username != "paul.martin" || !strings.HasPrefix(list[0].Started, "2026-10-14T07:00:00") {
		t.Errorf("work logs: %+v", list)
	}
	var d issueDetail
	paul.get("/api/issues/"+key, &d)
	if d.Time.Spent == nil || *d.Time.Spent != 6000 {
		t.Errorf("time spent: %v", d.Time.Spent)
	}
	if status, _ := paul.do("GET", "/api/issues/"+rhKey(t)+"/worklogs", nil); status != 404 {
		t.Errorf("work logs of an RH issue: %d", status)
	}
}

func TestCommentDeletion(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	key := scratchIssue(t, paul, "Commentaire supprimé")
	var c commentOut
	paul.call("POST", "/api/issues/"+key+"/comments", map[string]any{"body": "À supprimer"}, 201, &c)
	path := fmt.Sprintf("/api/comments/%d", c.ID)
	login(t, "hugo.robert", "motdepasse").call("DELETE", path, nil, 403, nil)
	paul.call("DELETE", path, nil, 204, nil)
	var list []commentOut
	paul.call("GET", "/api/issues/"+key+"/comments", nil, 200, &list)
	if len(list) != 0 {
		t.Errorf("comments after deletion: %+v", list)
	}
	paul.call("DELETE", path, nil, 404, nil)
	paul.call("PATCH", path, map[string]string{"body": "x"}, 404, nil)
}

func TestLinkDeletion(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	a, b := scratchIssue(t, paul, "Lien source"), scratchIssue(t, paul, "Lien cible")
	linkType := pick(t, `SELECT id FROM link_types ORDER BY position LIMIT 1`)
	for _, bad := range []map[string]any{
		{"type": 999999, "direction": "outward", "target": b},
		{"type": json.Number(linkType), "direction": "outward", "target": a},
		{"type": json.Number(linkType), "direction": "outward", "target": rhKey(t)},
		{"type": json.Number(linkType), "direction": "outward", "target": "NOPE-1"},
	} {
		if status, body := paul.do("POST", "/api/issues/"+a+"/links", bad); status != 400 {
			t.Errorf("link %v: %d %s", bad, status, body)
		}
	}
	var d issueDetail
	paul.call("POST", "/api/issues/"+a+"/links", map[string]any{"type": json.Number(linkType), "direction": "outward",
		"target": b}, 201, &d)
	if len(d.Links) != 1 || d.Links[0].Issue.Key != b {
		t.Fatalf("links of %s: %+v", a, d.Links)
	}
	paul.get("/api/issues/"+b, &d)
	if len(d.Links) != 1 || d.Links[0].Issue.Key != a {
		t.Fatalf("links of %s: %+v", b, d.Links)
	}
	path := fmt.Sprintf("/api/links/%d", d.Links[0].ID)
	paul.call("DELETE", path, nil, 204, nil)
	paul.get("/api/issues/"+a, &d)
	if len(d.Links) != 0 {
		t.Errorf("links after deletion: %+v", d.Links)
	}
	paul.call("DELETE", path, nil, 404, nil)
	var history []historyEntry
	paul.get("/api/issues/"+b+"/history", &history)
	if !historyHas(history, "Link") {
		t.Errorf("%s's history lacks the link: %+v", b, history)
	}
}

func TestPeopleAndRefs(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	var users []userRef
	paul.call("GET", "/api/users?q=MARIE", nil, 200, &users)
	if !slices.ContainsFunc(users, func(u userRef) bool { return u.Username == "marie.durand" }) {
		t.Errorf("users matching MARIE: %+v", users)
	}
	paul.call("GET", "/api/users?q=emma.morel", nil, 200, &users)
	if len(users) != 0 {
		t.Errorf("a deactivated account is offered: %+v", users)
	}

	// Someone's page lists only what the reader may see.
	var person struct {
		User     userRef
		Email    string
		Assigned []struct{ Key string }
	}
	assignee := pick(t, `SELECT u.username FROM issues i JOIN projects p ON p.id = i.project_id JOIN users u ON u.id = i.assignee_id
		JOIN statuses st ON st.id = i.status_id WHERE p.pkey = 'RH' AND st.category <> 'done' AND u.active
		GROUP BY u.id ORDER BY COUNT(*) DESC LIMIT 1`)
	paul.call("GET", "/api/people/"+strings.ToUpper(assignee), nil, 200, &person)
	if person.User.Username != assignee {
		t.Errorf("person: %+v", person.User)
	}
	for _, is := range person.Assigned {
		if strings.HasPrefix(is.Key, "RH-") {
			t.Errorf("paul.martin sees %s on %s's page", is.Key, assignee)
		}
	}
	paul.call("GET", "/api/people/personne.inconnue", nil, 404, nil)

	// Mentions and issue keys in wiki text.
	demo := pick(t, `SELECT `+keyExpr+` FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE p.pkey = 'DEMO' AND i.security_level_id IS NULL ORDER BY i.num LIMIT 1`)
	var refs struct {
		Users  map[string]string
		Issues map[string]bool
	}
	paul.call("POST", "/api/refs", map[string]any{"users": []string{"Marie.Durand", "personne.inconnue"},
		"issueKeys": []string{demo, rhKey(t), "NOPE-1"}}, 200, &refs)
	if len(refs.Users) != 1 || refs.Users["marie.durand"] == "" {
		t.Errorf("users: %v", refs.Users)
	}
	if len(refs.Issues) != 1 || !refs.Issues[demo] {
		t.Errorf("issues: %v", refs.Issues)
	}
}

func TestAssignable(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	var users []userRef
	paul.call("GET", "/api/projects/DEMO/assignable?q=paul", nil, 200, &users)
	if !slices.ContainsFunc(users, func(u userRef) bool { return u.Username == "paul.martin" }) {
		t.Errorf("assignable in DEMO: %+v", users)
	}
	paul.call("GET", "/api/projects/RH/assignable", nil, 404, nil)
}

func TestLogout(t *testing.T) {
	c := login(t, "hugo.robert", "motdepasse")
	c.call("POST", "/api/logout", nil, 204, nil)
	if status := c.get("/api/me", nil); status != 401 {
		t.Errorf("after signing out: %d", status)
	}
	c.call("POST", "/api/logout", nil, 204, nil)
}

func TestSubscriptionEdit(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	var f filterOut
	paul.call("POST", "/api/filters", map[string]any{"name": unique("Abonnement modifié "), "query": "project = DEMO"}, 201, &f)
	t.Cleanup(func() { paul.do("DELETE", fmt.Sprintf("/api/filters/%d", f.ID), nil) })
	base := fmt.Sprintf("/api/filters/%d/subscriptions", f.ID)
	var s subscriptionOut
	paul.call("POST", base, map[string]any{"schedule": "daily", "hour": 7}, 201, &s)
	path := fmt.Sprintf("%s/%d", base, s.ID)

	paul.call("PUT", path, map[string]any{"schedule": "weekly", "days": []int{1, 5}, "hour": 9}, 200, &s)
	if s.Hour != 9 || !slices.Equal(s.Days, []int{1, 5}) || s.User == nil || s.User.Username != "paul.martin" {
		t.Errorf("after PUT: %+v", s)
	}
	for _, bad := range []map[string]any{{"schedule": "daily", "hour": 25}, {"schedule": "weekly", "hour": 9},
		{"schedule": "hourly", "hour": 9}} {
		paul.call("PUT", path, bad, 400, nil)
	}
	var list []subscriptionOut
	paul.call("GET", base, nil, 200, &list)
	if len(list) != 1 || list[0].Hour != 9 {
		t.Errorf("subscriptions: %+v", list)
	}
	login(t, "chloe.simon", "motdepasse").call("PUT", path, map[string]any{"schedule": "daily", "hour": 7}, 404, nil)
	paul.call("DELETE", path, nil, 204, nil)
	paul.call("PUT", path, map[string]any{"schedule": "daily", "hour": 7}, 404, nil)
}
