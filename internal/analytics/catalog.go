package analytics

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CatalogEntry struct {
	Name               string          `json:"name"`
	Kind               string          `json:"kind"`
	Description        string          `json:"description"`
	Owner              string          `json:"owner"`
	FirstSchemaVersion int             `json:"first_schema_version"`
	Properties         json.RawMessage `json:"properties"`
	// Known is false for rows created by ingestion's auto-discovery
	// (section 7, non-strict projects) rather than a deliberate catalog
	// declaration — an empty description is the signal, since a
	// hand-declared entry is expected to have one.
	Known          bool       `json:"known"`
	FirstSeenAt    *time.Time `json:"first_seen_at,omitempty"`
	LastSeenAt     *time.Time `json:"last_seen_at,omitempty"`
	EventCount     int64      `json:"event_count"`
	PercentOfTotal float64    `json:"percent_of_total"`
	Sparkline      []DayCount `json:"sparkline"`
	RejectedCount  int64      `json:"rejected_count"`
	// RejectionRate is rejected / (rejected + accepted) over the requested
	// range — "this event is being sent wrong N% of the time" (Phase 4).
	RejectionRate float64           `json:"rejection_rate"`
	Drift         []DriftedProperty `json:"drift,omitempty"`
}

// DriftedProperty is one undeclared property observed on an event that
// does have a declared property list — a client sending "scoree" when
// the catalog says "score" (Phase 4 schema drift detection).
type DriftedProperty struct {
	PropertyKey string `json:"property_key"`
	Count       int64  `json:"count"`
}

type CatalogResult struct {
	Entries []CatalogEntry `json:"entries"`
}

// GetCatalog implements section 10.2 #6, plus Phase 1c's per-event volume
// breakdown (counts, share of total, daily sparkline) and Phase 4's
// per-name rejection rate and schema drift, so "which events came in,
// which didn't, and why" is visible at a glance.
func GetCatalog(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location) (*CatalogResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	result, err := queryCatalogEntries(ctx, pool, projectID)
	if err != nil {
		return nil, err
	}

	counts, total, err := countEventsByName(ctx, pool, projectID, from, to)
	if err != nil {
		return nil, err
	}
	sparklines, err := sparklinesByName(ctx, pool, projectID, from, to, loc)
	if err != nil {
		return nil, err
	}
	rejections, err := rejectionsByName(ctx, pool, projectID, from, to)
	if err != nil {
		return nil, err
	}
	drift, err := driftByName(ctx, pool, projectID, from, to)
	if err != nil {
		return nil, err
	}
	applyCatalogVolume(result, counts, total, sparklines, rejections, drift)
	return result, nil
}

func queryCatalogEntries(ctx context.Context, pool *pgxpool.Pool, projectID string) (*CatalogResult, error) {
	rows, err := pool.Query(ctx, `
		SELECT name, kind, description, owner, first_schema_version, properties, first_seen_at, last_seen_at
		FROM event_catalog
		WHERE project_id = $1
		ORDER BY name
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Entries starts as []CatalogEntry{}, not nil — encoding/json emits
	// null for a nil slice, which crashes a naive frontend list render on
	// a brand-new project with no catalog entries yet.
	result := &CatalogResult{Entries: []CatalogEntry{}}
	for rows.Next() {
		var e CatalogEntry
		if err := rows.Scan(&e.Name, &e.Kind, &e.Description, &e.Owner, &e.FirstSchemaVersion, &e.Properties, &e.FirstSeenAt, &e.LastSeenAt); err != nil {
			return nil, err
		}
		e.Known = e.Description != ""
		e.Sparkline = []DayCount{}
		result.Entries = append(result.Entries, e)
	}
	return result, rows.Err()
}

// applyCatalogVolume merges the range-scoped counts/sparklines/rejections/
// drift onto each catalog entry in place.
func applyCatalogVolume(result *CatalogResult, counts map[string]int64, total int64, sparklines map[string][]DayCount, rejections map[string]int64, drift map[string][]DriftedProperty) {
	for i := range result.Entries {
		e := &result.Entries[i]
		e.EventCount = counts[e.Name]
		if total > 0 {
			e.PercentOfTotal = float64(e.EventCount) / float64(total) * 100
		}
		if s, ok := sparklines[e.Name]; ok {
			e.Sparkline = s
		}
		e.RejectedCount = rejections[e.Name]
		if attempted := e.EventCount + e.RejectedCount; attempted > 0 {
			e.RejectionRate = float64(e.RejectedCount) / float64(attempted) * 100
		}
		e.Drift = drift[e.Name]
	}
}

func countEventsByName(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time) (map[string]int64, int64, error) {
	rows, err := pool.Query(ctx, `
		SELECT name, COUNT(*) FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
		GROUP BY name
	`, projectID, from, to)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	counts := map[string]int64{}
	var total int64
	for rows.Next() {
		var name string
		var count int64
		if err := rows.Scan(&name, &count); err != nil {
			return nil, 0, err
		}
		counts[name] = count
		total += count
	}
	return counts, total, rows.Err()
}

func sparklinesByName(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location) (map[string][]DayCount, error) {
	rows, err := pool.Query(ctx, `
		SELECT name, (effective_at AT TIME ZONE $4)::date AS day, COUNT(*)
		FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
		GROUP BY name, day
		ORDER BY name, day
	`, projectID, from, to, loc.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sparklines := map[string][]DayCount{}
	for rows.Next() {
		var name string
		var day time.Time
		var count int64
		if err := rows.Scan(&name, &day, &count); err != nil {
			return nil, err
		}
		sparklines[name] = append(sparklines[name], DayCount{Day: day.Format("2006-01-02"), Count: count})
	}
	return sparklines, rows.Err()
}

// rejectionsByName sums event_rejection_stats across all codes for the
// given range, day-bucketed in UTC (matching how recordRejectionStats
// writes it) rather than the project's display timezone — the range
// bounds are widened by a day on each side so a UTC-day bucket that
// straddles the caller's local-timezone range isn't dropped.
func rejectionsByName(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time) (map[string]int64, error) {
	rows, err := pool.Query(ctx, `
		SELECT name, SUM(count) FROM event_rejection_stats
		WHERE project_id = $1 AND day >= $2 AND day < $3
		GROUP BY name
	`, projectID, from.AddDate(0, 0, -1), to.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int64{}
	for rows.Next() {
		var name string
		var count int64
		if err := rows.Scan(&name, &count); err != nil {
			return nil, err
		}
		counts[name] = count
	}
	return counts, rows.Err()
}

// driftByName sums event_property_drift by (name, property_key) over the
// same widened range as rejectionsByName, returning each name's drifted
// properties ordered by observation count, highest first.
func driftByName(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time) (map[string][]DriftedProperty, error) {
	rows, err := pool.Query(ctx, `
		SELECT name, property_key, SUM(count) AS total FROM event_property_drift
		WHERE project_id = $1 AND day >= $2 AND day < $3
		GROUP BY name, property_key
		ORDER BY name, total DESC
	`, projectID, from.AddDate(0, 0, -1), to.AddDate(0, 0, 1))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	drift := map[string][]DriftedProperty{}
	for rows.Next() {
		var name, propertyKey string
		var count int64
		if err := rows.Scan(&name, &propertyKey, &count); err != nil {
			return nil, err
		}
		drift[name] = append(drift[name], DriftedProperty{PropertyKey: propertyKey, Count: count})
	}
	return drift, rows.Err()
}
