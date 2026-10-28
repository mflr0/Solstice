package report

import (
	"slices"
	"testing"
	"time"
)

var flowColumns = map[string]int{"todo": 0, "doing": 1, "done": 2}

// Day ends: the samples of days 1 to 4.
var dayEnds = []time.Time{day(1), day(2), day(3), day(4)}

func status(at time.Time, seq int64, old, next string) Change {
	return Change{At: at, Seq: seq, Field: FieldStatus, Old: old, New: next}
}

func TestFlowStatusChangesInsideADay(t *testing.T) {
	issues := []FlowIssue{{
		Created: t0, Status: "done",
		Changes: []Change{
			// Two changes on day 2 (between day(1) and day(2)): only where it lands counts.
			status(day(1).Add(2*time.Hour), 1, "todo", "doing"),
			status(day(1).Add(5*time.Hour), 2, "doing", "done"),
			// Back and forth on day 4, at the same time: the log's order decides.
			status(day(3).Add(time.Hour), 4, "doing", "done"),
			status(day(3).Add(time.Hour), 3, "done", "doing"),
			// Other fields don't move it.
			{At: day(2).Add(time.Hour), Field: FieldEstimate, Old: "1", New: "2"},
		},
	}}
	got := Flow(issues, flowColumns, 3, dayEnds)
	want := [][]int{{1, 0, 0}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("counts = %v, want %v", got, want)
	}
}

func TestFlowChangeAtTheDayEnd(t *testing.T) {
	issues := []FlowIssue{{Created: t0, Status: "doing",
		Changes: []Change{status(day(2), 1, "todo", "doing")}}}
	got := Flow(issues, flowColumns, 3, dayEnds)
	want := [][]int{{1, 0, 0}, {1, 0, 0}, {0, 1, 0}, {0, 1, 0}}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("counts = %v, want %v: a change at midnight belongs to the next day", got, want)
	}
}

func TestFlowUnmappedStatus(t *testing.T) {
	issues := []FlowIssue{
		// Through an unmapped status and back.
		{Created: t0, Status: "doing", Changes: []Change{
			status(day(1).Add(time.Hour), 1, "todo", "review"),
			status(day(3).Add(time.Hour), 2, "review", "doing"),
		}},
		// Unmapped now, mapped before.
		{Created: t0, Status: "archived", Changes: []Change{
			status(day(2).Add(time.Hour), 3, "todo", "archived"),
		}},
	}
	got := Flow(issues, flowColumns, 3, dayEnds)
	want := [][]int{{2, 0, 0}, {1, 0, 0}, {0, 0, 0}, {0, 1, 0}}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("counts = %v, want %v", got, want)
	}
}

func TestFlowIssueCreatedMidPeriod(t *testing.T) {
	issues := []FlowIssue{
		{Created: t0, Status: "todo"},
		// Created on day 3, moved the same day.
		{Created: day(2).Add(time.Hour), Status: "doing", Changes: []Change{
			status(day(2).Add(2*time.Hour), 1, "todo", "doing"),
		}},
		// Created exactly at the end of day 3: counts from day 4.
		{Created: day(3), Status: "todo"},
	}
	got := Flow(issues, flowColumns, 3, dayEnds)
	want := [][]int{{1, 0, 0}, {1, 0, 0}, {1, 1, 0}, {2, 1, 0}}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("counts = %v, want %v", got, want)
	}
}
