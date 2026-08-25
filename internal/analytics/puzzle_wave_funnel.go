package analytics

// Stage 4's wave staircase/funnel (docs/puzzle-analytics-remaining-plan.md
// section 7). The unit is house_run_id, not attempt_id or a placement
// count: "entered wave N" means the run has any event carrying that wave
// index, including a run that entered and placed nothing before giving
// up — exactly the drop-off the funnel exists to show. Reuses
// lostThresholdHours (puzzle_diagnostics.go) for the abandoned/recent
// split rather than inventing a second definition.

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PuzzleWaveFunnelStep struct {
	WaveIndex int `json:"wave_index"`
	// Entered is every fully-in-scope house run that ever carried an
	// event with this wave_index — not a placement count.
	Entered   int64 `json:"entered"`
	Completed int64 `json:"completed"`
	// ReachedNext is next step's Entered, copied here so the dashboard
	// doesn't have to look ahead in the array to draw one transition.
	ReachedNext int64 `json:"reached_next"`
	// AbandonedHere/RecentHere partition runs whose furthest wave was
	// this one and which never completed the house: AbandonedHere is
	// past lostThresholdHours (stale/lost), RecentHere is within it
	// (still might come back).
	AbandonedHere int64 `json:"abandoned_here"`
	RecentHere    int64 `json:"recent_here"`
}

type PuzzleWaveFunnel struct {
	TotalWaves         int                    `json:"total_waves"`
	Steps              []PuzzleWaveFunnelStep `json:"steps"`
	LostThresholdHours int                    `json:"lost_threshold_hours"`
}

func GetPuzzleWaveFunnel(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, from, to time.Time, scope TrafficScope, build ...*string) (*PuzzleWaveFunnel, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	selectedBuild := optionalBuild(build)
	revision, err := latestPuzzleRevision(ctx, pool, projectID)
	if err != nil {
		return nil, err
	}
	var totalWaves int
	if err := pool.QueryRow(ctx, `SELECT COUNT(DISTINCT wave_index) FROM puzzle_content_blocks WHERE project_id=$1 AND content_revision=$2 AND city_id=$3 AND house_id=$4`,
		projectID, revision, cityID, houseID).Scan(&totalWaves); err != nil {
		return nil, err
	}
	funnel := &PuzzleWaveFunnel{TotalWaves: totalWaves, LostThresholdHours: lostThresholdHours, Steps: make([]PuzzleWaveFunnelStep, totalWaves)}
	for i := range funnel.Steps {
		funnel.Steps[i].WaveIndex = i
	}
	entered, err := queryWaveEntryAndCompletion(ctx, pool, projectID, cityID, houseID, from, to, scope, selectedBuild)
	if err != nil {
		return nil, err
	}
	for waveIndex, counts := range entered {
		if waveIndex < 0 || waveIndex >= totalWaves {
			continue
		}
		funnel.Steps[waveIndex].Entered = counts[0]
		funnel.Steps[waveIndex].Completed = counts[1]
	}
	abandoned, err := queryWaveAbandonment(ctx, pool, projectID, cityID, houseID, from, to, scope, selectedBuild)
	if err != nil {
		return nil, err
	}
	for waveIndex, counts := range abandoned {
		if waveIndex < 0 || waveIndex >= totalWaves {
			continue
		}
		funnel.Steps[waveIndex].AbandonedHere = counts[0]
		funnel.Steps[waveIndex].RecentHere = counts[1]
	}
	for i := 0; i < totalWaves-1; i++ {
		funnel.Steps[i].ReachedNext = funnel.Steps[i+1].Entered
	}
	return funnel, nil
}

// waveScopeCTE is the run-level scope predicate shared by both funnel
// queries: which house runs (per the requested TrafficScope) count at
// all. Binds project_id=$1, city_id=$2, house_id=$3, from=$4, to=$5.
func waveScopeCTE(scope TrafficScope) string {
	return `runs AS (
		SELECT house_run_id, completed, last_event_at FROM puzzle_house_run_classification
		WHERE project_id=$1 AND city_id=$2 AND house_id=$3 AND last_event_at>=$4 AND last_event_at<$5
		  AND ($6::text IS NULL OR last_build_number=$6)
		  AND ` + scope.runPredicate("fully_natural") + `
	), waves AS (
		SELECT properties->>'house_run_id' house_run_id, (properties->>'wave_index')::int wave_index,
		       BOOL_OR(name='wave_completed') completed_here
		FROM events
		WHERE project_id=$1 AND (properties->>'city_id')::int=$2 AND (properties->>'house_id')::int=$3
		  AND properties ? 'house_run_id' AND properties ? 'wave_index'
		  AND properties->>'house_run_id' IN (SELECT house_run_id FROM runs)
		GROUP BY 1,2
	)`
}

// queryWaveEntryAndCompletion returns, per wave_index, [entered, completed].
func queryWaveEntryAndCompletion(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, from, to time.Time, scope TrafficScope, build *string) (map[int][2]int64, error) {
	query := `WITH ` + waveScopeCTE(scope) + `
	SELECT wave_index, COUNT(DISTINCT house_run_id), COUNT(DISTINCT house_run_id) FILTER (WHERE completed_here)
	FROM waves GROUP BY 1 ORDER BY 1`
	rows, err := pool.Query(ctx, query, projectID, cityID, houseID, from, to, build)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int][2]int64{}
	for rows.Next() {
		var waveIndex int
		var entered, completed int64
		if err := rows.Scan(&waveIndex, &entered, &completed); err != nil {
			return nil, err
		}
		result[waveIndex] = [2]int64{entered, completed}
	}
	return result, rows.Err()
}

// queryWaveAbandonment returns, per the furthest wave a run ever reached,
// [abandoned (stale/lost), recent (still might come back)].
func queryWaveAbandonment(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, from, to time.Time, scope TrafficScope, build *string) (map[int][2]int64, error) {
	query := `WITH ` + waveScopeCTE(scope) + `, progress AS (
		SELECT r.house_run_id, r.completed, r.last_event_at, MAX(w.wave_index) max_wave
		FROM runs r JOIN waves w ON w.house_run_id=r.house_run_id
		GROUP BY 1,2,3
	)
	SELECT max_wave,
	       COUNT(*) FILTER (WHERE NOT completed AND now()-last_event_at >= make_interval(hours=>$7::int)),
	       COUNT(*) FILTER (WHERE NOT completed AND now()-last_event_at < make_interval(hours=>$7::int))
	FROM progress GROUP BY 1`
	rows, err := pool.Query(ctx, query, projectID, cityID, houseID, from, to, build, lostThresholdHours)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[int][2]int64{}
	for rows.Next() {
		var waveIndex int
		var abandoned, recent int64
		if err := rows.Scan(&waveIndex, &abandoned, &recent); err != nil {
			return nil, err
		}
		result[waveIndex] = [2]int64{abandoned, recent}
	}
	return result, rows.Err()
}
