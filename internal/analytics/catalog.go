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
	Known       bool       `json:"known"`
	FirstSeenAt *time.Time `json:"first_seen_at,omitempty"`
	LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
}

type CatalogResult struct {
	Entries []CatalogEntry `json:"entries"`
}

// GetCatalog implements section 10.2 #6. "Validation issues" from the
// plan's screen description isn't populated — there is no per-event-name
// rejection tracking yet (only project-wide ingestion_stats totals from
// Phase S1), and fabricating one here would be guessing at data we don't
// have. Add it if/when rejection tracking gains per-name granularity.
func GetCatalog(ctx context.Context, pool *pgxpool.Pool, projectID string) (*CatalogResult, error) {
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
		result.Entries = append(result.Entries, e)
	}
	return &result, rows.Err()
}
