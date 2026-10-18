package tests

import (
	"fmt"
	"strconv"
	"testing"
)

// scopeSprint makes an upcoming sprint on a board; it is removed with the test.
func scopeSprint(t *testing.T, c *client, board string) int64 {
	t.Helper()
	var sp struct{ ID int64 }
	c.call("POST", "/api/boards/"+board+"/sprints", map[string]any{"name": unique("Portée ")}, 201, &sp)
	t.Cleanup(func() { testDB.Exec(`DELETE FROM sprints WHERE id = ?`, sp.ID) })
	return sp.ID
}

// scopeBoard makes a Scrum board on one project, with an upcoming sprint; both
// are removed with the test.
func scopeBoard(t *testing.T, c *client, project string) (board string, sprint int64) {
	t.Helper()
	var created struct{ ID int64 }
	c.call("POST", "/api/boards", map[string]any{"name": unique("Portée " + project + " "), "kind": "scrum", "project": project},
		201, &created)
	board = strconv.FormatInt(created.ID, 10)
	filter := pick(t, `SELECT CAST(filter_id AS CHAR) FROM boards WHERE id = ?`, board)
	t.Cleanup(func() {
		testDB.Exec(`DELETE FROM sprints WHERE board_id = ?`, board)
		testDB.Exec(`DELETE FROM boards WHERE id = ?`, board)
		testDB.Exec(`DELETE FROM filters WHERE id = ?`, filter)
	})
	return board, scopeSprint(t, c, board)
}

