package analytics

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/store"
)

// Integration tests against a real PostgreSQL instance — the fixture
// datasets required by the Phase S2 exit gate. Skipped unless
// MORTRIS_TEST_DSN is set, since no Postgres is available in a plain `go
// test` sandbox; run locally (or in CI with a Postgres service container)
// with:
//
//	MORTRIS_TEST_DSN=postgres://... go test ./internal/analytics/...
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("MORTRIS_TEST_DSN")
	if dsn == "" {
		t.Skip("MORTRIS_TEST_DSN not set, skipping analytics integration tests")
	}
	ctx := context.Background()
	pool, err := store.NewPool(ctx, dsn, 5)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := store.ApplyMigrations(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedProject creates an isolated project (unique per test via t.Name())
// and an installation, returning the install_id. Every test uses its own
// project so subtests never see each other's rows.
func seedProject(t *testing.T, pool *pgxpool.Pool, strictCatalog bool) string {
	t.Helper()
	ctx := context.Background()
	projectID := "test-" + t.Name()
	if _, err := pool.Exec(ctx, `
		INSERT INTO projects (id, environment, display_name, strict_catalog, enabled)
		VALUES ($1, 'test', $1, $2, true)
		ON CONFLICT (id) DO NOTHING
	`, projectID, strictCatalog); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return projectID
}

func seedInstallation(t *testing.T, pool *pgxpool.Pool, projectID, installID string, firstProductEventAt *time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO installations (project_id, install_id, credential_hash, first_product_event_at, activated_at)
		VALUES ($1, $2, '\x00', $3, $3)
		ON CONFLICT (project_id, install_id) DO NOTHING
	`, projectID, installID, firstProductEventAt); err != nil {
		t.Fatalf("seed installation: %v", err)
	}
}

type seedEvent struct {
	EventID          string
	InstallID        string
	SessionID        string
	Sequence         int64
	SessionElapsedMs int64
	Name             string
	Kind             string
	EffectiveAt      time.Time
	AppVersion       string
	BuildNumber      string
	Platform         string
	Properties       map[string]any
}

func seedEvents(t *testing.T, pool *pgxpool.Pool, projectID string, events []seedEvent) {
	t.Helper()
	ctx := context.Background()
	for _, e := range events {
		props, _ := json.Marshal(e.Properties)
		if e.AppVersion == "" {
			e.AppVersion = "1.0.0"
		}
		if e.Platform == "" {
			e.Platform = "android"
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO events (
				project_id, event_id, install_id, session_id, sequence, session_elapsed_ms,
				name, event_kind, occurred_at_client, sent_at_client, received_at, effective_at,
				clock_skew_ms, time_quality, app_version, build_number, platform, os_version,
				device_class, locale, timezone_offset_minutes, properties
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$9,$9,$9,0,'client',$10,$11,$12,'','','',0,$13
			)
			ON CONFLICT (project_id, event_id) DO NOTHING
		`, projectID, e.EventID, e.InstallID, e.SessionID, e.Sequence, e.SessionElapsedMs,
			e.Name, e.Kind, e.EffectiveAt, e.AppVersion, e.BuildNumber, e.Platform, props); err != nil {
			t.Fatalf("seed event %s: %v", e.EventID, err)
		}
	}
}

func TestNewInstallations_ExcludesRegistrationOnly(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)

	activated := now.Add(-1 * time.Hour)
	seedInstallation(t, pool, projectID, "11111111-1111-4111-8111-111111111111", &activated)
	seedInstallation(t, pool, projectID, "22222222-2222-4222-8222-222222222222", nil) // registered only

	o, err := GetOverview(context.Background(), pool, projectID, now.Add(-24*time.Hour), now.Add(time.Hour), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if o.NewInstallations != 1 {
		t.Errorf("expected 1 new installation (registration-only must not count), got %d", o.NewInstallations)
	}
}

func TestActiveInstallations_RequireProductEvent(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)

	productInstall := "33333333-3333-4333-8333-333333333333"
	systemOnlyInstall := "44444444-4444-4444-8444-444444444444"
	seedInstallation(t, pool, projectID, productInstall, &now)
	seedInstallation(t, pool, projectID, systemOnlyInstall, nil)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "a1111111-1111-4111-8111-111111111111", InstallID: productInstall, SessionID: "e1111111-1111-4111-8111-111111111111", Sequence: 1, Name: "level_start", Kind: "product", EffectiveAt: now},
		{EventID: "a2222222-2222-4222-8222-222222222222", InstallID: systemOnlyInstall, SessionID: "e2222222-2222-4222-8222-222222222222", Sequence: 1, Name: "sys_session_start", Kind: "system", EffectiveAt: now},
	})

	o, err := GetOverview(context.Background(), pool, projectID, now.Add(-24*time.Hour), now.Add(time.Hour), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if o.DailyActiveInstallations != 1 {
		t.Errorf("expected 1 active installation (system-only must not count), got %d", o.DailyActiveInstallations)
	}
	if o.Sessions != 1 {
		t.Errorf("expected 1 session (system-only session must not count), got %d", o.Sessions)
	}
}

