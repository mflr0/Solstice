package api

// Time tracking: estimates typed in Jira's duration syntax, and the
// arithmetic that keeps an issue's remaining estimate in step with the work
// logged on it.

import (
	"context"
	"database/sql"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"solstice/internal/i18n"
	"solstice/internal/issue"
	"solstice/internal/jql"
	"solstice/internal/sqlq"
)

// maxEstimate caps an estimate (ten thousand weeks) so that sums never
// overflow and typos like "99999999w" are refused.
const maxEstimate = 10000 * issue.WorkWeekSeconds

var (
	// A bare number is minutes, as in Jira; it is only read alone.
	estimateBare = regexp.MustCompile(`^\s*\d+(?:[.,]\d+)?\s*$`)
	// Otherwise every number carries its unit: "1w 2d 3h 30m", "1h30m".
	estimateUnits = regexp.MustCompile(`(?i)^\s*(?:\d+(?:[.,]\d+)?\s*[wdhm]\s*)+$`)
)

// parseEstimate reads a time estimate in Jira's syntax (a day is 8 hours, a
// week 5 days) into seconds. Unlike the lenient reader JQL uses for
// searches, it refuses text it does not fully understand.
func parseEstimate(text string) (int64, bool) {
	if !estimateBare.MatchString(text) && !estimateUnits.MatchString(text) {
		return 0, false
	}
	v, ok := jql.Seconds(text)
	if !ok || v < 0 || v > maxEstimate {
		return 0, false
	}
	return int64(math.Round(v)), true
}

// timeEstimates is an issue's time tracking in seconds; a null is "not set".
type timeEstimates struct {
	Original, Remaining, Spent sql.NullInt64
}

// estimateEdit is a requested change of the estimates; a field that was not
// given is left alone, one given as null is cleared.
type estimateEdit struct {
	OriginalSet, RemainingSet bool
	Original, Remaining       sql.NullInt64
}

// withEstimateEdit applies an edit. Jira keeps the remaining estimate equal
// to the original one until work is logged, so a new original estimate
// carries the remaining one along when it was never told apart; an
// explicit remaining estimate always wins.
func (cur timeEstimates) withEstimateEdit(e estimateEdit) timeEstimates {
	next := cur
	if e.OriginalSet {
		next.Original = e.Original
	}
	switch {
	case e.RemainingSet:
		next.Remaining = e.Remaining
	case e.OriginalSet && cur.Spent.Int64 == 0 && cur.Remaining == cur.Original:
		next.Remaining = e.Original
	}
	return next
}

// withLogged returns the time tracking once seconds more have been logged
// (fewer, when negative: a work log shortened or deleted). Jira's default
// is to adjust the remaining estimate automatically: it shrinks by what was
// logged, down to nothing, and grows back by what is taken back. An issue
// without a remaining estimate gets none.
func (cur timeEstimates) withLogged(seconds int64) timeEstimates {
	next := cur
	if spent := max(cur.Spent.Int64+seconds, 0); spent > 0 {
		next.Spent = sql.NullInt64{Int64: spent, Valid: true}
	} else {
		next.Spent = sql.NullInt64{}
	}
	if cur.Remaining.Valid {
		next.Remaining = sql.NullInt64{Int64: max(cur.Remaining.Int64-seconds, 0), Valid: true}
	}
	return next
}

// secondsText writes a duration the way history keeps it: seconds as text.
func secondsText(v sql.NullInt64) any {
	if !v.Valid {
		return nil
	}
	return strconv.FormatInt(v.Int64, 10)
}

// changesTo lists the history lines that turn one state of the time
// tracking into another. Field names are Jira's, as in imported history.
func (cur timeEstimates) changesTo(next timeEstimates) []change {
	var out []change
	for _, f := range []struct {
		field     string
		old, next sql.NullInt64
	}{
		{"timeoriginalestimate", cur.Original, next.Original},
		{"timeestimate", cur.Remaining, next.Remaining},
		{"timespent", cur.Spent, next.Spent},
	} {
		if f.old != f.next {
			out = append(out, change{field: f.field, oldValue: secondsText(f.old), oldString: secondsText(f.old),
				newValue: secondsText(f.next), newString: secondsText(f.next)})
		}
	}
	return out
}

// lockTimeTracking reads an issue's time tracking and holds its row until
// the transaction ends, so that two work logs never overwrite each other.
func lockTimeTracking(ctx context.Context, tx *sql.Tx, issueID int64) (timeEstimates, error) {
	var t timeEstimates
	err := tx.QueryRowContext(ctx, `SELECT original_estimate, remaining_estimate, time_spent FROM issues
		WHERE id = ? FOR UPDATE`, issueID).Scan(&t.Original, &t.Remaining, &t.Spent)
	return t, err
}

// storeTimeTracking writes the new time tracking of an issue and records
// what changed in its history.
func storeTimeTracking(ctx context.Context, tx *sql.Tx, issueID, author int64, now time.Time, cur,
	next timeEstimates) ([]change, error) {
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET original_estimate = ?, remaining_estimate = ?,
		time_spent = ?, updated_at = ? WHERE id = ?`, sqlq.Value(next.Original), sqlq.Value(next.Remaining),
		sqlq.Value(next.Spent), now, issueID); err != nil {
		return nil, err
	}
	changes := cur.changesTo(next)
	return changes, recordChanges(ctx, tx, issueID, author, now, changes)
}

// readEstimateEdit reads the estimate fields of a create or update request:
// strings in Jira's duration syntax, or null to clear one.
func readEstimateEdit(f fields) (estimateEdit, error) {
	var e estimateEdit
	for _, x := range []struct {
		key string
		set *bool
		to  *sql.NullInt64
	}{{"originalEstimate", &e.OriginalSet, &e.Original}, {"remainingEstimate", &e.RemainingSet, &e.Remaining}} {
		if !f.has(x.key) {
			continue
		}
		*x.set = true
		var v *string
		if err := f.get(x.key, &v); err != nil {
			return e, err
		}
		if v == nil || strings.TrimSpace(*v) == "" {
			continue
		}
		secs, ok := parseEstimate(*v)
		if !ok {
			return e, fail(http.StatusBadRequest, i18n.IssueEstimateInvalid)
		}
		*x.to = sql.NullInt64{Int64: secs, Valid: true}
	}
	return e, nil
}
