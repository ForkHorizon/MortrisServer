package analytics

// Stage 2 exit-criteria tests (docs/puzzle-analytics-remaining-plan.md
// section 5): each fixture below is built to trip exactly one integrity
// bucket, so a bug that makes one bucket over- or under-count shows up as
// a single failing test rather than a wall of unrelated red.
//
// Split across this file and puzzle_quality_integrity_test.go once the
// combined file hit the repo's 300-line gate.

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

// cleanChainEvents is a full natural chain — schema v2, an imported
// revision, house_local coordinates, contiguous sequence and both event
// indexes, a checkpoint whose hash actually matches, and a house run
// that closes.
func cleanChainEvents(install, session string, now time.Time) []seedEvent {
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
	return []seedEvent{
		{EventID: "a0000000-0000-4000-8000-000000000001", InstallID: install, SessionID: session, Sequence: 1, Name: "house_run_started", Kind: "product", EffectiveAt: now, Properties: props(map[string]any{"house_event_index": 0})},
		{EventID: "a0000000-0000-4000-8000-000000000002", InstallID: install, SessionID: session, Sequence: 2, Name: "wave_attempt_started", Kind: "product", EffectiveAt: now.Add(time.Second), Properties: props(map[string]any{"house_event_index": 1, "attempt_event_index": 0})},
		{EventID: "a0000000-0000-4000-8000-000000000003", InstallID: install, SessionID: session, Sequence: 3, Name: "detail_taken", Kind: "product", EffectiveAt: now.Add(2 * time.Second), Properties: props(map[string]any{"house_event_index": 2, "attempt_event_index": 1, "interaction_id": "move-1", "block_id": 10})},
		{EventID: "a0000000-0000-4000-8000-000000000004", InstallID: install, SessionID: session, Sequence: 4, Name: "detail_released", Kind: "product", EffectiveAt: now.Add(3 * time.Second), Properties: props(map[string]any{"house_event_index": 3, "attempt_event_index": 2, "interaction_id": "move-1", "block_id": 10, "candidate_target_id": 10})},
		{EventID: "a0000000-0000-4000-8000-000000000005", InstallID: install, SessionID: session, Sequence: 5, Name: "placement_resolved", Kind: "product", EffectiveAt: now.Add(4 * time.Second), Properties: props(map[string]any{"house_event_index": 4, "attempt_event_index": 3, "interaction_id": "move-1", "block_id": 10, "outcome": "placed"})},
		{EventID: "a0000000-0000-4000-8000-000000000006", InstallID: install, SessionID: session, Sequence: 6, Name: "state_checkpoint", Kind: "product", EffectiveAt: now.Add(5 * time.Second), Properties: props(map[string]any{"house_event_index": 5, "attempt_event_index": 4, "placed_block_ids": "10", "placed_state_hash": "4a44dc15364204a80fe80e9039455cc1608281820fe2b24f1e5233ade6af1dd5"})},
		{EventID: "a0000000-0000-4000-8000-000000000007", InstallID: install, SessionID: session, Sequence: 7, Name: "wave_completed", Kind: "product", EffectiveAt: now.Add(6 * time.Second), Properties: props(map[string]any{"house_event_index": 6, "attempt_event_index": 5})},
		{EventID: "a0000000-0000-4000-8000-000000000008", InstallID: install, SessionID: session, Sequence: 8, Name: "house_completed", Kind: "product", EffectiveAt: now.Add(7 * time.Second), Properties: props(map[string]any{"house_event_index": 7})},
	}
}

// A full natural chain must read green with zero of every defect counter.
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
	events := cleanChainEvents(install, session, now)
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
	day := now.Format("2006-01-02")
	if _, err := pool.Exec(ctx, `
		INSERT INTO event_rejection_stats (project_id, name, code, day, count, last_seen_at) VALUES
		($1,'placement_resolved','undeclared_property',$2,2,$3),
		($1,'detail_taken','unknown_event',$2,1,$3)
	`, projectID, day, now); err != nil {
		t.Fatalf("seed event_rejection_stats: %v", err)
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
