// Package report rebuilds board reports from the change log: what a sprint
// held when it started, what joined or left it, and how its remaining work
// went down.
//
// The replay runs backwards: it starts from each issue's state now and undoes
// the changes logged since, newest first, to find its state at any earlier
// time. Membership changes that the log misses (unfinished issues carried to
// the next sprint when one completes) happen before that sprint starts, so
// starting from today's membership still gives the right scope at start.
package report

import (
	"slices"
	"strconv"
	"strings"
	"time"
)

// Field is a field of the change log the replay follows.
type Field int

// The fields the replay follows.
const (
	FieldSprint Field = iota
	FieldStatus
	FieldEstimate
)

// Change is one logged change of a followed field. For FieldSprint, Old and
// New are the sprint ids the issue was in (Jira writes a comma-separated
// list); for FieldStatus, status ids; for FieldEstimate, the estimate as a
// number, "" when unset.
type Change struct {
	At       time.Time
	Seq      int64 // breaks ties between changes at the same time
	Field    Field
	Old, New string
}

// Issue is an issue as it is now, with the changes logged since the sprint
// started.
type Issue struct {
	ID       int64
	InSprint bool
	Status   string
	Estimate *float64
	Changes  []Change
}

// Sprint is the sprint reported on. End is when the report stops: its
// completion, or now for an active sprint.
type Sprint struct {
	ID         int64
	Start, End time.Time
}

// Outcome is where an issue ended up in the sprint.
type Outcome string

// The outcomes of an issue.
const (
	Completed    Outcome = "completed"
	NotCompleted Outcome = "notCompleted"
	Removed      Outcome = "removed"
)

// Entry is one issue of the report.
type Entry struct {
	ID        int64
	Committed bool // in the sprint when it started; otherwise added later
	Outcome   Outcome
	// Estimates when the sprint started (for committed issues, else when the
	// issue joined) and when the report ends.
	StartEstimate, EndEstimate *float64
}

// Step is a change of the remaining work during the sprint.
type Step struct {
	At        time.Time
	Issue     int64
	Remaining float64
}

// Totals is the work of each group of the report: estimates summed, or
// issues counted.
type Totals struct {
	Committed, Added, Removed, Completed, NotCompleted float64
}

// Report is a sprint report and the data of its burndown.
type Report struct {
	Entries []Entry
	Totals  Totals
	// Remaining work when the sprint started, then after each change.
	StartRemaining float64
	Steps          []Step
}

// state is an issue's followed fields at one time.
type state struct {
	in       bool
	status   string
	estimate *float64
}

// Build replays the issues' changes over the sprint. done tells whether a
// status counts as done; count, whether each issue counts 1 instead of its
// estimate.
func Build(sp Sprint, issues []Issue, done func(status string) bool, count bool) Report {
	var r Report
	type event struct {
		at    time.Time
		seq   int64
		issue int
		after state
	}
	var events []event
	starts := make([]state, len(issues))
	ends := make([]state, len(issues))
	for i, is := range issues {
		changes := slices.Clone(is.Changes)
		slices.SortFunc(changes, func(a, b Change) int {
			if c := a.At.Compare(b.At); c != 0 {
				return c
			}
			return int(a.Seq - b.Seq)
		})
		// Undo the changes newest first; states[k] is the state just after
		// changes[k], cur ends as the state before the first one.
		cur := state{in: is.InSprint, status: is.Status, estimate: is.Estimate}
		states := make([]state, len(changes))
		for k := len(changes) - 1; k >= 0; k-- {
			states[k] = cur
			cur = undo(cur, changes[k], sp.ID)
		}
		starts[i], ends[i] = cur, cur
		for k, c := range changes {
			if !c.At.After(sp.Start) {
				starts[i] = states[k]
			}
			if !c.At.After(sp.End) {
				ends[i] = states[k]
			}
		}
		for k, c := range changes {
			if c.At.After(sp.Start) && !c.At.After(sp.End) {
				events = append(events, event{c.At, c.Seq, i, states[k]})
			}
		}
	}

	weight := func(s state) float64 {
		if !s.in || done(s.status) {
			return 0
		}
		if count {
			return 1
		}
		if s.estimate == nil {
			return 0
		}
		return *s.estimate
	}
	slices.SortStableFunc(events, func(a, b event) int {
		if c := a.at.Compare(b.at); c != 0 {
			return c
		}
		return int(a.seq - b.seq)
	})
	cur := slices.Clone(starts)
	for _, s := range starts {
		r.StartRemaining += weight(s)
	}
	remaining := r.StartRemaining
	joined := make([]*state, len(issues))
	for _, e := range events {
		before := cur[e.issue]
		cur[e.issue] = e.after
		if !before.in && e.after.in && joined[e.issue] == nil {
			s := e.after
			joined[e.issue] = &s
		}
		if d := weight(e.after) - weight(before); d != 0 {
			remaining += d
			r.Steps = append(r.Steps, Step{At: e.at, Issue: issues[e.issue].ID, Remaining: remaining})
		}
	}

	for i, is := range issues {
		committed := starts[i].in
		if !committed && joined[i] == nil {
			continue
		}
		e := Entry{ID: is.ID, Committed: committed, EndEstimate: ends[i].estimate}
		if committed {
			e.StartEstimate = starts[i].estimate
		} else {
			e.StartEstimate = joined[i].estimate
		}
		work := func(p *float64) float64 {
			if count {
				return 1
			}
			if p == nil {
				return 0
			}
			return *p
		}
		if committed {
			r.Totals.Committed += work(e.StartEstimate)
		} else {
			r.Totals.Added += work(e.StartEstimate)
		}
		switch {
		case !ends[i].in:
			e.Outcome = Removed
			r.Totals.Removed += work(e.EndEstimate)
		case done(ends[i].status):
			e.Outcome = Completed
			r.Totals.Completed += work(e.EndEstimate)
		default:
			e.Outcome = NotCompleted
			r.Totals.NotCompleted += work(e.EndEstimate)
		}
		r.Entries = append(r.Entries, e)
	}
	return r
}

// undo returns the state before change c, given the state after it.
func undo(s state, c Change, sprint int64) state {
	switch c.Field {
	case FieldSprint:
		// A move between two other sprints leaves this one's membership
		// alone (a closed sprint keeps its issues).
		if hasID(c.Old, sprint) || hasID(c.New, sprint) {
			s.in = hasID(c.Old, sprint)
		}
	case FieldStatus:
		s.status = c.Old
	case FieldEstimate:
		s.estimate = ParseEstimate(c.Old)
	}
	return s
}

func hasID(list string, id int64) bool {
	for part := range strings.SplitSeq(list, ",") {
		if v, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil && v == id {
			return true
		}
	}
	return false
}

// ParseEstimate reads an estimate written in the change log; nil when unset
// or unreadable.
func ParseEstimate(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(strings.Replace(s, ",", ".", 1), 64)
	if err != nil {
		return nil
	}
	return &v
}
