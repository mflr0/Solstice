package issue

// Jira's working time, in which estimates and time spent are written: a day
// is 8 hours, a week 5 days.
const (
	WorkDaySeconds  = 8 * 3600
	WorkWeekSeconds = 5 * WorkDaySeconds
)
