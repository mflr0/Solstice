package tests

// What a board's settings and a card move write: columns removed, statuses
// moved between columns, quick filters and swimlanes updated by id, unknown
// ids, and moves that send a column and a rank in one request.

import (
	"fmt"
	"slices"
	"testing"
)

type boardSettings struct {
	Columns []struct {
		ID       int64       `json:"id"`
		Name     string      `json:"name"`
		Min      *int        `json:"min"`
		Max      *int        `json:"max"`
		Statuses []statusRef `json:"statuses"`
	} `json:"columns"`
	QuickFilters []boardQuery `json:"quickFilters"`
	Swimlanes    []boardQuery `json:"swimlanes"`
}

type boardQuery struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Query       *string `json:"query"`
	Description *string `json:"description"`
	Default     bool    `json:"default"`
}

// ownBoard creates a kanban board on OPS for one test and deletes it after.
func ownBoard(t *testing.T, c *client) string {
	t.Helper()
	var created struct{ ID int64 }
	c.call("POST", "/api/boards", map[string]any{"kind": "kanban", "project": "OPS"}, 201, &created)
	path := fmt.Sprintf("/api/boards/%d", created.ID)
	t.Cleanup(func() { c.do("DELETE", path, nil) })
	return path
}

func readBoard(t *testing.T, c *client, path string) boardSettings {
	t.Helper()
	var b boardSettings
	c.call("GET", path, nil, 200, &b)
	return b
}

func statusIDs(refs []statusRef) []string {
	ids := []string{}
	for _, s := range refs {
		ids = append(ids, s.ID)
	}
	return ids
}

func rowCount(t *testing.T, query string, args ...any) string {
	t.Helper()
	return pick(t, `SELECT CAST(COUNT(*) AS CHAR) FROM `+query, args...)
}

func TestBoardColumnRemovalAndStatusMove(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	path := ownBoard(t, marie)
	b := readBoard(t, marie, path)
	all := []string{}
	for _, c := range b.Columns {
		all = append(all, statusIDs(c.Statuses)...)
	}
	if len(b.Columns) != 3 || len(all) < 4 {
		t.Fatalf("new board columns: %+v", b.Columns)
	}
	// Start from a known layout: two statuses in the first column.
	start := []map[string]any{
		{"id": b.Columns[0].ID, "name": b.Columns[0].Name, "statuses": all[:2]},
		{"id": b.Columns[1].ID, "name": b.Columns[1].Name, "statuses": all[2:3]},
		{"id": b.Columns[2].ID, "name": b.Columns[2].Name, "statuses": all[3:]},
	}
	marie.call("PUT", path, map[string]any{"columns": start}, 200, nil)
	b = readBoard(t, marie, path)
	first, second, last := b.Columns[0], b.Columns[1], b.Columns[2]
	moved := first.Statuses[0].ID
	kept := statusIDs(first.Statuses[1:])
	marie.call("PUT", path, map[string]any{"columns": []map[string]any{
		{"name": "Nouvelle", "statuses": []string{}},
		{"id": first.ID, "name": first.Name, "statuses": kept},
		{"id": second.ID, "name": "Deux", "min": 1, "max": 2, "statuses": append(statusIDs(second.Statuses), moved)},
	}}, 200, nil)

	b = readBoard(t, marie, path)
	if len(b.Columns) != 3 {
		t.Fatalf("columns after update: %+v", b.Columns)
	}
	added, got1, got2 := b.Columns[0], b.Columns[1], b.Columns[2]
	if added.Name != "Nouvelle" || added.ID == first.ID || added.ID == second.ID || added.ID == last.ID {
		t.Errorf("new column, given first: %+v", added)
	}
	if got1.ID != first.ID || !slices.Equal(statusIDs(got1.Statuses), kept) {
		t.Errorf("column %d: %+v, want statuses %v", first.ID, got1, kept)
	}
	want2 := append(statusIDs(second.Statuses), moved)
	if got2.ID != second.ID || got2.Name != "Deux" || !slices.Equal(statusIDs(got2.Statuses), want2) {
		t.Errorf("column %d: %+v, want Deux with %v", second.ID, got2, want2)
	}
	if got2.Min == nil || *got2.Min != 1 || got2.Max == nil || *got2.Max != 2 {
		t.Errorf("limits of Deux: %v %v", got2.Min, got2.Max)
	}
	if n := rowCount(t, `board_column_statuses WHERE status_id = ? AND column_id IN (?, ?)`, moved, first.ID, second.ID); n != "1" {
		t.Errorf("status %s is in %s columns, want 1", moved, n)
	}
	if n := rowCount(t, `board_columns WHERE id = ?`, last.ID); n != "0" {
		t.Errorf("the removed column is still there")
	}
	if n := rowCount(t, `board_column_statuses WHERE column_id = ?`, last.ID); n != "0" {
		t.Errorf("the removed column keeps %s statuses", n)
	}

	// Limits given and then left out are cleared.
	marie.call("PUT", path, map[string]any{"columns": []map[string]any{
		{"id": added.ID, "name": "Nouvelle", "statuses": []string{}},
		{"id": first.ID, "name": first.Name, "statuses": kept},
		{"id": second.ID, "name": "Deux", "statuses": want2},
	}}, 200, nil)
	if c := readBoard(t, marie, path).Columns[2]; c.Min != nil || c.Max != nil {
		t.Errorf("limits after leaving them out: %v %v", c.Min, c.Max)
	}
}

