package analytics

// Ingestion, content-catalogue and delivery queries behind PuzzleQuality
// (puzzle_quality.go). Split out once the main file reached its readable
// -length ceiling — this half owns "what arrived and does it resolve
// against the catalogue," the other half (puzzle_quality_integrity.go)
// owns "did the event chain arrive intact."

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// loadQualityIngestion answers "total received/accepted, duplicates" from
// the durable per-batch counters (batch_persist.go) rather than the events
// table, since rejections and duplicates are never inserted as event rows.
// Not build-scoped: a rejected/duplicate event carries no parsed build.
func loadQualityIngestion(ctx context.Context, pool *pgxpool.Pool, q *PuzzleQuality, projectID string, from, to time.Time) error {
	var accepted, duplicates, rejected int64
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(accepted_count),0), COALESCE(SUM(duplicate_count),0), COALESCE(SUM(rejected_count),0)
		FROM ingestion_stats WHERE project_id=$1 AND received_at>=$2 AND received_at<$3
	`, projectID, from, to).Scan(&accepted, &duplicates, &rejected)
	if err != nil {
		return err
	}
	q.AcceptedEvents, q.DuplicateEvents, q.RejectedEvents = accepted, duplicates, rejected
	q.ReceivedEvents = accepted + duplicates + rejected
	return nil
}

// loadQualityRejections groups by (name, code) — the catalog-governance
// counters already carry both, so no new table is needed. day is the
// server-received UTC day, matching how recordRejectionStats buckets it.
func loadQualityRejections(ctx context.Context, pool *pgxpool.Pool, q *PuzzleQuality, projectID string, from, to time.Time) error {
	// day is a coarse date bucket (recordRejectionStats), not a timestamp,
	// so the usual half-open [from, to) doesn't apply: to's own calendar
	// day must stay included or a same-day range would match nothing.
	rows, err := pool.Query(ctx, `
		SELECT name, code, SUM(count) FROM event_rejection_stats
		WHERE project_id=$1 AND day>=$2::date AND day<=$3::date
		GROUP BY 1,2 ORDER BY 3 DESC
	`, projectID, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r PuzzleQualityRejection
		if err := rows.Scan(&r.Name, &r.Code, &r.Count); err != nil {
			return err
		}
		q.Rejections = append(q.Rejections, r)
	}
	return rows.Err()
}

// loadQualitySchemaVersion counts the total product event population this
// scope's percentages are drawn against, plus how much of it declares
// schema_version=2 versus omits it entirely (item 4 of the plan's
// required list).
func loadQualitySchemaVersion(ctx context.Context, pool *pgxpool.Pool, q *PuzzleQuality, projectID string, from, to time.Time, build *string) error {
	return pool.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE (properties->>'schema_version')::int = 2),
		       COUNT(*) FILTER (WHERE NOT (properties ? 'schema_version'))
		FROM events
		WHERE project_id=$1 AND event_kind='product' AND effective_at>=$2 AND effective_at<$3
		  AND ($4::text IS NULL OR build_number=$4)
	`, projectID, from, to, build).Scan(&q.TotalGameplayEvents, &q.SchemaV2Events, &q.MissingSchemaVersion)
}

// loadQualityRevisionAndCoordinateSpace covers items 5 and 6: events whose
// content_revision was never imported, and placement/release events whose
// coordinate_space is missing or not house_local. Only events that carry
// a content_revision are eligible for the first check — developer
// lifecycle events (menu opened/closed) legitimately have none.
func loadQualityRevisionAndCoordinateSpace(ctx context.Context, pool *pgxpool.Pool, q *PuzzleQuality, projectID string, from, to time.Time, build *string) error {
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM events e
		LEFT JOIN puzzle_content_revisions r
		  ON r.project_id=e.project_id AND r.content_revision=e.properties->>'content_revision'
		WHERE e.project_id=$1 AND e.effective_at>=$2 AND e.effective_at<$3
		  AND ($4::text IS NULL OR e.build_number=$4)
		  AND e.properties ? 'content_revision' AND r.content_revision IS NULL
	`, projectID, from, to, build).Scan(&q.UnknownRevisionEvents); err != nil {
		return err
	}
	return pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3
		  AND ($4::text IS NULL OR build_number=$4)
		  AND name IN ('placement_resolved','detail_released')
		  AND COALESCE(properties->>'coordinate_space','') <> 'house_local'
	`, projectID, from, to, build).Scan(&q.InvalidCoordinateSpaceEvents)
}

// loadQualityDistinctCounts answers item 7: distinct installs, sessions,
// house runs, wave attempts and interactions in scope.
func loadQualityDistinctCounts(ctx context.Context, pool *pgxpool.Pool, q *PuzzleQuality, projectID string, from, to time.Time, build *string) error {
	return pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT install_id),
		       COUNT(DISTINCT (install_id, session_id)),
		       COUNT(DISTINCT properties->>'house_run_id') FILTER (WHERE properties ? 'house_run_id'),
		       COUNT(DISTINCT properties->>'attempt_id') FILTER (WHERE properties ? 'attempt_id'),
		       COUNT(DISTINCT properties->>'interaction_id') FILTER (WHERE properties ? 'interaction_id')
		FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3
		  AND ($4::text IS NULL OR build_number=$4)
	`, projectID, from, to, build).Scan(&q.DistinctInstalls, &q.DistinctSessions, &q.DistinctHouseRuns, &q.DistinctWaveAttempts, &q.DistinctInteractions)
}

// loadQualityDeliveryDelay is item 17: the received_at - sent_at_client
// distribution, not just an average, so a long tail can't hide behind a
// reasonable mean.
func loadQualityDeliveryDelay(ctx context.Context, pool *pgxpool.Pool, q *PuzzleQuality, projectID string, from, to time.Time, build *string) error {
	var samples *int64
	var p50, p90, p99, max *float64
	err := pool.QueryRow(ctx, `
		WITH delays AS (
			SELECT EXTRACT(EPOCH FROM (received_at - sent_at_client)) * 1000 AS delay_ms
			FROM events
			WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3
			  AND ($4::text IS NULL OR build_number=$4)
		)
		SELECT COUNT(*), PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY delay_ms),
		       PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY delay_ms),
		       PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY delay_ms), MAX(delay_ms)
		FROM delays
	`, projectID, from, to, build).Scan(&samples, &p50, &p90, &p99, &max)
	if err != nil {
		return err
	}
	d := PuzzleQualityDelay{}
	if samples != nil {
		d.Samples = *samples
	}
	if p50 != nil {
		d.P50MS = *p50
	}
	if p90 != nil {
		d.P90MS = *p90
	}
	if p99 != nil {
		d.P99MS = *p99
	}
	if max != nil {
		d.MaxMS = *max
	}
	q.DeliveryDelayMS = d
	return nil
}

// loadQualityBuilds is item 18. Always unfiltered by build — it is the
// list the build filter itself is built from.
func loadQualityBuilds(ctx context.Context, pool *pgxpool.Pool, q *PuzzleQuality, projectID string, from, to time.Time) error {
	rows, err := pool.Query(ctx, `
		SELECT app_version, build_number, COUNT(*) FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND build_number <> ''
		GROUP BY 1,2 ORDER BY 3 DESC
	`, projectID, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var b PuzzleQualityBuild
		if err := rows.Scan(&b.AppVersion, &b.BuildNumber, &b.Events); err != nil {
			return err
		}
		q.Builds = append(q.Builds, b)
	}
	return rows.Err()
}
