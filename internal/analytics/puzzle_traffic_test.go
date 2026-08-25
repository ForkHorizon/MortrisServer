package analytics

// Stage 3 exit-criteria tests (docs/puzzle-analytics-remaining-plan.md
// section 6): the shared classification views plus the metric-specific
// inclusion rules they feed.

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The plan doc's central edge case: natural placements, then
// CompleteHouse99. The placements must remain natural at event level
// (rule 1), but the house run must not count as a natural completion —
// and a second, untouched natural run elsewhere must be unaffected.
func TestPuzzleTraffic_CompleteHouse99DoesNotCountAsNaturalCompletion(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "11111111-1111-4111-8111-111111111111"
	seedInstallation(t, pool, projectID, install, &now)
	seedRevisionForHouseWall(t, pool, projectID)

	// Run A: fully natural, completes normally.
	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "a0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 1, Name: "placement_resolved", Kind: "product", EffectiveAt: now,
			Properties: map[string]any{"city_id": 1, "house_id": 1, "attempt_id": "attempt-a", "house_run_id": "run-a", "block_id": 1, "outcome": "placed"}},
		{EventID: "a0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 2, Name: "house_completed", Kind: "product", EffectiveAt: now.Add(time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "attempt_id": "attempt-a", "house_run_id": "run-a"}},
	})

	// Run B: a natural placement, then the tester opens the dev menu and
	// runs a progress-changing command. The started event still carries
	// the pre-cheat attempt/house_run IDs (PuzzleAnalyticsService.cs fires
	// it before clearing them) — that is what must poison this run.
	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "a0000000-0000-4000-8000-000000000003", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 3, Name: "placement_resolved", Kind: "product", EffectiveAt: now.Add(2 * time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "attempt_id": "attempt-b", "house_run_id": "run-b", "block_id": 2, "outcome": "placed"}},
		{EventID: "a0000000-0000-4000-8000-000000000004", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 4, Name: "developer_command_started", Kind: "product", EffectiveAt: now.Add(3 * time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "attempt_id": "attempt-b", "house_run_id": "run-b", "origin": "developer_menu", "developer_action_id": "action-1", "developer_command": "CompleteHouse99"}},
		// The completion mutation itself never carries attempt_id/house_run_id
		// (the client clears them before emitting it) — it must not need to
		// for run-b to already be correctly excluded.
		{EventID: "a0000000-0000-4000-8000-000000000005", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 5, Name: "developer_progress_mutated", Kind: "product", EffectiveAt: now.Add(4 * time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "origin": "developer_menu", "developer_action_id": "action-1", "developer_command": "CompleteHouse99", "after_completed": true, "before_state_hash": "b", "after_state_hash": "a"}},
	})

	houses, err := GetPuzzleHouses(ctx, pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), ScopeNaturalOnly)
	if err != nil {
		t.Fatalf("GetPuzzleHouses: %v", err)
	}
	if len(houses.Houses) != 1 {
		t.Fatalf("houses = %d, want 1", len(houses.Houses))
	}
	house := houses.Houses[0]
	// Both placements are natural at event level (rule 1): 2 placements.
	if house.Placements != 2 {
		t.Errorf("placements = %d, want 2 (both natural placements count individually)", house.Placements)
	}
	// Only run-a's completion counts naturally; run-b's cheat completion
	// must not, even though its placement was natural.
	if house.CompletedAttempts != 1 {
		t.Errorf("completed_attempts = %d, want 1 (run-b's cheat completion must not count)", house.CompletedAttempts)
	}
	// "Wave entry" is a run-level fact too (rule 3): attempt-b is excluded
	// from the opened-attempts count entirely, even though its own
	// placement event stayed natural for fall-rate purposes above.
	if house.Attempts != 1 {
		t.Errorf("attempts = %d, want 1 (attempt-b is developer-touched, so it must not count as a natural entry either)", house.Attempts)
	}

	// House-run classification directly: run-a fully natural and
	// completed, run-b not fully natural.
	var runANatural, runBNatural, runACompleted bool
	if err := pool.QueryRow(ctx, `SELECT fully_natural, completed FROM puzzle_house_run_classification WHERE project_id=$1 AND house_run_id='run-a'`, projectID).Scan(&runANatural, &runACompleted); err != nil {
		t.Fatalf("run-a classification: %v", err)
	}
	if !runANatural || !runACompleted {
		t.Errorf("run-a = natural:%v completed:%v, want true/true", runANatural, runACompleted)
	}
	if err := pool.QueryRow(ctx, `SELECT fully_natural FROM puzzle_house_run_classification WHERE project_id=$1 AND house_run_id='run-b'`, projectID).Scan(&runBNatural); err != nil {
		t.Fatalf("run-b classification: %v", err)
	}
	if runBNatural {
		t.Errorf("run-b fully_natural = true, want false (the developer_command_started event should poison it)")
	}
}