func TestBoardColumnUnknownID(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	path := ownBoard(t, marie)
	before := readBoard(t, marie, path)
	var other boardDetail
	marie.call("GET", "/api/boards/"+boardID(t, "WEB — Kanban"), nil, 200, &other)

	for name, id := range map[string]int64{"unknown": 999999999, "another board's": other.Columns[0].ID} {
		columns := []map[string]any{{"id": id, "name": "Ailleurs", "statuses": []string{}}}
		for _, c := range before.Columns {
			columns = append(columns, map[string]any{"id": c.ID, "name": c.Name, "statuses": statusIDs(c.Statuses)})
		}
		status, msg, _ := marie.errorIn("en", "PUT", path, map[string]any{"columns": columns})
		if status != 400 || msg != "Unknown column on this board." {
			t.Errorf("%s column id: %d %q, want 400 BoardColumnUnknown", name, status, msg)
		}
	}
	after := readBoard(t, marie, path)
	if len(after.Columns) != len(before.Columns) {
		t.Fatalf("a refused update changed the columns: %+v", after.Columns)
	}
	for i, c := range after.Columns {
		if c.ID != before.Columns[i].ID || !slices.Equal(statusIDs(c.Statuses), statusIDs(before.Columns[i].Statuses)) {
			t.Errorf("column %d after a refused update: %+v, was %+v", i, c, before.Columns[i])
		}
	}
	var otherAfter boardDetail
	marie.call("GET", "/api/boards/"+boardID(t, "WEB — Kanban"), nil, 200, &otherAfter)
	if c, was := otherAfter.Columns[0], other.Columns[0]; c.Name != was.Name || len(c.Statuses) != len(was.Statuses) {
		t.Errorf("another board's column changed: %+v, was %+v", c, was)
	}
}

func TestBoardQuickFilterUpdateByID(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	path := ownBoard(t, marie)
	marie.call("PUT", path, map[string]any{"quickFilters": []map[string]any{
		{"name": "Les miens", "query": "assignee = currentUser()", "description": "À moi"},
		{"name": "Bugs", "query": "type = Bug"},
	}}, 200, nil)
	b := readBoard(t, marie, path)
	if len(b.QuickFilters) != 2 {
		t.Fatalf("quick filters: %+v", b.QuickFilters)
	}
	mine, bugs := b.QuickFilters[0], b.QuickFilters[1]

	// The first is kept by id and changed, the second dropped, a new one added before it.
	marie.call("PUT", path, map[string]any{"quickFilters": []map[string]any{
		{"name": "Récents", "query": "created >= -1w"},
		{"id": mine.ID, "name": "Les miens, ouverts", "query": "assignee = currentUser() AND resolution is EMPTY", "description": " "},
	}}, 200, nil)
	b = readBoard(t, marie, path)
	if len(b.QuickFilters) != 2 {
		t.Fatalf("quick filters after update: %+v", b.QuickFilters)
	}
	recent, got := b.QuickFilters[0], b.QuickFilters[1]
	if recent.Name != "Récents" || recent.ID == mine.ID || recent.ID == bugs.ID {
		t.Errorf("new quick filter: %+v", recent)
	}
	if got.ID != mine.ID || got.Name != "Les miens, ouverts" || got.Query == nil ||
		*got.Query != "assignee = currentUser() AND resolution is EMPTY" || got.Description != nil {
		t.Errorf("quick filter %d after update: %+v", mine.ID, got)
	}
	if n := rowCount(t, `board_quick_filters WHERE id = ?`, bugs.ID); n != "0" {
		t.Errorf("the dropped quick filter is still there")
	}

	// Ids the board does not own are ignored: neither created nor touched elsewhere.
	other := ownBoard(t, marie)
	for name, id := range map[string]int64{"unknown": 999999999, "another board's": recent.ID} {
		marie.call("PUT", other, map[string]any{"quickFilters": []map[string]any{
			{"id": id, "name": "Volé", "query": "type = Bug"},
		}}, 200, nil)
		if q := readBoard(t, marie, other).QuickFilters; len(q) != 0 {
			t.Errorf("%s quick filter id: the board now has %+v", name, q)
		}
	}
	if q := readBoard(t, marie, path).QuickFilters; len(q) != 2 || q[0].Name != "Récents" {
		t.Errorf("the first board's quick filters changed: %+v", q)
	}
}

