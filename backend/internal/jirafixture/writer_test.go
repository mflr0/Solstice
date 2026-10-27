package jirafixture

import (
	"reflect"
	"strings"
	"testing"

	"solstice/internal/jiradb"
)

func TestTableLeavesOutNewerColumns(t *testing.T) {
	m, err := ParseModel(strings.NewReader(sampleModel))
	if err != nil {
		t.Fatal(err)
	}
	w := &Writer{eng: jiradb.Postgres, model: m}
	tb := w.Table("jiraissue", "id", "archived", "summary")
	if want := `INSERT INTO "jiraissue" ("id", "summary") VALUES `; tb.head != want {
		t.Errorf("head = %s", tb.head)
	}
	tb.Add(1, "N", "s")
	if !reflect.DeepEqual(tb.vals, []any{1, "s"}) {
		t.Errorf("vals = %v", tb.vals)
	}

	defer func() {
		if recover() == nil {
			t.Error("a missing column outside newerColumns was accepted")
		}
	}()
	w.Table("jiraissue", "id", "nope")
}
