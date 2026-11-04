package tests

import (
	"fmt"
	"strings"
	"testing"

	"solstice/internal/jirafixture"
)

type boardShare struct {
	Kind  string `json:"kind"`
	ID    int64  `json:"id"`
	Group int64  `json:"group"`
	Name  string `json:"name"`
}

type boardSharing struct {
	Board struct {
		FilterName string `json:"filterName"`
	} `json:"board"`
	Shares []boardShare `json:"shares"`
	Can    struct {
		Manage bool `json:"manage"`
		Share  bool `json:"share"`
	} `json:"can"`
}

func listsBoard(t *testing.T, c *client, board string) bool {
	t.Helper()
	var boards []struct{ ID int64 }
	c.call("GET", "/api/boards", nil, 200, &boards)
	for _, b := range boards {
		if fmt.Sprint(b.ID) == board {
			return true
		}
	}
	return false
}

// shareBoardEnv is a board of a new project, shared with it: jeanne.dupont owns
// it, paul.martin administers the project, lucas.bernard only sees it.
type shareBoardEnv struct {
	admin, owner, projectAdmin, viewer *client
	project, board, filter, path       string
	projectShare                       map[string]any
}

func newShareBoardEnv(t *testing.T) *shareBoardEnv {
	t.Helper()
	e := &shareBoardEnv{
		admin:        login(t, "admin", "admin"),
		owner:        login(t, "jeanne.dupont", jirafixture.Password),
		projectAdmin: login(t, "paul.martin", jirafixture.Password),
		viewer:       login(t, "lucas.bernard", jirafixture.Password),
	}
	e.project = newProject(t, e.admin, nil)
	var perms struct {
		Grants []grantOut `json:"grants"`
	}
	e.admin.call("GET", "/api/projects/"+e.project+"/permissions", nil, 200, &perms)
	grants := append(perms.Grants, grantOut{Permission: "admin", Holder: "user", ID: userID(t, "paul.martin")})
	e.admin.call("PUT", "/api/projects/"+e.project+"/permissions", map[string]any{"grants": grants}, 200, nil)
	e.board, _ = scopeBoard(t, e.admin, e.project)
	if _, err := testDB.Exec(`UPDATE boards SET owner_id = ? WHERE id = ?`, userID(t, "jeanne.dupont"), e.board); err != nil {
		t.Fatal(err)
	}
	e.filter = pick(t, `SELECT CAST(filter_id AS CHAR) FROM boards WHERE id = ?`, e.board)
	e.path = "/api/boards/" + e.board
	var project int64
	if err := testDB.QueryRow(`SELECT project_id FROM board_projects WHERE board_id = ?`, e.board).Scan(&project); err != nil {
		t.Fatal(err)
	}
	e.projectShare = map[string]any{"kind": "project", "id": project}
	return e
}

