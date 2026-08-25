package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// pauseGapCapMS caps how much a single gap between two consecutive
// events in the same attempt can contribute to wall time. Without this,
// an attempt left backgrounded for days (app never force-closed) inflates
// the whole-project wall-time sum by that many days, even though only a
// few minutes were actually played — the aggregate stops meaning
// anything. 30 minutes matches the common web-analytics session-timeout
// convention.
const pauseGapCapMS = int64(30 * time.Minute / time.Millisecond)

// lostThresholdHours: an attempt with no wave_completed/house_completed
// and no attempt_closed is "in_progress" if its last event is more
// recent than this, else "lost" — the app was almost certainly killed by
// the OS mid-attempt and never resumed, rather than genuinely still
// being played. Measured against the real current time, not the query's
// `to` bound, since an attempt can't still be "in progress" relative to
// a past date range.
const lostThresholdHours = 24

// GameplayOutcomeStat breaks ActiveElapsedMS/WallElapsedMS/PauseElapsedMS
// down by how an attempt ended, since those durations are only
// meaningful once an attempt has actually settled (completed or
// explicitly closed) — see GameplaySummary's own fields for the
// settled-only rollup, and lostThresholdHours' doc comment for why
// "no terminal event yet" needs its own in_progress/lost split.
type GameplayOutcomeStat struct {
	Outcome         string `json:"outcome"`
	Attempts        int64  `json:"attempts"`
	ActiveElapsedMS int64  `json:"active_elapsed_ms"`
	WallElapsedMS   int64  `json:"wall_elapsed_ms"`
	PauseCount      int64  `json:"pause_count"`
	PauseElapsedMS  int64  `json:"pause_elapsed_ms"`
}

type GameplaySummary struct {
	Attempts        int64 `json:"attempts"`
	Placements      int64 `json:"placements"`
	Falls           int64 `json:"falls"`
	Hints           int64 `json:"hints"`
	CompletedWaves  int64 `json:"completed_waves"`
	CompletedHouses int64 `json:"completed_houses"`
	// ActiveElapsedMS, WallElapsedMS, and PauseElapsedMS are summed over
	// completed and gave_up attempts only (see ByOutcome) — an attempt
	// that's still in progress or was silently lost doesn't have a real
	// final duration yet, and including it would reintroduce the same
	// "meaningless aggregate" problem pauseGapCapMS was added to fix.
	ActiveElapsedMS int64                 `json:"active_elapsed_ms"`
	WallElapsedMS   int64                 `json:"wall_elapsed_ms"`
	PauseCount      int64                 `json:"pause_count"`
	PauseElapsedMS  int64                 `json:"pause_elapsed_ms"`
	ByOutcome       []GameplayOutcomeStat `json:"by_outcome"`
}

type GameplayScope struct {
	CityID          int   `json:"city_id"`
	HouseID         int   `json:"house_id"`
	WaveIndex       int   `json:"wave_index"`
	Attempts        int64 `json:"attempts"`
	Placements      int64 `json:"placements"`
	Falls           int64 `json:"falls"`
	Hints           int64 `json:"hints"`
	CompletedWaves  int64 `json:"completed_waves"`
	CompletedHouses int64 `json:"completed_houses"`
	ActiveElapsedMS int64 `json:"active_elapsed_ms"`
	WallElapsedMS   int64 `json:"wall_elapsed_ms"`
	PauseCount      int64 `json:"pause_count"`
	PauseElapsedMS  int64 `json:"pause_elapsed_ms"`
}

type GameplayFriction struct {
	BlockID                 int     `json:"block_id"`
	TargetID                int     `json:"target_id"`
	Attempts                int64   `json:"attempts"`
	Placements              int64   `json:"placements"`
	Falls                   int64   `json:"falls"`
	Hints                   int64   `json:"hints"`
	FallRate                float64 `json:"fall_rate"`
	FirstAttemptFailureRate float64 `json:"first_attempt_failure_rate"`
}

type GameplayDaily struct {
	Day             string `json:"day"`
	CityID          int    `json:"city_id"`
	HouseID         int    `json:"house_id"`
	WaveIndex       int    `json:"wave_index"`
	ContentRevision string `json:"content_revision"`
	BuildNumber     string `json:"build_number"`
	Attempts        int64  `json:"attempts"`
	Placements      int64  `json:"placements"`
	Falls           int64  `json:"falls"`
	Hints           int64  `json:"hints"`
}

