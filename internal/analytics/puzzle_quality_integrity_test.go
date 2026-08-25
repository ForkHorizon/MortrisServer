package analytics

// Continuation of puzzle_quality_test.go, split out once the combined
// file hit the repo's 300-line gate. Same Stage 2 exit-criteria coverage
// (docs/puzzle-analytics-remaining-plan.md section 5), items 8 onward:
// sequence/index integrity, developer-action pairing, open-run recency,
// delivery delay, and the build filter.

import (
	"context"
	"testing"
	"time"
)

func TestPuzzleQuality_SequenceGapAndDuplicateCountedSeparately(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "55555555-5555-4555-8555-555555555555"
	session := "65555555-5555-4555-8555-555555555555"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "d0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1, Name: "house_run_started", Kind: "product", EffectiveAt: now},
		// Sequence 2 never arrives: one gap.
		{EventID: "d0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: session, Sequence: 3, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now.Add(time.Second)},
		// A retried upload re-delivering sequence 3: one duplicate.
		{EventID: "d0000000-0000-4000-8000-000000000003", InstallID: install, SessionID: session, Sequence: 3, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now.Add(2 * time.Second)},
	})

	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.SequenceGaps != 1 {
		t.Errorf("sequence gaps = %d, want 1", q.SequenceGaps)
	}
	if q.SequenceDuplicates != 1 {
		t.Errorf("sequence duplicates = %d, want 1", q.SequenceDuplicates)
	}
}

// A batch that arrives late (high received_at, unrelated to this — the
// query orders purely by the sequence property) but whose sequence
// numbers are internally contiguous must not read as a sequence failure.
func TestPuzzleQuality_LateBatchIsNotASequenceFailure(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "66666666-6666-4666-8666-666666666666"
	session := "76666666-6666-4666-8666-666666666666"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "e0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1, Name: "house_run_started", Kind: "product", EffectiveAt: now, SentAtClient: now, ReceivedAt: now.Add(3 * time.Hour)},
		{EventID: "e0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: session, Sequence: 2, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now.Add(time.Second), SentAtClient: now.Add(time.Second), ReceivedAt: now.Add(3 * time.Hour)},
	})

	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(4*time.Hour), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.SequenceGaps != 0 || q.SequenceDuplicates != 0 {
		t.Errorf("a late but internally-ordered batch must not report a sequence failure, got gaps=%d dups=%d", q.SequenceGaps, q.SequenceDuplicates)
	}
}

func TestPuzzleQuality_HouseEventIndexGapAndAttemptEventIndexDuplicate(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "77777777-7777-4777-8777-777777777777"
	session := "87777777-7777-4777-8777-777777777777"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "f0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1, Name: "house_run_started", Kind: "product", EffectiveAt: now, Properties: map[string]any{"house_run_id": "run-x", "house_event_index": 0}},
		// house_event_index jumps 0 -> 2: one gap.
		{EventID: "f0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: session, Sequence: 2, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now.Add(time.Second), Properties: map[string]any{"house_run_id": "run-x", "house_event_index": 2, "attempt_id": "attempt-x", "attempt_event_index": 0}},
		// attempt_event_index repeats 0: one duplicate.
		{EventID: "f0000000-0000-4000-8000-000000000003", InstallID: install, SessionID: session, Sequence: 3, Name: "detail_taken", Kind: "product", EffectiveAt: now.Add(2 * time.Second), Properties: map[string]any{"house_run_id": "run-x", "house_event_index": 3, "attempt_id": "attempt-x", "attempt_event_index": 0}},
	})

	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.HouseEventIndexGaps != 1 || q.HouseEventIndexDuplicates != 0 {
		t.Errorf("house_event_index = gaps:%d dups:%d, want 1/0", q.HouseEventIndexGaps, q.HouseEventIndexDuplicates)
	}
	if q.AttemptEventIndexDuplicates != 1 || q.AttemptEventIndexGaps != 0 {
		t.Errorf("attempt_event_index = gaps:%d dups:%d, want 0/1", q.AttemptEventIndexGaps, q.AttemptEventIndexDuplicates)
	}
}