func sprintMembers(t *testing.T, sprint int64, key string) string {
	t.Helper()
	return pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM sprint_issues si JOIN issues i ON i.id = si.issue_id
		JOIN projects p ON p.id = i.project_id WHERE si.sprint_id = ? AND `+keyExpr+` = ?`, sprint, key)
}

// An issue only enters a sprint of a board that includes its project.
func TestSprintMoveStaysWithinBoardProjects(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	paul := login(t, "paul.martin", "motdepasse")
	webBoard, webSprint := scopeBoard(t, marie, "WEB")
	demoBoard := boardID(t, "DEMO — Scrum")
	demoSprint := scopeSprint(t, marie, demoBoard)
	foreign := newTestIssue(t, paul, "Hors du tableau", nil)

	// A DEMO issue into a sprint of a board without DEMO: refused, on the
	// board and on the REST route.
	status, msg, _ := marie.errorIn("en", "POST", "/api/boards/"+webBoard+"/move", map[string]any{"issue": foreign, "sprint": webSprint})
	if status != 400 || msg != "The issue's project is not on this sprint's board." {
		t.Errorf("board move: %d %q", status, msg)
	}
	restPath := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", webSprint)
	if status, _, body := call(t, marie, testURL, "POST", restPath, map[string]any{"issues": []string{foreign}},
		header("X-Atlassian-Token", "no-check")); status != 400 {
		t.Errorf("REST move: %d %s", status, body)
	}
	if n := sprintMembers(t, webSprint, foreign); n != "0" {
		t.Errorf("%s entered the sprint of another project's board", foreign)
	}

	// The normal moves still work: into a sprint of the issue's own board and
	// back to the backlog, on the board and on the REST route.
	own := createIssue(t, marie, "WEB", "Dans le tableau", "")
	if status, body := marie.do("POST", "/api/boards/"+webBoard+"/move", map[string]any{"issue": own, "sprint": webSprint}); status != 204 {
		t.Fatalf("board move into the sprint: %d %s", status, body)
	}
	if n := sprintMembers(t, webSprint, own); n != "1" {
		t.Errorf("%s is not in the sprint", own)
	}
	if status, body := marie.do("POST", "/api/boards/"+webBoard+"/move", map[string]any{"issue": own, "sprint": 0}); status != 204 {
		t.Fatalf("board move to the backlog: %d %s", status, body)
	}
	if n := sprintMembers(t, webSprint, own); n != "0" {
		t.Errorf("%s is still in the sprint", own)
	}

	restPath = fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", demoSprint)
	if status, _, body := call(t, marie, testURL, "POST", restPath, map[string]any{"issues": []string{foreign}},
		header("X-Atlassian-Token", "no-check")); status != 204 {
		t.Fatalf("REST move into the sprint: %d %s", status, body)
	}
	if n := sprintMembers(t, demoSprint, foreign); n != "1" {
		t.Errorf("%s is not in the sprint", foreign)
	}
	if status, _, body := call(t, marie, testURL, "POST", "/rest/agile/1.0/backlog/issue", map[string]any{"issues": []string{foreign}},
		header("X-Atlassian-Token", "no-check")); status != 204 {
		t.Fatalf("REST move to the backlog: %d %s", status, body)
	}
	if n := sprintMembers(t, demoSprint, foreign); n != "0" {
		t.Errorf("%s is still in the sprint", foreign)
	}
}

// Moving an issue to the backlog leaves the open sprints of boards the user
// does not see as they are.
func TestBacklogMoveKeepsSprintsOfHiddenBoards(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	paul := login(t, "paul.martin", "motdepasse")
	// Nobody can create a board on RH (only the rh group browses it, and no
	// one there administers it), so marie makes a WEB board and keeps its
	// filter to herself: paul, who edits issues of DEMO and WEB, cannot see it.
	hiddenBoard, hiddenSprint := scopeBoard(t, marie, "WEB")
	filter := pick(t, `SELECT CAST(filter_id AS CHAR) FROM boards WHERE id = ?`, hiddenBoard)
	marie.call("PATCH", "/api/filters/"+filter, map[string]any{"shares": []map[string]any{}}, 200, nil)
	if status := paul.get("/api/boards/"+hiddenBoard, nil); status != 404 {
		t.Fatalf("paul.martin sees the private board: %d", status)
	}
	demoSprint := scopeSprint(t, marie, boardID(t, "DEMO — Scrum"))
	toBacklog := func(key string) {
		t.Helper()
		if status, _, body := call(t, paul, testURL, "POST", "/rest/agile/1.0/backlog/issue", map[string]any{"issues": []string{key}},
			header("X-Atlassian-Token", "no-check")); status != 204 {
			t.Fatalf("move %s to the backlog: %d %s", key, status, body)
		}
	}
	hide := func(key string) {
		t.Helper()
		id := pick(t, `SELECT CAST(i.id AS CHAR) FROM issues i JOIN projects p ON p.id = i.project_id WHERE `+keyExpr+` = ?`, key)
		if _, err := testDB.Exec(`INSERT INTO sprint_issues (sprint_id, issue_id) VALUES (?, ?)`, hiddenSprint, id); err != nil {
			t.Fatal(err)
		}
	}

	// Only in a sprint of the hidden board: nothing happens, nothing is recorded.
	alone := newTestIssue(t, paul, "Sprint caché seul", nil)
	hide(alone)
	toBacklog(alone)
	if n := sprintMembers(t, hiddenSprint, alone); n != "1" {
		t.Errorf("%s left the sprint of the hidden board", alone)
	}
	if h := historyFields(t, paul, alone); h["Sprint"] != 0 {
		t.Errorf("history of the untouched issue: %v", h)
	}

	// In a sprint of a visible board too: only that one is left.
	both := newTestIssue(t, paul, "Sprint caché et visible", nil)
	if status, body := marie.do("POST", "/api/boards/"+boardID(t, "DEMO — Scrum")+"/move",
		map[string]any{"issue": both, "sprint": demoSprint}); status != 204 {
		t.Fatalf("plan the issue: %d %s", status, body)
	}
	hide(both)
	toBacklog(both)
	if n := sprintMembers(t, demoSprint, both); n != "0" {
		t.Errorf("%s is still in the sprint it sees", both)
	}
	if n := sprintMembers(t, hiddenSprint, both); n != "1" {
		t.Errorf("%s left the sprint of the hidden board", both)
	}

	// The same drag to the backlog on a board paul sees.
	dragged := newTestIssue(t, paul, "Sprint caché et glissé", nil)
	if status, body := marie.do("POST", "/api/boards/"+boardID(t, "DEMO — Scrum")+"/move",
		map[string]any{"issue": dragged, "sprint": demoSprint}); status != 204 {
		t.Fatalf("plan the issue: %d %s", status, body)
	}
	hide(dragged)
	if status, body := paul.do("POST", "/api/boards/"+boardID(t, "DEMO — Scrum")+"/move",
		map[string]any{"issue": dragged, "sprint": 0}); status != 204 {
		t.Fatalf("board move to the backlog: %d %s", status, body)
	}
	if n := sprintMembers(t, demoSprint, dragged); n != "0" {
		t.Errorf("%s is still in the sprint it sees", dragged)
	}
	if n := sprintMembers(t, hiddenSprint, dragged); n != "1" {
		t.Errorf("%s left the sprint of the hidden board", dragged)
	}
}

// A board lists what its filter says today: after the filter is edited to
// include a second project, that project's issues can enter the board's sprints.
func TestSprintMoveFollowsTheBoardFilter(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	paul := login(t, "paul.martin", "motdepasse")
	board, sprint := scopeBoard(t, marie, "WEB")
	filter := pick(t, `SELECT CAST(filter_id AS CHAR) FROM boards WHERE id = ?`, board)
	foreign := newTestIssue(t, paul, "Ajouté par le filtre", nil)
	move := map[string]any{"issue": foreign, "sprint": sprint}

	if status, _ := marie.do("POST", "/api/boards/"+board+"/move", move); status != 400 {
		t.Fatalf("move before the filter changes: %d", status)
	}
	marie.call("PATCH", "/api/filters/"+filter, map[string]any{"query": `project in ("WEB", "DEMO") ORDER BY Rank ASC`}, 200, nil)
	if status, body := marie.do("POST", "/api/boards/"+board+"/move", move); status != 204 {
		t.Fatalf("move after the filter includes DEMO: %d %s", status, body)
	}
	if n := sprintMembers(t, sprint, foreign); n != "1" {
		t.Errorf("%s is not in the sprint", foreign)
	}

	// And the other way: a filter that no longer covers WEB refuses its issues.
	own := createIssue(t, marie, "WEB", "Sorti par le filtre", "")
	marie.call("PATCH", "/api/filters/"+filter, map[string]any{"query": `project = "DEMO" ORDER BY Rank ASC`}, 200, nil)
	if status, _ := marie.do("POST", "/api/boards/"+board+"/move", map[string]any{"issue": own, "sprint": sprint}); status != 400 {
		t.Errorf("move of an issue the filter dropped: %d", status)
	}
}

func TestSprintMoveWithoutChangeIgnoresTheFilter(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	board, sprint := scopeBoard(t, marie, "WEB")
	filter := pick(t, `SELECT CAST(filter_id AS CHAR) FROM boards WHERE id = ?`, board)
	key := createIssue(t, marie, "WEB", "Déjà dans le sprint", "")
	move := map[string]any{"issue": key, "sprint": sprint}

	marie.call("POST", "/api/boards/"+board+"/move", move, 204, nil)
	marie.call("PATCH", "/api/filters/"+filter, map[string]any{"query": `project = "DEMO" ORDER BY Rank ASC`}, 200, nil)
	if status, body := marie.do("POST", "/api/boards/"+board+"/move", move); status != 204 {
		t.Errorf("move into the sprint the issue is already in: %d %s", status, body)
	}
}

// A REST move of several issues is refused as a whole: none moves when one of
// them is not on the sprint's board.
func TestSprintMoveOfSeveralIssuesIsAllOrNothing(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	paul := login(t, "paul.martin", "motdepasse")
	_, sprint := scopeBoard(t, marie, "WEB")
	first := createIssue(t, marie, "WEB", "Premier du lot", "")
	foreign := newTestIssue(t, paul, "Dernier du lot", nil)
	second := createIssue(t, marie, "WEB", "Deuxième du lot", "")

	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", sprint)
	if status, _, body := call(t, marie, testURL, "POST", path, map[string]any{"issues": []string{first, foreign, second}},
		header("X-Atlassian-Token", "no-check")); status != 400 {
		t.Fatalf("move of a batch with a foreign issue: %d %s", status, body)
	}
	for _, key := range []string{first, second, foreign} {
		if n := sprintMembers(t, sprint, key); n != "0" {
			t.Errorf("%s moved although the batch was refused", key)
		}
	}
	if status, _, body := call(t, marie, testURL, "POST", path, map[string]any{"issues": []string{first, second}},
		header("X-Atlassian-Token", "no-check")); status != 204 {
		t.Fatalf("move of a batch on the board: %d %s", status, body)
	}
	for _, key := range []string{first, second} {
		if n := sprintMembers(t, sprint, key); n != "1" {
			t.Errorf("%s is not in the sprint", key)
		}
	}
}

func TestSprintMoveOfSeveralIssuesSkipsThoseAlreadyInTheSprint(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	paul := login(t, "paul.martin", "motdepasse")
	board, sprint := scopeBoard(t, marie, "WEB")
	filter := pick(t, `SELECT CAST(filter_id AS CHAR) FROM boards WHERE id = ?`, board)
	inside := createIssue(t, marie, "WEB", "Déjà dans le lot", "")
	foreign := newTestIssue(t, paul, "Nouveau dans le lot", nil)

	marie.call("POST", "/api/boards/"+board+"/move", map[string]any{"issue": inside, "sprint": sprint}, 204, nil)
	marie.call("PATCH", "/api/filters/"+filter, map[string]any{"query": `project = "DEMO" ORDER BY Rank ASC`}, 200, nil)
	path := fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", sprint)
	if status, _, body := call(t, marie, testURL, "POST", path, map[string]any{"issues": []string{inside, foreign}},
		header("X-Atlassian-Token", "no-check")); status != 204 {
		t.Fatalf("move of a batch holding an issue the filter dropped: %d %s", status, body)
	}
	for _, key := range []string{inside, foreign} {
		if n := sprintMembers(t, sprint, key); n != "1" {
			t.Errorf("%s is not in the sprint", key)
		}
	}
}
