package report

import (
	"slices"
	"time"
)

// FlowIssue is an issue as it is now, with the status changes logged since
// the period started (changes of other fields are ignored).
type FlowIssue struct {
	Created time.Time
	Status  string
	Changes []Change
}

// Flow counts, at each time of at, the issues in each column: counts[k][c]
// is how many issues sat in column c at at[k]. column maps a status to its
// column; an issue whose status maps to none, or that didn't exist yet, is
// left out at that time.
func Flow(issues []FlowIssue, column map[string]int, columns int, at []time.Time) [][]int {
	counts := make([][]int, len(at))
	for k := range counts {
		counts[k] = make([]int, columns)
	}
	for _, is := range issues {
		var changes []Change
		for _, c := range is.Changes {
			if c.Field == FieldStatus {
				changes = append(changes, c)
			}
		}
		slices.SortFunc(changes, func(a, b Change) int {
			if c := a.At.Compare(b.At); c != 0 {
				return c
			}
			return int(a.Seq - b.Seq)
		})
		// statuses[j] is the status before changes[j]; the last, the status now.
		statuses := make([]string, len(changes)+1)
		statuses[len(changes)] = is.Status
		for j := len(changes) - 1; j >= 0; j-- {
			statuses[j] = changes[j].Old
		}
		for k, t := range at {
			if !is.Created.Before(t) {
				continue
			}
			j, _ := slices.BinarySearchFunc(changes, t, func(c Change, t time.Time) int {
				if c.At.Before(t) {
					return -1
				}
				return 1
			})
			if c, ok := column[statuses[j]]; ok {
				counts[k][c]++
			}
		}
	}
	return counts
}
