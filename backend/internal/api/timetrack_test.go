package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func n(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }

func TestParseEstimate(t *testing.T) {
	const day, week = 8 * 3600, 5 * 8 * 3600
	good := map[string]int64{
		"30m": 1800, "1h": 3600, "1h 30m": 5400, "1h30m": 5400, "2d": 2 * day, "1w": week,
		"1w 2d 3h 30m": week + 2*day + 3*3600 + 1800, "1.5h": 5400, "1,5h": 5400, "  2H  ": 7200, "1W 1D": week + day,
		"45": 45 * 60, "0m": 0, "1 h 30 m": 5400,
	}
	for in, want := range good {
		if got, ok := parseEstimate(in); !ok || got != want {
			t.Errorf("parseEstimate(%q) = %d, %v; want %d", in, got, ok, want)
		}
	}
	// Text the lenient JQL reader would take: here it must be refused.
	for _, in := range []string{"", "  ", "abc", "1h abc", "abc 1h", "1x", "-1h", "1h 30", "1 2h", "2d 4", "h", "1h,", "1..5h",
		"99999999w", "1h\n30m x"} {
		if got, ok := parseEstimate(in); ok {
			t.Errorf("parseEstimate(%q) = %d, accepted", in, got)
		}
	}
}

func TestEstimateEditCarriesRemaining(t *testing.T) {
	// A new issue: the remaining estimate starts equal to the original one.
	got := timeEstimates{}.withEstimateEdit(estimateEdit{OriginalSet: true, Original: n(3600)})
	if got.Original != n(3600) || got.Remaining != n(3600) {
		t.Errorf("creation: %+v", got)
	}
	// An explicit remaining estimate wins.
	got = timeEstimates{}.withEstimateEdit(estimateEdit{OriginalSet: true, Original: n(3600), RemainingSet: true,
		Remaining: n(1800)})
	if got.Original != n(3600) || got.Remaining != n(1800) {
		t.Errorf("explicit remaining: %+v", got)
	}
	// Before any work is logged, a new original estimate carries the remaining one.
	cur := timeEstimates{Original: n(3600), Remaining: n(3600)}
	if got := cur.withEstimateEdit(estimateEdit{OriginalSet: true, Original: n(7200)}); got.Remaining != n(7200) {
		t.Errorf("before work: %+v", got)
	}
	// Once work is logged, it does not.
	cur = timeEstimates{Original: n(3600), Remaining: n(1800), Spent: n(1800)}
	if got := cur.withEstimateEdit(estimateEdit{OriginalSet: true, Original: n(7200)}); got.Remaining != n(1800) {
		t.Errorf("after work: %+v", got)
	}
	// A remaining estimate that was moved by hand stays.
	cur = timeEstimates{Original: n(3600), Remaining: n(900)}
	if got := cur.withEstimateEdit(estimateEdit{OriginalSet: true, Original: n(7200)}); got.Remaining != n(900) {
		t.Errorf("hand-set remaining: %+v", got)
	}
	// Clearing the original estimate clears a remaining one that followed it.
	cur = timeEstimates{Original: n(3600), Remaining: n(3600)}
	if got := cur.withEstimateEdit(estimateEdit{OriginalSet: true}); got.Original.Valid || got.Remaining.Valid {
		t.Errorf("cleared: %+v", got)
	}
	// Setting only the remaining estimate leaves the original alone.
	cur = timeEstimates{Original: n(3600), Remaining: n(3600)}
	if got := cur.withEstimateEdit(estimateEdit{RemainingSet: true, Remaining: n(60)}); got.Original != n(3600) ||
		got.Remaining != n(60) {
		t.Errorf("remaining only: %+v", got)
	}
	// Nothing given: nothing changes.
	if got := cur.withEstimateEdit(estimateEdit{}); got != cur {
		t.Errorf("empty edit: %+v", got)
	}
}

