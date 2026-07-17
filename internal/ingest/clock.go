package ingest

import "time"

// nowRFC3339Millis formats the current UTC time to match
// internal/contracts' timestamp pattern exactly (millisecond precision,
// literal Z) — every response's server_time uses this.
func nowRFC3339Millis() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}