type GameplayDiagnostics struct {
	Summary  GameplaySummary    `json:"summary"`
	Scopes   []GameplayScope    `json:"scopes"`
	Friction []GameplayFriction `json:"friction"`
	Daily    []GameplayDaily    `json:"daily"`
}

func GetGameplayDiagnostics(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location, filter GameplayFilter, scope TrafficScope) (*GameplayDiagnostics, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	result := &GameplayDiagnostics{
		Summary:  GameplaySummary{ByOutcome: []GameplayOutcomeStat{}},
		Scopes:   []GameplayScope{},
		Friction: []GameplayFriction{},
		Daily:    []GameplayDaily{},
	}
	if err := loadGameplaySummary(ctx, pool, result, projectID, from, to, filter, scope); err != nil {
		return nil, err
	}
	if err := loadGameplayOutcomes(ctx, pool, result, projectID, from, to, filter, scope); err != nil {
		return nil, err
	}
	if err := loadGameplayScopes(ctx, pool, result, projectID, from, to, filter, scope); err != nil {
		return nil, err
	}
	if err := loadGameplayFriction(ctx, pool, result, projectID, from, to, filter, scope); err != nil {
		return nil, err
	}
	if err := loadGameplayDaily(ctx, pool, result, projectID, from, to, loc, filter, scope); err != nil {
		return nil, err
	}
	return result, nil
}

// eligibleAttemptsCTE is the run-level half every duration/completion
// query below joins against — see puzzle_traffic.go. Placement-quality
// counts (placements/falls/hints) never join it and stay event-level.
func eligibleAttemptsCTE(scope TrafficScope) string {
	return `eligible AS (
		SELECT attempt_id, house_id FROM puzzle_wave_attempt_classification
		WHERE project_id=$1 AND last_event_at>=$2 AND last_event_at<$3
		  AND ($4::int IS NULL OR city_id=$4) AND ($5::int IS NULL OR house_id=$5) AND ($6::int IS NULL OR wave_index=$6)
		  AND ` + scope.runPredicate("fully_natural") + `
	)`
}

// loadGameplaySummary splits two populations (plan section 6): e is
// event-level scoped and backs placements/falls/hints; eligible/g/a are
// gated to whole attempts in scope and back attempts/completions/
// durations. The maxima are client boundary snapshots; wall time uses
// server-normalized timestamps, summing consecutive-event gaps within an
// attempt with each gap capped at pauseGapCapMS (see its doc comment).
// active_ms/wall_ms/pause_ms are summed only over attempts that
// completed or were explicitly closed — see GameplaySummary.
// ActiveElapsedMS's doc comment. PauseCount (a raw event count, not a
// duration) still covers every eligible attempt regardless of outcome.
func loadGameplaySummary(ctx context.Context, pool *pgxpool.Pool, result *GameplayDiagnostics, projectID string, from, to time.Time, filter GameplayFilter, scope TrafficScope) error {
	query := `WITH e AS (
		SELECT name, effective_at, properties FROM events
		WHERE project_id=$1 AND event_kind='product' AND effective_at>=$2 AND effective_at<$3
		  AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6)
		  AND ` + scope.eventPredicate() + `
	), ` + eligibleAttemptsCTE(scope) + `, g AS (
		SELECT properties->>'attempt_id' attempt_id, name,
		       (properties->>'active_elapsed_ms')::bigint active_elapsed_ms,
		       LEAST(GREATEST(EXTRACT(EPOCH FROM effective_at - LAG(effective_at) OVER (PARTITION BY properties->>'attempt_id' ORDER BY effective_at))*1000, 0), $7::bigint) gap_ms
		FROM events
		WHERE project_id=$1 AND event_kind='product' AND effective_at>=$2 AND effective_at<$3
		  AND properties->>'attempt_id' IN (SELECT attempt_id FROM eligible)
	), a AS (
		SELECT attempt_id, MAX(active_elapsed_ms) active_ms, COALESCE(SUM(gap_ms),0)::bigint wall_ms,
		       COUNT(*) FILTER (WHERE name='app_backgrounded') pauses,
		       BOOL_OR(name IN ('wave_completed','house_completed')) completed,
		       BOOL_OR(name='attempt_closed') closed
		FROM g GROUP BY 1
	)
	SELECT
	  (SELECT COUNT(*) FROM eligible),
	  (SELECT COUNT(*) FROM e WHERE name='placement_resolved'),
	  (SELECT COUNT(*) FROM e WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'),
	  (SELECT COUNT(*) FROM e WHERE name='hint_used'),
	  (SELECT COUNT(*) FROM g WHERE name='wave_completed'),
	  (SELECT COUNT(*) FROM g WHERE name='house_completed'),
	  COALESCE((SELECT SUM(active_ms) FROM a WHERE completed OR closed),0),
	  COALESCE((SELECT SUM(wall_ms) FROM a WHERE completed OR closed),0)::bigint,
	  COALESCE((SELECT SUM(pauses) FROM a),0),
	  COALESCE((SELECT SUM(GREATEST(wall_ms-active_ms,0)) FROM a WHERE completed OR closed),0)::bigint`
	return pool.QueryRow(ctx, query, projectID, from, to, filter.CityID, filter.HouseID, filter.WaveIndex, pauseGapCapMS).Scan(
		&result.Summary.Attempts, &result.Summary.Placements, &result.Summary.Falls, &result.Summary.Hints,
		&result.Summary.CompletedWaves, &result.Summary.CompletedHouses,
		&result.Summary.ActiveElapsedMS, &result.Summary.WallElapsedMS, &result.Summary.PauseCount, &result.Summary.PauseElapsedMS)
}