func TestBoardLaneDefault(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	path := ownBoard(t, marie)
	createIssue(t, marie, "OPS", "Une carte pour les couloirs", "")
	marie.call("PUT", path, map[string]any{"swimlanes": "custom", "lanes": []map[string]any{
		{"name": "OPS", "query": "project = OPS"},
		{"name": "Reste", "default": true},
	}}, 200, nil)
	b := readBoard(t, marie, path)
	if len(b.Swimlanes) != 2 || b.Swimlanes[0].Default || !b.Swimlanes[1].Default {
		t.Fatalf("lanes: %+v", b.Swimlanes)
	}
	ops, rest := b.Swimlanes[0], b.Swimlanes[1]
	lanesOfCards := func() map[int64]int {
		t.Helper()
		var cards struct{ Cards []boardCard }
		marie.call("GET", path+"/cards", nil, 200, &cards)
		if len(cards.Cards) == 0 {
			t.Fatal("no card on the board")
		}
		out := map[int64]int{}
		for _, c := range cards.Cards {
			if c.Lane == nil {
				t.Fatalf("%s has no lane", c.Key)
			}
			out[*c.Lane]++
		}
		return out
	}
	if got := lanesOfCards(); len(got) != 1 || got[ops.ID] == 0 {
		t.Errorf("cards by lane: %v, want all in %d", got, ops.ID)
	}

	// The default moves by id: the default lane's query is not read, so every
	// card falls to the other lane's query.
	marie.call("PUT", path, map[string]any{"lanes": []map[string]any{
		{"id": ops.ID, "name": "OPS", "query": "project = OPS", "default": true},
		{"id": rest.ID, "name": "Reste", "query": "project = OPS"},
	}}, 200, nil)
	b = readBoard(t, marie, path)
	if len(b.Swimlanes) != 2 || b.Swimlanes[0].ID != ops.ID || !b.Swimlanes[0].Default ||
		b.Swimlanes[1].ID != rest.ID || b.Swimlanes[1].Default {
		t.Fatalf("lanes after moving the default: %+v", b.Swimlanes)
	}
	if got := lanesOfCards(); len(got) != 1 || got[rest.ID] == 0 {
		t.Errorf("cards by lane: %v, want all in %d", got, rest.ID)
	}

	// An unknown lane id is ignored; the lanes not sent are removed.
	marie.call("PUT", path, map[string]any{"lanes": []map[string]any{
		{"id": rest.ID, "name": "Reste", "default": true},
		{"id": 999999999, "name": "Fantôme"},
	}}, 200, nil)
	if l := readBoard(t, marie, path).Swimlanes; len(l) != 1 || l[0].ID != rest.ID || !l[0].Default {
		t.Errorf("lanes after an unknown id: %+v", l)
	}
}

// rankOf reads an issue's rank key.
func rankOf(t *testing.T, key string) string {
	t.Helper()
	return pick(t, `SELECT COALESCE(i.rank_key, '') FROM issues i JOIN projects p ON p.id = i.project_id WHERE `+keyExpr+` = ?`, key)
}

func statusOf(t *testing.T, key string) string {
	t.Helper()
	return pick(t, `SELECT i.status_id FROM issues i JOIN projects p ON p.id = i.project_id WHERE `+keyExpr+` = ?`, key)
}

func statusChanges(t *testing.T, key string) string {
	t.Helper()
	return rowCount(t, `change_items ci JOIN change_groups g ON g.id = ci.group_id
		WHERE ci.field = 'status' AND g.issue_id = ?`, issueID(t, key))
}

