package analytics

import (
	"context"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

type EventExplorerFilter struct {
	Name          *string
	AppVersion    *string
	BuildNumber   *string
	Platform      *string
	PropertyKey   *string
	PropertyValue *string
}

// ParseEventExplorerFilter reads and allowlist-validates the Event
// Explorer's filters (section 10.1: "Allowlist ... cataloged property
// filters"). name and property_key, if given, must be in the project's
// event catalog — not just well-formed strings.
func ParseEventExplorerFilter(ctx context.Context, pool *pgxpool.Pool, projectID string, q url.Values) (EventExplorerFilter, error) {
	f := EventExplorerFilter{
		Name:        optional(q, "name"),
		AppVersion:  optional(q, "app_version"),
		BuildNumber: optional(q, "build_number"),
		Platform:    optional(q, "platform"),
	}
	propKey := optional(q, "property_key")
	propValue := optional(q, "property_value")
	if (propKey == nil) != (propValue == nil) {
		return f, apierr.New(400, contracts.CodeInvalidRequest, "property_key and property_value must be given together")
	}

	if f.Name != nil {
		if !contracts.ReservedSystemEvents[*f.Name] {
			var exists bool
			if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_catalog WHERE project_id = $1 AND name = $2)`, projectID, *f.Name).Scan(&exists); err != nil {
				return f, err
			}
			if !exists {
				return f, apierr.New(400, contracts.CodeInvalidRequest, "unknown event name: "+*f.Name)
			}
		}
	}

	if propKey != nil {
		if f.Name == nil {
			return f, apierr.New(400, contracts.CodeInvalidRequest, "property_key filter requires a name filter (properties are defined per event)")
		}
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM event_catalog, jsonb_array_elements(properties) p
				WHERE project_id = $1 AND name = $2 AND p->>'name' = $3
			)
		`, projectID, *f.Name, *propKey).Scan(&exists); err != nil {
			return f, err
		}
		if !exists {
			return f, apierr.New(400, contracts.CodeInvalidRequest, "property is not in the event catalog: "+*propKey)
		}
		f.PropertyKey, f.PropertyValue = propKey, propValue
	}

	return f, nil
}

type DayCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
}

type EventExplorerResult struct {
	TotalEvents         int64      `json:"total_events"`
	ActiveInstallations int64      `json:"active_installations"`
	Trend               []DayCount `json:"trend"`
}

// GetEventExplorer implements the Event Explorer screen (section 10.2
// #2): trends, total events, active installations, filtered by
// app/build/platform and an approved cataloged property.
func GetEventExplorer(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location, f EventExplorerFilter) (*EventExplorerResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var result EventExplorerResult
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT install_id) FROM events
		WHERE project_id = $1 AND event_kind = 'product' AND effective_at >= $2 AND effective_at < $3
		  AND ($4::text IS NULL OR name = $4)
		  AND ($5::text IS NULL OR app_version = $5)
		  AND ($6::text IS NULL OR build_number = $6)
		  AND ($7::text IS NULL OR platform = $7)
		  AND ($8::text IS NULL OR properties ->> $8 = $9)
	`, projectID, from, to, f.Name, f.AppVersion, f.BuildNumber, f.Platform, f.PropertyKey, f.PropertyValue,
	).Scan(&result.TotalEvents, &result.ActiveInstallations); err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, `
		SELECT (effective_at AT TIME ZONE $10)::date AS day, COUNT(*)
		FROM events
		WHERE project_id = $1 AND event_kind = 'product' AND effective_at >= $2 AND effective_at < $3
		  AND ($4::text IS NULL OR name = $4)
		  AND ($5::text IS NULL OR app_version = $5)
		  AND ($6::text IS NULL OR build_number = $6)
		  AND ($7::text IS NULL OR platform = $7)
		  AND ($8::text IS NULL OR properties ->> $8 = $9)
		GROUP BY day
		ORDER BY day
		LIMIT 92
	`, projectID, from, to, f.Name, f.AppVersion, f.BuildNumber, f.Platform, f.PropertyKey, f.PropertyValue, loc.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var day time.Time
		var count int64
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		result.Trend = append(result.Trend, DayCount{Day: day.Format("2006-01-02"), Count: count})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &result, nil
}
