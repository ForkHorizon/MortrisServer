package analytics

import (
	"encoding/json"
	"testing"
)

func replayEvent(name string, payload map[string]any) GameplayAttemptEvent {
	raw, _ := json.Marshal(payload)
	return GameplayAttemptEvent{Name: name, Properties: raw}
}

// Each step must be a self-contained snapshot of the house, or a scrubber
// would have to replay every earlier step to draw one.
func TestReplayStepsCarryFullPlacedState(t *testing.T) {
	raw := &GameplayAttempt{Events: []GameplayAttemptEvent{
		replayEvent("wave_presented", map[string]any{"city_id": 1.0, "house_id": 2.0}),
		replayEvent("placement_resolved", map[string]any{"city_id": 1.0, "house_id": 2.0, "block_id": 10.0, "outcome": "placed"}),
		replayEvent("placement_resolved", map[string]any{"city_id": 1.0, "house_id": 2.0, "block_id": 11.0, "outcome": "placed"}),
	}}
	replay := buildReplay("a", raw, testCatalog())
	if len(replay.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(replay.Steps))
	}
	if len(replay.Steps[0].Placed) != 0 {
		t.Fatalf("step 0 placed = %v, want empty", replay.Steps[0].Placed)
	}
	if got := replay.Steps[2].Placed; len(got) != 2 || got[0] != 10 || got[1] != 11 {
		t.Fatalf("step 2 placed = %v, want [10 11]", got)
	}
	if replay.CityID != 1 || replay.HouseID != 2 {
		t.Fatalf("scope = %d/%d, want 1/2", replay.CityID, replay.HouseID)
	}
}

// A fall must not add the block to the house — that is the whole point of
// it falling.
func TestReplayFallDoesNotPlaceTheBlock(t *testing.T) {
	raw := &GameplayAttempt{Events: []GameplayAttemptEvent{
		replayEvent("placement_resolved", map[string]any{"city_id": 1.0, "house_id": 2.0, "block_id": 10.0, "outcome": "fell_missing_support", "candidate_target_id": 10.0}),
	}}
	replay := buildReplay("a", raw, testCatalog())
	if len(replay.Steps[0].Placed) != 0 {
		t.Fatalf("a fallen block was recorded as placed: %v", replay.Steps[0].Placed)
	}
	if replay.Steps[0].Outcome != "fell_missing_support" {
		t.Fatalf("outcome = %q", replay.Steps[0].Outcome)
	}
}

// "returned" means the player took the detail back off the house.
func TestReplayReturnRemovesTheBlock(t *testing.T) {
	raw := &GameplayAttempt{Events: []GameplayAttemptEvent{
		replayEvent("placement_resolved", map[string]any{"city_id": 1.0, "house_id": 2.0, "block_id": 10.0, "outcome": "placed"}),
		replayEvent("placement_resolved", map[string]any{"city_id": 1.0, "house_id": 2.0, "block_id": 10.0, "outcome": "returned"}),
	}}
	replay := buildReplay("a", raw, testCatalog())
	if len(replay.Steps[1].Placed) != 0 {
		t.Fatalf("returned block still placed: %v", replay.Steps[1].Placed)
	}
}

// The reconstruction contract: when the client omits placed_block_ids for
// size, the running set carried from earlier steps is the only record.
func TestReplayKeepsRunningStateWhenPlacedIDsOmitted(t *testing.T) {
	raw := &GameplayAttempt{Events: []GameplayAttemptEvent{
		replayEvent("placement_resolved", map[string]any{"city_id": 1.0, "house_id": 2.0, "block_id": 10.0, "outcome": "placed", "placed_block_ids": ""}),
		replayEvent("placement_resolved", map[string]any{"city_id": 1.0, "house_id": 2.0, "block_id": 11.0, "outcome": "placed"}),
	}}
	replay := buildReplay("a", raw, testCatalog())
	if got := replay.Steps[1].Placed; len(got) != 2 {
		t.Fatalf("placed = %v, want both blocks retained", got)
	}
}

// Block 10's rule in testCatalog is [[11],[10,11]], so with nothing
// standing both alternatives report 11 as missing.
func TestReplayReportsMissingSupport(t *testing.T) {
	raw := &GameplayAttempt{Events: []GameplayAttemptEvent{
		replayEvent("placement_resolved", map[string]any{"city_id": 1.0, "house_id": 2.0, "block_id": 10.0, "outcome": "fell_missing_support", "candidate_target_id": 10.0}),
	}}
	replay := buildReplay("a", raw, testCatalog())
	if len(replay.Steps[0].MissingSupport) != 2 {
		t.Fatalf("missing support = %v, want two alternatives", replay.Steps[0].MissingSupport)
	}
}
