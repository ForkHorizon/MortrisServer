package analytics

// Stage 2 exit-criteria tests (docs/puzzle-analytics-remaining-plan.md
// section 5): each fixture below is built to trip exactly one integrity
// bucket, so each bug maps to a focused failing test.

import (
	"context"
	"testing"
	"time"
)

func TestPuzzleQuality_EmptyRangeIsGreyNotDivideByZero(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)

	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Hour), now, nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.Status != "grey" {
		t.Errorf("status = %q, want grey for zero events", q.Status)
	}
	if q.TotalGameplayEvents != 0 || len(q.Rejections) != 0 || len(q.Builds) != 0 {
		t.Errorf("expected all-zero/empty on an empty range, got %+v", q)
	}
}

// A full natural chain — schema v2, an imported revision, house_local
// coordinates, contiguous sequence and both event indexes, a checkpoint
// whose hash actually matches, and a house run that closes — must read
// green with zero of every defect counter.
func TestPuzzleQuality_CleanChainIsGreen(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "11111111-1111-4111-8111-111111111111"
	session := "21111111-1111-4111-8111-111111111111"
	seedInstallation(t, pool, projectID, install, &now)

	if _, err := pool.Exec(ctx, `INSERT INTO puzzle_content_revisions (project_id, content_revision, schema_version, catalog) VALUES ($1,'rev-clean',2,'{}'::jsonb)`, projectID); err != nil {
		t.Fatalf("seed revision: %v", err)
	}

	props := func(extra map[string]any) map[string]any {
		base := map[string]any{
			"schema_version": 2, "content_revision": "rev-clean", "city_id": 1, "house_id": 1,
			"house_run_id": "run-1", "attempt_id": "attempt-1", "coordinate_space": "house_local",
		}
		for k, v := range extra {
			base[k] = v
		}
		return base
	}

	events := []seedEvent{
		{EventID: "a0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1, Name: "house_run_started", Kind: "product", EffectiveAt: now, Properties: props(map[string]any{"house_event_index": 0})},
		{EventID: "a0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: session, Sequence: 2, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now.Add(time.Second), Properties: props(map[string]any{"house_event_index": 1, "attempt_event_index": 0})},
		{EventID: "a0000000-0000-4000-8000-000000000003", InstallID: install, SessionID: session, Sequence: 3, Name: "detail_taken", Kind: "product", EffectiveAt: now.Add(2 * time.Second), Properties: props(map[string]any{"house_event_index": 2, "attempt_event_index": 1, "interaction_id": "move-1", "block_id": 10})},
		{EventID: "a0000000-0000-4000-8000-000000000004", InstallID: install, SessionID: session, Sequence: 4, Name: "detail_released", Kind: "product", EffectiveAt: now.Add(3 * time.Second), Properties: props(map[string]any{"house_event_index": 3, "attempt_event_index": 2, "interaction_id": "move-1", "block_id": 10, "candidate_target_id": 10})},
		{EventID: "a0000000-0000-4000-8000-000000000005", InstallID: install, SessionID: session, Sequence: 5, Name: "placement_resolved", Kind: "product", EffectiveAt: now.Add(4 * time.Second), Properties: props(map[string]any{"house_event_index": 4, "attempt_event_index": 3, "interaction_id": "move-1", "block_id": 10, "outcome": "fell_no_snap_target"})},
		{EventID: "a0000000-0000-4000-8000-000000000006", InstallID: install, SessionID: session, Sequence: 6, Name: "detail_returned", Kind: "product", EffectiveAt: now.Add(5 * time.Second), Properties: props(map[string]any{"house_event_index": 5, "attempt_event_index": 4, "interaction_id": "move-1", "block_id": 10})},
		{EventID: "a0000000-0000-4000-8000-000000000007", InstallID: install, SessionID: session, Sequence: 7, Name: "state_checkpoint", Kind: "product", EffectiveAt: now.Add(6 * time.Second), Properties: props(map[string]any{"house_event_index": 6, "attempt_event_index": 5, "placed_block_ids": "", "placed_state_hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"})},
		{EventID: "a0000000-0000-4000-8000-000000000008", InstallID: install, SessionID: session, Sequence: 8, Name: "detail_taken", Kind: "product", EffectiveAt: now.Add(7 * time.Second), Properties: props(map[string]any{"house_event_index": 7, "attempt_event_index": 6, "interaction_id": "move-2", "block_id": 10})},
		{EventID: "a0000000-0000-4000-8000-000000000009", InstallID: install, SessionID: session, Sequence: 9, Name: "detail_released", Kind: "product", EffectiveAt: now.Add(8 * time.Second), Properties: props(map[string]any{"house_event_index": 8, "attempt_event_index": 7, "interaction_id": "move-2", "block_id": 10, "candidate_target_id": 10})},
		{EventID: "a0000000-0000-4000-8000-000000000010", InstallID: install, SessionID: session, Sequence: 10, Name: "placement_resolved", Kind: "product", EffectiveAt: now.Add(9 * time.Second), Properties: props(map[string]any{"house_event_index": 9, "attempt_event_index": 8, "interaction_id": "move-2", "block_id": 10, "outcome": "placed"})},
		{EventID: "a0000000-0000-4000-8000-000000000011", InstallID: install, SessionID: session, Sequence: 11, Name: "wave_completed", Kind: "product", EffectiveAt: now.Add(10 * time.Second), Properties: props(map[string]any{"house_event_index": 10, "attempt_event_index": 9})},
		{EventID: "a0000000-0000-4000-8000-000000000012", InstallID: install, SessionID: session, Sequence: 12, Name: "house_completed", Kind: "product", EffectiveAt: now.Add(11 * time.Second), Properties: props(map[string]any{"house_event_index": 11})},
	}
	seedEvents(t, pool, projectID, events)

	q, err := GetPuzzleQuality(ctx, pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.Status != "green" {
		t.Fatalf("status = %q, reasons = %v, want green", q.Status, q.Reasons)
	}
	if q.SchemaV2Events != int64(len(events)) || q.MissingSchemaVersion != 0 {
		t.Errorf("schema version = %d v2 / %d missing, want %d / 0", q.SchemaV2Events, q.MissingSchemaVersion, len(events))
	}
	if q.UnknownRevisionEvents != 0 || q.InvalidCoordinateSpaceEvents != 0 {
		t.Errorf("expected clean revision/coordinate space, got unknown=%d invalid_coord=%d", q.UnknownRevisionEvents, q.InvalidCoordinateSpaceEvents)
	}
	if q.SequenceGaps != 0 || q.SequenceDuplicates != 0 || q.HouseEventIndexGaps != 0 || q.HouseEventIndexDuplicates != 0 || q.AttemptEventIndexGaps != 0 || q.AttemptEventIndexDuplicates != 0 {
		t.Errorf("expected zero index integrity failures, got %+v", q)
	}
	if q.OrphanInteractions != 0 {
		t.Errorf("orphan interactions = %d, want 0 (take/release/resolve all present)", q.OrphanInteractions)
	}
	if q.CheckpointMismatches != 0 {
		t.Errorf("checkpoint mismatches = %d, want 0 (hash matches placed_block_ids)", q.CheckpointMismatches)
	}
	if q.DistinctHouseRuns != 1 || q.DistinctWaveAttempts != 1 || q.DistinctInteractions != 1 {
		t.Errorf("distinct counts = runs:%d attempts:%d interactions:%d, want 1/1/1", q.DistinctHouseRuns, q.DistinctWaveAttempts, q.DistinctInteractions)
	}
	// Regression: a nil Reasons slice on the green path marshals to JSON
	// `null`, and the dashboard's `quality.status_reasons.length` crashes
	// the whole page on a null. Must always be a real (possibly empty) slice.
	if q.Reasons == nil {
		t.Errorf("Reasons is nil on the green path, want a non-nil empty slice")
	}
}

func TestPuzzleQuality_RejectionsAreGroupedByNameAndCode(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC()
	install := "22222222-2222-4222-8222-222222222222"

	if _, err := pool.Exec(ctx, `
		INSERT INTO ingestion_stats (project_id, install_id, accepted_count, duplicate_count, rejected_count)
		VALUES ($1,$2,10,2,3)
	`, projectID, install); err != nil {
		t.Fatalf("seed ingestion_stats: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO event_rejection_occurrences (project_id, install_id, name, code, received_at) VALUES
		($1,$2,'placement_resolved','undeclared_property',$3),
		($1,$2,'placement_resolved','undeclared_property',$3),
		($1,$2,'detail_taken','unknown_event',$3)
	`, projectID, install, now); err != nil {
		t.Fatalf("seed event_rejection_occurrences: %v", err)
	}

	q, err := GetPuzzleQuality(ctx, pool, projectID, now.Add(-time.Hour), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.AcceptedEvents != 10 || q.DuplicateEvents != 2 || q.RejectedEvents != 3 || q.ReceivedEvents != 15 {
		t.Errorf("ingestion totals = %+v, want accepted=10 duplicate=2 rejected=3 received=15", q)
	}
	if len(q.Rejections) != 2 {
		t.Fatalf("rejections = %+v, want 2 distinct (name, code) buckets", q.Rejections)
	}
}

func TestPuzzleQuality_RejectedOnlyRangeIsRed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC()
	install := "21212121-2121-4212-8212-212121212121"
	seedInstallation(t, pool, projectID, install, &now)
	if _, err := pool.Exec(ctx, `
		INSERT INTO ingestion_stats (project_id, install_id, received_at, accepted_count, duplicate_count, rejected_count)
		VALUES ($1,$2,$3,0,0,1)
	`, projectID, install, now); err != nil {
		t.Fatalf("seed rejected outcome: %v", err)
	}
	q, err := GetPuzzleQuality(ctx, pool, projectID, now.Add(-time.Minute), now.Add(time.Minute), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.Status != "red" || q.RejectedEvents != 1 {
		t.Fatalf("rejected-only range = status %q, rejected %d; want red/1", q.Status, q.RejectedEvents)
	}
}

func TestPuzzleQuality_InvalidNumericPropertiesAreRedNotDatabaseErrors(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "31313131-3131-4313-8313-313131313131"
	session := "41414141-4141-4414-8414-414141414141"
	seedInstallation(t, pool, projectID, install, &now)
	seedEvents(t, pool, projectID, []seedEvent{{
		EventID: "a1000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1,
		Name: "house_run_started", Kind: "product", EffectiveAt: now,
		Properties: map[string]any{"schema_version": "two", "house_run_id": "run-bad", "house_event_index": "not-an-index"},
	}})
	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(time.Minute), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality must report malformed properties, not fail: %v", err)
	}
	if q.Status != "red" || q.InvalidSchemaVersion != 1 || q.InvalidEventIndexes != 1 {
		t.Fatalf("malformed properties = status %q schema %d indexes %d; want red/1/1", q.Status, q.InvalidSchemaVersion, q.InvalidEventIndexes)
	}
}

func TestPuzzleQuality_SchemaV2UnknownRevisionIsRed(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "51515151-5151-4515-8515-515151515151"
	session := "61616161-6161-4616-8616-616161616161"
	seedInstallation(t, pool, projectID, install, &now)
	seedEvents(t, pool, projectID, []seedEvent{{
		EventID: "a2000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1,
		Name: "house_run_started", Kind: "product", EffectiveAt: now,
		Properties: map[string]any{"schema_version": 2, "content_revision": "not-imported", "house_run_id": "run-unknown"},
	}})
	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(time.Minute), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.Status != "red" || q.UnknownRevisionEvents != 1 {
		t.Fatalf("unknown revision = status %q, count %d; want red/1", q.Status, q.UnknownRevisionEvents)
	}
}

// The plan doc calls out explicitly: a fall whose terminal event is a
// later detail_returned must not be flagged orphaned merely because
// placement_resolved said it fell. Only the interaction with no terminal
// event at all should count.
func TestPuzzleQuality_OrphanInteractionExcludesLateReturn(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "33333333-3333-4333-8333-333333333333"
	session := "43333333-3333-4333-8333-333333333333"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		// Truly orphaned: taken and never resolved, released or returned.
		{EventID: "b0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1, Name: "detail_taken", Kind: "product", EffectiveAt: now, Properties: map[string]any{"interaction_id": "orphan-1", "block_id": 1}},
		// Fell, then returned later with the same interaction_id: complete.
		{EventID: "b0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: session, Sequence: 2, Name: "detail_taken", Kind: "product", EffectiveAt: now.Add(time.Second), Properties: map[string]any{"interaction_id": "fall-1", "block_id": 2}},
		{EventID: "b0000000-0000-4000-8000-000000000003", InstallID: install, SessionID: session, Sequence: 3, Name: "detail_released", Kind: "product", EffectiveAt: now.Add(2 * time.Second), Properties: map[string]any{"interaction_id": "fall-1", "block_id": 2}},
		{EventID: "b0000000-0000-4000-8000-000000000004", InstallID: install, SessionID: session, Sequence: 4, Name: "placement_resolved", Kind: "product", EffectiveAt: now.Add(3 * time.Second), Properties: map[string]any{"interaction_id": "fall-1", "block_id": 2, "outcome": "fell_no_snap_target"}},
		{EventID: "b0000000-0000-4000-8000-000000000005", InstallID: install, SessionID: session, Sequence: 5, Name: "detail_returned", Kind: "product", EffectiveAt: now.Add(4 * time.Second), Properties: map[string]any{"interaction_id": "fall-1", "block_id": 2}},
	})

	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.OrphanInteractions != 1 {
		t.Errorf("orphan interactions = %d, want exactly 1 (the fall+return chain must not count)", q.OrphanInteractions)
	}
}

func TestPuzzleQuality_InteractionCompletionOutsideRangeAndOtherInstallDoNotHideOrphans(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	installA := "71717171-7171-4717-8717-717171717171"
	installB := "81818181-8181-4818-8818-818181818181"
	sessionA := "91919191-9191-4919-8919-919191919191"
	sessionB := "a1919191-9191-4919-8919-919191919191"
	seedInstallation(t, pool, projectID, installA, &now)
	seedInstallation(t, pool, projectID, installB, &now)
	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "a3000000-0000-4000-8000-000000000001", InstallID: installA, SessionID: sessionA, Sequence: 1, Name: "detail_taken", Kind: "product", EffectiveAt: now, Properties: map[string]any{"interaction_id": "shared"}},
		{EventID: "a3000000-0000-4000-8000-000000000002", InstallID: installA, SessionID: sessionA, Sequence: 2, Name: "detail_released", Kind: "product", EffectiveAt: now.Add(time.Second), Properties: map[string]any{"interaction_id": "shared"}},
		{EventID: "a3000000-0000-4000-8000-000000000003", InstallID: installA, SessionID: sessionA, Sequence: 3, Name: "placement_resolved", Kind: "product", EffectiveAt: now.Add(2 * time.Second), Properties: map[string]any{"interaction_id": "shared", "outcome": "fell_no_snap_target"}},
		{EventID: "a3000000-0000-4000-8000-000000000004", InstallID: installB, SessionID: sessionB, Sequence: 1, Name: "detail_returned", Kind: "product", EffectiveAt: now.Add(3 * time.Second), Properties: map[string]any{"interaction_id": "shared"}},
		{EventID: "a3000000-0000-4000-8000-000000000005", InstallID: installA, SessionID: sessionA, Sequence: 4, Name: "detail_returned", Kind: "product", EffectiveAt: now.Add(2 * time.Hour), Properties: map[string]any{"interaction_id": "shared"}},
	})
	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(time.Minute), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.OrphanInteractions != 0 {
		t.Fatalf("return after range must close the touched interaction, got %d orphans", q.OrphanInteractions)
	}
}

func TestPuzzleQuality_DistinctLifecycleIDsAreInstallationScoped(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	installA := "c1919191-9191-4919-8919-919191919191"
	installB := "d1919191-9191-4919-8919-919191919191"
	seedInstallation(t, pool, projectID, installA, &now)
	seedInstallation(t, pool, projectID, installB, &now)
	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "a4000000-0000-4000-8000-000000000001", InstallID: installA, SessionID: "e1919191-9191-4919-8919-919191919191", Sequence: 1, Name: "house_run_started", Kind: "product", EffectiveAt: now, Properties: map[string]any{"house_run_id": "same", "attempt_id": "same", "interaction_id": "same"}},
		{EventID: "a4000000-0000-4000-8000-000000000002", InstallID: installB, SessionID: "f1919191-9191-4919-8919-919191919191", Sequence: 1, Name: "house_run_started", Kind: "product", EffectiveAt: now, Properties: map[string]any{"house_run_id": "same", "attempt_id": "same", "interaction_id": "same"}},
	})
	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(time.Minute), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.DistinctHouseRuns != 2 || q.DistinctWaveAttempts != 2 || q.DistinctInteractions != 2 {
		t.Fatalf("distinct IDs = runs:%d attempts:%d interactions:%d, want 2/2/2", q.DistinctHouseRuns, q.DistinctWaveAttempts, q.DistinctInteractions)
	}
}

func TestPuzzleQuality_CheckpointHashMismatch(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "44444444-4444-4444-8444-444444444444"
	session := "54444444-4444-4444-8444-444444444444"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "c0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1, Name: "state_checkpoint", Kind: "product", EffectiveAt: now, Properties: map[string]any{"placed_block_ids": "10,11", "placed_state_hash": "not-a-real-hash"}},
	})

	q, err := GetPuzzleQuality(context.Background(), pool, projectID, now.Add(-time.Minute), now.Add(time.Hour), nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	if q.CheckpointMismatches != 1 {
		t.Errorf("checkpoint mismatches = %d, want 1", q.CheckpointMismatches)
	}
}

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
