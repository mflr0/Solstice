package jirafixture

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"solstice/internal/jiradb"
)

func TestMojibake(t *testing.T) {
	if got := mojibake("é à — ok"); got != "Ã© Ã  â\u0080\u0094 ok" {
		t.Errorf("mojibake = %q", got)
	}
	if mojibake("plain") != "plain" {
		t.Error("ASCII must stay as it is")
	}
}

func TestOddTexts(t *testing.T) {
	texts := oddTexts()
	for _, s := range append(texts, oddSummaries()...) {
		if !utf8.ValidString(s) {
			t.Errorf("not UTF-8: %.40q", s)
		}
		if strings.ContainsRune(s, 0) {
			t.Errorf("NUL in %.40q: the databases refuse it", s)
		}
	}
	for _, s := range oddSummaries() {
		if utf8.RuneCountInString(s) > 255 {
			t.Errorf("summary of %d characters", utf8.RuneCountInString(s))
		}
	}
	mixed := texts[0]
	if !strings.Contains(mixed, "\r\n") || !strings.Contains(strings.ReplaceAll(mixed, "\r\n", ""), "\r") ||
		!strings.Contains(strings.ReplaceAll(mixed, "\r\n", ""), "\n") {
		t.Errorf("line endings are not mixed: %q", mixed)
	}
	if long := texts[2]; strings.ContainsAny(long, "\r\n") || utf8.RuneCountInString(long) < 100_000 {
		t.Error("no very long line")
	}
	if !reflect.DeepEqual(texts, oddTexts()) {
		t.Error("the texts change from one call to the next")
	}
}

func TestQuirkRandomSource(t *testing.T) {
	draw := func(seed uint64, name string) []int {
		g := &generator{opt: Options{Seed: seed}}
		var got []int
		g.quirk(name, func() {
			for range 5 {
				got = append(got, g.rng.IntN(1000))
			}
		})
		if g.rng != nil {
			t.Error("the generator's own source was not put back")
		}
		return got
	}
	if !reflect.DeepEqual(draw(42, "orphans"), draw(42, "orphans")) {
		t.Error("the same seed and quirk must give the same draws")
	}
	if reflect.DeepEqual(draw(42, "orphans"), draw(43, "orphans")) {
		t.Error("another seed must give other draws")
	}
	if reflect.DeepEqual(draw(42, "orphans"), draw(42, "odd-text")) {
		t.Error("another quirk must give other draws")
	}
}

func TestGhostUsersHaveNoUserRow(t *testing.T) {
	for _, ghost := range append([]string{ghostWatcher}, ghostUsers...) {
		for _, u := range internalUsers {
			if strings.EqualFold(ghost, u.name) {
				t.Errorf("%s is a real user", ghost)
			}
		}
		if strings.HasPrefix(ghost, "JIRAUSER") {
			t.Errorf("%s looks like a generated key", ghost)
		}
	}
}

func TestUpdateSQL(t *testing.T) {
	m, err := ParseModel(strings.NewReader(sampleModel))
	if err != nil {
		t.Fatal(err)
	}
	e := m.Entity("jiraissue")
	q, args := updateSQL(jiradb.Postgres, e, 7, []any{"summary", "s", "issuenum", 3})
	if want := `UPDATE "jiraissue" SET "summary" = $1, "issuenum" = $2 WHERE "id" = $3`; q != want {
		t.Errorf("postgres: %s", q)
	}
	if !reflect.DeepEqual(args, []any{"s", 3, int64(7)}) {
		t.Errorf("args = %v", args)
	}
	q, _ = updateSQL(jiradb.MySQL, e, 7, []any{"summary", "s"})
	if want := "UPDATE `jiraissue` SET `SUMMARY` = ? WHERE `ID` = ?"; q != want {
		t.Errorf("mysql: %s", q)
	}
}
