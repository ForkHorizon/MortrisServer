package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OverviewDaily is one trailing day of the same metrics above, for each
// stat tile's sparkline (dashboard Phase 3). Always 7 entries, oldest
// first, ending on `to`'s local calendar day — independent of the
// from/to range picker, since a sparkline is a "last week" snapshot.
type OverviewDaily struct {
	Day                          string  `json:"day"`
	ProductEvents                int64   `json:"product_events"`
	NewInstallations             int64   `json:"new_installations"`
	DailyActiveInstallations     int64   `json:"daily_active_installations"`
	WeeklyActiveInstallations    int64   `json:"weekly_active_installations"`
	MonthlyActiveInstallations   int64   `json:"monthly_active_installations"`
	Sessions                     int64   `json:"sessions"`
	AvgObservedSessionDurationMs float64 `json:"avg_observed_session_duration_ms"`
	IngestionAccepted            int64   `json:"ingestion_accepted"`
	IngestionDuplicates          int64   `json:"ingestion_duplicates"`
	IngestionRejected            int64   `json:"ingestion_rejected"`
	// Partial marks the trailing day when it's today — still
	// accumulating events, not a complete day.
	Partial bool `json:"partial,omitempty"`
}

// EventKindDay is one day's product/system event split, over the full
// selected range, for the Overview "events per day, stacked by kind" chart.
type EventKindDay struct {
	Day     string `json:"day"`
	Product int64  `json:"product"`
	System  int64  `json:"system"`
	Partial bool   `json:"partial,omitempty"`
}

// dailyIndex is the 7 trailing calendar days ending at dayEnd (exclusive),
// keyed for the query result mergers below to scatter rows into.
type dailyIndex struct {
	days      []OverviewDaily
	byDay     map[string]int
	weekStart time.Time
}

func newDailyIndex(dayEnd time.Time, loc *time.Location) dailyIndex {
	weekStart := dayEnd.Add(-7 * 24 * time.Hour)
	idx := dailyIndex{days: make([]OverviewDaily, 7), byDay: make(map[string]int, 7), weekStart: weekStart}
	for i := range idx.days {
		key := weekStart.Add(time.Duration(i) * 24 * time.Hour).In(loc).Format("2006-01-02")
		idx.days[i].Day = key
		idx.byDay[key] = i
	}
	return idx
}

// queryOverviewDaily fills 7 trailing days ending at dayEnd for every
// stat tile's sparkline. One query per metric group, each scattered into
// the shared day slice by calendar day.
func queryOverviewDaily(ctx context.Context, pool *pgxpool.Pool, projectID string, dayEnd time.Time, loc *time.Location) ([]OverviewDaily, error) {
	idx := newDailyIndex(dayEnd, loc)
	tz := loc.String()

	if err := fillEventsDauSessionsDaily(ctx, pool, projectID, idx, tz); err != nil {
		return nil, err
	}
	if err := fillNewInstallationsDaily(ctx, pool, projectID, idx, tz); err != nil {
		return nil, err
	}
	if err := fillIngestionDaily(ctx, pool, projectID, idx, tz); err != nil {
		return nil, err
	}
	if err := fillAvgDurationDaily(ctx, pool, projectID, idx, tz); err != nil {
		return nil, err
	}
	if err := fillActiveWindowsDaily(ctx, pool, projectID, idx); err != nil {
		return nil, err
	}

	if last := &idx.days[len(idx.days)-1]; isToday(last.Day, loc) {
		last.Partial = true
	}
	return idx.days, nil
}

func fillEventsDauSessionsDaily(ctx context.Context, pool *pgxpool.Pool, projectID string, idx dailyIndex, tz string) error {
	rows, err := pool.Query(ctx, `
		SELECT (effective_at AT TIME ZONE $4)::date AS day,
		       COUNT(*) FILTER (WHERE event_kind = 'product') AS product_events,
		       COUNT(DISTINCT install_id) FILTER (WHERE event_kind = 'product') AS dau,
		       COUNT(DISTINCT (install_id, session_id)) FILTER (WHERE event_kind = 'product') AS sessions
		FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
		GROUP BY day
	`, projectID, idx.weekStart, dayEndOf(idx), tz)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var day time.Time
		var events, dau, sessions int64
		if err := rows.Scan(&day, &events, &dau, &sessions); err != nil {
			return err
		}
		if i, ok := idx.byDay[day.Format("2006-01-02")]; ok {
			idx.days[i].ProductEvents, idx.days[i].DailyActiveInstallations, idx.days[i].Sessions = events, dau, sessions
		}
	}
	return rows.Err()
}

func fillNewInstallationsDaily(ctx context.Context, pool *pgxpool.Pool, projectID string, idx dailyIndex, tz string) error {
	rows, err := pool.Query(ctx, `
		SELECT (first_product_event_at AT TIME ZONE $4)::date AS day, COUNT(*)
		FROM installations
		WHERE project_id = $1 AND first_product_event_at >= $2 AND first_product_event_at < $3
		GROUP BY day
	`, projectID, idx.weekStart, dayEndOf(idx), tz)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var day time.Time
		var count int64
		if err := rows.Scan(&day, &count); err != nil {
			return err
		}
		if i, ok := idx.byDay[day.Format("2006-01-02")]; ok {
			idx.days[i].NewInstallations = count
		}
	}
	return rows.Err()
}