// A developer action pairs by developer_action_id alone, so a start and
// its completion after a scene reload (a new session_id, same action id)
// must still pair. A command with no terminal, and a mutation with no
// matching start, must each be flagged.
func TestPuzzleQuality_DeveloperActionPairingCrossesSceneReload(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "88888888-8888-4888-8888-888888888888"
	sessionBefore := "98888888-8888-4888-8888-888888888888"
	sessionAfter := "a8888888-8888-4888-8888-888888888888"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		// Paired across a scene reload (different session_id).
		{EventID: "10000000-0000-4000-8000-000000000001", InstallID: install, SessionID: sessionBefore, Sequence: 1, Name: "developer_command_started", Kind: "product", EffectiveAt: now, Properties: map[string]any{"developer_action_id": "action-paired"}},
		{EventID: "10000000-0000-4000-8000-000000000002", InstallID: install, SessionID: sessionAfter, Sequence: 1, Name: "developer_command_completed", Kind: "product", EffectiveAt: now.Add(time.Minute), Properties: map[string]any{"developer_action_id": "action-paired"}},
		{EventID: "10000000-0000-4000-8000-000000000003", InstallID: install, SessionID: sessionAfter, Sequence: 2, Name: "developer_progress_mutated", Kind: "product", EffectiveAt: now.Add(time.Minute), Properties: map[string]any{"developer_action_id": "action-paired"}},
		// Started, never a terminal result.
		{EventID: "10000000-0000-4000-8000-000000000004", InstallID: install, SessionID: sessionBefore, Sequence: 2, Name: "developer_command_started", Kind: "product", EffectiveAt: now.Add(2 * time.Minute), Properties: map[string]any{"developer_action_id": "action-unpaired"}},
		// A mutation with no matching start at all.
		{EventID: "10000000-0000-4000-8000-000000000005", InstallID: install, SessionID: sessionAfter, Sequence: 3, Name: "developer_progress_mutated", Kind: "product", EffectiveAt: now.Add(3 * time.Minute), Properties: map[string]any{"developer_action_id": "action-orphan-mutation"}},
	})

	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.UnpairedDeveloperCommands != 1 {
		t.Errorf("unpaired developer commands = %d, want 1", q.UnpairedDeveloperCommands)
	}
	if q.UnpairedDeveloperMutations != 1 {
		t.Errorf("unpaired developer mutations = %d, want 1", q.UnpairedDeveloperMutations)
	}
}

// Reuses lostThresholdHours (puzzle_diagnostics.go) rather than a second
// definition: a house run whose last event is recent is "still might
// finish," one well past the threshold is "stale/lost" — neither is a
// completed run, so both stay out of the natural-completion numerator
// elsewhere, but the quality view must not call the recent one lost.
func TestPuzzleQuality_OpenRunsSplitByRecentVsStale(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "99999999-9999-4999-8999-999999999999"
	session := "a9999999-9999-4999-8999-999999999999"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "20000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1, Name: "house_run_started", Kind: "product", EffectiveAt: now.Add(-time.Minute), Properties: map[string]any{"house_run_id": "run-recent"}},
		{EventID: "20000000-0000-4000-8000-000000000002", InstallID: install, SessionID: session, Sequence: 2, Name: "house_run_started", Kind: "product", EffectiveAt: now.Add(-(lostThresholdHours + 2) * time.Hour), Properties: map[string]any{"house_run_id": "run-stale"}},
	})

	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-(lostThresholdHours+3)*time.Hour), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.OpenHouseRunsRecent != 1 {
		t.Errorf("open house runs recent = %d, want 1", q.OpenHouseRunsRecent)
	}
	if q.OpenHouseRunsStale != 1 {
		t.Errorf("open house runs stale = %d, want 1", q.OpenHouseRunsStale)
	}
}

func TestPuzzleQuality_DeliveryDelayDistribution(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "12121212-1212-4212-8212-121212121212"
	session := "22121212-1212-4212-8212-121212121212"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "30000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1, Name: "house_run_started", Kind: "product", EffectiveAt: now, SentAtClient: now, ReceivedAt: now.Add(500 * time.Millisecond)},
		{EventID: "30000000-0000-4000-8000-000000000002", InstallID: install, SessionID: session, Sequence: 2, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now.Add(time.Second), SentAtClient: now.Add(time.Second), ReceivedAt: now.Add(time.Second + 500*time.Millisecond)},
	})

	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.DeliveryDelayMS.Samples != 2 {
		t.Fatalf("delay samples = %d, want 2", q.DeliveryDelayMS.Samples)
	}
	if q.DeliveryDelayMS.P50MS < 490 || q.DeliveryDelayMS.P50MS > 510 {
		t.Errorf("p50 delay = %v, want ~500ms", q.DeliveryDelayMS.P50MS)
	}
}

// The build filter scopes per-event counts to one build while Builds
// itself stays unfiltered — the plan doc requires the list to always show
// every build in range so the dashboard can offer it as a choice.
func TestPuzzleQuality_BuildFilterScopesCountsButNotBuildList(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "13131313-1313-4313-8313-131313131313"
	session := "23131313-1313-4313-8313-131313131313"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "40000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1, Name: "house_run_started", Kind: "product", EffectiveAt: now, BuildNumber: "build-a"},
		{EventID: "40000000-0000-4000-8000-000000000002", InstallID: install, SessionID: session, Sequence: 2, Name: "house_run_started", Kind: "product", EffectiveAt: now.Add(time.Second), BuildNumber: "build-b"},
	})

	buildA := "build-a"
	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), &buildA)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.TotalGameplayEvents != 1 {
		t.Errorf("total gameplay events scoped to build-a = %d, want 1", q.TotalGameplayEvents)
	}
	if len(q.Builds) != 2 {
		t.Errorf("builds list = %+v, want both builds regardless of the filter", q.Builds)
	}
}
