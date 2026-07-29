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
		if err := validateEventName(ctx, pool, projectID, *f.Name); err != nil {
			return f, err
		}
	}

	if propKey != nil {
		if f.Name == nil {
			return f, apierr.New(400, contracts.CodeInvalidRequest, "property_key filter requires a name filter (properties are defined per event)")
		}
		if err := validateCatalogProperty(ctx, pool, projectID, *f.Name, *propKey); err != nil {
			return f, err
		}
		f.PropertyKey, f.PropertyValue = propKey, propValue
	}

	return f, nil
}

// validateEventName allowlists a name against the project's event catalog
// (reserved system events are exempt). Shared by the Event Explorer's
// optional name filter and the Property Value Inspector's required one.
func validateEventName(ctx context.Context, pool *pgxpool.Pool, projectID, name string) error {
	if contracts.ReservedSystemEvents[name] {
		return nil
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_catalog WHERE project_id = $1 AND name = $2)`, projectID, name).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return apierr.New(400, contracts.CodeInvalidRequest, "unknown event name: "+name)
	}
	return nil
}

// validateCatalogProperty allowlists a property key against the given
// event's declared catalog properties.
func validateCatalogProperty(ctx context.Context, pool *pgxpool.Pool, projectID, name, key string) error {
	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM event_catalog, jsonb_array_elements(properties) p
			WHERE project_id = $1 AND name = $2 AND p->>'name' = $3
		)
	`, projectID, name, key).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return apierr.New(400, contracts.CodeInvalidRequest, "property is not in the event catalog: "+key)
	}
	return nil
}

type DayCount struct {
	Day   string `json:"day"`
	Count int64  `json:"count"`
	// Partial marks a day bucket that's today (in the requested
	// timezone) and so is still accumulating events, not a complete day.
	Partial bool `json:"partial,omitempty"`
}

// trendDayLimit caps every day-bucketed trend/sparkline query — with the
// 90-day max window (maxWindow) it's a safety valve, not a real boundary,
// but Truncated still flags it the same way as every other capped query.
const trendDayLimit = 92

type EventExplorerResult struct {
	TotalEvents         int64      `json:"total_events"`
	ActiveInstallations int64      `json:"active_installations"`
	Trend               []DayCount `json:"trend"`
	// Truncated is true if the trend hit trendDayLimit and may be missing days.
	Truncated bool `json:"truncated"`
	// UntrustedPct is the share of matching events with time_quality =
	// 'untrusted' over [from, to) — so a trend blip isn't mistaken for
	// real signal when it's actually bad client clocks.
	UntrustedPct float64 `json:"untrusted_pct"`
}

// GetEventExplorer implements the Event Explorer screen (section 10.2
// #2): trends, total events, active installations, filtered by
// app/build/platform and an approved cataloged property.
func GetEventExplorer(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location, f EventExplorerFilter) (*EventExplorerResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	result := EventExplorerResult{Trend: []DayCount{}}
	total, active, err := countEventExplorerTotals(ctx, pool, projectID, from, to, f)
	if err != nil {
		return nil, err
	}
	result.TotalEvents, result.ActiveInstallations = total, active

	trend, err := queryEventExplorerTrend(ctx, pool, projectID, from, to, loc, f)
	if err != nil {
		return nil, err
	}
	result.Truncated = len(trend) >= trendDayLimit
	if len(trend) > 0 && isToday(trend[len(trend)-1].Day, loc) {
		trend[len(trend)-1].Partial = true
	}
	result.Trend = trend

	untrustedPct, err := queryUntrustedPct(ctx, pool, projectID, from, to)
	if err != nil {
		return nil, err
	}
	result.UntrustedPct = untrustedPct

	return &result, nil
}

func countEventExplorerTotals(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, f EventExplorerFilter) (total, active int64, err error) {
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(DISTINCT install_id) FROM events
		WHERE project_id = $1 AND event_kind = 'product' AND effective_at >= $2 AND effective_at < $3
		  AND ($4::text IS NULL OR name = $4)
		  AND ($5::text IS NULL OR app_version = $5)
		  AND ($6::text IS NULL OR build_number = $6)
		  AND ($7::text IS NULL OR platform = $7)
		  AND ($8::text IS NULL OR properties ->> $8 = $9)
	`, projectID, from, to, f.Name, f.AppVersion, f.BuildNumber, f.Platform, f.PropertyKey, f.PropertyValue,
	).Scan(&total, &active)
	return total, active, err
}

// queryEventExplorerTrend returns []DayCount{}, not nil, on the
// zero-rows path — encoding/json emits null for a nil slice, which
// crashes a naive frontend list render whenever the filtered range has
// zero matching days.
func queryEventExplorerTrend(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location, f EventExplorerFilter) ([]DayCount, error) {
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
	// The LIMIT above must match trendDayLimit — see its doc comment.
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trend := []DayCount{}
	for rows.Next() {
		var day time.Time
		var count int64
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		trend = append(trend, DayCount{Day: day.Format("2006-01-02"), Count: count})
	}
	return trend, rows.Err()
}
