package analytics

import (
	"context"
	"sort"
	"time"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A replay is one attempt played back step by step: which details were
// already standing, which one the player was moving, and what happened to
// it. A table of the same events tells you a placement failed; the replay
// shows the half-built house it failed against, which is the thing that
// makes "why would anyone try that" answerable.
type PuzzleReplayStep struct {
	Index           int       `json:"index"`
	Name            string    `json:"name"`
	At              time.Time `json:"at"`
	BlockID         int       `json:"block_id"`
	TargetID        int       `json:"target_id"`
	Outcome         string    `json:"outcome"`
	RuleState       string    `json:"rule_state"`
	WaveIndex       int       `json:"wave_index"`
	ReleaseX        int       `json:"release_x_milli"`
	ReleaseY        int       `json:"release_y_milli"`
	ActiveElapsedMS int64     `json:"active_elapsed_ms"`
	// Placed is the house state after this step, so a scrubber can render
	// any step without replaying the ones before it.
	Placed []int `json:"placed"`
	// MissingSupport is OR of AND groups still unmet at this step. Empty
	// groups mean that alternative was satisfied.
	MissingSupport [][]int `json:"missing_support,omitempty"`
}

type PuzzleReplay struct {
	AttemptID string             `json:"attempt_id"`
	CityID    int                `json:"city_id"`
	HouseID   int                `json:"house_id"`
	InstallID string             `json:"install_id"`
	Steps     []PuzzleReplayStep `json:"steps"`
	Truncated bool               `json:"truncated"`
}

func GetPuzzleReplay(ctx context.Context, pool *pgxpool.Pool, projectID, attemptID string) (*PuzzleReplay, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	raw, err := loadGameplayAttemptEvents(ctx, pool, projectID, attemptID)
	if err != nil {
		return nil, err
	}
	if len(raw.Events) == 0 {
		return nil, apierr.New(404, "not_found", "attempt not found")
	}
	catalog, err := loadReplayCatalog(ctx, pool, projectID, raw.ContentRevision)
	if err != nil {
		return nil, err
	}
	return buildReplay(attemptID, raw, catalog), nil
}

// loadReplayCatalog falls back to the newest imported revision when the
// attempt's own revision was never imported.
//
// That is currently every attempt: the client ships per-house JSON-hash
// revisions that no catalogue upload matches, so a strict lookup returns
// an empty catalogue and every placement silently reports no missing
// support — the reconstruction looks like it works and always says
// "nothing was missing". Falling back is the difference between a working
// feature and a quietly empty one, and it is safe while block indices are
// stable. See the plan doc's note on the revision mismatch.
func loadReplayCatalog(ctx context.Context, pool *pgxpool.Pool, projectID, revision string) (PuzzleCatalog, error) {
	catalog, err := loadAttemptCatalog(ctx, pool, projectID, revision)
	if err != nil {
		return catalog, err
	}
	if len(catalog.Cities) > 0 {
		return catalog, nil
	}
	latest, err := latestPuzzleRevision(ctx, pool, projectID)
	if err != nil {
		return catalog, err
	}
	return loadAttemptCatalog(ctx, pool, projectID, latest)
}

func buildReplay(attemptID string, raw *GameplayAttempt, catalog PuzzleCatalog) *PuzzleReplay {
	replay := &PuzzleReplay{AttemptID: attemptID, InstallID: raw.InstallID, Steps: []PuzzleReplayStep{}, Truncated: raw.Truncated, CityID: -1, HouseID: -1}
	installed := map[int]bool{}
	for i := range raw.Events {
		payload, ok := attemptEventPayload(raw.Events[i].Properties)
		if !ok {
			continue
		}
		if replay.CityID < 0 {
			replay.CityID, replay.HouseID = number(payload["city_id"]), number(payload["house_id"])
		}
		step := PuzzleReplayStep{
			Index: len(replay.Steps), Name: raw.Events[i].Name, At: raw.Events[i].EffectiveAt,
			BlockID: -1, TargetID: -1, WaveIndex: number(payload["wave_index"]),
			ReleaseX: number(payload["release_x_milli"]), ReleaseY: number(payload["release_y_milli"]), ActiveElapsedMS: number64(payload["active_elapsed_ms"]),
		}
		if raw.Events[i].Name == "placement_resolved" {
			installed = applyReplayPlacement(&step, catalog, installed, payload)
		}
		step.Placed = sortedBlockIDs(installed)
		replay.Steps = append(replay.Steps, step)
	}
	return replay
}

// applyReplayPlacement mirrors applyPlacementState but records the block,
// target and verdict on the step rather than only the missing groups.
func applyReplayPlacement(step *PuzzleReplayStep, catalog PuzzleCatalog, installed map[int]bool, payload map[string]any) map[int]bool {
	// The client omits placed_block_ids when it would exceed the property
	// limit, in which case the running set carried from earlier steps is
	// the only record of what was standing.
	if ids, ok := payload["placed_block_ids"].(string); ok && ids != "" {
		installed = integerSet(ids)
	}
	step.BlockID = number(payload["block_id"])
	step.TargetID = number(payload["candidate_target_id"])
	step.Outcome, _ = payload["outcome"].(string)
	step.RuleState, _ = payload["rule_state"].(string)
	step.MissingSupport = missingGroups(catalog, number(payload["city_id"]), number(payload["house_id"]), step.TargetID, installed)
	switch step.Outcome {
	case "placed":
		installed[step.BlockID] = true
	case "returned":
		delete(installed, step.BlockID)
	}
	return installed
}

// sortedBlockIDs copies the running set into each step, so a step is a
// self-contained snapshot rather than a diff the client has to fold.
// Named apart from dimensions.go's string-keyed sortedKeys.
func sortedBlockIDs(set map[int]bool) []int {
	keys := make([]int, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}

// PuzzleAttemptSummary is one row of the attempt picker.
type PuzzleAttemptSummary struct {
	AttemptID        string    `json:"attempt_id"`
	InstallID        string    `json:"install_id"`
	StartedAt        time.Time `json:"started_at"`
	WaveIndex        int       `json:"wave_index"`
	Placements       int64     `json:"placements"`
	Falls            int64     `json:"falls"`
	Hints            int64     `json:"hints"`
	Completed        bool      `json:"completed"`
	AppVersion       string    `json:"app_version"`
	BuildNumber      string    `json:"build_number"`
	ActiveDurationMS int64     `json:"active_duration_ms"`
	LastWaveIndex    int       `json:"last_wave_index"`
	DominantFailure  string    `json:"dominant_failure"`
}

func GetPuzzleAttempts(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, from, to time.Time) ([]PuzzleAttemptSummary, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := pool.Query(ctx, `
SELECT properties->>'attempt_id', MIN(install_id::text),
       MIN(effective_at),
       MIN((properties->>'wave_index')::int),
       COUNT(*) FILTER (WHERE name='placement_resolved'),
       COUNT(*) FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'),
       COUNT(*) FILTER (WHERE name='hint_used'),
       BOOL_OR(name IN ('wave_completed','house_completed')),
       (array_agg(app_version ORDER BY effective_at DESC))[1],
       (array_agg(build_number ORDER BY effective_at DESC))[1],
       MAX(COALESCE((properties->>'active_elapsed_ms')::bigint,0)) - MIN(COALESCE((properties->>'active_elapsed_ms')::bigint,0)),
       MAX(COALESCE((properties->>'wave_index')::int,0)),
       COALESCE(MODE() WITHIN GROUP (ORDER BY properties->>'outcome') FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'),'')
FROM events
WHERE project_id=$1 AND effective_at>=$4 AND effective_at<$5
  AND properties ? 'attempt_id'
  AND (properties->>'city_id')::int=$2 AND (properties->>'house_id')::int=$3
GROUP BY 1
ORDER BY 2 DESC
LIMIT 200`, projectID, cityID, houseID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PuzzleAttemptSummary{}
	for rows.Next() {
		var row PuzzleAttemptSummary
		if err := rows.Scan(&row.AttemptID, &row.InstallID, &row.StartedAt, &row.WaveIndex,
			&row.Placements, &row.Falls, &row.Hints, &row.Completed, &row.AppVersion, &row.BuildNumber,
			&row.ActiveDurationMS, &row.LastWaveIndex, &row.DominantFailure); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