// loadGameplayOutcomes buckets eligible attempts by how they ended —
// completed, gave_up (closed without completing), in_progress, or lost
// (see lostThresholdHours) — so the dashboard can show settled-attempt
// duration separately from attempts that don't have a final duration
// yet, and surface "lost" as its own churn signal instead of silently
// folding it into (or out of) the other numbers.
func loadGameplayOutcomes(ctx context.Context, pool *pgxpool.Pool, result *GameplayDiagnostics, projectID string, from, to time.Time, filter GameplayFilter, scope TrafficScope) error {
	query := `WITH ` + eligibleAttemptsCTE(scope) + `, g AS (
		SELECT properties->>'attempt_id' attempt_id, name, effective_at,
		       (properties->>'active_elapsed_ms')::bigint active_elapsed_ms,
		       LEAST(GREATEST(EXTRACT(EPOCH FROM effective_at - LAG(effective_at) OVER (PARTITION BY properties->>'attempt_id' ORDER BY effective_at))*1000, 0), $7::bigint) gap_ms
		FROM events
		WHERE project_id=$1 AND event_kind='product' AND effective_at>=$2 AND effective_at<$3
		  AND properties->>'attempt_id' IN (SELECT attempt_id FROM eligible)
	), a AS (
		SELECT attempt_id, MAX(active_elapsed_ms) active_ms, COALESCE(SUM(gap_ms),0)::bigint wall_ms,
		       COUNT(*) FILTER (WHERE name='app_backgrounded') pauses, MAX(effective_at) last_event_at,
		       BOOL_OR(name IN ('wave_completed','house_completed')) completed,
		       BOOL_OR(name='attempt_closed') closed
		FROM g GROUP BY 1
	), o AS (
		SELECT active_ms, wall_ms, pauses,
		       CASE WHEN completed THEN 'completed' WHEN closed THEN 'gave_up'
		            WHEN now()-last_event_at < make_interval(hours => $8::int) THEN 'in_progress' ELSE 'lost' END outcome
		FROM a
	)
	SELECT outcome, COUNT(*), COALESCE(SUM(active_ms),0), COALESCE(SUM(wall_ms),0)::bigint, COALESCE(SUM(pauses),0), COALESCE(SUM(GREATEST(wall_ms-active_ms,0)),0)::bigint
	FROM o GROUP BY 1 ORDER BY 1`
	rows, err := pool.Query(ctx, query, projectID, from, to, filter.CityID, filter.HouseID, filter.WaveIndex, pauseGapCapMS, lostThresholdHours)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var row GameplayOutcomeStat
		if err := rows.Scan(&row.Outcome, &row.Attempts, &row.ActiveElapsedMS, &row.WallElapsedMS, &row.PauseCount, &row.PauseElapsedMS); err != nil {
			return err
		}
		result.Summary.ByOutcome = append(result.Summary.ByOutcome, row)
	}
	return rows.Err()
}

// loadGameplayScopes, loadGameplayFriction and loadGameplayDaily live in
// puzzle_diagnostics_scopes.go — split out once this file hit the repo's
// 300-line gate.