// A tester opening the developer menu without ever issuing a command
// still taints the attempt/run it happened in (plan section 6: "developer
// affected at any point"), even though nothing failed or changed.
func TestPuzzleTraffic_MenuOpenedWithoutCommandStillMarksRunDeveloperAffected(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "22222222-2222-4222-8222-222222222222"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "b0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 1, Name: "placement_resolved", Kind: "product", EffectiveAt: now,
			Properties: map[string]any{"city_id": 1, "house_id": 1, "attempt_id": "attempt-c", "house_run_id": "run-c", "block_id": 1, "outcome": "placed"}},
		{EventID: "b0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 2, Name: "developer_menu_opened", Kind: "product", EffectiveAt: now.Add(time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "attempt_id": "attempt-c", "house_run_id": "run-c", "origin": "developer_menu"}},
		{EventID: "b0000000-0000-4000-8000-000000000003", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 3, Name: "developer_menu_closed", Kind: "product", EffectiveAt: now.Add(2 * time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "attempt_id": "attempt-c", "house_run_id": "run-c", "origin": "developer_menu"}},
	})

	var fullyNatural bool
	if err := pool.QueryRow(ctx, `SELECT fully_natural FROM puzzle_wave_attempt_classification WHERE project_id=$1 AND attempt_id='attempt-c'`, projectID).Scan(&fullyNatural); err != nil {
		t.Fatalf("classification: %v", err)
	}
	if fullyNatural {
		t.Errorf("fully_natural = true, want false (a menu-open/close event is still a developer-menu event)")
	}
}

// GetPuzzleTesterImpact should surface the two production smoke commands
// (a progress completion and a reset) as separate, correctly-bucketed
// actions.
func TestPuzzleTesterImpact_CountsCompletionAndResetSeparately(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "33333333-3333-4333-8333-333333333333"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "c0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 1, Name: "developer_command_started", Kind: "product", EffectiveAt: now,
			Properties: map[string]any{"developer_action_id": "action-complete", "developer_command": "CompleteHouse99"}},
		{EventID: "c0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 2, Name: "developer_command_completed", Kind: "product", EffectiveAt: now.Add(time.Second),
			Properties: map[string]any{"developer_action_id": "action-complete", "developer_command": "CompleteHouse99", "progress_changed": true}},
		{EventID: "c0000000-0000-4000-8000-000000000003", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 3, Name: "developer_progress_mutated", Kind: "product", EffectiveAt: now.Add(2 * time.Second),
			Properties: map[string]any{"developer_action_id": "action-complete", "developer_command": "CompleteHouse99", "before_state_hash": "b1", "after_state_hash": "a1"}},

		{EventID: "c0000000-0000-4000-8000-000000000004", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 4, Name: "developer_command_started", Kind: "product", EffectiveAt: now.Add(3 * time.Second),
			Properties: map[string]any{"developer_action_id": "action-reset", "developer_command": "ResetHouse"}},
		{EventID: "c0000000-0000-4000-8000-000000000005", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 5, Name: "developer_command_completed", Kind: "product", EffectiveAt: now.Add(4 * time.Second),
			Properties: map[string]any{"developer_action_id": "action-reset", "developer_command": "ResetHouse", "progress_changed": true}},
		{EventID: "c0000000-0000-4000-8000-000000000006", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 6, Name: "developer_progress_mutated", Kind: "product", EffectiveAt: now.Add(5 * time.Second),
			Properties: map[string]any{"developer_action_id": "action-reset", "developer_command": "ResetHouse", "before_state_hash": "b2", "after_state_hash": "a2"}},
	})

	impact, err := GetPuzzleTesterImpact(ctx, pool, projectID, now.Add(-time.Minute), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetPuzzleTesterImpact: %v", err)
	}
	if impact.DeveloperActions != 2 {
		t.Errorf("developer actions = %d, want 2", impact.DeveloperActions)
	}
	if impact.ProgressCompletions != 1 {
		t.Errorf("progress completions = %d, want 1", impact.ProgressCompletions)
	}
	if impact.Resets != 1 {
		t.Errorf("resets = %d, want 1", impact.Resets)
	}
	if impact.MutationsWithBeforeAfter != 2 || impact.MutationsMissingBeforeAfter != 0 {
		t.Errorf("mutations = with:%d missing:%d, want 2/0", impact.MutationsWithBeforeAfter, impact.MutationsMissingBeforeAfter)
	}
}

