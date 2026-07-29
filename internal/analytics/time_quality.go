package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// queryUntrustedPct is the share of events in [from, to) whose ingestion
// clock couldn't be trusted (time_quality = 'untrusted') — surfaced next to
// time-bucketed aggregate views, which otherwise blend untrusted-clock
// events into their day buckets with no visible signal that happened.
func queryUntrustedPct(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time) (float64, error) {
	var total, untrusted int64
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE time_quality = 'untrusted')
		FROM events
		WHERE project_id = $1 AND effective_at >= $2 AND effective_at < $3
	`, projectID, from, to).Scan(&total, &untrusted)
	if err != nil || total == 0 {
		return 0, err
	}
	return float64(untrusted) / float64(total) * 100, nil
}
