package analytics

// loadGameplayScopes/Friction/Daily — split out of puzzle_diagnostics.go
// once it hit the repo's 300-line file gate. All three share the same
// Stage 3 split as the rest of that file: attempt/duration counts are
// gated to eligible (fully natural, per scope) attempts, placement-
// quality counts stay event-level. See puzzle_traffic.go.

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// loadGameplayScopes is the same summary split by (city, house, wave):
// m is attempt/duration metrics gated to eligible attempts, c is
// placement-quality counts and stays event-level.
func loadGameplayScopes(ctx context.Context, pool *pgxpool.Pool, result *GameplayDiagnostics, projectID string, from, to time.Time, filter GameplayFilter, scope TrafficScope) error {
	query := `WITH ` + eligibleAttemptsCTE(scope) + `, g AS (
		SELECT (properties->>'city_id')::int city_id, (properties->>'house_id')::int house_id, (properties->>'wave_index')::int wave_index,
		       properties->>'attempt_id' attempt_id, name,
		       (properties->>'active_elapsed_ms')::bigint active_elapsed_ms,
		       LEAST(GREATEST(EXTRACT(EPOCH FROM effective_at - LAG(effective_at) OVER (PARTITION BY properties->>'attempt_id' ORDER BY effective_at))*1000, 0), $7::bigint) gap_ms
		FROM events
		WHERE project_id=$1 AND event_kind='product' AND effective_at>=$2 AND effective_at<$3
		  AND properties->>'attempt_id' IN (SELECT attempt_id FROM eligible)
	), a AS (
		SELECT city_id, house_id, wave_index, attempt_id, MAX(active_elapsed_ms) active_ms, COALESCE(SUM(gap_ms),0) wall_ms,
		       COUNT(*) FILTER (WHERE name='app_backgrounded') pauses
		FROM g GROUP BY 1,2,3,4
	), m AS (
		SELECT city_id, house_id, wave_index, COUNT(*) attempts, COALESCE(SUM(active_ms),0)::bigint active_ms,
		       COALESCE(SUM(wall_ms),0)::bigint wall_ms, COALESCE(SUM(pauses),0)::bigint pauses,
		       COALESCE(SUM(GREATEST(wall_ms-active_ms,0)),0)::bigint pause_ms
		FROM a GROUP BY 1,2,3
	), c AS (
		SELECT (properties->>'city_id')::int city_id, (properties->>'house_id')::int house_id, (properties->>'wave_index')::int wave_index,
		       COUNT(*) FILTER (WHERE name='placement_resolved') placements,
		       COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%') falls,
		       COUNT(*) FILTER (WHERE name='hint_used') hints,
		       COUNT(*) FILTER (WHERE name='wave_completed') completed_waves,
		       COUNT(*) FILTER (WHERE name='house_completed') completed_houses
		FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND properties ? 'attempt_id'
		  AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6)
		  AND ` + scope.eventPredicate() + `
		GROUP BY 1,2,3
	)
	SELECT m.city_id, m.house_id, m.wave_index, m.attempts, c.placements, c.falls, c.hints, c.completed_waves, c.completed_houses, m.active_ms, m.wall_ms, m.pauses, m.pause_ms
	FROM m JOIN c USING (city_id,house_id,wave_index) ORDER BY 1,2,3 LIMIT 500`
	rows, err := pool.Query(ctx, query, projectID, from, to, filter.CityID, filter.HouseID, filter.WaveIndex, pauseGapCapMS)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var row GameplayScope
		if err := rows.Scan(&row.CityID, &row.HouseID, &row.WaveIndex, &row.Attempts, &row.Placements, &row.Falls, &row.Hints, &row.CompletedWaves, &row.CompletedHouses, &row.ActiveElapsedMS, &row.WallElapsedMS, &row.PauseCount, &row.PauseElapsedMS); err != nil {
			return err
		}
		result.Scopes = append(result.Scopes, row)
	}
	return rows.Err()
}

// loadGameplayFriction stays event-level throughout — a per-block/target
// fall rate is placement-quality (plan section 6, rule 1), not gated by
// whole-attempt eligibility.
func loadGameplayFriction(ctx context.Context, pool *pgxpool.Pool, result *GameplayDiagnostics, projectID string, from, to time.Time, filter GameplayFilter, scope TrafficScope) error {
	query := `WITH p AS (
		SELECT properties, ROW_NUMBER() OVER (PARTITION BY properties->>'attempt_id', properties->>'block_id', properties->>'candidate_target_id' ORDER BY effective_at) ordinal
		FROM events
		WHERE project_id=$1 AND name='placement_resolved' AND effective_at>=$2 AND effective_at<$3
		  AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6)
		  AND ` + scope.eventPredicate() + `
	)
	SELECT (properties->>'block_id')::int, COALESCE((properties->>'candidate_target_id')::int,-1), COUNT(*),
	       COUNT(*) FILTER (WHERE properties->>'outcome'='placed'), COUNT(*) FILTER (WHERE properties->>'outcome' LIKE 'fell_%'),
	       COUNT(*) FILTER (WHERE ordinal=1 AND properties->>'outcome' LIKE 'fell_%')
	FROM p GROUP BY 1,2 ORDER BY 5 DESC,3 DESC LIMIT 500`
	rows, err := pool.Query(ctx, query, projectID, from, to, filter.CityID, filter.HouseID, filter.WaveIndex)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var row GameplayFriction
		var firstFailures int64
		if err := rows.Scan(&row.BlockID, &row.TargetID, &row.Attempts, &row.Placements, &row.Falls, &firstFailures); err != nil {
			return err
		}
		if row.Attempts > 0 {
			row.FallRate = float64(row.Falls) / float64(row.Attempts)
			row.FirstAttemptFailureRate = float64(firstFailures) / float64(row.Attempts)
		}
		result.Friction = append(result.Friction, row)
	}
	return rows.Err()
}

// loadGameplayDaily's attempts column is gated to eligible attempts per
// day (a trend of natural play, matching every other completion-shaped
// count); placements/falls/hints stay event-level.
func loadGameplayDaily(ctx context.Context, pool *pgxpool.Pool, result *GameplayDiagnostics, projectID string, from, to time.Time, loc *time.Location, filter GameplayFilter, scope TrafficScope) error {
	query := `WITH ` + eligibleAttemptsCTE(scope) + `
	SELECT TO_CHAR(effective_at AT TIME ZONE $7,'YYYY-MM-DD'), (properties->>'city_id')::int, (properties->>'house_id')::int, (properties->>'wave_index')::int,
	       properties->>'content_revision', build_number,
	       COUNT(DISTINCT properties->>'attempt_id') FILTER (WHERE properties->>'attempt_id' IN (SELECT attempt_id FROM eligible)),
	       COUNT(*) FILTER (WHERE name='placement_resolved'),
	       COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'),
	       COUNT(*) FILTER (WHERE name='hint_used')
	FROM events
	WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND properties ? 'attempt_id'
	  AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6)
	  AND ` + scope.eventPredicate() + `
	GROUP BY 1,2,3,4,5,6 ORDER BY 1 DESC,2,3,4 LIMIT 1000`
	rows, err := pool.Query(ctx, query, projectID, from, to, filter.CityID, filter.HouseID, filter.WaveIndex, loc.String())
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var row GameplayDaily
		if err := rows.Scan(&row.Day, &row.CityID, &row.HouseID, &row.WaveIndex, &row.ContentRevision, &row.BuildNumber, &row.Attempts, &row.Placements, &row.Falls, &row.Hints); err != nil {
			return err
		}
		result.Daily = append(result.Daily, row)
	}
	return rows.Err()
}
