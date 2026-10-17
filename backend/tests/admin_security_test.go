package tests

// The administration of a project's issue security levels: their members
// decide who sees the issues at the level, and the change counts at once.

import (
	"fmt"
	"testing"
	"time"
)

type securityMember struct {
	ID     int64
	Holder string
	User   *struct{ Username string }
	Group  *struct{ Name string }
}

type securityLevelAdmin struct {
	ID          int64
	Name        string
	Description *string
	Issues      int
	MemberCount int
	Members     []securityMember
}

func TestSecurityLevelsAdmin(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")  // developpeurs, sees the project
	chloe := login(t, "chloe.simon", "motdepasse") // not a developer
	key, other := newProject(t, admin, nil), newProject(t, admin, nil)
	base := "/api/projects/" + key + "/security-levels"

	// Project administrators and global administrators only; a project one
	// cannot see does not exist.
	paul.call("GET", base, nil, 403, nil)
	paul.call("POST", base, map[string]any{"name": "Pirate"}, 403, nil)
	chloe.call("GET", base, nil, 404, nil)

	var team, board, foreign securityLevelAdmin
	admin.call("POST", base, map[string]any{"name": "Équipe", "description": "Les développeurs"}, 201, &team)
	admin.call("POST", base, map[string]any{"name": "Direction"}, 201, &board)
	admin.call("POST", "/api/projects/"+other+"/security-levels", map[string]any{"name": "Ailleurs"}, 201, &foreign)
	if team.Description == nil || *team.Description != "Les développeurs" || board.Description != nil ||
		team.Issues != 0 || team.MemberCount != 0 || len(team.Members) != 0 {
		t.Fatalf("created levels: %+v %+v", team, board)
	}
	for _, bad := range []struct {
		in   map[string]any
		want int
	}{
		{map[string]any{"name": "équipe"}, 409},
		{map[string]any{"description": "sans nom"}, 400},
		{map[string]any{"name": "   "}, 400},
	} {
		if status, body := admin.do("POST", base, bad.in); status != bad.want {
			t.Errorf("create %v: %d %s, want %d", bad.in, status, body, bad.want)
		}
	}
	teamPath := fmt.Sprintf("%s/%d", base, team.ID)
	boardPath := fmt.Sprintf("%s/%d", base, board.ID)
	admin.call("PATCH", teamPath, map[string]any{"name": "Direction"}, 409, nil)
	admin.call("PATCH", teamPath, map[string]any{"name": "Développeurs", "description": nil}, 200, &team)
	if team.Name != "Développeurs" || team.Description != nil {
		t.Errorf("after PATCH: %+v", team)
	}
	// Another project's level is not reachable through this one.
	admin.call("PATCH", fmt.Sprintf("%s/%d", base, foreign.ID), map[string]any{"name": "x"}, 404, nil)
	admin.call("POST", fmt.Sprintf("%s/%d/members", base, foreign.ID), map[string]any{"holder": "everyone"}, 404, nil)
	admin.call("DELETE", fmt.Sprintf("%s/%d", base, foreign.ID), nil, 404, nil)

	// An issue at a level without a member is seen by nobody; members count at once.
	issue := createIssue(t, paul, key, "Confidentiel", "")
	if _, err := testDB.Exec(`UPDATE issues SET security_level_id = ? WHERE id = ?`, team.ID, issueID(t, issue)); err != nil {
		t.Fatal(err)
	}
	seen := func(want int) {
		t.Helper()
		if status := paul.get("/api/issues/"+issue, nil); status != want {
			t.Errorf("paul.martin GET %s: %d, want %d", issue, status, want)
		}
	}
	seen(404)
	var level securityLevelAdmin
	admin.call("POST", teamPath+"/members", map[string]any{"holder": "user", "id": userID(t, "paul.martin")}, 201, &level)
	if level.MemberCount != 1 || len(level.Members) != 1 || level.Members[0].User == nil ||
		level.Members[0].User.Username != "paul.martin" || level.Issues != 1 {
		t.Fatalf("after adding a user: %+v", level)
	}
	seen(200)
	admin.call("POST", teamPath+"/members", map[string]any{"holder": "user", "id": userID(t, "paul.martin")}, 409, nil)
	userMember := level.Members[0].ID
	admin.call("DELETE", fmt.Sprintf("%s/members/%d", teamPath, userMember), nil, 204, nil)
	admin.call("DELETE", fmt.Sprintf("%s/members/%d", teamPath, userMember), nil, 404, nil)
	seen(404)

	admin.call("POST", teamPath+"/members", map[string]any{"holder": "group", "id": groupID(t, "developpeurs")}, 201, &level)
	if len(level.Members) != 1 || level.Members[0].Group == nil || level.Members[0].Group.Name != "developpeurs" {
		t.Fatalf("after adding a group: %+v", level)
	}
	seen(200)
	for _, holder := range []string{"everyone", "lead", "reporter", "assignee"} {
		admin.call("POST", teamPath+"/members", map[string]any{"holder": holder}, 201, nil)
	}
	for _, bad := range []map[string]any{
		{"holder": "someone"}, {"holder": "user"}, {"holder": "user", "id": 999999999},
		{"holder": "group", "id": 999999999}, {"holder": "role", "id": 999999999},
	} {
		admin.call("POST", teamPath+"/members", bad, 400, nil)
	}
	var list []securityLevelAdmin
	admin.call("GET", base, nil, 200, &list)
	if len(list) != 2 {
		t.Fatalf("levels: %+v", list)
	}
	for _, l := range list {
		if l.ID == team.ID && (l.Issues != 1 || l.MemberCount != 5 || len(l.Members) != 5) {
			t.Errorf("level %s: %d issues, %d members (%d listed)", l.Name, l.Issues, l.MemberCount, len(l.Members))
		}
	}

	// A level in use goes only with its issues moved or cleared.
	status, raw := admin.do("DELETE", teamPath, nil)
	if status != 409 || num(obj(t, raw), "usage", "issues") != 1 {
		t.Errorf("delete a level in use: %d %s", status, raw)
	}
	admin.call("DELETE", teamPath, map[string]any{"moveTo": foreign.ID}, 400, nil)
	admin.call("DELETE", teamPath, map[string]any{"moveTo": team.ID}, 400, nil)
	admin.call("DELETE", teamPath, map[string]any{"moveTo": board.ID, "clear": true}, 400, nil)
	admin.call("DELETE", teamPath, map[string]any{"moveTo": board.ID}, 204, nil)
	if got := pick(t, `SELECT security_level_id FROM issues WHERE id = ?`, issueID(t, issue)); got != fmt.Sprint(board.ID) {
		t.Errorf("issue moved to level %s, want %d", got, board.ID)
	}
	if got := pick(t, `SELECT COUNT(*) FROM change_items i JOIN change_groups g ON g.id = i.group_id
		WHERE g.issue_id = ? AND i.field = 'security' AND i.new_value = ?`, issueID(t, issue), board.ID); got != "1" {
		t.Errorf("history of the move: %s entries", got)
	}
	seen(404) // Direction has no member

	admin.call("DELETE", boardPath, map[string]any{"clear": true}, 204, nil)
	if got := pick(t, `SELECT COUNT(*) FROM issues WHERE id = ? AND security_level_id IS NULL`, issueID(t, issue)); got != "1" {
		t.Error("the level of the issue was not cleared")
	}
	seen(200)
	admin.call("GET", base, nil, 200, &list)
	if len(list) != 0 {
		t.Errorf("levels left: %+v", list)
	}

	// Every change is in the audit log.
	for action, want := range map[string]int{
		"securitylevel.create":        2,
		"securitylevel.update":        1,
		"securitylevel.member_add":    6,
		"securitylevel.member_remove": 1,
		"securitylevel.delete":        2,
		"securitylevel":               12, // the family
	} {
		var audit struct{ Total int }
		admin.call("GET", "/api/admin/audit?action="+action+"&q="+key, nil, 200, &audit)
		if audit.Total != want {
			t.Errorf("audit entries %s: %d, want %d", action, audit.Total, want)
		}
	}
}

