package analytics

// Sequence/index/interaction/checkpoint/developer-pairing integrity
// queries behind PuzzleQuality (puzzle_quality.go). Split out once the
// main file reached its readable-length ceiling — this half owns
// "did the event chain arrive intact," the other half owns ingestion,
// content and delivery.

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// loadQualityIndexIntegrity is items 8-10: SDK-sequence gaps/duplicates
// within a session, house_event_index within a house run, and
// attempt_event_index within a wave attempt. Ordering by the index itself
// (not effective_at/received_at) is deliberate — a batch delivered late
// but internally contiguous must not read as a sequence failure.
func loadQualityIndexIntegrity(ctx context.Context, pool *pgxpool.Pool, q *PuzzleQuality, projectID string, from, to time.Time, build *string) error {
	var err error
	if q.SequenceGaps, q.SequenceDuplicates, err = querySequenceIntegrity(ctx, pool, projectID, from, to, build); err != nil {
		return err
	}
	var invalid int64
	if q.HouseEventIndexGaps, q.HouseEventIndexDuplicates, invalid, err = queryPropertyIndexIntegrity(ctx, pool, projectID, from, to, build, "house_run_id", "house_event_index"); err != nil {
		return err
	}
	q.InvalidEventIndexes += invalid
	if q.AttemptEventIndexGaps, q.AttemptEventIndexDuplicates, invalid, err = queryPropertyIndexIntegrity(ctx, pool, projectID, from, to, build, "attempt_id", "attempt_event_index"); err != nil {
		return err
	}
	q.InvalidEventIndexes += invalid
	return nil
}

func querySequenceIntegrity(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, build *string) (gaps, dups int64, err error) {
	err = pool.QueryRow(ctx, `
		WITH touched AS (
			SELECT DISTINCT install_id, session_id FROM events
			WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND ($4::text IS NULL OR build_number=$4)
		), ordered AS (
			SELECT e.sequence, LAG(e.sequence) OVER (PARTITION BY e.install_id, e.session_id ORDER BY e.sequence, e.effective_at, e.event_id) AS prev
			FROM events e JOIN touched t USING (install_id, session_id)
			WHERE e.project_id=$1 AND ($4::text IS NULL OR e.build_number=$4)
		)
		SELECT COALESCE(SUM(GREATEST(sequence-prev-1,0)) FILTER (WHERE prev IS NOT NULL AND sequence>prev),0),
		       COUNT(*) FILTER (WHERE prev IS NOT NULL AND sequence=prev)
		FROM ordered
	`, projectID, from, to, build).Scan(&gaps, &dups)
	return
}

// queryPropertyIndexIntegrity is the shared shape behind house_event_index
// and attempt_event_index: both are a monotonic counter scoped by a
// property-carried ID rather than a real column, so they can't reuse
// querySequenceIntegrity's PARTITION BY directly. scopeKey/indexKey are
// call-site literals (never user input), so building the query text is
// safe.
func queryPropertyIndexIntegrity(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, build *string, scopeKey, indexKey string) (gaps, dups, invalid int64, err error) {
	sql := fmt.Sprintf(`
		WITH touched AS (
			SELECT DISTINCT install_id, properties->>'%s' AS scope_id
			FROM events
			WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND ($4::text IS NULL OR build_number=$4)
			  AND properties ? '%s'
		), scoped AS (
			SELECT e.install_id, e.properties->>'%s' AS scope_id,
			       CASE WHEN properties->>'%s' ~ '^[0-9]+$' THEN (properties->>'%s')::int END AS idx
			FROM events e JOIN touched t ON t.install_id=e.install_id AND t.scope_id=e.properties->>'%s'
			WHERE e.project_id=$1 AND ($4::text IS NULL OR e.build_number=$4)
			  AND e.properties ? '%s'
		), ordered AS (
			SELECT idx, LAG(idx) OVER (PARTITION BY install_id, scope_id ORDER BY idx) AS prev FROM scoped WHERE idx IS NOT NULL
		)
		SELECT COALESCE(SUM(GREATEST(idx-prev-1,0)) FILTER (WHERE prev IS NOT NULL AND idx>prev),0),
		       COUNT(*) FILTER (WHERE prev IS NOT NULL AND idx=prev),
		       (SELECT COUNT(*) FROM scoped WHERE idx IS NULL)
		FROM ordered
	`, scopeKey, scopeKey, scopeKey, indexKey, indexKey, scopeKey, indexKey)
	err = pool.QueryRow(ctx, sql, projectID, from, to, build).Scan(&gaps, &dups, &invalid)
	return
}

