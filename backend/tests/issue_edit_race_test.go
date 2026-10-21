package tests

import (
	"strings"
	"testing"
	"time"
)

func TestEditRefusesWhenTheIssueChangedMeanwhile(t *testing.T) {
	paul := login(t, "paul.martin", "motdepasse")
	key := createIssue(t, paul, "DEMO", unique("Édition concurrente "), "")
	id := issueIDOf(t, key)
	tx, err := testDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SELECT id FROM issues WHERE id = ? FOR UPDATE`, id); err != nil {
		t.Fatal(err)
	}
	// The edit checks without a lock, then waits for this one.
	type answer struct {
		status int
		msg    string
	}
	answers := make(chan answer, 1)
	go func() {
		status, msg, _ := paul.errorIn("en", "PATCH", "/api/issues/"+key, map[string]any{"summary": "Écrasé"})
		answers <- answer{status, msg}
	}()
	time.Sleep(time.Second)
	if _, err := tx.Exec(`UPDATE issues SET summary = 'Changé entre-temps', updated_at = NOW(3) WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if a := <-answers; a.status != 409 || !strings.Contains(a.msg, "changed while it was being saved") {
		t.Errorf("edit of an issue changed meanwhile: %d %q, want 409", a.status, a.msg)
	}
	if got := pick(t, `SELECT summary FROM issues WHERE id = ?`, id); got != "Changé entre-temps" {
		t.Errorf("summary: %q, the concurrent change was overwritten", got)
	}
	if n := dbInt(t, `SELECT COUNT(*) FROM change_items ci JOIN change_groups g ON g.id = ci.group_id
		WHERE ci.field = 'summary' AND g.issue_id = ?`, id); n != 0 {
		t.Errorf("%d summary lines in the history, want none", n)
	}
}
