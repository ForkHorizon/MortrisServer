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
}

type CatalogResult struct {
	Entries []CatalogEntry `json:"entries"`
}

// GetCatalog implements section 10.2 #6, plus Phase 1c's per-event volume
// breakdown: counts, share of total, and a tiny daily sparkline for the
// given range, so "which events came in and which didn't" is visible at a
// glance. "Validation issues" from the plan's screen description isn't
// populated — there is no per-event-name rejection tracking yet (only
// project-wide ingestion_stats totals from Phase S1), and fabricating one
// here would be guessing at data we don't have. Add it if/when rejection
// tracking gains per-name granularity.
func GetCatalog(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location) (*CatalogResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

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
	result := CatalogResult{Entries: []CatalogEntry{}}
	for rows.Next() {
		var e CatalogEntry
		if err := rows.Scan(&e.Name, &e.Kind, &e.Description, &e.Owner, &e.FirstSchemaVersion, &e.Properties, &e.FirstSeenAt, &e.LastSeenAt); err != nil {
			return nil, err
		}
		e.Known = e.Description != ""
		e.Sparkline = []DayCount{}
		result.Entries = append(result.Entries, e)
	}
	if err := rows.Err(); err != nil {
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
	for i := range result.Entries {
		e := &result.Entries[i]
		e.EventCount = counts[e.Name]
		if total > 0 {
			e.PercentOfTotal = float64(e.EventCount) / float64(total) * 100
		}
		if s, ok := sparklines[e.Name]; ok {
			e.Sparkline = s
		}
	}
	return &result, nil
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
