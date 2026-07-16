package maintenance

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const batchSize = 10000

// DeleteUnactivatedInstallations removes registrations that never sent a
// product event within 7 days (section 5.2), in bounded batches so one
// sweep never holds a long-running transaction against a busy table.
func DeleteUnactivatedInstallations(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var total int64
	for {
		tag, err := pool.Exec(ctx, `
			DELETE FROM installations
			WHERE (project_id, install_id) IN (
				SELECT project_id, install_id FROM installations
				WHERE activated_at IS NULL
				  AND registered_at < clock_timestamp() - interval '7 days'
				LIMIT $1
			)
		`, batchSize)
		if err != nil {
			return total, err
		}
		n := tag.RowsAffected()
		total += n
		if n < batchSize {
			return total, nil
		}
		time.Sleep(50 * time.Millisecond) // yield to ingestion (section 8.5)
	}
}

// DeleteExpiredEvents enforces each project's retention_days (default 90,
// section 8.5) in bounded batches, deliberately never disabled by disk
// pressure (section 12) — callers must always run this regardless of
// disk state.
func DeleteExpiredEvents(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var total int64
	for {
		tag, err := pool.Exec(ctx, `
			DELETE FROM events
			WHERE (project_id, event_id) IN (
				SELECT e.project_id, e.event_id
				FROM events e
				JOIN projects p ON p.id = e.project_id
				WHERE e.received_at < clock_timestamp() - make_interval(days => p.retention_days)
				LIMIT $1
			)
		`, batchSize)
		if err != nil {
			return total, err
		}
		n := tag.RowsAffected()
		total += n
		if n < batchSize {
			return total, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
}
