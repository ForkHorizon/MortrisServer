package analytics

// Stage 4 exit-criteria tests (docs/puzzle-analytics-remaining-plan.md
// section 7): house-run-scoped coverage counts, not wave-attempt-scoped
// ones — a many-wave house must not look more played than it is.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedRevisionWithWaves gives GetPuzzleHouses/GetPuzzleWaveFunnel a
// catalogue for city 1 / house 1 with the given number of waves, one
// block per wave.
func seedRevisionWithWaves(t *testing.T, pool *pgxpool.Pool, projectID string, waves int) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO puzzle_content_revisions (project_id, content_revision, schema_version, catalog) VALUES ($1,'rev-coverage',2,'{}'::jsonb)`, projectID); err != nil {
		t.Fatalf("seed revision: %v", err)
	}
	for wave := 0; wave < waves; wave++ {
		if _, err := pool.Exec(ctx, `INSERT INTO puzzle_content_blocks (project_id, content_revision, city_id, house_id, wave_index, block_id, order_in_layer) VALUES ($1,'rev-coverage',1,1,$2,$3,0)`,
			projectID, wave, wave); err != nil {
			t.Fatalf("seed block for wave %d: %v", wave, err)
		}
	}
}

func TestPuzzleCoverage_OneRunAcrossFourWavesCountsAsOneStart(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "11111111-1111-4111-8111-111111111111"
	seedInstallation(t, pool, projectID, install, &now)
	seedRevisionWithWaves(t, pool, projectID, 4)

	events := make([]seedEvent, 0, 5)
	for wave := 0; wave < 4; wave++ {
		events = append(events, seedEvent{
			EventID: fmt.Sprintf("a0000000-0000-4000-8000-00000000000%d", wave), InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999",
			Sequence: int64(wave + 1), Name: "wave_attempt_started", Kind: "product", EffectiveAt: now.Add(time.Duration(wave) * time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": wave, "house_run_id": "run-1", "attempt_id": fmt.Sprintf("attempt-wave-%d", wave)},
		})
	}
	events = append(events, seedEvent{
		EventID: "a0000000-0000-4000-8000-000000000009", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999",
		Sequence: 5, Name: "house_completed", Kind: "product", EffectiveAt: now.Add(4 * time.Second),
		Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 3, "house_run_id": "run-1", "attempt_id": "attempt-wave-3"},
	})
	seedEvents(t, pool, projectID, events)

	houses, err := GetPuzzleHouses(ctx, pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), ScopeNaturalOnly)
	if err != nil {
		t.Fatalf("GetPuzzleHouses: %v", err)
	}
	house := houses.Houses[0]
	if house.NaturalHouseRunsStarted != 1 {
		t.Errorf("natural house runs started = %d, want 1 (one run across four waves is one start)", house.NaturalHouseRunsStarted)
	}
	if house.NaturalHouseRunsComplete != 1 {
		t.Errorf("natural house runs completed = %d, want 1", house.NaturalHouseRunsComplete)
	}
	if house.NaturalCompletionRate != 1 {
		t.Errorf("natural completion rate = %v, want 1", house.NaturalCompletionRate)
	}
	if house.WavesReached != 4 {
		t.Errorf("waves reached = %d, want 4", house.WavesReached)
	}
	if house.TotalWaves != 4 {
		t.Errorf("total waves = %d, want 4 (from the catalogue)", house.TotalWaves)
	}
	if house.EvidenceState != "thin" {
		t.Errorf("evidence state = %q, want thin (one run is below minPlacementsForRate placements)", house.EvidenceState)
	}
}

// A developer completion on an otherwise separate run must not inflate
// the natural numerator — mirrors the Stage 3 edge case but for the
// house-run coverage fields specifically.
func TestPuzzleCoverage_DeveloperCompletionAbsentFromNaturalNumerator(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "22222222-2222-4222-8222-222222222222"
	seedInstallation(t, pool, projectID, install, &now)
	seedRevisionWithWaves(t, pool, projectID, 1)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "a0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 1, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now,
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 0, "house_run_id": "run-natural", "attempt_id": "attempt-natural"}},
		{EventID: "a0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 2, Name: "developer_command_started", Kind: "product", EffectiveAt: now.Add(time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "origin": "developer_menu", "developer_action_id": "action-1", "developer_command": "CompleteHouse99", "house_run_id": "run-cheat"}},
	})

	houses, err := GetPuzzleHouses(ctx, pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), ScopeNaturalOnly)
	if err != nil {
		t.Fatalf("GetPuzzleHouses: %v", err)
	}
	house := houses.Houses[0]
	if house.NaturalHouseRunsStarted != 1 {
		t.Errorf("natural house runs started = %d, want 1 (only run-natural)", house.NaturalHouseRunsStarted)
	}
	if house.NaturalHouseRunsComplete != 0 {
		t.Errorf("natural house runs completed = %d, want 0", house.NaturalHouseRunsComplete)
	}
	if !house.DeveloperRunsObserved {
		t.Errorf("developer_runs_observed = false, want true (run-cheat exists in the same range)")
	}
}