func TestWithLogged(t *testing.T) {
	cur := timeEstimates{Original: n(4 * 3600), Remaining: n(4 * 3600)}
	// Logging an hour: spent grows, remaining shrinks.
	got := cur.withLogged(3600)
	if got.Spent != n(3600) || got.Remaining != n(3*3600) || got.Original != cur.Original {
		t.Errorf("log: %+v", got)
	}
	// Logging more than what remains stops at nothing.
	got = got.withLogged(5 * 3600)
	if got.Spent != n(6*3600) || got.Remaining != n(0) {
		t.Errorf("overrun: %+v", got)
	}
	// Taking time back (an edit to less, a deletion) gives it back to the remaining estimate.
	got = timeEstimates{Remaining: n(3600), Spent: n(7200)}.withLogged(-3600)
	if got.Spent != n(3600) || got.Remaining != n(7200) {
		t.Errorf("take back: %+v", got)
	}
	// All of the time taken back: nothing spent any more.
	got = timeEstimates{Remaining: n(0), Spent: n(3600)}.withLogged(-3600)
	if got.Spent.Valid || got.Remaining != n(3600) {
		t.Errorf("delete the last log: %+v", got)
	}
	// Without a remaining estimate there is none to adjust.
	got = timeEstimates{Spent: n(60)}.withLogged(600)
	if got.Remaining.Valid || got.Spent != n(660) {
		t.Errorf("no estimate: %+v", got)
	}
	// Never below zero spent, even with inconsistent data.
	got = timeEstimates{Spent: n(60)}.withLogged(-600)
	if got.Spent.Valid {
		t.Errorf("negative: %+v", got)
	}
}

func TestTimeChanges(t *testing.T) {
	cur := timeEstimates{Original: n(3600), Remaining: n(3600)}
	changes := cur.changesTo(cur.withLogged(1800))
	if len(changes) != 2 || changes[0].field != "timeestimate" || changes[1].field != "timespent" {
		t.Fatalf("changes: %+v", changes)
	}
	if changes[0].oldValue != "3600" || changes[0].newValue != "1800" || changes[0].newString != "1800" {
		t.Errorf("remaining estimate line: %+v", changes[0])
	}
	if changes[1].oldValue != nil || changes[1].newValue != "1800" {
		t.Errorf("time spent line: %+v", changes[1])
	}
	if got := cur.changesTo(cur); len(got) != 0 {
		t.Errorf("no change: %+v", got)
	}
	cleared := cur.changesTo(timeEstimates{})
	if len(cleared) != 2 || cleared[0].field != "timeoriginalestimate" || cleared[0].newValue != nil {
		t.Errorf("cleared: %+v", cleared)
	}
}

func TestReadEstimateEdit(t *testing.T) {
	f := fields{"originalEstimate": json.RawMessage(`"1d 2h"`), "remainingEstimate": json.RawMessage(`null`)}
	e, err := readEstimateEdit(f)
	if err != nil || !e.OriginalSet || e.Original != n(10*3600) || !e.RemainingSet || e.Remaining.Valid {
		t.Errorf("edit: %+v, %v", e, err)
	}
	if e, err := readEstimateEdit(fields{"summary": json.RawMessage(`"x"`)}); err != nil || e.OriginalSet || e.RemainingSet {
		t.Errorf("no estimate: %+v, %v", e, err)
	}
	if e, err := readEstimateEdit(fields{"originalEstimate": json.RawMessage(`""`)}); err != nil || !e.OriginalSet ||
		e.Original.Valid {
		t.Errorf("empty clears: %+v, %v", e, err)
	}
	for _, raw := range []string{`"soon"`, `"1h 30"`, `12`} {
		_, err := readEstimateEdit(fields{"remainingEstimate": json.RawMessage(raw)})
		var he *httpError
		if !errors.As(err, &he) || he.status != http.StatusBadRequest {
			t.Errorf("remainingEstimate %s: %v", raw, err)
		}
	}
}
