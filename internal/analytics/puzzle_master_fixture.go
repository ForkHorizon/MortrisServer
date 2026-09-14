package analytics

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FixtureEvent is a raw event representation for master fixtures.
type FixtureEvent struct {
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
	SentAtClient     time.Time
	ReceivedAt       time.Time
}

// MasterFixture holds a complete, reusable schema-v2 server test dataset
// representing all 15 key lifecycle scenarios (docs/puzzle-analytics-remaining-plan.md
// section 10).
type MasterFixture struct {
	ProjectID   string
	AppVersion  string
	BuildNumber string
	Revision    string
	BaseTime    time.Time
	InstallID   string
	SessionID   string
	HouseRunID  string
	AttemptID   string
	Events      []FixtureEvent
	Rejections  []FixtureRejection
	Stats       []FixtureIngestionStat
	CityID      int
	HouseID     int
}

type FixtureRejection struct {
	InstallID  string
	Name       string
	Code       string
	ReceivedAt time.Time
}

type FixtureIngestionStat struct {
	InstallID  string
	Accepted   int64
	Duplicates int64
	Rejected   int64
	ReceivedAt time.Time
}

// CheckpointHash computes the sha256 hash for placed block IDs.
func CheckpointHash(placedIDs string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(placedIDs)))
}

