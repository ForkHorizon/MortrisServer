package analytics

import "time"

// isToday reports whether dayStr (YYYY-MM-DD) is today's calendar date in
// loc — the last bucket of a trend/sparkline ending "today" is still
// accumulating events, not a complete day.
func isToday(dayStr string, loc *time.Location) bool {
	return dayStr == time.Now().In(loc).Format("2006-01-02")
}
