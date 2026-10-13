package jirafixture

import (
	"strings"
	"testing"

	"solstice/internal/jiradb"
)

func TestJavaNameToDBName(t *testing.T) {
	cases := map[string]string{
		"id":                 "ID",
		"workflowId":         "WORKFLOW_ID",
		"sourceNodeId":       "SOURCE_NODE_ID",
		"customfieldtypekey": "CUSTOMFIELDTYPEKEY",
		"statusColor":        "STATUS_COLOR",
		"seqName":            "SEQ_NAME",
	}
	for in, want := range cases {
		if got := JavaNameToDBName(in); got != want {
			t.Errorf("JavaNameToDBName(%q) = %q, want %q", in, got, want)
		}
	}
}

// A trimmed-down model in the shape of Jira's entitymodel.xml.
const sampleModel = `<?xml version="1.0" encoding="UTF-8"?>
<entitymodel>
    <entity entity-name="Issue" table-name="jiraissue" package-name="">
        <field name="id" type="numeric"/>
        <field name="number" col-name="issuenum" type="numeric"/>
        <field name="summary" type="long-varchar"/>
        <field name="created" type="date-time-with-precision"/>
        <field name="workflowId" type="numeric"/>
        <prim-key field="id"/>
    </entity>
    <entity entity-name="OSCurrentStep" table-name="OS_CURRENTSTEP" package-name="">
        <field name="id" col-name="ID" type="numeric"/>
        <field name="status" col-name="STATUS" type="short-varchar"/>
        <prim-key field="id"/>
    </entity>
    <entity entity-name="Group" table-name="cwd_group" package-name="">
        <field name="id" type="numeric"/>
        <field name="local" type="integer"/>
        <prim-key field="id"/>
    </entity>
    <view-entity entity-name="IssueCount" package-name="">
        <member-entity entity-alias="I" entity-name="Issue"/>
    </view-entity>
</entitymodel>`

var sampleTypes = map[string]string{
	"numeric":                  "NUMERIC(18,0)",
	"long-varchar":             "VARCHAR(255)",
	"short-varchar":            "VARCHAR(60)",
	"date-time-with-precision": "TIMESTAMPTZ",
	"integer":                  "NUMERIC(9,0)",
}

func TestParseModel(t *testing.T) {
	m, err := ParseModel(strings.NewReader(sampleModel))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Entities) != 3 {
		t.Fatalf("got %d entities, want 3 (view entities are not tables)", len(m.Entities))
	}
	issue := m.Entity("JIRAISSUE")
	if issue == nil {
		t.Fatal("jiraissue not found (lookup must ignore case)")
	}
	if f, ok := issue.Column("workflow_id"); !ok || f.Column != "WORKFLOW_ID" {
		t.Errorf("workflowId column = %+v, %v", f, ok)
	}
	if f, ok := issue.Column("issuenum"); !ok || f.Name != "number" {
		t.Errorf("issuenum column = %+v, %v", f, ok)
	}
	if len(issue.PK) != 1 || issue.PK[0] != "ID" {
		t.Errorf("pk = %v", issue.PK)
	}
}

func TestDDL(t *testing.T) {
	m, err := ParseModel(strings.NewReader(sampleModel))
	if err != nil {
		t.Fatal(err)
	}

	pg, err := m.DDL(jiradb.Postgres, sampleTypes)
	if err != nil {
		t.Fatal(err)
	}
	wantPG := []string{
		`DROP TABLE IF EXISTS "jiraissue" CASCADE`,
		`CREATE TABLE "jiraissue" ("id" NUMERIC(18,0) NOT NULL, "issuenum" NUMERIC(18,0), "summary" VARCHAR(255), "created" TIMESTAMPTZ, "workflow_id" NUMERIC(18,0), CONSTRAINT "pk_jiraissue" PRIMARY KEY ("id"))`,
		`DROP TABLE IF EXISTS "os_currentstep" CASCADE`,
	}
	for i, want := range wantPG {
		if pg[i] != want {
			t.Errorf("postgres statement %d:\n got %s\nwant %s", i, pg[i], want)
		}
	}

	my, err := m.DDL(jiradb.MySQL, sampleTypes)
	if err != nil {
		t.Fatal(err)
	}
	if want := "CREATE TABLE `OS_CURRENTSTEP` (`ID` NUMERIC(18,0) NOT NULL, `STATUS` VARCHAR(60), PRIMARY KEY (`ID`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin"; my[3] != want {
		t.Errorf("mysql keeps the declared case:\n got %s\nwant %s", my[3], want)
	}
	// "local" is reserved on MySQL: every identifier is quoted.
	if !strings.Contains(my[5], "`LOCAL` NUMERIC(9,0)") {
		t.Errorf("cwd_group.local not quoted: %s", my[5])
	}

	if _, err := m.DDL(jiradb.Postgres, map[string]string{}); err == nil {
		t.Error("unknown field types must be reported")
	}
}
