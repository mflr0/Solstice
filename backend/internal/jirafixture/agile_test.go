package jirafixture

import (
	"strings"
	"testing"

	"solstice/internal/jiradb"
)

func TestAODDL(t *testing.T) {
	pg := AODDL(jiradb.Postgres)
	if len(pg) != 2*len(aoTables) {
		t.Fatalf("%d statements for %d tables", len(pg), len(aoTables))
	}
	for _, s := range pg {
		t.Log(s)
		// Jira's AO identifiers are quoted upper case on PostgreSQL.
		if strings.Contains(s, "AO_60DB71_") && !strings.Contains(s, `"AO_60DB71_`) {
			t.Errorf("unquoted AO table: %s", s)
		}
	}
	sprint := pg[2*3+1] // AO_60DB71_SPRINT
	for _, want := range []string{`CREATE TABLE "AO_60DB71_SPRINT"`, `"CLOSED" BOOLEAN NOT NULL`, `"START_DATE" BIGINT`,
		`"GOAL" TEXT`, `PRIMARY KEY ("ID")`} {
		if !strings.Contains(sprint, want) {
			t.Errorf("sprint DDL lacks %q: %s", want, sprint)
		}
	}
	my := AODDL(jiradb.MySQL)
	if !strings.Contains(my[2*3+1], "`CLOSED` TINYINT(1) NOT NULL") || !strings.Contains(my[2*3+1], "ENGINE=InnoDB") {
		t.Errorf("MySQL sprint DDL: %s", my[2*3+1])
	}
}

func TestAOTableRejectsUnknownColumns(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("an unknown AO column was accepted")
		}
	}()
	(&Writer{eng: jiradb.MySQL}).AOTable("AO_60DB71_SPRINT", "NOPE")
}
