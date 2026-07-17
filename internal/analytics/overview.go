package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Overview is the Overview screen's metric set (section 10.2 #1): product
// event count, new installations, DAU/WAU/MAU, sessions, average observed
// session duration, and ingestion health.
type Overview struct {
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
}

// GetOverview implements every definition in docs/metrics.md except
// retention/funnel (Phase S3). DAU uses the calendar day of `to` in loc;
// WAU/MAU are trailing 7/30-day windows ending at `to`, matching section
// 9's "trailing 7-day/30-day interval" wording. Average observed session
// duration is computed only from events inside [from, to) — a session
// whose true end lies outside the requested window will be clipped; this
// is a deliberate scoping choice for a windowed query, not a bug.
func GetOverview(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location) (*Overview, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	var o Overview

	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM events
		WHERE project_id = $1 AND event_kind = 'product' AND effective_at >= $2 AND effective_at < $3
	`, projectID, from, to).Scan(&o.ProductEvents); err != nil {
		return nil, err
	}

	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM installations
		WHERE project_id = $1 AND first_product_event_at >= $2 AND first_product_event_at < $3
	`, projectID, from, to).Scan(&o.NewInstallations); err != nil {
		return nil, err
	}

	toLocal := to.In(loc)
	dayStart := time.Date(toLocal.Year(), toLocal.Month(), toLocal.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.Add(24 * time.Hour)
	if err := activeInstallations(ctx, pool, projectID, dayStart, dayEnd, &o.DailyActiveInstallations); err != nil {
		return nil, err
	}
	if err := activeInstallations(ctx, pool, projectID, to.Add(-7*24*time.Hour), to, &o.WeeklyActiveInstallations); err != nil {
		return nil, err
	}
	if err := activeInstallations(ctx, pool, projectID, to.Add(-30*24*time.Hour), to, &o.MonthlyActiveInstallations); err != nil {
		return nil, err
	}

	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM (
			SELECT DISTINCT install_id, session_id FROM events
			WHERE project_id = $1 AND event_kind = 'product' AND effective_at >= $2 AND effective_at < $3
		) sessions
	`, projectID, from, to).Scan(&o.Sessions); err != nil {
		return nil, err
	}

	if err := pool.QueryRow(ctx, `
		WITH qualifying_sessions AS (
			SELECT DISTINCT install_id, session_id FROM events
			WHERE project_id = $1 AND event_kind = 'product' AND effective_at >= $2 AND effective_at < $3
		), session_maxes AS (
			SELECT MAX(e.session_elapsed_ms) AS max_elapsed
			FROM events e
			JOIN qualifying_sessions qs ON qs.install_id = e.install_id AND qs.session_id = e.session_id
			WHERE e.project_id = $1 AND e.effective_at >= $2 AND e.effective_at < $3
			GROUP BY e.install_id, e.session_id
		)
		SELECT COALESCE(AVG(max_elapsed), 0) FROM session_maxes
	`, projectID, from, to).Scan(&o.AvgObservedSessionDurationMs); err != nil {
		return nil, err
	}

	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(accepted_count), 0), COALESCE(SUM(duplicate_count), 0), COALESCE(SUM(rejected_count), 0)
		FROM ingestion_stats
		WHERE project_id = $1 AND received_at >= $2 AND received_at < $3
	`, projectID, from, to).Scan(&o.IngestionAccepted, &o.IngestionDuplicates, &o.IngestionRejected); err != nil {
		return nil, err
	}

	return &o, nil
}

func activeInstallations(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, dest *int64) error {
	return pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT install_id) FROM events
		WHERE project_id = $1 AND event_kind = 'product' AND effective_at >= $2 AND effective_at < $3
	`, projectID, from, to).Scan(dest)
}