func fillIngestionDaily(ctx context.Context, pool *pgxpool.Pool, projectID string, idx dailyIndex, tz string) error {
	rows, err := pool.Query(ctx, `
		SELECT (received_at AT TIME ZONE $4)::date AS day,
		       COALESCE(SUM(accepted_count), 0), COALESCE(SUM(duplicate_count), 0), COALESCE(SUM(rejected_count), 0)
		FROM ingestion_stats
		WHERE project_id = $1 AND received_at >= $2 AND received_at < $3
		GROUP BY day
	`, projectID, idx.weekStart, dayEndOf(idx), tz)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var day time.Time
		var accepted, duplicates, rejected int64
		if err := rows.Scan(&day, &accepted, &duplicates, &rejected); err != nil {
			return err
		}
		if i, ok := idx.byDay[day.Format("2006-01-02")]; ok {
			idx.days[i].IngestionAccepted, idx.days[i].IngestionDuplicates, idx.days[i].IngestionRejected = accepted, duplicates, rejected
		}
	}
	return rows.Err()
}

func fillAvgDurationDaily(ctx context.Context, pool *pgxpool.Pool, projectID string, idx dailyIndex, tz string) error {
	rows, err := pool.Query(ctx, `
		WITH qualifying_sessions AS (
			SELECT install_id, session_id, MIN((effective_at AT TIME ZONE $4)::date) AS day
			FROM events
			WHERE project_id = $1 AND event_kind = 'product' AND effective_at >= $2 AND effective_at < $3
			GROUP BY install_id, session_id
		), session_maxes AS (
			SELECT qs.day, MAX(e.session_elapsed_ms) AS max_elapsed
			FROM events e
			JOIN qualifying_sessions qs ON qs.install_id = e.install_id AND qs.session_id = e.session_id
			WHERE e.project_id = $1 AND e.effective_at >= $2 AND e.effective_at < $3
			GROUP BY qs.day, e.install_id, e.session_id
		)
		SELECT day, AVG(max_elapsed) FROM session_maxes GROUP BY day
	`, projectID, idx.weekStart, dayEndOf(idx), tz)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var day time.Time
		var avg float64
		if err := rows.Scan(&day, &avg); err != nil {
			return err
		}
		if i, ok := idx.byDay[day.Format("2006-01-02")]; ok {
			idx.days[i].AvgObservedSessionDurationMs = avg
		}
	}
	return rows.Err()
}

// fillActiveWindowsDaily fills WAU/MAU per day by reusing activeInstallations
// in a small loop rather than a single windowed query.
// ponytail: 14 sequential round trips (2 per day x 7 days) — fine at
// dashboard scale; fold into one generate_series query if this endpoint's
// latency ever matters.
func fillActiveWindowsDaily(ctx context.Context, pool *pgxpool.Pool, projectID string, idx dailyIndex) error {
	for i := range idx.days {
		dayEndI := idx.weekStart.Add(time.Duration(i+1) * 24 * time.Hour)
		if err := activeInstallations(ctx, pool, projectID, dayEndI.Add(-7*24*time.Hour), dayEndI, &idx.days[i].WeeklyActiveInstallations); err != nil {
			return err
		}
		if err := activeInstallations(ctx, pool, projectID, dayEndI.Add(-30*24*time.Hour), dayEndI, &idx.days[i].MonthlyActiveInstallations); err != nil {
			return err
		}
	}
	return nil
}

func dayEndOf(idx dailyIndex) time.Time {
	return idx.weekStart.Add(7 * 24 * time.Hour)
}

// queryEventsByKind returns the full [from, to) range's daily product/system
// split for the Overview stacked chart (unlike queryOverviewDaily, this
// follows the selected date range rather than a fixed trailing week). The
// LIMIT below must match trendDayLimit — see its doc comment.
func queryEventsByKind(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location) ([]EventKindDay, error) {
	rows, err := pool.Query(ctx, `
		SELECT (effective_at AT TIME ZONE $4)::date AS day,
		       COUNT(*) FILTER (WHERE event_kind = 'product') AS product,
		       COUNT(*) FILTER (WHERE event_kind = 'system') AS system
		FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
		GROUP BY day
		ORDER BY day
		LIMIT 92
	`, projectID, from, to, loc.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []EventKindDay{}
	for rows.Next() {
		var day time.Time
		var product, system int64
		if err := rows.Scan(&day, &product, &system); err != nil {
			return nil, err
		}
		result = append(result, EventKindDay{Day: day.Format("2006-01-02"), Product: product, System: system})
	}
	if len(result) > 0 && isToday(result[len(result)-1].Day, loc) {
		result[len(result)-1].Partial = true
	}
	return result, rows.Err()
}
