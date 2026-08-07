package analytics

// Stage 4 exit-criteria tests for the wave staircase (docs/puzzle-analytics-remaining-plan.md
// section 7).

import (
	"context"
	"testing"
	"time"
)

// A run that enters wave 1 and makes no placement before giving up must
// still show up as an entry — the funnel exists precisely to see that
// drop-off, so counting only placement events would hide it.
func TestPuzzleWaveFunnel_ZeroPlacementEntryStillCounts(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "11111111-1111-4111-8111-111111111111"
	seedInstallation(t, pool, projectID, install, &now)
	seedRevisionWithWaves(t, pool, projectID, 2)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "a0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 1, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now,
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 0, "house_run_id": "run-1", "attempt_id": "attempt-0"}},
		// Entered wave 1, no placement events at all, then went quiet.
		{EventID: "a0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 2, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now.Add(time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 1, "house_run_id": "run-1", "attempt_id": "attempt-1"}},
	})

	funnel, err := GetPuzzleWaveFunnel(ctx, pool, projectID, 1, 1, now.Add(-time.Minute), now.Add(time.Hour), ScopeNaturalOnly)
	if err != nil {
		t.Fatalf("GetPuzzleWaveFunnel: %v", err)
	}
	if len(funnel.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(funnel.Steps))
	}
	if funnel.Steps[0].Entered != 1 || funnel.Steps[1].Entered != 1 {
		t.Errorf("entered = wave0:%d wave1:%d, want 1/1 (a zero-placement entry still counts)", funnel.Steps[0].Entered, funnel.Steps[1].Entered)
	}
	if funnel.Steps[0].ReachedNext != 1 {
		t.Errorf("wave0 reached_next = %d, want 1", funnel.Steps[0].ReachedNext)
	}
}

// A restart mid-wave (attempt_recovered, same house_run_id/wave_index)
// must not double-count the entry — it is still the same run in the same
// wave, not a second one.
func TestPuzzleWaveFunnel_RecoveryDoesNotDoubleCountEntry(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "22222222-2222-4222-8222-222222222222"
	seedInstallation(t, pool, projectID, install, &now)
	seedRevisionWithWaves(t, pool, projectID, 1)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "b0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 1, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now,
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 0, "house_run_id": "run-1", "attempt_id": "attempt-0"}},
		{EventID: "b0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 2, Name: "attempt_recovered", Kind: "product", EffectiveAt: now.Add(time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 0, "house_run_id": "run-1", "attempt_id": "attempt-0"}},
		{EventID: "b0000000-0000-4000-8000-000000000003", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 3, Name: "placement_resolved", Kind: "product", EffectiveAt: now.Add(2 * time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 0, "house_run_id": "run-1", "attempt_id": "attempt-0", "block_id": 0, "outcome": "placed"}},
	})

	funnel, err := GetPuzzleWaveFunnel(ctx, pool, projectID, 1, 1, now.Add(-time.Minute), now.Add(time.Hour), ScopeNaturalOnly)
	if err != nil {
		t.Fatalf("GetPuzzleWaveFunnel: %v", err)
	}
	if funnel.Steps[0].Entered != 1 {
		t.Errorf("entered = %d, want 1 (recovery must not double-count)", funnel.Steps[0].Entered)
	}
}

// A recent open run (last event within lostThresholdHours, house not
// completed) belongs in RecentHere, not AbandonedHere.
func TestPuzzleWaveFunnel_RecentOpenRunIsNotAbandoned(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "33333333-3333-4333-8333-333333333333"
	seedInstallation(t, pool, projectID, install, &now)
	seedRevisionWithWaves(t, pool, projectID, 2)

	seedEvents(t, pool, projectID, []seedEvent{
		// Recent: last event a minute ago.
		{EventID: "c0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 1, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now.Add(-time.Minute),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 0, "house_run_id": "run-recent", "attempt_id": "attempt-recent"}},
		// Stale: last event well past lostThresholdHours.
		{EventID: "c0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 2, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now.Add(-(lostThresholdHours + 2) * time.Hour),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 0, "house_run_id": "run-stale", "attempt_id": "attempt-stale"}},
	})

	funnel, err := GetPuzzleWaveFunnel(ctx, pool, projectID, 1, 1, now.Add(-(lostThresholdHours+3)*time.Hour), now.Add(time.Hour), ScopeNaturalOnly)
	if err != nil {
		t.Fatalf("GetPuzzleWaveFunnel: %v", err)
	}
	if funnel.Steps[0].RecentHere != 1 {
		t.Errorf("recent_here = %d, want 1", funnel.Steps[0].RecentHere)
	}
	if funnel.Steps[0].AbandonedHere != 1 {
		t.Errorf("abandoned_here = %d, want 1", funnel.Steps[0].AbandonedHere)
	}
}

// A range boundary must not split one house run into a false completion
// or loss: this run's first event is before `from`, its last (the
// completion) is inside [from, to) — the documented rule (puzzle_wave_funnel.go,
// puzzle_house_run_classification) is that a run is attributed by its
// last event, so the whole run, including its earlier wave, is visible
// and counted once.
func TestPuzzleWaveFunnel_RangeBoundaryDoesNotSplitARun(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "44444444-4444-4444-8444-444444444444"
	seedInstallation(t, pool, projectID, install, &now)
	seedRevisionWithWaves(t, pool, projectID, 2)

	rangeFrom := now.Add(-time.Hour)
	seedEvents(t, pool, projectID, []seedEvent{
		// Started well before the range.
		{EventID: "d0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 1, Name: "wave_attempt_started", Kind: "product", EffectiveAt: rangeFrom.Add(-24 * time.Hour),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 0, "house_run_id": "run-1", "attempt_id": "attempt-0"}},
		// Completed inside the range.
		{EventID: "d0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 2, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now,
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 1, "house_run_id": "run-1", "attempt_id": "attempt-1"}},
		{EventID: "d0000000-0000-4000-8000-000000000003", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 3, Name: "wave_completed", Kind: "product", EffectiveAt: now.Add(time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 1, "house_run_id": "run-1", "attempt_id": "attempt-1"}},
		{EventID: "d0000000-0000-4000-8000-000000000004", InstallID: install, SessionID: "99999999-9999-4999-8999-999999999999", Sequence: 4, Name: "house_completed", Kind: "product", EffectiveAt: now.Add(2 * time.Second),
			Properties: map[string]any{"city_id": 1, "house_id": 1, "wave_index": 1, "house_run_id": "run-1", "attempt_id": "attempt-1"}},
	})

	funnel, err := GetPuzzleWaveFunnel(ctx, pool, projectID, 1, 1, rangeFrom, now.Add(time.Hour), ScopeNaturalOnly)
	if err != nil {
		t.Fatalf("GetPuzzleWaveFunnel: %v", err)
	}
	// The whole run (both waves) is attributed to this range by its last
	// event, so wave 0's entry is visible too, not silently dropped.
	if funnel.Steps[0].Entered != 1 {
		t.Errorf("wave0 entered = %d, want 1 (the whole run counts once, by its last event)", funnel.Steps[0].Entered)
	}
	if funnel.Steps[1].Entered != 1 || funnel.Steps[1].Completed != 1 {
		t.Errorf("wave1 entered/completed = %d/%d, want 1/1", funnel.Steps[1].Entered, funnel.Steps[1].Completed)
	}
}
