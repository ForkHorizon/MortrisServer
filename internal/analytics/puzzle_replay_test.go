package analytics

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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

func TestReplayCarriesInteractionAndAppliesCheckpoint(t *testing.T) {
	raw := &GameplayAttempt{Events: []GameplayAttemptEvent{
		replayEvent("detail_taken", map[string]any{"city_id": 1.0, "house_id": 2.0, "block_id": 10.0, "interaction_id": "move-1", "progress_origin": "natural"}),
		replayEvent("detail_released", map[string]any{"city_id": 1.0, "house_id": 2.0, "block_id": 10.0, "candidate_target_id": 10.0, "interaction_id": "move-1"}),
		replayEvent("state_checkpoint", map[string]any{"city_id": 1.0, "house_id": 2.0, "placed_block_ids": "10,11"}),
	}}
	replay := buildReplay("a", raw, testCatalog())
	if replay.Steps[0].BlockID != 10 || replay.Steps[0].InteractionID != "move-1" {
		t.Fatalf("take step lost interaction context: %+v", replay.Steps[0])
	}
	if replay.Steps[1].TargetID != 10 {
		t.Fatalf("release target = %d, want 10", replay.Steps[1].TargetID)
	}
	if got := replay.Steps[2].Placed; len(got) != 2 || got[0] != 10 || got[1] != 11 {
		t.Fatalf("checkpoint placed = %v, want [10 11]", got)
	}
}

func TestReplayReportsSequenceAndInteractionIntegrity(t *testing.T) {
	raw := &GameplayAttempt{Events: []GameplayAttemptEvent{
		replayEvent("detail_taken", map[string]any{"city_id": 1.0, "house_id": 2.0, "block_id": 10.0, "interaction_id": "orphan", "attempt_event_index": 0.0}),
		replayEvent("state_checkpoint", map[string]any{"city_id": 1.0, "house_id": 2.0, "placed_block_ids": "10", "placed_state_hash": "wrong", "attempt_event_index": 2.0}),
	}}
	replay := buildReplay("a", raw, testCatalog())
	if replay.SequenceGaps != 1 || replay.OrphanInteractions != 1 || replay.StateHashMismatches != 1 {
		t.Fatalf("integrity = gaps:%d orphans:%d hashes:%d", replay.SequenceGaps, replay.OrphanInteractions, replay.StateHashMismatches)
	}
}

// In house_local space, negative coordinates are valid and must be preserved as
// non-nil pointers. Absence of coordinates must result in nil pointers and be
// omitted from serialized JSON.
func TestReplayPreservesNegativeCoordinatesAndDistinguishesAbsence(t *testing.T) {
	raw := &GameplayAttempt{Events: []GameplayAttemptEvent{
		replayEvent("detail_released", map[string]any{
			"city_id": 1.0, "house_id": 2.0, "block_id": 10.0,
			"release_x_milli": -350.0, "release_y_milli": -1.0,
		}),
		replayEvent("detail_taken", map[string]any{
			"city_id": 1.0, "house_id": 2.0, "block_id": 10.0,
		}),
		replayEvent("detail_released", map[string]any{
			"city_id": 1.0, "house_id": 2.0, "block_id": 10.0,
			"release_x_milli": 0.0, "release_y_milli": 0.0,
		}),
	}}
	replay := buildReplay("a", raw, testCatalog())
	if len(replay.Steps) != 3 {
		t.Fatalf("steps = %d, want 3", len(replay.Steps))
	}
	step0 := replay.Steps[0]
	if step0.ReleaseX == nil || *step0.ReleaseX != -350 {
		t.Fatalf("step 0 ReleaseX = %v, want -350", step0.ReleaseX)
	}
	if step0.ReleaseY == nil || *step0.ReleaseY != -1 {
		t.Fatalf("step 0 ReleaseY = %v, want -1", step0.ReleaseY)
	}

	step1 := replay.Steps[1]
	if step1.ReleaseX != nil || step1.ReleaseY != nil {
		t.Fatalf("step 1 coords should be nil, got x=%v y=%v", step1.ReleaseX, step1.ReleaseY)
	}

	step2 := replay.Steps[2]
	if step2.ReleaseX == nil || *step2.ReleaseX != 0 || step2.ReleaseY == nil || *step2.ReleaseY != 0 {
		t.Fatalf("step 2 coords should be 0, got x=%v y=%v", step2.ReleaseX, step2.ReleaseY)
	}

	marshaled, err := json.Marshal(replay.Steps)
	if err != nil {
		t.Fatalf("marshal err = %v", err)
	}
	jsonStr := string(marshaled)
	if !strings.Contains(jsonStr, `"release_x_milli":-350`) || !strings.Contains(jsonStr, `"release_y_milli":-1`) {
		t.Fatalf("json missing negative coords: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"release_x_milli":0`) || !strings.Contains(jsonStr, `"release_y_milli":0`) {
		t.Fatalf("json missing zero coords: %s", jsonStr)
	}
}

// Targets in puzzle_content_targets have their own coordinates and need not
// equal block_id. When candidate_target_id is present, TargetX and TargetY must
// be resolved from the catalog.
func TestReplayDecouplesTargetIDFromBlockID(t *testing.T) {
	catalog := testCatalog()
	// Add a target where target_id (99) != block_id (10) with known coordinates
	catalog.Cities[0].Houses[0].Targets = append(catalog.Cities[0].Houses[0].Targets, PuzzleCatalogTarget{
		TargetID:           99,
		WaveIndex:          0,
		CompatibleBlockIDs: []int{10},
		LocalXMilli:        1500,
		LocalYMilli:        -2200,
	})

	raw := &GameplayAttempt{Events: []GameplayAttemptEvent{
		replayEvent("placement_resolved", map[string]any{
			"city_id": 1.0, "house_id": 2.0, "block_id": 10.0,
			"candidate_target_id": 99.0, "outcome": "placed",
			"release_x_milli": 1490.0, "release_y_milli": -2210.0,
		}),
	}}
	replay := buildReplay("a", raw, catalog)
	if len(replay.Steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(replay.Steps))
	}
	step := replay.Steps[0]
	if step.BlockID != 10 {
		t.Fatalf("block_id = %d, want 10", step.BlockID)
	}
	if step.TargetID != 99 {
		t.Fatalf("target_id = %d, want 99", step.TargetID)
	}
	if step.TargetX == nil || *step.TargetX != 1500 {
		t.Fatalf("TargetX = %v, want 1500", step.TargetX)
	}
	if step.TargetY == nil || *step.TargetY != -2200 {
		t.Fatalf("TargetY = %v, want -2200", step.TargetY)
	}
}

