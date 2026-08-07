package analytics

import (
	"context"
	"crypto/sha256"
	"fmt"
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
	Index             int       `json:"index"`
	Name              string    `json:"name"`
	At                time.Time `json:"at"`
	BlockID           int       `json:"block_id"`
	TargetID          int       `json:"target_id"`
	Outcome           string    `json:"outcome"`
	RuleState         string    `json:"rule_state"`
	WaveIndex         int       `json:"wave_index"`
	ReleaseX          int       `json:"release_x_milli"`
	ReleaseY          int       `json:"release_y_milli"`
	ActiveElapsedMS   int64     `json:"active_elapsed_ms"`
	InteractionID     string    `json:"interaction_id,omitempty"`
	Origin            string    `json:"origin,omitempty"`
	ProgressOrigin    string    `json:"progress_origin,omitempty"`
	DeveloperActionID string    `json:"developer_action_id,omitempty"`
	// Placed is the house state after this step, so a scrubber can render
	// any step without replaying the ones before it.
	Placed []int `json:"placed"`
	// MissingSupport is OR of AND groups still unmet at this step. Empty
	// groups mean that alternative was satisfied.
	MissingSupport [][]int `json:"missing_support,omitempty"`
}

type PuzzleReplay struct {
	AttemptID           string             `json:"attempt_id"`
	CityID              int                `json:"city_id"`
	HouseID             int                `json:"house_id"`
	InstallID           string             `json:"install_id"`
	Steps               []PuzzleReplayStep `json:"steps"`
	Truncated           bool               `json:"truncated"`
	SequenceGaps        int                `json:"sequence_gaps"`
	SequenceDuplicates  int                `json:"sequence_duplicates"`
	OrphanInteractions  int                `json:"orphan_interactions"`
	StateHashMismatches int                `json:"state_hash_mismatches"`
	RevisionWarning     string             `json:"revision_warning,omitempty"`
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
	catalog, legacyFallback, err := loadReplayCatalog(ctx, pool, projectID, raw.ContentRevision, !attemptUsesSchemaV2(raw))
	if err != nil {
		return nil, err
	}
	replay := buildReplay(attemptID, raw, catalog)
	if legacyFallback {
		replay.RevisionWarning = "legacy replay uses the newest imported catalogue because its event revision is unavailable"
	}
	return replay, nil
}

// loadReplayCatalog rejects a current-schema revision mismatch. Legacy
// replays retain an explicitly-labelled fallback so old diagnostic data is
// still viewable without making current content-pipeline failures invisible.
func loadReplayCatalog(ctx context.Context, pool *pgxpool.Pool, projectID, revision string, allowLegacyFallback bool) (PuzzleCatalog, bool, error) {
	catalog, err := loadAttemptCatalog(ctx, pool, projectID, revision)
	if err != nil {
		return catalog, false, err
	}
	if len(catalog.Cities) > 0 {
		return catalog, false, nil
	}
	if !allowLegacyFallback {
		return catalog, false, apierr.New(409, "unknown_content_revision", "schema-v2 replay revision is not imported; replay was not resolved against a different catalogue")
	}
	latest, err := latestPuzzleRevision(ctx, pool, projectID)
	if err != nil {
		return catalog, false, err
	}
	catalog, err = loadAttemptCatalog(ctx, pool, projectID, latest)
	return catalog, true, err
}

func attemptUsesSchemaV2(raw *GameplayAttempt) bool {
	for _, event := range raw.Events {
		payload, ok := attemptEventPayload(event.Properties)
		if !ok {
			continue
		}
		if number(payload["schema_version"]) >= 2 {
			return true
		}
	}
	return false
}

func buildReplay(attemptID string, raw *GameplayAttempt, catalog PuzzleCatalog) *PuzzleReplay {
	replay := &PuzzleReplay{AttemptID: attemptID, InstallID: raw.InstallID, Steps: []PuzzleReplayStep{}, Truncated: raw.Truncated, CityID: -1, HouseID: -1}
	installed := map[int]bool{}
	openInteractions := map[string]bool{}
	lastEventIndex := -1
	for i := range raw.Events {
		payload, ok := attemptEventPayload(raw.Events[i].Properties)
		if !ok {
			continue
		}
		if replay.CityID < 0 {
			replay.CityID, replay.HouseID = number(payload["city_id"]), number(payload["house_id"])
		}
		lastEventIndex = updateReplaySequence(replay, number(payload["attempt_event_index"]), lastEventIndex)
		step := PuzzleReplayStep{
			Index: len(replay.Steps), Name: raw.Events[i].Name, At: raw.Events[i].EffectiveAt,
			BlockID: -1, TargetID: -1, WaveIndex: number(payload["wave_index"]),
			ReleaseX: number(payload["release_x_milli"]), ReleaseY: number(payload["release_y_milli"]), ActiveElapsedMS: number64(payload["active_elapsed_ms"]),
		}
		step.BlockID = number(payload["block_id"])
		step.TargetID = number(payload["candidate_target_id"])
		step.Outcome, _ = payload["outcome"].(string)
		step.RuleState, _ = payload["rule_state"].(string)
		step.InteractionID, _ = payload["interaction_id"].(string)
		step.Origin, _ = payload["origin"].(string)
		step.ProgressOrigin, _ = payload["progress_origin"].(string)
		step.DeveloperActionID, _ = payload["developer_action_id"].(string)
		updateReplayInteraction(openInteractions, step)
		installed = applyReplayEvent(replay, &step, catalog, installed, payload)
		step.Placed = sortedBlockIDs(installed)
		replay.Steps = append(replay.Steps, step)
	}
	replay.OrphanInteractions = len(openInteractions)
	return replay
}