func (e *shareBoardEnv) shareCount(t *testing.T) string {
	t.Helper()
	return pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM filter_shares WHERE filter_id = ?`, e.filter)
}

func (e *shareBoardEnv) put(c *client, want int, shares ...map[string]any) []byte {
	if shares == nil {
		shares = []map[string]any{}
	}
	return c.call("PUT", e.path+"/shares", map[string]any{"shares": shares}, want, nil)
}

// GET /api/boards/{id} says who sees the board, and who may change that.
func TestBoardDetailSaysWhoSeesIt(t *testing.T) {
	e := newShareBoardEnv(t)
	wantName := pick(t, `SELECT name FROM filters WHERE id = ?`, e.filter)
	for _, c := range []struct {
		name  string
		who   *client
		share bool
	}{{"owner", e.owner, true}, {"project admin", e.projectAdmin, true}, {"admin", e.admin, true}, {"viewer", e.viewer, false}} {
		var got boardSharing
		c.who.call("GET", e.path, nil, 200, &got)
		if got.Can.Share != c.share || got.Can.Manage != c.share {
			t.Errorf("%s: can %+v, want share %v", c.name, got.Can, c.share)
		}
		if got.Board.FilterName != wantName {
			t.Errorf("%s: filter name %q, want %q", c.name, got.Board.FilterName, wantName)
		}
		if len(got.Shares) != 1 || got.Shares[0].Kind != "project" || got.Shares[0].Name != e.project {
			t.Errorf("%s: shares %+v", c.name, got.Shares)
		}
	}
}

// Those who manage a board replace its shares, which decides who sees it. Each
// can undo it even when the new shares leave them out.
func TestBoardSharesReplacedByManagers(t *testing.T) {
	e := newShareBoardEnv(t)
	thomas := login(t, "thomas.moreau", jirafixture.Password)
	thomasShare := map[string]any{"kind": "user", "id": userID(t, "thomas.moreau")}
	// Without the project share, thomas.moreau sees the board only through the user share.
	for _, c := range []struct {
		name string
		who  *client
	}{{"owner", e.owner}, {"project admin", e.projectAdmin}, {"admin", e.admin}} {
		t.Run(c.name, func(t *testing.T) {
			var saved struct {
				Shares []boardShare `json:"shares"`
			}
			c.who.call("PUT", e.path+"/shares", map[string]any{"shares": []map[string]any{thomasShare}}, 200, &saved)
			if len(saved.Shares) != 1 || saved.Shares[0].Kind != "user" || saved.Shares[0].Name == "" {
				t.Errorf("answer: %+v", saved.Shares)
			}
			if !listsBoard(t, thomas, e.board) {
				t.Error("the user now included doesn't see the board")
			}
			thomas.call("GET", e.path, nil, 200, nil)
			if listsBoard(t, e.viewer, e.board) {
				t.Error("the user no longer included still lists the board")
			}
			e.viewer.call("GET", e.path, nil, 404, nil)
			e.viewer.call("PUT", e.path+"/shares", map[string]any{"shares": []map[string]any{e.projectShare}}, 404, nil)

			e.put(c.who, 200, e.projectShare)
			e.viewer.call("GET", e.path, nil, 200, nil)
		})
	}
}

// A user who sees the board but doesn't manage it may not change who sees it.
func TestBoardSharesRefusedToOthers(t *testing.T) {
	e := newShareBoardEnv(t)
	e.put(e.viewer, 403, map[string]any{"kind": "everyone"})
	if n := e.shareCount(t); n != "1" {
		t.Errorf("%s shares after a refused change", n)
	}
	e.admin.call("PUT", "/api/boards/999999999/shares", map[string]any{"shares": []map[string]any{}}, 404, nil)
}

// Only the shares a manager adds are validated: one already there can stay,
// even with a group they aren't in.
func TestBoardSharesValidateWhatIsAdded(t *testing.T) {
	e := newShareBoardEnv(t)
	kept, other := newGroup(t, e.admin, "partage "), newGroup(t, e.admin, "partage ")
	keptShare := map[string]any{"kind": "group", "id": kept.ID}

	e.put(e.projectAdmin, 400, keptShare)
	if n := e.shareCount(t); n != "1" {
		t.Errorf("%s shares after an invalid change", n)
	}

	e.put(e.admin, 200, e.projectShare, keptShare)
	e.put(e.projectAdmin, 200, keptShare)
	e.put(e.projectAdmin, 400, keptShare, map[string]any{"kind": "group", "id": other.ID})
	if n := e.shareCount(t); n != "1" {
		t.Errorf("%s shares after an invalid change", n)
	}

	// Asking for a project group on top of a kept project share is a new share.
	e.put(e.admin, 200, e.projectShare)
	e.put(e.projectAdmin, 400, map[string]any{"kind": "project", "id": e.projectShare["id"], "group": kept.ID})
}

// A global administrator may share with any group, but it has to exist.
func TestBoardSharesUnknownGroup(t *testing.T) {
	e := newShareBoardEnv(t)
	e.put(e.admin, 400, map[string]any{"kind": "group", "id": 999999999})
	e.put(e.admin, 400, map[string]any{"kind": "project", "id": e.projectShare["id"], "group": 999999999})
	if n := e.shareCount(t); n != "1" {
		t.Errorf("%s shares after an invalid change", n)
	}
}

// A body without shares changes nothing: it must not read as "no one".
func TestBoardSharesNeedTheList(t *testing.T) {
	e := newShareBoardEnv(t)
	for _, body := range []map[string]any{{}, {"shares": nil}} {
		e.admin.call("PUT", e.path+"/shares", body, 400, nil)
	}
	if n := e.shareCount(t); n != "1" {
		t.Errorf("%s shares after a body without shares", n)
	}
	e.put(e.admin, 200)
	if n := e.shareCount(t); n != "0" {
		t.Errorf("%s shares after an empty list", n)
	}
}

// Changing a board's shares records the same audit entry as changing its filter's.
func TestBoardSharesAudit(t *testing.T) {
	e := newShareBoardEnv(t)
	entries := func() string {
		return pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM audit_log WHERE action = 'filter.share' AND target_id = ?`, e.filter)
	}
	e.put(e.owner, 200, e.projectShare)
	if n := entries(); n != "0" {
		t.Errorf("%s entries for a change that changes nothing", n)
	}
	e.put(e.owner, 200, map[string]any{"kind": "everyone"})
	if n := entries(); n != "1" {
		t.Fatalf("%s entries, want 1", n)
	}
	details := pick(t, `SELECT COALESCE(details, '') FROM audit_log WHERE action = 'filter.share' AND target_id = ?`, e.filter)
	if !strings.Contains(details, e.project) || !strings.Contains(details, "everyone") {
		t.Errorf("details: %s", details)
	}
}
