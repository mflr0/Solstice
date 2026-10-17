package importer

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"solstice/internal/jiradb"
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

// The import's SQL casts the number with CAST AS UNSIGNED, which rounds, and
// takes a string of digits whatever its length before looking at the number.
func TestSprintMembersRoundAndLongStrings(t *testing.T) {
	str := func(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }
	num := func(f float64) sql.NullFloat64 { return sql.NullFloat64{Float64: f, Valid: true} }
	for name, tc := range map[string]struct {
		str  sql.NullString
		num  sql.NullFloat64
		want int64 // the sprint the issue is in, 0 for none
	}{
		"rounded up":          {sql.NullString{}, num(5.6), 6},
		"rounded down":        {sql.NullString{}, num(5.4), 5},
		"whole":               {str("name=x"), num(5), 5},
		"negative":            {sql.NullString{}, num(-5), 0},
		"too big":             {sql.NullString{}, num(1e30), 0},
		"digits win":          {str("5"), num(6), 5},
		"long digits win":     {str("99999999999999999999"), num(6), 0},
		"long digits, no id":  {str("99999999999999999999"), sql.NullFloat64{}, 0},
		"leading zeros":       {str("0006"), sql.NullFloat64{}, 6},
		"signed is no digits": {str("-6"), num(5), 5},
		"digits and a space":  {str("6 "), num(5), 5},
		"no value at all":     {sql.NullString{}, sql.NullFloat64{}, 0},
	} {
		c := &checkData{sprints: map[int64]bool{5: true, 6: true}, sprintRefs: map[[2]int64]bool{}}
		c.sprintMember(100, tc.str, tc.num)
		want := int64(0)
		if tc.want != 0 {
			want = 1
		}
		if n := c.sprintMembers(); n != want || (want == 1 && !c.sprintRefs[[2]int64{tc.want, 100}]) {
			t.Errorf("%s: refs %v, members %d, want sprint %d", name, c.sprintRefs, n, tc.want)
		}
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

// fakeJira is a Jira database with no rows. Every query finds nothing, except
// the COUNT(*) ones, which find count; a query that holds failOn fails, as does
// a ping while down is set.
type fakeJira struct {
	count  int64
	failOn string
	down   bool
}

func (f *fakeJira) Connect(context.Context) (driver.Conn, error) { return fakeConn{f}, nil }
func (f *fakeJira) Driver() driver.Driver                        { return nil }

type fakeConn struct{ *fakeJira }

func (fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (fakeConn) Close() error                        { return nil }
func (fakeConn) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }

func (c fakeConn) Ping(context.Context) error {
	if c.down {
		return errors.New("connection reset by peer")
	}
	return nil
}

func (c fakeConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.failOn != "" && strings.Contains(query, c.failOn) {
		return nil, errors.New("connection reset by peer")
	}
	if strings.Contains(query, "COUNT(*)") {
		return &fakeRows{rows: [][]driver.Value{{c.count}}}, nil
	}
	return &fakeRows{}, nil
}

type fakeRows struct{ rows [][]driver.Value }

func (*fakeRows) Columns() []string { return []string{"n"} }
func (*fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if len(r.rows) == 0 {
		return io.EOF
	}
	copy(dest, r.rows[0])
	r.rows = r.rows[1:]
	return nil
}

// A source that cannot be read is an error, not a report that says the import
// would refuse it.
func TestCheckFailsOnAnUnreadableSource(t *testing.T) {
	for name, jira := range map[string]*fakeJira{
		"a query fails":   {count: 1, failOn: "cwd_user"},
		"connection lost": {failOn: "information_schema", down: true},
	} {
		src := sql.OpenDB(jira)
		rep, err := check(context.Background(), src, jiradb.MySQL, Options{Log: slog.Default(), SourceTZ: time.UTC})
		src.Close()
		if err == nil {
			t.Errorf("%s: no error, report %+v", name, rep)
		}
	}
}

// A refused source reports the steps that ran before the refusal, not figures
// that are all zero.
func TestCheckRefusalKeepsTheStepsBefore(t *testing.T) {
	src := sql.OpenDB(&fakeJira{}) // no column at all: schemepermissions.permission_key neither
	defer src.Close()
	rep, err := check(context.Background(), src, jiradb.MySQL, Options{Log: slog.Default(), SourceTZ: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rep.Refused, "too old") {
		t.Fatalf("refused = %q", rep.Refused)
	}
	var names []string
	for _, s := range rep.Steps {
		names = append(names, s.Name)
	}
	if len(names) == 0 || names[0] != "users" || slices.Contains(names, "issues") {
		t.Errorf("steps = %v, want those before permissions only", names)
	}
}