func TestLoadReplayCatalogRejectsUnknownCurrentRevision(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	if _, _, err := loadReplayCatalog(context.Background(), pool, projectID, "not-imported", false); err == nil {
		t.Fatal("schema-v2 revision mismatch must not silently use the newest catalogue")
	}
}

func TestReplayCarriesDeveloperCommand(t *testing.T) {
	raw := &GameplayAttempt{Events: []GameplayAttemptEvent{
		replayEvent("developer_command_started", map[string]any{
			"city_id": 1.0, "house_id": 2.0, "origin": "developer_menu",
			"developer_action_id": "act-1", "developer_command": "CompleteHouse99",
		}),
	}}
	replay := buildReplay("a", raw, testCatalog())
	if len(replay.Steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(replay.Steps))
	}
	if replay.Steps[0].DeveloperCommand != "CompleteHouse99" {
		t.Fatalf("developer_command = %q, want CompleteHouse99", replay.Steps[0].DeveloperCommand)
	}
	if replay.Steps[0].DeveloperActionID != "act-1" {
		t.Fatalf("developer_action_id = %q, want act-1", replay.Steps[0].DeveloperActionID)
	}
}

func seedOverflowTestEvents(t *testing.T, pool *pgxpool.Pool, projectID, installID string, now time.Time) {
	seedEvents(t, pool, projectID, []seedEvent{
		{
			EventID: "a1111111-1111-4111-8111-111111111111", InstallID: installID, SessionID: "s1", Sequence: 1,
			Name: "wave_started", Kind: "product", EffectiveAt: now,
			Properties: map[string]any{"attempt_id": "attempt-overflow", "city_id": 1, "house_id": 1, "wave_index": "99999999999999999999999999", "active_elapsed_ms": "99999999999999999999999999"},
		},
		{
			EventID: "a2222222-2222-4222-8222-222222222222", InstallID: installID, SessionID: "s1", Sequence: 2,
			Name: "wave_started", Kind: "product", EffectiveAt: now.Add(time.Second),
			Properties: map[string]any{"attempt_id": "", "city_id": 1, "house_id": 1},
		},
		{
			EventID: "a3333333-3333-4333-8333-333333333333", InstallID: installID, SessionID: "s1", Sequence: 3,
			Name: "wave_started", Kind: "product", EffectiveAt: now.Add(2 * time.Second),
			Properties: map[string]any{"attempt_id": "attempt-valid", "city_id": 1, "house_id": 1, "wave_index": 1, "active_elapsed_ms": 5000},
		},
	})
}

func TestGetPuzzleAttempts_SafelyFiltersOverflowValuesAndEmptyAttemptID(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	installID := "88888888-8888-4888-8888-888888888888"
	seedInstallation(t, pool, projectID, installID, &now)
	seedOverflowTestEvents(t, pool, projectID, installID, now)

	attempts, err := GetPuzzleAttempts(context.Background(), pool, projectID, 1, 1, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetPuzzleAttempts failed on overflow values: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("attempts count = %d, want 2", len(attempts))
	}
	for _, a := range attempts {
		if a.AttemptID == "" {
			t.Errorf("empty attempt_id should have been filtered out")
		}
		if a.AttemptID == "attempt-overflow" && (a.WaveIndex != 0 || a.ActiveDurationMS != 0) {
			t.Errorf("overflow attempt fields corrupted: wave_index=%d, active_ms=%d", a.WaveIndex, a.ActiveDurationMS)
		}
	}
}