// loadQualityInteractionAndCheckpointIntegrity is items 11 and 12. An
// interaction is only orphaned if it never reaches a terminal event at
// all — a fall whose return is a later detail_returned with the same
// interaction_id is complete, not orphaned (see the plan doc's explicit
// warning against the naive check).
func loadQualityInteractionAndCheckpointIntegrity(ctx context.Context, pool *pgxpool.Pool, q *PuzzleQuality, projectID string, from, to time.Time, build *string) error {
	rows, err := pool.Query(ctx, `
		WITH touched AS (
			SELECT DISTINCT install_id, properties->>'interaction_id' AS interaction_id
			FROM events
			WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND ($4::text IS NULL OR build_number=$4)
			  AND properties ? 'interaction_id'
		)
		SELECT
		  BOOL_OR(name='detail_taken'),
		  BOOL_OR(name='detail_released'),
		  BOOL_OR(name='placement_resolved'),
		  BOOL_OR(name='placement_resolved' AND properties->>'outcome'='placed'),
		  BOOL_OR(name='detail_returned'),
		  BOOL_OR(name='interaction_abandoned')
		FROM events e JOIN touched t ON t.install_id=e.install_id AND t.interaction_id=e.properties->>'interaction_id'
		WHERE e.project_id=$1 AND ($4::text IS NULL OR e.build_number=$4)
		GROUP BY e.install_id, e.properties->>'interaction_id'
	`, projectID, from, to, build)
	if err != nil {
		return err
	}
	var orphans int64
	for rows.Next() {
		var hasTake, hasRelease, hasResolution, placed, hasReturn, hasAbandoned bool
		if err := rows.Scan(&hasTake, &hasRelease, &hasResolution, &placed, &hasReturn, &hasAbandoned); err != nil {
			rows.Close()
			return err
		}
		terminal := placed || hasReturn || hasAbandoned
		if !hasTake || !hasRelease || !hasResolution || !terminal {
			orphans++
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	q.OrphanInteractions = orphans

	checkpoints, err := pool.Query(ctx, `
		SELECT properties->>'placed_block_ids', properties->>'placed_state_hash'
		FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND ($4::text IS NULL OR build_number=$4)
		  AND name='state_checkpoint' AND COALESCE(properties->>'placed_state_hash','')<>''
	`, projectID, from, to, build)
	if err != nil {
		return err
	}
	defer checkpoints.Close()
	var mismatches int64
	for checkpoints.Next() {
		var ids, hash string
		if err := checkpoints.Scan(&ids, &hash); err != nil {
			return err
		}
		if hash != fmt.Sprintf("%x", sha256.Sum256([]byte(ids))) {
			mismatches++
		}
	}
	q.CheckpointMismatches = mismatches
	return checkpoints.Err()
}

// loadQualityRecoveredAndOpenRuns is items 13 and 14. "Recent" reuses
// lostThresholdHours (puzzle_diagnostics.go) rather than inventing a
// second lost/stale boundary, per the plan doc's instruction.
func loadQualityRecoveredAndOpenRuns(ctx context.Context, pool *pgxpool.Pool, q *PuzzleQuality, projectID string, from, to time.Time, build *string) error {
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT properties->>'attempt_id') FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND ($4::text IS NULL OR build_number=$4)
		  AND name='attempt_recovered'
	`, projectID, from, to, build).Scan(&q.RecoveredAttempts); err != nil {
		return err
	}

	if err := pool.QueryRow(ctx, `
		WITH touched AS (
			SELECT DISTINCT install_id, properties->>'house_run_id' AS run_id FROM events
			WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND ($4::text IS NULL OR build_number=$4)
			  AND properties ? 'house_run_id'
		), runs AS (
			SELECT e.install_id, e.properties->>'house_run_id' AS run_id, MAX(e.effective_at) AS last_event_at, BOOL_OR(e.name='house_completed') AS completed
			FROM events e JOIN touched t ON t.install_id=e.install_id AND t.run_id=e.properties->>'house_run_id'
			WHERE e.project_id=$1 AND ($4::text IS NULL OR e.build_number=$4)
			GROUP BY 1,2
		)
		SELECT COUNT(*) FILTER (WHERE NOT completed AND now()-last_event_at < make_interval(hours=>$5::int)),
		       COUNT(*) FILTER (WHERE NOT completed AND now()-last_event_at >= make_interval(hours=>$5::int))
		FROM runs
	`, projectID, from, to, build, lostThresholdHours).Scan(&q.OpenHouseRunsRecent, &q.OpenHouseRunsStale); err != nil {
		return err
	}

	return pool.QueryRow(ctx, `
		WITH touched AS (
			SELECT DISTINCT install_id, properties->>'attempt_id' AS attempt_id FROM events
			WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND ($4::text IS NULL OR build_number=$4)
			  AND properties ? 'attempt_id'
		), attempts AS (
			SELECT e.install_id, e.properties->>'attempt_id' AS attempt_id, MAX(e.effective_at) AS last_event_at,
			       BOOL_OR(e.name IN ('wave_completed','house_completed')) AS completed,
			       BOOL_OR(e.name='attempt_closed') AS closed
			FROM events e JOIN touched t ON t.install_id=e.install_id AND t.attempt_id=e.properties->>'attempt_id'
			WHERE e.project_id=$1 AND ($4::text IS NULL OR e.build_number=$4)
			GROUP BY 1,2
		)
		SELECT COUNT(*) FILTER (WHERE NOT completed AND NOT closed AND now()-last_event_at < make_interval(hours=>$5::int)),
		       COUNT(*) FILTER (WHERE NOT completed AND NOT closed AND now()-last_event_at >= make_interval(hours=>$5::int))
		FROM attempts
	`, projectID, from, to, build, lostThresholdHours).Scan(&q.OpenAttemptsRecent, &q.OpenAttemptsStale)
}

// loadQualityDeveloperPairing is items 15 and 16: a developer command with
// no terminal result, and a progress mutation with no matching start.
func loadQualityDeveloperPairing(ctx context.Context, pool *pgxpool.Pool, q *PuzzleQuality, projectID string, from, to time.Time, build *string) error {
	return pool.QueryRow(ctx, `
		WITH touched AS (
			SELECT DISTINCT install_id, properties->>'developer_action_id' AS action_id FROM events
			WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND ($4::text IS NULL OR build_number=$4)
			  AND properties ? 'developer_action_id'
		), actions AS (
			SELECT e.install_id, e.properties->>'developer_action_id' AS action_id,
			       BOOL_OR(e.name='developer_command_started') AS has_start,
			       BOOL_OR(e.name IN ('developer_command_completed','developer_command_failed','developer_command_noop')) AS has_terminal,
			       BOOL_OR(e.name='developer_progress_mutated') AS has_mutation
			FROM events e JOIN touched t ON t.install_id=e.install_id AND t.action_id=e.properties->>'developer_action_id'
			WHERE e.project_id=$1 AND ($4::text IS NULL OR e.build_number=$4)
			GROUP BY 1,2
		)
		SELECT COUNT(*) FILTER (WHERE has_start AND NOT has_terminal),
		       COUNT(*) FILTER (WHERE has_mutation AND NOT has_start)
		FROM actions
	`, projectID, from, to, build).Scan(&q.UnpairedDeveloperCommands, &q.UnpairedDeveloperMutations)
}