// An issue set to a level between the count of its issues and the delete
// must keep the level: 409, not a foreign key error.
func TestSecurityLevelDeleteRace(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	key := newProject(t, admin, nil)
	var level securityLevelAdmin
	admin.call("POST", "/api/projects/"+key+"/security-levels", map[string]any{"name": "Course"}, 201, &level)
	levelPath := fmt.Sprintf("/api/projects/%s/security-levels/%d", key, level.ID)
	issue := createIssue(t, paul, key, "Course", "")

	// The issue joins the level in a transaction that is still open when the
	// delete starts.
	tx, err := testDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE issues SET security_level_id = ? WHERE id = ?`, level.ID, issueID(t, issue)); err != nil {
		t.Fatal(err)
	}
	type answer struct {
		status int
		body   []byte
	}
	done := make(chan answer)
	go func() {
		status, body := admin.do("DELETE", levelPath, nil)
		done <- answer{status, body}
	}()
	time.Sleep(300 * time.Millisecond)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.status != 409 || num(obj(t, got.body), "usage", "issues") != 1 {
		t.Errorf("delete a level that gained an issue: %d %s", got.status, got.body)
	}
	if n := pick(t, `SELECT COUNT(*) FROM security_levels WHERE id = ?`, level.ID); n != "1" {
		t.Errorf("the level was deleted (%s rows)", n)
	}
}

// Every issue at a deleted level moves with the level, and each keeps its own
// history entry.
func TestSecurityLevelDeleteMovesIssues(t *testing.T) {
	admin := login(t, "admin", "admin")
	paul := login(t, "paul.martin", "motdepasse")
	key := newProject(t, admin, nil)
	base := "/api/projects/" + key + "/security-levels"
	var from, to securityLevelAdmin
	admin.call("POST", base, map[string]any{"name": "Source"}, 201, &from)
	admin.call("POST", base, map[string]any{"name": "Cible"}, 201, &to)
	ids := []string{}
	for _, summary := range []string{"Un", "Deux", "Trois"} {
		id := issueID(t, createIssue(t, paul, key, summary, ""))
		if _, err := testDB.Exec(`UPDATE issues SET security_level_id = ? WHERE id = ?`, from.ID, id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	other := issueID(t, createIssue(t, paul, key, "Ailleurs", ""))
	if _, err := testDB.Exec(`UPDATE issues SET security_level_id = ? WHERE id = ?`, to.ID, other); err != nil {
		t.Fatal(err)
	}
	admin.call("DELETE", fmt.Sprintf("%s/%d", base, from.ID), map[string]any{"moveTo": to.ID}, 204, nil)
	for _, id := range ids {
		if got := pick(t, `SELECT security_level_id FROM issues WHERE id = ?`, id); got != fmt.Sprint(to.ID) {
			t.Errorf("issue %s at level %s, want %d", id, got, to.ID)
		}
		if got := pick(t, `SELECT COUNT(*) FROM change_items i JOIN change_groups g ON g.id = i.group_id
			WHERE g.issue_id = ? AND i.field = 'security' AND i.old_value = ? AND i.new_value = ?`, id, from.ID, to.ID); got != "1" {
			t.Errorf("history of issue %s: %s entries", id, got)
		}
	}
	if got := pick(t, `SELECT COUNT(*) FROM change_groups WHERE issue_id = ?`, other); got != "0" {
		t.Errorf("history of the issue that stayed: %s groups", got)
	}
	var audit struct{ Total int }
	admin.call("GET", "/api/admin/audit?action=securitylevel.delete&q="+key, nil, 200, &audit)
	if audit.Total != 1 {
		t.Errorf("audit entries: %d", audit.Total)
	}
}
