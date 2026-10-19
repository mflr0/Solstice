package importer

import (
	"testing"

	"solstice/internal/jiradb"
)

// The SELECT the importer runs on an Active Objects table: every name quoted
// in its exact case, on both engines.
func TestAOSelect(t *testing.T) {
	for eng, want := range map[jiradb.Engine]string{
		jiradb.MySQL:    "SELECT `ID`, `NAME`, `RANK` FROM `AO_60DB71_SPRINT`",
		jiradb.Postgres: `SELECT "ID", "NAME", "RANK" FROM "AO_60DB71_SPRINT"`,
	} {
		r := &run{engine: eng}
		if got := r.aoSelect("AO_60DB71_SPRINT", "ID", "NAME", "RANK"); got != want {
			t.Errorf("%s: aoSelect = %q, want %q", eng, got, want)
		}
	}
}
