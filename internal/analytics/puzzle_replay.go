package analytics

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
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
	ReleaseX          *int      `json:"release_x_milli,omitempty"`
	ReleaseY          *int      `json:"release_y_milli,omitempty"`
	TargetX           *int      `json:"target_x_milli,omitempty"`
	TargetY           *int      `json:"target_y_milli,omitempty"`
	ActiveElapsedMS   int64     `json:"active_elapsed_ms"`
	InteractionID     string    `json:"interaction_id,omitempty"`
	Origin            string    `json:"origin,omitempty"`
	ProgressOrigin    string    `json:"progress_origin,omitempty"`
	DeveloperActionID string    `json:"developer_action_id,omitempty"`
	DeveloperCommand  string    `json:"developer_command,omitempty"`
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
			ReleaseX: coordinatePtr(payload["release_x_milli"]), ReleaseY: coordinatePtr(payload["release_y_milli"]),
			ActiveElapsedMS: number64(payload["active_elapsed_ms"]),
		}
		step.BlockID = number(payload["block_id"])
		step.TargetID = number(payload["candidate_target_id"])
		step.TargetX, step.TargetY = catalogTargetCoords(catalog, replay.CityID, replay.HouseID, step.TargetID)
		step.Outcome, _ = payload["outcome"].(string)
		step.RuleState, _ = payload["rule_state"].(string)
		step.InteractionID, _ = payload["interaction_id"].(string)
		step.Origin, _ = payload["origin"].(string)
		step.ProgressOrigin, _ = payload["progress_origin"].(string)
		step.DeveloperActionID, _ = payload["developer_action_id"].(string)
		step.DeveloperCommand, _ = payload["developer_command"].(string)
		updateReplayInteraction(openInteractions, step)
		installed = applyReplayEvent(replay, &step, catalog, installed, payload)
		step.Placed = sortedBlockIDs(installed)
		replay.Steps = append(replay.Steps, step)
	}
	replay.OrphanInteractions = len(openInteractions)
	return replay
}

func coordinatePtr(value any) *int {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case int:
		v := typed
		return &v
	case int64:
		v := int(typed)
		return &v
	case float64:
		v := int(typed)
		return &v
	case string:
		if typed == "" {
			return nil
		}
		if v, err := strconv.Atoi(typed); err == nil {
			return &v
		}
	}
	return nil
}

func catalogTargetCoords(catalog PuzzleCatalog, cityID, houseID, targetID int) (*int, *int) {
	if targetID < 0 {
		return nil, nil
	}
	for _, city := range catalog.Cities {
		if city.CityID != cityID {
			continue
		}
		for _, house := range city.Houses {
			if house.HouseID != houseID {
				continue
			}
			for _, target := range house.Targets {
				if target.TargetID == targetID {
					x, y := target.LocalXMilli, target.LocalYMilli
					return &x, &y
				}
			}
		}
	}
	return nil, nil
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
	} else if step.Name == "detail_returned" || step.Name == "interaction_abandoned" || (step.Name == "placement_resolved" && step.Outcome == "placed") {
		delete(open, step.InteractionID)
	}
}

func applyReplayEvent(replay *PuzzleReplay, step *PuzzleReplayStep, catalog PuzzleCatalog, installed map[int]bool, payload map[string]any) map[int]bool {
	if step.Name == "placement_resolved" {
		return applyReplayPlacement(replay, step, catalog, installed, payload)
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
func applyReplayPlacement(replay *PuzzleReplay, step *PuzzleReplayStep, catalog PuzzleCatalog, installed map[int]bool, payload map[string]any) map[int]bool {
	// The client omits placed_block_ids when it would exceed the property
	// limit, in which case the running set carried from earlier steps is
	// the only record of what was standing.
	if ids, ok := payload["placed_block_ids"].(string); ok && ids != "" {
		installed = integerSet(ids)
	}
	step.BlockID = number(payload["block_id"])
	step.TargetID = number(payload["candidate_target_id"])
	cityID := number(payload["city_id"])
	if cityID < 0 {
		cityID = replay.CityID
	}
	houseID := number(payload["house_id"])
	if houseID < 0 {
		houseID = replay.HouseID
	}
	if step.TargetX == nil || step.TargetY == nil {
		step.TargetX, step.TargetY = catalogTargetCoords(catalog, cityID, houseID, step.TargetID)
	}
	step.Outcome, _ = payload["outcome"].(string)
	step.RuleState, _ = payload["rule_state"].(string)
	step.MissingSupport = missingGroups(catalog, cityID, houseID, step.TargetID, installed)
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