func updateReplaySequence(replay *PuzzleReplay, eventIndex, previous int) int {
	if eventIndex < 0 {
		return previous
	}
	if eventIndex == previous {
		replay.SequenceDuplicates++
	} else if previous >= 0 && eventIndex > previous+1 {
		replay.SequenceGaps += eventIndex - previous - 1
	}
	return eventIndex
}

func updateReplayInteraction(open map[string]bool, step PuzzleReplayStep) {
	if step.InteractionID == "" {
		return
	}
	if step.Name == "detail_taken" {
		open[step.InteractionID] = true
	} else if step.Name == "detail_returned" || step.Name == "interaction_abandoned" || step.Name == "placement_resolved" && step.Outcome == "placed" {
		delete(open, step.InteractionID)
	}
}

func applyReplayEvent(replay *PuzzleReplay, step *PuzzleReplayStep, catalog PuzzleCatalog, installed map[int]bool, payload map[string]any) map[int]bool {
	if step.Name == "placement_resolved" {
		return applyReplayPlacement(step, catalog, installed, payload)
	}
	if step.Name != "state_checkpoint" {
		return installed
	}
	ids, exists := payload["placed_block_ids"].(string)
	if !exists {
		return installed
	}
	if hash, _ := payload["placed_state_hash"].(string); hash != "" && hash != fmt.Sprintf("%x", sha256.Sum256([]byte(ids))) {
		replay.StateHashMismatches++
	}
	return integerSet(ids)
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
	AttemptID         string    `json:"attempt_id"`
	InstallID         string    `json:"install_id"`
	StartedAt         time.Time `json:"started_at"`
	WaveIndex         int       `json:"wave_index"`
	Placements        int64     `json:"placements"`
	Falls             int64     `json:"falls"`
	Hints             int64     `json:"hints"`
	Completed         bool      `json:"completed"`
	AppVersion        string    `json:"app_version"`
	BuildNumber       string    `json:"build_number"`
	ActiveDurationMS  int64     `json:"active_duration_ms"`
	LastWaveIndex     int       `json:"last_wave_index"`
	DominantFailure   string    `json:"dominant_failure"`
	ProgressOrigin    string    `json:"progress_origin"`
	DeveloperAffected bool      `json:"developer_affected"`
}

func GetPuzzleAttempts(ctx context.Context, pool *pgxpool.Pool, projectID string, cityID, houseID int, from, to time.Time, build ...*string) ([]PuzzleAttemptSummary, error) {
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
       COALESCE(MODE() WITHIN GROUP (ORDER BY properties->>'outcome') FILTER (WHERE name='placement_resolved' AND properties->>'outcome' LIKE 'fell_%'),''),
       COALESCE((array_agg(properties->>'progress_origin' ORDER BY effective_at DESC) FILTER (WHERE properties ? 'progress_origin'))[1],'natural'),
       COALESCE(BOOL_OR(properties->>'origin'='developer_menu' OR properties->>'close_reason'='developer_command'),false)
FROM events
WHERE project_id=$1 AND effective_at>=$4 AND effective_at<$5
	  AND ($6::text IS NULL OR build_number=$6)
  AND properties ? 'attempt_id'
  AND (properties->>'city_id')::int=$2 AND (properties->>'house_id')::int=$3
GROUP BY 1
ORDER BY MIN(effective_at) DESC
LIMIT 200`, projectID, cityID, houseID, from, to, optionalBuild(build))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PuzzleAttemptSummary{}
	for rows.Next() {
		var row PuzzleAttemptSummary
		if err := rows.Scan(&row.AttemptID, &row.InstallID, &row.StartedAt, &row.WaveIndex,
			&row.Placements, &row.Falls, &row.Hints, &row.Completed, &row.AppVersion, &row.BuildNumber,
			&row.ActiveDurationMS, &row.LastWaveIndex, &row.DominantFailure, &row.ProgressOrigin, &row.DeveloperAffected); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
