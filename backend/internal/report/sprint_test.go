package report

import (
	"testing"
	"time"
)

var (
	t0    = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	start = t0.Add(24 * time.Hour)
	end   = start.Add(14 * 24 * time.Hour)
	sp    = Sprint{ID: 7, Start: start, End: end}
)

func day(n int) time.Time { return start.Add(time.Duration(n) * 24 * time.Hour) }

func pts(v float64) *float64 { return &v }

func isDone(status string) bool { return status == "done" }

func entry(t *testing.T, r Report, id int64) Entry {
	t.Helper()
	for _, e := range r.Entries {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("issue %d not in the report: %+v", id, r.Entries)
	return Entry{}
}

func estimate(p *float64) float64 {
	if p == nil {
		return -1
	}
	return *p
}

func TestScopeAtStart(t *testing.T) {
	issues := []Issue{
		// In since before the start, through a log line.
		{ID: 1, InSprint: true, Status: "todo", Estimate: pts(3),
			Changes: []Change{{At: t0, Field: FieldSprint, New: "7"}}},
		// In with no log line (carried over from the previous sprint).
		{ID: 2, InSprint: true, Status: "todo", Estimate: pts(5)},
		// Moved between two other sprints: not this sprint's business.
		{ID: 3, InSprint: false, Status: "todo", Estimate: pts(8),
			Changes: []Change{{At: day(2), Field: FieldSprint, Old: "5", New: "9"}}},
		// Jira's list form: in 6 and 7 at the start.
		{ID: 4, InSprint: true, Status: "todo", Estimate: pts(1),
			Changes: []Change{{At: t0, Field: FieldSprint, Old: "6", New: "6, 7"}}},
		// Still in this sprint, once closed, when it moves from the backlog
		// into another one.
		{ID: 5, InSprint: true, Status: "todo", Estimate: pts(2),
			Changes: []Change{{At: day(20), Field: FieldSprint, New: "9"}}},
	}
	r := Build(sp, issues, isDone, false)
	if len(r.Entries) != 4 {
		t.Fatalf("entries = %+v, want issues 1, 2, 4, 5", r.Entries)
	}
	for _, id := range []int64{1, 2, 4, 5} {
		if e := entry(t, r, id); !e.Committed || e.Outcome != NotCompleted {
			t.Errorf("issue %d = %+v, want committed, not completed", id, e)
		}
	}
	if r.StartRemaining != 11 || len(r.Steps) != 0 {
		t.Errorf("start remaining %v, steps %+v; want 11 and none", r.StartRemaining, r.Steps)
	}
}

func TestAddedAndRemoved(t *testing.T) {
	issues := []Issue{
		{ID: 1, InSprint: true, Status: "todo", Estimate: pts(3)},
		{ID: 2, InSprint: true, Status: "todo", Estimate: pts(2),
			Changes: []Change{{At: day(3), Field: FieldSprint, New: "7"}}},
		{ID: 3, InSprint: false, Status: "todo", Estimate: pts(5),
			Changes: []Change{{At: day(5), Field: FieldSprint, Old: "7"}}},
		// Added then removed within the sprint.
		{ID: 4, InSprint: false, Status: "todo", Estimate: pts(1),
			Changes: []Change{{At: day(1), Field: FieldSprint, New: "7"}, {At: day(2), Field: FieldSprint, Old: "7"}}},
		// Added after the sprint completed: not in the report.
		{ID: 5, InSprint: true, Status: "todo", Estimate: pts(1),
			Changes: []Change{{At: day(20), Field: FieldSprint, New: "7"}}},
	}
	r := Build(sp, issues, isDone, false)
	if e := entry(t, r, 2); e.Committed || e.Outcome != NotCompleted {
		t.Errorf("added issue = %+v", e)
	}
	if e := entry(t, r, 3); !e.Committed || e.Outcome != Removed {
		t.Errorf("removed issue = %+v", e)
	}
	if e := entry(t, r, 4); e.Committed || e.Outcome != Removed {
		t.Errorf("added then removed issue = %+v", e)
	}
	if len(r.Entries) != 4 {
		t.Errorf("entries = %+v, want 4 (issue 5 joined after the end)", r.Entries)
	}
	want := []float64{9, 8, 10, 5}
	if r.StartRemaining != 8 || len(r.Steps) != len(want) {
		t.Fatalf("start %v, steps %+v", r.StartRemaining, r.Steps)
	}
	for i, s := range r.Steps {
		if s.Remaining != want[i] {
			t.Errorf("step %d remaining %v, want %v", i, s.Remaining, want[i])
		}
	}
}

func TestReestimated(t *testing.T) {
	issues := []Issue{
		{ID: 1, InSprint: true, Status: "todo", Estimate: pts(8), Changes: []Change{
			{At: t0, Field: FieldEstimate, Old: "", New: "2"},
			{At: day(2), Field: FieldEstimate, Old: "2", New: "5"},
			{At: day(30), Field: FieldEstimate, Old: "5", New: "8"},
		}},
	}
	r := Build(sp, issues, isDone, false)
	e := entry(t, r, 1)
	if estimate(e.StartEstimate) != 2 || estimate(e.EndEstimate) != 5 {
		t.Errorf("estimates %v → %v, want 2 → 5", estimate(e.StartEstimate), estimate(e.EndEstimate))
	}
	if r.StartRemaining != 2 || len(r.Steps) != 1 || r.Steps[0].Remaining != 5 {
		t.Errorf("start %v, steps %+v; want 2 then 5", r.StartRemaining, r.Steps)
	}
}

func TestDoneThenReopened(t *testing.T) {
	issues := []Issue{
		{ID: 1, InSprint: true, Status: "todo", Estimate: pts(3), Changes: []Change{
			{At: day(2), Field: FieldStatus, Old: "todo", New: "done"},
			{At: day(4), Field: FieldStatus, Old: "done", New: "todo"},
		}},
		{ID: 2, InSprint: true, Status: "done", Estimate: pts(2), Changes: []Change{
			{At: day(1), Field: FieldStatus, Old: "todo", New: "done"},
			{At: day(3), Field: FieldStatus, Old: "done", New: "todo"},
			{At: day(6), Field: FieldStatus, Old: "todo", New: "done"},
		}},
	}
	r := Build(sp, issues, isDone, false)
	if e := entry(t, r, 1); e.Outcome != NotCompleted {
		t.Errorf("reopened issue = %+v", e)
	}
	if e := entry(t, r, 2); e.Outcome != Completed {
		t.Errorf("done again issue = %+v", e)
	}
	want := []float64{3, 0, 2, 5, 3}
	if r.StartRemaining != 5 || len(r.Steps) != len(want) {
		t.Fatalf("start %v, steps %+v", r.StartRemaining, r.Steps)
	}
	for i, s := range r.Steps {
		if s.Remaining != want[i] {
			t.Errorf("step %d remaining %v, want %v", i, s.Remaining, want[i])
		}
	}
}

func TestCountIssues(t *testing.T) {
	issues := []Issue{
		{ID: 1, InSprint: true, Status: "todo"},
		{ID: 2, InSprint: true, Status: "todo", Estimate: pts(5)},
		{ID: 3, InSprint: true, Status: "done",
			Changes: []Change{{At: day(1), Field: FieldStatus, Old: "todo", New: "done"}}},
	}
	if r := Build(sp, issues, isDone, true); r.StartRemaining != 3 || len(r.Steps) != 1 || r.Steps[0].Remaining != 2 {
		t.Errorf("counted: start %v, steps %+v; want 3 then 2", r.StartRemaining, r.Steps)
	}
	if r := Build(sp, issues, isDone, false); r.StartRemaining != 5 || len(r.Steps) != 0 {
		t.Errorf("estimated: start %v, steps %+v; want 5, unestimated issues count 0", r.StartRemaining, r.Steps)
	}
}

func TestParseEstimate(t *testing.T) {
	for in, want := range map[string]float64{"3": 3, " 2.5 ": 2.5, "1,5": 1.5, "": -1, "abc": -1} {
		if got := estimate(ParseEstimate(in)); got != want {
			t.Errorf("ParseEstimate(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestTotals(t *testing.T) {
	issues := []Issue{
		{ID: 1, InSprint: true, Status: "done", Estimate: pts(3),
			Changes: []Change{{At: day(2), Field: FieldStatus, Old: "todo", New: "done"}}},
		{ID: 2, InSprint: true, Status: "todo", Estimate: pts(5), Changes: []Change{
			{At: day(1), Field: FieldSprint, New: "7"},
			{At: day(4), Field: FieldEstimate, Old: "4", New: "5"},
		}},
		{ID: 3, InSprint: false, Status: "todo", Estimate: pts(2),
			Changes: []Change{{At: day(3), Field: FieldSprint, Old: "7"}}},
	}
	want := Totals{Committed: 5, Added: 4, Removed: 2, Completed: 3, NotCompleted: 5}
	if got := Build(sp, issues, isDone, false).Totals; got != want {
		t.Errorf("totals %+v, want %+v", got, want)
	}
	want = Totals{Committed: 2, Added: 1, Removed: 1, Completed: 1, NotCompleted: 1}
	if got := Build(sp, issues, isDone, true).Totals; got != want {
		t.Errorf("counted totals %+v, want %+v", got, want)
	}
}
