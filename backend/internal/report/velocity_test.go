package report

import "testing"

// Velocity reads each sprint's committed and completed totals: an unfinished
// issue carried over counts in both sprints' commitment but is completed only
// in the second, and an issue removed mid-sprint stays committed.
func TestVelocity(t *testing.T) {
	next := Sprint{ID: 8, Start: day(15), End: day(29)}
	carried := Issue{ID: 1, InSprint: true, Status: "done", Estimate: pts(5),
		Changes: []Change{{At: day(20), Field: FieldStatus, Old: "todo", New: "done"}}}
	removed := Issue{ID: 2, InSprint: false, Status: "todo", Estimate: pts(3),
		Changes: []Change{{At: day(4), Field: FieldSprint, Old: "7"}}}
	finished := Issue{ID: 3, InSprint: true, Status: "done", Estimate: pts(2),
		Changes: []Change{{At: day(6), Field: FieldStatus, Old: "todo", New: "done"}}}

	for _, c := range []struct {
		name                 string
		sprint               Sprint
		issues               []Issue
		count                bool
		committed, completed float64
	}{
		{"first, estimated", sp, []Issue{carried, removed, finished}, false, 10, 2},
		{"next, estimated", next, []Issue{carried}, false, 5, 5},
		{"first, counted", sp, []Issue{carried, removed, finished}, true, 3, 1},
		{"next, counted", next, []Issue{carried}, true, 1, 1},
	} {
		got := Build(c.sprint, c.issues, isDone, c.count).Totals
		if got.Committed != c.committed || got.Completed != c.completed {
			t.Errorf("%s: committed %v, completed %v; want %v, %v", c.name, got.Committed, got.Completed,
				c.committed, c.completed)
		}
	}
}
