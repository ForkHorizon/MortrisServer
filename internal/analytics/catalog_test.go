package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func seedCatalogEntry(t *testing.T, pool *pgxpool.Pool, projectID, name, kind string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO event_catalog (project_id, name, kind)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, name) DO NOTHING
	`, projectID, name, kind); err != nil {
		t.Fatalf("seed catalog entry %s: %v", name, err)
	}
}

func TestGetCatalog_CountsAndPercentInRange(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	installID := "66666666-6666-4666-8666-666666666666"
	seedInstallation(t, pool, projectID, installID, &now)

	seedCatalogEntry(t, pool, projectID, "level_start", "product")
	seedCatalogEntry(t, pool, projectID, "level_end", "product")

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "d1111111-1111-4111-8111-111111111111", InstallID: installID, SessionID: "e1111111-1111-4111-8111-111111111111", Sequence: 1, Name: "level_start", Kind: "product", EffectiveAt: now},
		{EventID: "d2222222-2222-4222-8222-222222222222", InstallID: installID, SessionID: "e1111111-1111-4111-8111-111111111111", Sequence: 2, Name: "level_start", Kind: "product", EffectiveAt: now.Add(time.Minute)},
		{EventID: "d3333333-3333-4333-8333-333333333333", InstallID: installID, SessionID: "e1111111-1111-4111-8111-111111111111", Sequence: 3, Name: "level_start", Kind: "product", EffectiveAt: now.Add(2 * time.Minute)},
		{EventID: "d4444444-4444-4444-8444-444444444444", InstallID: installID, SessionID: "e1111111-1111-4111-8111-111111111111", Sequence: 4, Name: "level_end", Kind: "product", EffectiveAt: now.Add(3 * time.Minute)},
	})

	result, err := GetCatalog(context.Background(), pool, projectID, now.Add(-time.Hour), now.Add(time.Hour), time.UTC)
	if err != nil {
		t.Fatal(err)
	}

	byName := map[string]CatalogEntry{}
	for _, e := range result.Entries {
		byName[e.Name] = e
	}

	assertCatalogVolume(t, byName, "level_start", 3, 75)
	assertCatalogVolume(t, byName, "level_end", 1, 25)
	if len(byName["level_start"].Sparkline) == 0 {
		t.Error("expected a non-empty sparkline for an event with occurrences in range")
	}
}

func TestGetCatalog_RejectionsAndDrift(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	installID := "77777777-7777-4777-8777-777777777777"
	seedInstallation(t, pool, projectID, installID, &now)

	seedCatalogEntry(t, pool, projectID, "level_end", "product")
	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "f1111111-1111-4111-8111-111111111111", InstallID: installID, SessionID: "e1111111-1111-4111-8111-111111111111", Sequence: 1, Name: "level_end", Kind: "product", EffectiveAt: now},
	})

	day := now.Format("2006-01-02")
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO event_rejection_stats (project_id, name, code, day, count, last_seen_at)
		VALUES ($1, 'level_end', 'invalid_property_key', $2, 3, $3)
		ON CONFLICT (project_id, name, code, day) DO UPDATE SET count = EXCLUDED.count, last_seen_at = EXCLUDED.last_seen_at
	`, projectID, day, now); err != nil {
		t.Fatalf("seed rejection stats: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO event_property_drift (project_id, name, property_key, day, count, last_seen_at)
		VALUES ($1, 'level_end', 'scoree', $2, 5, $3)
		ON CONFLICT (project_id, name, property_key, day) DO UPDATE SET count = EXCLUDED.count, last_seen_at = EXCLUDED.last_seen_at
	`, projectID, day, now); err != nil {
		t.Fatalf("seed drift stats: %v", err)
	}

	result, err := GetCatalog(context.Background(), pool, projectID, now.Add(-time.Hour), now.Add(time.Hour), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]CatalogEntry{}
	for _, e := range result.Entries {
		byName[e.Name] = e
	}

	entry := byName["level_end"]
	if entry.RejectedCount != 3 {
		t.Errorf("expected rejected_count 3, got %d", entry.RejectedCount)
	}
	// 1 accepted + 3 rejected = 4 attempted, 3/4 = 75%.
	if entry.RejectionRate != 75 {
		t.Errorf("expected rejection_rate 75, got %v", entry.RejectionRate)
	}
	if len(entry.Drift) != 1 || entry.Drift[0].PropertyKey != "scoree" || entry.Drift[0].Count != 5 {
		t.Errorf("expected drift [scoree:5], got %+v", entry.Drift)
	}
}

func assertCatalogVolume(t *testing.T, byName map[string]CatalogEntry, name string, wantCount int64, wantPercent float64) {
	t.Helper()
	e, ok := byName[name]
	if !ok {
		t.Fatalf("expected %s in catalog result", name)
	}
	if e.EventCount != wantCount {
		t.Errorf("expected %s count %d, got %d", name, wantCount, e.EventCount)
	}
	if e.PercentOfTotal != wantPercent {
		t.Errorf("expected %s at %v%% of total, got %v", name, wantPercent, e.PercentOfTotal)
	}
}
