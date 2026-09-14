package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestMasterFixture_ScenarioCoverage verifies that the master fixture builder
// accurately represents all 15 key lifecycle scenarios required by Stage 7.
func TestMasterFixture_ScenarioCoverage(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	fix := NewMasterFixture("test-coverage", "1.2.0", "120", now)
	fix.AddNetworkRetryScenario()
	fix.AddStrictCatalogRejectionScenario()

	scenarioCounts := make(map[string]int)
	for _, e := range fix.Events {
		scenarioCounts[e.Name]++
	}

	assertGte(t, scenarioCounts["device_profile"], 1, "S1: device_profile")
	assertGte(t, scenarioCounts["house_run_started"], 1, "S2: house_run_started")
	assertGte(t, scenarioCounts["wave_attempt_started"], 1, "S2: wave_attempt_started")
	assertGte(t, scenarioCounts["detail_taken"], 3, "S3-5: detail_taken count")
	assertGte(t, scenarioCounts["detail_released"], 3, "S3-5: detail_released count")
	assertGte(t, scenarioCounts["placement_resolved"], 3, "S3-5: placement_resolved count")
	assertGte(t, scenarioCounts["detail_returned"], 2, "S4-5: detail_returned count")
	assertGte(t, scenarioCounts["state_checkpoint"], 1, "S6: state_checkpoint")
	assertGte(t, scenarioCounts["app_backgrounded"], 1, "S7: app_backgrounded")
	assertGte(t, scenarioCounts["app_foregrounded"], 1, "S7: app_foregrounded")
	assertGte(t, scenarioCounts["attempt_recovered"], 1, "S8: attempt_recovered")
	assertGte(t, scenarioCounts["house_completed"], 2, "S9 & S10: house_completed (natural + dev)")
	assertGte(t, scenarioCounts["developer_command_started"], 4, "S10-12: developer_command_started")
	assertGte(t, scenarioCounts["developer_progress_mutated"], 2, "S10-11: developer_progress_mutated")
	assertGte(t, scenarioCounts["developer_command_completed"], 2, "S10-11: developer_command_completed")
	assertGte(t, scenarioCounts["developer_command_failed"], 1, "S12: developer_command_failed")
	assertGte(t, scenarioCounts["developer_command_noop"], 1, "S12: developer_command_noop")
	if len(fix.Stats) == 0 || fix.Stats[0].Duplicates == 0 {
		t.Errorf("S13: expected duplicate event ingestion stats")
	}
	assertGte(t, scenarioCounts["attempt_closed"], 1, "S14: late delivery attempt_closed")
	if len(fix.Rejections) == 0 {
		t.Errorf("S15: expected rejection occurrence for strict catalog")
	}
}

// TestMasterFixture_DatabaseIntegration verifies quality indicators, traffic
// segmentation, and aggregates against a real PostgreSQL instance.
func TestMasterFixture_DatabaseIntegration(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	projectID := seedProject(t, pool, true)

	fix := NewMasterFixture(projectID, "1.2.0", "120", now)
	if err := fix.Seed(ctx, pool); err != nil {
		t.Fatalf("Seed master fixture: %v", err)
	}

	from := now.Add(-time.Hour)
	to := now.Add(2 * time.Hour)
	build := "120"

	q, err := GetPuzzleQuality(ctx, pool, projectID, from, to, &build)
	if err != nil {
		t.Fatalf("GetPuzzleQuality: %v", err)
	}

	verifyQualityMetrics(t, q)
	verifyTrafficSegmentation(t, pool, projectID, from, to, &build)
	verifyAggregates(t, pool, projectID, from, to, &build)
}

