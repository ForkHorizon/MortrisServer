package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type GameplaySummary struct {
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

func GetGameplayDiagnostics(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time, loc *time.Location, filter GameplayFilter) (*GameplayDiagnostics, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	result := &GameplayDiagnostics{Scopes: []GameplayScope{}, Friction: []GameplayFriction{}, Daily: []GameplayDaily{}}
	if err := loadGameplaySummary(ctx, pool, result, projectID, from, to, filter); err != nil {
		return nil, err
	}
	if err := loadGameplayScopes(ctx, pool, result, projectID, from, to, filter); err != nil {
		return nil, err
	}
	if err := loadGameplayFriction(ctx, pool, result, projectID, from, to, filter); err != nil {
		return nil, err
	}
	if err := loadGameplayDaily(ctx, pool, result, projectID, from, to, loc, filter); err != nil {
		return nil, err
	}
	return result, nil
}

func loadGameplaySummary(ctx context.Context, pool *pgxpool.Pool, result *GameplayDiagnostics, projectID string, from, to time.Time, filter GameplayFilter) error {
	// The maxima are client boundary snapshots; wall time uses server-normalized timestamps.
	return pool.QueryRow(ctx, `WITH e AS (SELECT name, effective_at, properties FROM events WHERE project_id=$1 AND event_kind='product' AND effective_at >= $2 AND effective_at < $3 AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6)), a AS (SELECT properties->>'attempt_id' attempt_id, MAX((properties->>'active_elapsed_ms')::bigint) active_ms, EXTRACT(EPOCH FROM MAX(effective_at)-MIN(effective_at))*1000 wall_ms, COUNT(*) FILTER (WHERE name='app_backgrounded') pauses FROM e WHERE properties ? 'attempt_id' GROUP BY 1) SELECT (SELECT COUNT(DISTINCT attempt_id) FROM a), (SELECT COUNT(*) FROM e WHERE name='placement_resolved'), (SELECT COUNT(*) FROM e WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'), (SELECT COUNT(*) FROM e WHERE name='hint_used'), (SELECT COUNT(*) FROM e WHERE name='wave_completed'), (SELECT COUNT(*) FROM e WHERE name='house_completed'), COALESCE((SELECT SUM(active_ms) FROM a),0), COALESCE((SELECT SUM(wall_ms) FROM a),0)::bigint, COALESCE((SELECT SUM(pauses) FROM a),0), COALESCE((SELECT SUM(GREATEST(wall_ms-active_ms,0)) FROM a),0)::bigint`, projectID, from, to, filter.CityID, filter.HouseID, filter.WaveIndex).Scan(&result.Summary.Attempts, &result.Summary.Placements, &result.Summary.Falls, &result.Summary.Hints, &result.Summary.CompletedWaves, &result.Summary.CompletedHouses, &result.Summary.ActiveElapsedMS, &result.Summary.WallElapsedMS, &result.Summary.PauseCount, &result.Summary.PauseElapsedMS)
}

func loadGameplayScopes(ctx context.Context, pool *pgxpool.Pool, result *GameplayDiagnostics, projectID string, from, to time.Time, filter GameplayFilter) error {
	rows, err := pool.Query(ctx, `WITH e AS (SELECT name,effective_at,properties FROM events WHERE project_id=$1 AND effective_at >=$2 AND effective_at<$3 AND properties ? 'attempt_id' AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6)), a AS (SELECT (properties->>'city_id')::int city_id,(properties->>'house_id')::int house_id,(properties->>'wave_index')::int wave_index,properties->>'attempt_id' attempt_id,MAX((properties->>'active_elapsed_ms')::bigint) active_ms,EXTRACT(EPOCH FROM MAX(effective_at)-MIN(effective_at))*1000 wall_ms,COUNT(*) FILTER (WHERE name='app_backgrounded') pauses FROM e GROUP BY 1,2,3,4), m AS (SELECT city_id,house_id,wave_index,COUNT(*) attempts,COALESCE(SUM(active_ms),0)::bigint active_ms,COALESCE(SUM(wall_ms),0)::bigint wall_ms,COALESCE(SUM(pauses),0)::bigint pauses,COALESCE(SUM(GREATEST(wall_ms-active_ms,0)),0)::bigint pause_ms FROM a GROUP BY 1,2,3), c AS (SELECT (properties->>'city_id')::int city_id,(properties->>'house_id')::int house_id,(properties->>'wave_index')::int wave_index,COUNT(*) FILTER (WHERE name='placement_resolved') placements,COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%') falls,COUNT(*) FILTER (WHERE name='hint_used') hints,COUNT(*) FILTER (WHERE name='wave_completed') completed_waves,COUNT(*) FILTER (WHERE name='house_completed') completed_houses FROM e GROUP BY 1,2,3) SELECT m.city_id,m.house_id,m.wave_index,m.attempts,c.placements,c.falls,c.hints,c.completed_waves,c.completed_houses,m.active_ms,m.wall_ms,m.pauses,m.pause_ms FROM m JOIN c USING (city_id,house_id,wave_index) ORDER BY 1,2,3 LIMIT 500`, projectID, from, to, filter.CityID, filter.HouseID, filter.WaveIndex)
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

func loadGameplayFriction(ctx context.Context, pool *pgxpool.Pool, result *GameplayDiagnostics, projectID string, from, to time.Time, filter GameplayFilter) error {
	rows, err := pool.Query(ctx, `WITH p AS (SELECT properties, ROW_NUMBER() OVER (PARTITION BY properties->>'attempt_id', properties->>'block_id', properties->>'candidate_target_id' ORDER BY effective_at) ordinal FROM events WHERE project_id=$1 AND name='placement_resolved' AND effective_at >=$2 AND effective_at<$3 AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6)) SELECT (properties->>'block_id')::int, COALESCE((properties->>'candidate_target_id')::int,-1),COUNT(*),COUNT(*) FILTER (WHERE properties->>'outcome'='placed'),COUNT(*) FILTER (WHERE properties->>'outcome' LIKE 'fell_%'),COUNT(*) FILTER (WHERE ordinal=1 AND properties->>'outcome' LIKE 'fell_%') FROM p GROUP BY 1,2 ORDER BY 5 DESC,3 DESC LIMIT 500`, projectID, from, to, filter.CityID, filter.HouseID, filter.WaveIndex)
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

func loadGameplayDaily(ctx context.Context, pool *pgxpool.Pool, result *GameplayDiagnostics, projectID string, from, to time.Time, loc *time.Location, filter GameplayFilter) error {
	rows, err := pool.Query(ctx, `SELECT TO_CHAR(effective_at AT TIME ZONE $7,'YYYY-MM-DD'),(properties->>'city_id')::int,(properties->>'house_id')::int,(properties->>'wave_index')::int,properties->>'content_revision',build_number,COUNT(DISTINCT properties->>'attempt_id'),COUNT(*) FILTER (WHERE name='placement_resolved'),COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'),COUNT(*) FILTER (WHERE name='hint_used') FROM events WHERE project_id=$1 AND effective_at >=$2 AND effective_at<$3 AND properties ? 'attempt_id' AND ($4::int IS NULL OR (properties->>'city_id')::int=$4) AND ($5::int IS NULL OR (properties->>'house_id')::int=$5) AND ($6::int IS NULL OR (properties->>'wave_index')::int=$6) GROUP BY 1,2,3,4,5,6 ORDER BY 1 DESC,2,3,4 LIMIT 1000`, projectID, from, to, filter.CityID, filter.HouseID, filter.WaveIndex, loc.String())
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