// The scope control (natural_only/developer_affected/all) swaps the
// population a metric answers for, rather than just adding a filter on
// top of the natural one — no dashboard metric may silently combine them.
func TestTrafficScope_SwapsPopulationAcrossHouseWall(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "44444444-4444-4444-8444-444444444444"
	seedInstallation(t, pool, projectID, install, &now)
	seedRevisionForHouseWall(t, pool, projectID)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "d0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 1, Name: "placement_resolved", Kind: "product", EffectiveAt: now,
			Properties: map[string]any{"city_id": 1, "house_id": 1, "attempt_id": "attempt-natural", "house_run_id": "run-natural", "block_id": 1, "outcome": "placed"}},
		{EventID: "d0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 2, Name: "placement_resolved", Kind: "product", EffectiveAt: now.Add(time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "attempt_id": "attempt-tainted", "house_run_id": "run-tainted", "block_id": 1, "outcome": "placed", "progress_origin": "developer_modified"}},
	})

	natural, err := GetPuzzleHouses(ctx, pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), ScopeNaturalOnly)
	if err != nil {
		t.Fatalf("GetPuzzleHouses natural: %v", err)
	}
	developerAffected, err := GetPuzzleHouses(ctx, pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), ScopeDeveloperAffected)
	if err != nil {
		t.Fatalf("GetPuzzleHouses developer_affected: %v", err)
	}
	all, err := GetPuzzleHouses(ctx, pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), ScopeAllTraffic)
	if err != nil {
		t.Fatalf("GetPuzzleHouses all: %v", err)
	}
	if natural.Houses[0].Placements != 1 {
		t.Errorf("natural placements = %d, want 1", natural.Houses[0].Placements)
	}
	if developerAffected.Houses[0].Placements != 1 {
		t.Errorf("developer_affected placements = %d, want 1", developerAffected.Houses[0].Placements)
	}
	if all.Houses[0].Placements != 2 {
		t.Errorf("all placements = %d, want 2", all.Houses[0].Placements)
	}
}

// seedRevisionForHouseWall gives GetPuzzleHouses a catalogue to join
// against: its "house" CTE is spined on puzzle_content_blocks, not
// events, so city 1 / house 1 needs one row to appear in the wall at all.
func seedRevisionForHouseWall(t *testing.T, pool *pgxpool.Pool, projectID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO puzzle_content_revisions (project_id, content_revision, schema_version, catalog) VALUES ($1,'rev-traffic',2,'{}'::jsonb)`, projectID); err != nil {
		t.Fatalf("seed revision: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO puzzle_content_blocks (project_id, content_revision, city_id, house_id, wave_index, block_id, order_in_layer) VALUES ($1,'rev-traffic',1,1,0,1,0), ($1,'rev-traffic',1,1,0,2,1)`, projectID); err != nil {
		t.Fatalf("seed blocks: %v", err)
	}
}

// GetGameplayDiagnostics (the older table-heavy page) must apply the same
// split as the house wall: Placements stays event-level, Attempts and
// CompletedHouses are gated to fully natural attempts.
func TestPuzzleTraffic_GameplayDiagnosticsGatesAttemptsNotPlacements(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "55555555-5555-4555-8555-555555555555"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "e0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 1, Name: "placement_resolved", Kind: "product", EffectiveAt: now,
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 0, "attempt_id": "attempt-e", "house_run_id": "run-e", "block_id": 1, "outcome": "placed", "content_revision": "rev-traffic"}},
		{EventID: "e0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 2, Name: "house_completed", Kind: "product", EffectiveAt: now.Add(time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 0, "attempt_id": "attempt-e", "house_run_id": "run-e", "content_revision": "rev-traffic"}},
		{EventID: "e0000000-0000-4000-8000-000000000003", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 3, Name: "developer_command_started", Kind: "product", EffectiveAt: now.Add(2 * time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 0, "attempt_id": "attempt-e", "house_run_id": "run-e", "origin": "developer_menu", "developer_action_id": "action-e", "developer_command": "CompleteHouse99", "content_revision": "rev-traffic"}},
	})

	result, err := GetGameplayDiagnostics(ctx, pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), time.UTC, GameplayFilter{}, ScopeNaturalOnly)
	if err != nil {
		t.Fatalf("GetGameplayDiagnostics: %v", err)
	}
	if result.Summary.Placements != 1 {
		t.Errorf("placements = %d, want 1 (event-level, unaffected by the later developer command)", result.Summary.Placements)
	}
	if result.Summary.Attempts != 0 {
		t.Errorf("attempts = %d, want 0 (the attempt is developer-touched, so it must not count as a natural entry)", result.Summary.Attempts)
	}
	if result.Summary.CompletedHouses != 0 {
		t.Errorf("completed houses = %d, want 0", result.Summary.CompletedHouses)
	}
}