func verifyQualityMetrics(t *testing.T, q *PuzzleQuality) {
	t.Helper()
	if q.Status != "green" {
		t.Errorf("expected clean fixture status green, got %s (reasons: %v)", q.Status, q.Reasons)
	}
	if q.SequenceGaps != 0 || q.SequenceDuplicates != 0 {
		t.Errorf("SequenceGaps=%d, SequenceDuplicates=%d, want 0", q.SequenceGaps, q.SequenceDuplicates)
	}
	if q.HouseEventIndexGaps != 0 || q.AttemptEventIndexGaps != 0 {
		t.Errorf("Index gaps: house=%d, attempt=%d, want 0", q.HouseEventIndexGaps, q.AttemptEventIndexGaps)
	}
	if q.OrphanInteractions != 0 {
		t.Errorf("OrphanInteractions=%d, want 0", q.OrphanInteractions)
	}
	if q.CheckpointMismatches != 0 {
		t.Errorf("CheckpointMismatches=%d, want 0", q.CheckpointMismatches)
	}
	if q.InvalidCoordinateSpaceEvents != 0 {
		t.Errorf("InvalidCoordinateSpaceEvents=%d, want 0", q.InvalidCoordinateSpaceEvents)
	}
	if q.UnknownRevisionEvents != 0 {
		t.Errorf("UnknownRevisionEvents=%d, want 0", q.UnknownRevisionEvents)
	}
	if q.UnpairedDeveloperCommands != 0 || q.UnpairedDeveloperMutations != 0 {
		t.Errorf("Unpaired dev commands=%d, mutations=%d, want 0", q.UnpairedDeveloperCommands, q.UnpairedDeveloperMutations)
	}
	if len(q.Alerts) != 0 {
		t.Errorf("expected 0 alerts on clean master fixture, got: %+v", q.Alerts)
	}
}

func verifyTrafficSegmentation(t *testing.T, pool *pgxpool.Pool, projectID string, from, to time.Time, build *string) {
	t.Helper()
	ctx := context.Background()

	// Stage 3 natural metrics: natural house run completed must equal 1 (ignoring cheated run)
	houses, err := GetPuzzleHouses(ctx, pool, projectID, from, to, ScopeNaturalOnly, build)
	if err != nil {
		t.Fatalf("GetPuzzleHouses (natural): %v", err)
	}
	if len(houses.Houses) == 0 {
		t.Fatalf("expected house coverage row")
	}
	if houses.Houses[0].NaturalHouseRunsComplete != 1 {
		t.Errorf("Natural completed house runs = %d, want 1 (cheated completion must be ignored)", houses.Houses[0].NaturalHouseRunsComplete)
	}

	// Tester impact: developer actions isolated
	impact, err := GetPuzzleTesterImpact(ctx, pool, projectID, from, to)
	if err != nil {
		t.Fatalf("GetPuzzleTesterImpact: %v", err)
	}
	if impact.DeveloperActions < 4 {
		t.Errorf("DeveloperActions = %d, want >= 4", impact.DeveloperActions)
	}
	if impact.AffectedHouseRuns < 1 {
		t.Errorf("AffectedHouseRuns = %d, want >= 1", impact.AffectedHouseRuns)
	}
	if impact.ProgressCompletions != 1 {
		t.Errorf("ProgressCompletions = %d, want 1", impact.ProgressCompletions)
	}
	if impact.Resets != 1 {
		t.Errorf("Resets = %d, want 1", impact.Resets)
	}
	if impact.MutationsWithBeforeAfter != 2 || impact.MutationsMissingBeforeAfter != 0 {
		t.Errorf("Mutations before/after = %d/%d, want 2/0", impact.MutationsWithBeforeAfter, impact.MutationsMissingBeforeAfter)
	}
}

func verifyAggregates(t *testing.T, pool *pgxpool.Pool, projectID string, from, to time.Time, build *string) {
	t.Helper()
	ctx := context.Background()

	funnel, err := GetPuzzleWaveFunnel(ctx, pool, projectID, 1, 1, from, to, ScopeNaturalOnly, build)
	if err != nil {
		t.Fatalf("GetPuzzleWaveFunnel: %v", err)
	}
	if funnel.TotalWaves == 0 {
		t.Errorf("expected non-zero total waves in funnel")
	}

	drops, err := GetPuzzleDrops(ctx, pool, projectID, 1, 1, nil, from, to, ScopeNaturalOnly, build)
	if err != nil {
		t.Fatalf("GetPuzzleDrops: %v", err)
	}
	if len(drops.Drops) != 3 {
		t.Errorf("Drops count = %d, want 3 (placed + no-snap + missing-support)", len(drops.Drops))
	}
}

func assertGte(t *testing.T, got, want int, name string) {
	t.Helper()
	if got < want {
		t.Errorf("%s = %d, want >= %d", name, got, want)
	}
}
