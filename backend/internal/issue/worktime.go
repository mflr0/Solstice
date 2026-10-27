package issue

import (
	"strconv"
	"strings"
)

// Jira's working time, in which estimates and time spent are written: a day
// is 8 hours, a week 5 days.
const (
	WorkDaySeconds  = 8 * 3600
	WorkWeekSeconds = 5 * WorkDaySeconds
)

// Duration writes seconds as Jira does: "1w 2d 3h 20m".
func Duration(secs int64) string {
	var parts []string
	for _, u := range []struct {
		n    int64
		unit string
	}{{WorkWeekSeconds, "w"}, {WorkDaySeconds, "d"}, {3600, "h"}, {60, "m"}} {
		if secs >= u.n {
			parts = append(parts, strconv.FormatInt(secs/u.n, 10)+u.unit)
			secs %= u.n
		}
	}
	if len(parts) == 0 {
		return "0m"
	}
	return strings.Join(parts, " ")
}
