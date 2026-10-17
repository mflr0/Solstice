package importer

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"
)

const logicDescriptor = `<workflow>
  <steps>
    <step id="1" name="To do">
      <meta name="jira.status.id">10000</meta>
      <actions>
        <action id="11" name="Start">
          <restrict-to>
            <conditions type="AND">
              <condition type="class"><arg name="class.name">com.atlassian.jira.workflow.condition.PermissionCondition</arg></condition>
              <condition type="class"><arg name="class.name">com.example.OnlyOnMondays</arg></condition>
            </conditions>
          </restrict-to>
          <validators>
            <validator name="" type="class"><arg name="class.name">com.atlassian.jira.workflow.validator.PermissionValidator</arg></validator>
          </validators>
          <results>
            <unconditional-result old-status="null" status="underway" step="2">
              <post-functions>
                <function type="class"><arg name="class.name">com.atlassian.jira.workflow.function.issue.UpdateIssueStatusFunction</arg></function>
                <function type="class"><arg name="class.name">com.atlassian.jira.workflow.function.event.FireIssueEventFunction</arg><arg name="eventTypeId">13</arg></function>
                <function type="class"><arg name="class.name">com.example.NotifyTheChannel</arg></function>
              </post-functions>
            </unconditional-result>
          </results>
        </action>
      </actions>
    </step>
  </steps>
</workflow>`

func TestCountWorkflowLogic(t *testing.T) {
	got := countWorkflowLogic(logicDescriptor)
	want := workflowLogic{conditions: 2, validators: 1, functions: 1}
	if got != want {
		t.Errorf("got %+v, want %+v (built-in post-functions are not counted)", got, want)
	}
	if got := countWorkflowLogic(descriptor); got != (workflowLogic{}) {
		t.Errorf("a workflow without logic counts %+v", got)
	}
	if got := countWorkflowLogic("<workflow><step"); got != (workflowLogic{}) {
		t.Errorf("an unreadable descriptor counts %+v", got)
	}
}

func TestParseWorkflowKeepsLogic(t *testing.T) {
	w, err := parseWorkflow("Logic", logicDescriptor)
	if err != nil {
		t.Fatal(err)
	}
	if w.logic.conditions != 2 {
		t.Errorf("conditions = %d", w.logic.conditions)
	}
}

func TestCounterWritesNothing(t *testing.T) {
	var dst target = counter{}
	res, err := dst.ExecContext(context.Background(), "DELETE FROM issues")
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 0 {
		t.Errorf("rows affected = %d", n)
	}
}

// A nil checkData is an import's: its methods do nothing.
func TestCheckDataOnAnImport(t *testing.T) {
	var c *checkData
	c.password()
	c.attachment(true)
	c.changeGroup(1)
	c.sortChangeGroups()
	c.changeItem(2)
	c.sprint(3)
	c.sprintMember(4, sql.NullString{}, sql.NullFloat64{})
}

func TestOrphanChangeItems(t *testing.T) {
	c := &checkData{}
	for _, id := range []int64{30, 10, 20} {
		c.changeGroup(id)
	}
	c.sortChangeGroups()
	for _, group := range []int64{10, 10, 20, 99, 0, 30} {
		c.changeItem(group)
	}
	if c.orphanItems != 2 {
		t.Errorf("orphans = %d, want 2 (groups 99 and 0)", c.orphanItems)
	}
}

func TestSprintMembers(t *testing.T) {
	c := &checkData{sprints: map[int64]bool{}, sprintRefs: map[[2]int64]bool{}}
	c.sprint(5)
	c.sprint(6)
	str := func(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }
	num := func(f float64) sql.NullFloat64 { return sql.NullFloat64{Float64: f, Valid: true} }
	c.sprintMember(100, str("5"), sql.NullFloat64{})  // in a sprint
	c.sprintMember(100, str("5"), sql.NullFloat64{})  // the same twice
	c.sprintMember(101, str("name=Sprint 6"), num(6)) // not a number: the number field
	c.sprintMember(102, str("7"), sql.NullFloat64{})  // a sprint that does not exist
	c.sprintMember(103, str(""), sql.NullFloat64{})   // nothing
	if n := c.sprintMembers(); n != 2 {
		t.Errorf("members = %d, want 2", n)
	}
}

func TestWriteText(t *testing.T) {
	rep := &CheckReport{
		Source:   CheckSource{Database: "postgres", Server: "PostgreSQL 16"},
		Counts:   Counts{Issues: 12, Users: 3, UsersWithPassword: 2, CustomFieldTypes: map[string]int{"x:float": 2}},
		LeftOut:  []LeftOut{{"comments", 4, "orphan: issue not imported"}},
		Warnings: []string{"a warning"},
	}
	var b bytes.Buffer
	rep.WriteText(&b)
	for _, want := range []string{"postgres (PostgreSQL 16)", "not recorded in the database", "The import would work",
		"(2 with a usable password)", "files not checked", "x:float", "- comments: 4 (orphan", "- a warning"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, b.String())
		}
	}
	b.Reset()
	rep.Refused = "too old"
	rep.WriteText(&b)
	if !strings.Contains(b.String(), "would refuse this source: too old") {
		t.Errorf("a refusal is not reported:\n%s", b.String())
	}
}