func TestBoardMoveColumnAndRank(t *testing.T) {
	marie := login(t, "marie.durand", "motdepasse")
	path := ownBoard(t, marie)
	keys := []string{}
	for i := range 5 {
		keys = append(keys, createIssue(t, marie, "OPS", fmt.Sprintf("Carte %d", i), ""))
	}
	above, below, moving, refused, unknown := keys[0], keys[1], keys[2], keys[3], keys[4]

	// Columns built from the workflow as it is: the start status, one it
	// reaches, one it does not.
	var d struct {
		Status      statusRef
		Transitions []struct{ To statusRef }
	}
	marie.call("GET", "/api/issues/"+moving, nil, 200, &d)
	start := d.Status.ID
	if len(d.Transitions) == 0 {
		t.Fatalf("%s has no transition", moving)
	}
	reached := d.Transitions[0].To.ID
	unreached := pick(t, `SELECT id FROM statuses WHERE id NOT IN (?, ?) ORDER BY id LIMIT 1`, start, reached)
	for _, tr := range d.Transitions {
		if tr.To.ID == unreached {
			t.Skipf("every status is reachable from %s", start)
		}
	}
	marie.call("PUT", path, map[string]any{"columns": []map[string]any{
		{"name": "Départ", "statuses": []string{start}},
		{"name": "Atteint", "statuses": []string{reached}},
		{"name": "Hors d'atteinte", "statuses": []string{unreached}},
	}}, 200, nil)
	b := readBoard(t, marie, path)
	reachedCol, unreachedCol := b.Columns[1].ID, b.Columns[2].ID
	move := func(body map[string]any) (int, string) {
		t.Helper()
		status, msg, _ := marie.errorIn("en", "POST", path+"/move", body)
		return status, msg
	}

	// Make the order of the two anchors certain.
	if status, msg := move(map[string]any{"issue": below, "after": above}); status != 204 {
		t.Fatalf("rank %s after %s: %d %s", below, above, status, msg)
	}
	lo, hi := rankOf(t, above), rankOf(t, below)
	if lo == "" || lo >= hi {
		t.Fatalf("anchors out of order: %q %q", lo, hi)
	}

	// A column and a rank in one request: both apply.
	if status, msg := move(map[string]any{"issue": moving, "column": reachedCol, "after": above, "before": below}); status != 204 {
		t.Fatalf("move with column and rank: %d %s", status, msg)
	}
	if got := statusOf(t, moving); got != reached {
		t.Errorf("%s is in status %s, want %s", moving, got, reached)
	}
	if r := rankOf(t, moving); r <= lo || r >= hi {
		t.Errorf("%s ranked %q, want between %q and %q", moving, r, lo, hi)
	}
	if n := statusChanges(t, moving); n != "1" {
		t.Errorf("%s has %s status history lines, want 1", moving, n)
	}

	// The column it is already in: no transition, the rank still applies.
	if status, msg := move(map[string]any{"issue": moving, "column": reachedCol, "before": above}); status != 204 {
		t.Fatalf("move within its column: %d %s", status, msg)
	}
	if got, n := statusOf(t, moving), statusChanges(t, moving); got != reached || n != "1" {
		t.Errorf("%s after a move within its column: status %s, %s history lines", moving, got, n)
	}
	if r := rankOf(t, moving); r >= lo {
		t.Errorf("%s ranked %q, want before %q", moving, r, lo)
	}

	// A column the workflow does not reach, or one the board does not have:
	// refused before the rank is written.
	for _, c := range []struct {
		key    string
		column int64
		status int
		msg    string
	}{
		{refused, unreachedCol, 409, ""},
		{unknown, 999999999, 400, "Unknown column on this board."},
	} {
		rank := rankOf(t, c.key)
		status, msg := move(map[string]any{"issue": c.key, "column": c.column, "after": above, "before": below})
		if status != c.status || (c.msg != "" && msg != c.msg) {
			t.Errorf("move %s to column %d: %d %q, want %d %q", c.key, c.column, status, msg, c.status, c.msg)
		}
		if got := statusOf(t, c.key); got != start {
			t.Errorf("%s moved to status %s by a refused move", c.key, got)
		}
		if r := rankOf(t, c.key); r != rank {
			t.Errorf("%s ranked %q by a refused move, was %q", c.key, r, rank)
		}
	}
}
