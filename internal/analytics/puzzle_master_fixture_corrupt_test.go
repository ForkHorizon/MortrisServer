package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMasterFixture_CorruptedVariantsVerifyAlerts checks that deliberately
// corrupted variations of schema-v2 data trip the exact expected alerts.
func TestMasterFixture_CorruptedSequenceGap(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	projectID := seedProject(t, pool, false)
	fix := NewMasterFixture(projectID, "1.2.0", "120", now)
	fix.Events = fix.Events[:10]
	fix.Events[5].Sequence = 50
	if err := fix.Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	assertAlertCategory(t, ctx, pool, projectID, "sequence_integrity")
}

func TestMasterFixture_CorruptedOrphanInteraction(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	projectID := seedProject(t, pool, false)
	fix := NewMasterFixture(projectID, "1.2.0", "120", now)
	fix.Events = []FixtureEvent{
		{
			EventID: "11111111-2222-4333-8444-555555555555", InstallID: fix.InstallID,
			SessionID: fix.SessionID, Sequence: 1, Name: "detail_taken", Kind: "product",
			EffectiveAt: now, AppVersion: fix.AppVersion, BuildNumber: fix.BuildNumber,
			Properties: fix.baseProps(map[string]any{"interaction_id": "orphan-only"}),
		},
	}
	if err := fix.Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	assertAlertCategory(t, ctx, pool, projectID, "orphan_interaction")
}

func TestMasterFixture_CorruptedCheckpointMismatch(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	projectID := seedProject(t, pool, false)
	fix := NewMasterFixture(projectID, "1.2.0", "120", now)
	fix.Events = []FixtureEvent{
		{
			EventID: "22222222-3333-4444-8555-666666666666", InstallID: fix.InstallID,
			SessionID: fix.SessionID, Sequence: 1, Name: "state_checkpoint", Kind: "product",
			EffectiveAt: now, AppVersion: fix.AppVersion, BuildNumber: fix.BuildNumber,
			Properties: fix.baseProps(map[string]any{
				"placed_block_ids": "10", "placed_state_hash": "deadbeefdeadbeef",
			}),
		},
	}
	if err := fix.Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	assertAlertCategory(t, ctx, pool, projectID, "checkpoint_mismatch")
}

func TestMasterFixture_CorruptedCoordinateMismatch(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	projectID := seedProject(t, pool, false)
	fix := NewMasterFixture(projectID, "1.2.0", "120", now)
	fix.Events = []FixtureEvent{
		{
			EventID: "33333333-4444-4555-8666-777777777777", InstallID: fix.InstallID,
			SessionID: fix.SessionID, Sequence: 1, Name: "placement_resolved", Kind: "product",
			EffectiveAt: now, AppVersion: fix.AppVersion, BuildNumber: fix.BuildNumber,
			Properties: fix.baseProps(map[string]any{
				"coordinate_space": "world", "outcome": "placed",
			}),
		},
	}
	if err := fix.Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	assertAlertCategory(t, ctx, pool, projectID, "coordinate_mismatch")
}

func TestMasterFixture_CorruptedStrictCatalogRejection(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	projectID := seedProject(t, pool, false)
	fix := NewMasterFixture(projectID, "1.2.0", "120", now)
	fix.AddStrictCatalogRejectionScenario()
	if err := fix.Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	assertAlertCategory(t, ctx, pool, projectID, "rejection_spike")
}

func TestMasterFixture_CorruptedIndexGap(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	projectID := seedProject(t, pool, false)
	fix := NewMasterFixture(projectID, "1.2.0", "120", now)
	fix.Events = fix.Events[:10]
	// Inject index gap in house_event_index
	fix.Events[8].Properties["house_event_index"] = 40
	if err := fix.Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	assertAlertCategory(t, ctx, pool, projectID, "index_gap")
}

func TestMasterFixture_CorruptedUnpairedDeveloperCommand(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	projectID := seedProject(t, pool, false)
	fix := NewMasterFixture(projectID, "1.2.0", "120", now)
	// Add terminal command without start
	fix.Events = []FixtureEvent{
		{
			EventID: "44444444-5555-4666-8777-888888888888", InstallID: fix.InstallID,
			SessionID: fix.SessionID, Sequence: 1, Name: "developer_command_completed", Kind: "product",
			EffectiveAt: now, AppVersion: fix.AppVersion, BuildNumber: fix.BuildNumber,
			Properties: fix.baseProps(map[string]any{
				"developer_action_id": "orphan-dev-act", "developer_command": "CompleteHouse99",
				"developer_result": "success", "progress_changed": true,
			}),
		},
	}
	if err := fix.Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	assertAlertCategory(t, ctx, pool, projectID, "unpaired_developer_action")
}

func TestMasterFixture_CorruptedUnknownRevision(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	projectID := seedProject(t, pool, false)
	fix := NewMasterFixture(projectID, "1.2.0", "120", now)
	fix.Events = []FixtureEvent{
		{
			EventID: "55555555-6666-4777-8888-999999999999", InstallID: fix.InstallID,
			SessionID: fix.SessionID, Sequence: 1, Name: "house_run_started", Kind: "product",
			EffectiveAt: now, AppVersion: fix.AppVersion, BuildNumber: fix.BuildNumber,
			Properties: fix.baseProps(map[string]any{
				"content_revision": "rev-non-existent", "house_run_id": "run-rev-test",
				"house_event_index": 0,
			}),
		},
	}
	if err := fix.Seed(ctx, pool); err != nil {
		t.Fatalf("seed: %v", err)
	}
	assertAlertCategory(t, ctx, pool, projectID, "unknown_revision")
}

func assertAlertCategory(t *testing.T, ctx context.Context, pool *pgxpool.Pool, projectID, category string) {
	t.Helper()
	now := time.Now().UTC()
	from := now.Add(-2 * time.Hour)
	to := now.Add(2 * time.Hour)

	q, err := GetPuzzleQuality(ctx, pool, projectID, from, to, nil)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}
	for _, a := range q.Alerts {
		if a.Category == category {
			return
		}
	}
	t.Errorf("expected alert category %q, got: %+v (status_reasons: %v)", category, q.Alerts, q.Reasons)
}
