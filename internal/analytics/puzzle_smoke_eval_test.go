package analytics

import (
	"context"
	"testing"
	"time"
)

func TestEvaluateSmokeCheck_PassesOnCleanFixture(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	projectID := seedProject(t, pool, true)
	build := "build-smoke-pass"

	fix := NewMasterFixture(projectID, "2.0.0", build, now)
	fix.Rejections = nil
	fix.Stats = []FixtureIngestionStat{
		{InstallID: fix.InstallID, Accepted: int64(len(fix.Events)), Duplicates: 0, Rejected: 0, ReceivedAt: now},
	}
	if err := fix.Seed(ctx, pool); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	report, err := EvaluateSmokeCheck(ctx, pool, SmokeEvalParams{
		ProjectID:   projectID,
		BuildNumber: build,
		From:        now.Add(-time.Hour),
		To:          now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("EvaluateSmokeCheck: %v", err)
	}

	if !report.Passed {
		t.Fatalf("report failed: %s, items: %+v", report.Summary, report.Items)
	}
	if len(report.Items) != 13 {
		t.Fatalf("items count = %d, want 13", len(report.Items))
	}
	for _, item := range report.Items {
		if !item.Passed {
			t.Errorf("Item %d (%s) failed: %s", item.Index, item.Name, item.Detail)
		}
	}
}

func TestEvaluateSmokeCheck_FailsWhenCriteriaMissing(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	projectID := seedProject(t, pool, false)
	build := "build-smoke-fail"

	// Only seed a device profile event, missing all other criteria
	fix := NewMasterFixture(projectID, "2.0.0", build, now)
	fix.Events = fix.Events[:1]
	fix.Rejections = nil
	fix.Stats = nil
	if err := fix.Seed(ctx, pool); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	report, err := EvaluateSmokeCheck(ctx, pool, SmokeEvalParams{
		ProjectID:   projectID,
		BuildNumber: build,
		From:        now.Add(-time.Hour),
		To:          now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("EvaluateSmokeCheck: %v", err)
	}

	if report.Passed {
		t.Errorf("expected report to fail on incomplete telemetry")
	}
}