func TestAvgObservedSessionDuration_UsesMaxAcrossAllEvents(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "55555555-5555-4555-8555-555555555555"
	session := "e5555555-5555-4555-8555-555555555555"
	seedInstallation(t, pool, projectID, install, &now)

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "b1111111-1111-4111-8111-111111111111", InstallID: install, SessionID: session, Sequence: 1, SessionElapsedMs: 100, Name: "level_start", Kind: "product", EffectiveAt: now},
		{EventID: "b2222222-2222-4222-8222-222222222222", InstallID: install, SessionID: session, Sequence: 2, SessionElapsedMs: 500, Name: "level_end", Kind: "product", EffectiveAt: now},
		// sys_app_background arrives later with a higher elapsed time — the
		// definition says this must still be used as the session's max.
		{EventID: "b3333333-3333-4333-8333-333333333333", InstallID: install, SessionID: session, Sequence: 3, SessionElapsedMs: 800, Name: "sys_app_background", Kind: "system", EffectiveAt: now},
	})

	o, err := GetOverview(context.Background(), pool, projectID, now.Add(-24*time.Hour), now.Add(time.Hour), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if o.AvgObservedSessionDurationMs != 800 {
		t.Errorf("expected avg observed session duration 800 (max incl. sys_app_background), got %v", o.AvgObservedSessionDurationMs)
	}
}

func TestReinstall_CountsAsTwoInstallations(t *testing.T) {
	pool := testPool(t)
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)

	// Same "person", two installation rows — the metric contract (section
	// 9, non-goal) says this must count as two, not be merged.
	seedInstallation(t, pool, projectID, "66666666-6666-4666-8666-666666666666", &now)
	seedInstallation(t, pool, projectID, "77777777-7777-4777-8777-777777777777", &now)

	o, err := GetOverview(context.Background(), pool, projectID, now.Add(-24*time.Hour), now.Add(time.Hour), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if o.NewInstallations != 2 {
		t.Errorf("expected reinstall to count as 2 new installations, got %d", o.NewInstallations)
	}
}

func TestEventExplorer_FiltersByAppVersionAndCatalogedProperty(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)
	now := time.Now().UTC().Truncate(time.Second)
	install := "88888888-8888-4888-8888-888888888888"
	seedInstallation(t, pool, projectID, install, &now)

	if _, err := pool.Exec(ctx, `
		INSERT INTO event_catalog (project_id, name, kind, properties)
		VALUES ($1, 'level_start', 'product', '[{"name":"house_id","type":"string"}]')
		ON CONFLICT (project_id, name) DO NOTHING
	`, projectID); err != nil {
		t.Fatal(err)
	}

	seedEvents(t, pool, projectID, []seedEvent{
		{EventID: "c1111111-1111-4111-8111-111111111111", InstallID: install, SessionID: "e8888888-8888-4888-8888-888888888888", Sequence: 1, Name: "level_start", Kind: "product", EffectiveAt: now, AppVersion: "1.0.0", Properties: map[string]any{"house_id": "rome"}},
		{EventID: "c2222222-2222-4222-8222-222222222222", InstallID: install, SessionID: "e8888888-8888-4888-8888-888888888888", Sequence: 2, Name: "level_start", Kind: "product", EffectiveAt: now, AppVersion: "2.0.0", Properties: map[string]any{"house_id": "milan"}},
	})

	q := make(map[string][]string)
	q["name"] = []string{"level_start"}
	q["app_version"] = []string{"1.0.0"}
	filter, err := ParseEventExplorerFilter(ctx, pool, projectID, q)
	if err != nil {
		t.Fatalf("ParseEventExplorerFilter: %v", err)
	}
	result, err := GetEventExplorer(ctx, pool, projectID, now.Add(-time.Hour), now.Add(time.Hour), time.UTC, filter)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalEvents != 1 {
		t.Errorf("expected 1 event for app_version=1.0.0, got %d", result.TotalEvents)
	}

	q2 := make(map[string][]string)
	q2["name"] = []string{"level_start"}
	q2["property_key"] = []string{"house_id"}
	q2["property_value"] = []string{"milan"}
	filter2, err := ParseEventExplorerFilter(ctx, pool, projectID, q2)
	if err != nil {
		t.Fatalf("ParseEventExplorerFilter: %v", err)
	}
	result2, err := GetEventExplorer(ctx, pool, projectID, now.Add(-time.Hour), now.Add(time.Hour), time.UTC, filter2)
	if err != nil {
		t.Fatal(err)
	}
	if result2.TotalEvents != 1 {
		t.Errorf("expected 1 event for house_id=milan, got %d", result2.TotalEvents)
	}
}

func TestEventExplorer_RejectsUncatalogedPropertyFilter(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	projectID := seedProject(t, pool, false)

	if _, err := pool.Exec(ctx, `
		INSERT INTO event_catalog (project_id, name, kind, properties)
		VALUES ($1, 'level_start', 'product', '[]')
		ON CONFLICT (project_id, name) DO NOTHING
	`, projectID); err != nil {
		t.Fatal(err)
	}

	q := make(map[string][]string)
	q["name"] = []string{"level_start"}
	q["property_key"] = []string{"not_declared"}
	q["property_value"] = []string{"x"}
	_, err := ParseEventExplorerFilter(ctx, pool, projectID, q)
	if err == nil {
		t.Error("expected an error for a property_key not in the event catalog")
	}
}