// CleanProjectData removes all project-scoped rows across tables.
func CleanProjectData(ctx context.Context, pool *pgxpool.Pool, projectID string) {
	_, _ = pool.Exec(ctx, `DELETE FROM events WHERE project_id=$1`, projectID)
	_, _ = pool.Exec(ctx, `DELETE FROM installations WHERE project_id=$1`, projectID)
	_, _ = pool.Exec(ctx, `DELETE FROM ingestion_stats WHERE project_id=$1`, projectID)
	_, _ = pool.Exec(ctx, `DELETE FROM event_rejection_occurrences WHERE project_id=$1`, projectID)
	_, _ = pool.Exec(ctx, `DELETE FROM event_rejection_stats WHERE project_id=$1`, projectID)
	_, _ = pool.Exec(ctx, `DELETE FROM event_property_drift WHERE project_id=$1`, projectID)
	_, _ = pool.Exec(ctx, `DELETE FROM puzzle_content_revisions WHERE project_id=$1`, projectID)
	_, _ = pool.Exec(ctx, `DELETE FROM puzzle_content_blocks WHERE project_id=$1`, projectID)
	_, _ = pool.Exec(ctx, `DELETE FROM puzzle_content_targets WHERE project_id=$1`, projectID)
	_, _ = pool.Exec(ctx, `DELETE FROM puzzle_block_geometries WHERE project_id=$1`, projectID)
	_, _ = pool.Exec(ctx, `DELETE FROM puzzle_house_art WHERE project_id=$1`, projectID)
	if projectID != "puzzle_gravity_test" {
		_, _ = pool.Exec(ctx, `DELETE FROM event_catalog WHERE project_id=$1`, projectID)
	}
	_, _ = pool.Exec(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
}

// NewMasterFixture initializes a master fixture with all 15 lifecycle scenarios.
func NewMasterFixture(projectID, appVersion, buildNumber string, baseTime time.Time) *MasterFixture {
	f := &MasterFixture{
		ProjectID:   projectID,
		AppVersion:  appVersion,
		BuildNumber: buildNumber,
		Revision:    "rev-schema-v2-master",
		BaseTime:    baseTime.UTC().Truncate(time.Second),
		InstallID:   "10101010-1010-4010-8010-101010101010",
		SessionID:   "20202020-2020-4020-8020-202020202020",
		HouseRunID:  "run-natural-01",
		AttemptID:   "attempt-natural-01",
		Events:      make([]FixtureEvent, 0, 50),
		CityID:      1,
		HouseID:     1,
	}

	f.buildNaturalScenarios()
	f.buildDeveloperScenarios()
	f.buildRetryAndCatalogScenarios()

	return f
}

// Seed persists the master fixture into the PostgreSQL database.
func (f *MasterFixture) Seed(ctx context.Context, pool *pgxpool.Pool) error {
	CleanProjectData(ctx, pool, f.ProjectID)

	if err := f.seedProjectAndRevision(ctx, pool); err != nil {
		return err
	}
	if err := f.seedInstallations(ctx, pool); err != nil {
		return err
	}
	if err := f.seedStatsAndRejections(ctx, pool); err != nil {
		return err
	}

	return seedFixtureEvents(ctx, pool, f.ProjectID, f.Events)
}

func (f *MasterFixture) seedProjectAndRevision(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		INSERT INTO projects (id, environment, display_name, strict_catalog, enabled)
		VALUES ($1, 'test', $1, true, true)
		ON CONFLICT (id) DO NOTHING
	`, f.ProjectID); err != nil {
		return fmt.Errorf("seed project: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO puzzle_content_revisions (project_id, content_revision, schema_version, catalog)
		VALUES ($1, $2, 2, '{"cities":[{"id":1,"name":"TestCity","houses":[{"id":1,"name":"House1","waves":[{"wave_index":0}]}]}]}'::jsonb)
		ON CONFLICT (project_id, content_revision) DO NOTHING
	`, f.ProjectID, f.Revision); err != nil {
		return fmt.Errorf("seed revision: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO puzzle_content_blocks (project_id, content_revision, city_id, house_id, wave_index, block_id, order_in_layer, local_x_milli, local_y_milli)
		VALUES ($1, $2, $3, $4, 0, 10, 0, 1000, 2000), ($1, $2, $3, $4, 0, 11, 1, 1500, 2500), ($1, $2, $3, $4, 0, 12, 2, 1200, 2200)
		ON CONFLICT (project_id, content_revision, city_id, house_id, block_id) DO NOTHING
	`, f.ProjectID, f.Revision, f.CityID, f.HouseID); err != nil {
		return fmt.Errorf("seed blocks: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO puzzle_content_targets (project_id, content_revision, city_id, house_id, wave_index, target_id, compatible_block_ids, local_x_milli, local_y_milli)
		VALUES ($1, $2, $3, $4, 0, 10, '[10]'::jsonb, 1000, 2000),
		       ($1, $2, $3, $4, 0, 11, '[11]'::jsonb, 1500, 2500),
		       ($1, $2, $3, $4, 0, 12, '[12]'::jsonb, 1200, 2200)
		ON CONFLICT (project_id, content_revision, city_id, house_id, target_id) DO NOTHING
	`, f.ProjectID, f.Revision, f.CityID, f.HouseID); err != nil {
		return fmt.Errorf("seed targets: %w", err)
	}
	return nil
}

func (f *MasterFixture) seedInstallations(ctx context.Context, pool *pgxpool.Pool) error {
	installs := map[string]bool{f.InstallID: true}
	for _, e := range f.Events {
		installs[e.InstallID] = true
	}
	for inst := range installs {
		if _, err := pool.Exec(ctx, `
			INSERT INTO installations (project_id, install_id, credential_hash, first_product_event_at, activated_at, registered_at, last_app_version, last_build_number)
			VALUES ($1, $2, '\x00', $3, $3, $3, $4, $5)
			ON CONFLICT (project_id, install_id) DO NOTHING
		`, f.ProjectID, inst, f.BaseTime, f.AppVersion, f.BuildNumber); err != nil {
			return fmt.Errorf("seed install %s: %w", inst, err)
		}
	}
	return nil
}

func (f *MasterFixture) seedStatsAndRejections(ctx context.Context, pool *pgxpool.Pool) error {
	for _, s := range f.Stats {
		if _, err := pool.Exec(ctx, `
			INSERT INTO ingestion_stats (project_id, install_id, build_number, accepted_count, duplicate_count, rejected_count, received_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, f.ProjectID, s.InstallID, f.BuildNumber, s.Accepted, s.Duplicates, s.Rejected, s.ReceivedAt); err != nil {
			return fmt.Errorf("seed ingestion_stats: %w", err)
		}
	}
	for _, r := range f.Rejections {
		if _, err := pool.Exec(ctx, `
			INSERT INTO event_rejection_occurrences (project_id, install_id, build_number, name, code, received_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, f.ProjectID, r.InstallID, f.BuildNumber, r.Name, r.Code, r.ReceivedAt); err != nil {
			return fmt.Errorf("seed rejection: %w", err)
		}
	}
	return nil
}

func seedFixtureEvents(ctx context.Context, pool *pgxpool.Pool, projectID string, events []FixtureEvent) error {
	for _, e := range events {
		props, _ := json.Marshal(e.Properties)
		appVer := e.AppVersion
		if appVer == "" {
			appVer = "1.0.0"
		}
		platform := e.Platform
		if platform == "" {
			platform = "android"
		}
		sentAt := e.SentAtClient
		if sentAt.IsZero() {
			sentAt = e.EffectiveAt
		}
		recvAt := e.ReceivedAt
		if recvAt.IsZero() {
			recvAt = e.EffectiveAt
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO events (
				project_id, event_id, install_id, session_id, sequence, session_elapsed_ms,
				name, event_kind, occurred_at_client, sent_at_client, received_at, effective_at,
				clock_skew_ms, time_quality, app_version, build_number, platform, os_version,
				device_class, locale, timezone_offset_minutes, properties
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$14,$15,$9,0,'client',$10,$11,$12,'','','',0,$13
			)
			ON CONFLICT (project_id, event_id) DO NOTHING
		`, projectID, e.EventID, e.InstallID, e.SessionID, e.Sequence, e.SessionElapsedMs,
			e.Name, e.Kind, e.EffectiveAt, appVer, e.BuildNumber, platform, props,
			sentAt, recvAt); err != nil {
			return fmt.Errorf("seed fixture event %s: %w", e.EventID, err)
		}
	}
	return nil
}
