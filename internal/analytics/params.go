// Package analytics implements the fixed, read-only metrics queries
// behind the dashboard (section 9, 10.1). No endpoint here accepts SQL —
// every dimension, filter, and sort is allowlisted and passed as a bound
// parameter.
package analytics

import (
	"net/url"
	"time"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

const (
	defaultWindow = 7 * 24 * time.Hour
	maxWindow     = 90 * 24 * time.Hour // section 10.1: "Maximum raw-data window is 90 days"
	queryTimeout  = 5 * time.Second
)

// ParseTimezone validates the "timezone" query parameter against the IANA
// database (section 9: "a validated IANA timezone"), defaulting to UTC.
func ParseTimezone(q url.Values) (*time.Location, error) {
	tz := q.Get("timezone")
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, apierr.New(400, contracts.CodeInvalidRequest, "invalid timezone: "+tz)
	}
	return loc, nil
}

// ParseDateRange validates "from"/"to" (RFC 3339), defaulting to the
// trailing 7 days and rejecting a window over 90 days (section 10.1).
func ParseDateRange(q url.Values) (from, to time.Time, err error) {
	now := time.Now().UTC()

	if s := q.Get("to"); s != "" {
		to, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, time.Time{}, apierr.New(400, contracts.CodeInvalidRequest, "invalid to: must be RFC 3339")
		}
	} else {
		to = now
	}

	if s := q.Get("from"); s != "" {
		from, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}, time.Time{}, apierr.New(400, contracts.CodeInvalidRequest, "invalid from: must be RFC 3339")
		}
	} else {
		from = to.Add(-defaultWindow)
	}

	if !to.After(from) {
		return time.Time{}, time.Time{}, apierr.New(400, contracts.CodeInvalidRequest, "to must be after from")
	}
	if to.Sub(from) > maxWindow {
		return time.Time{}, time.Time{}, apierr.New(400, contracts.CodeInvalidRequest, "date range cannot exceed 90 days")
	}
	return from, to, nil
}

// optional returns nil for an empty query parameter, so callers can pass
// it straight to pgx as a nullable bind parameter ($n IS NULL OR ...).
func optional(q url.Values, key string) *string {
	v := q.Get(key)
	if v == "" {
		return nil
	}
	return &v
}
