package ingest

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ForkHorizon/Mortris/internal/contracts"
)

type persistedEvents struct {
	accepted                []string
	duplicates              []string
	activated               bool
	firstProductEffectiveAt *time.Time
}

func (s *Service) persistBatch(ctx context.Context, req *contracts.BatchIngestRequest, prepared []preparedEvent, strictCatalog bool, now time.Time) ([]string, []string, string, string, error) {
	if len(prepared) == 0 {
		return nil, nil, "", "", nil
	}
	last := prepared[len(prepared)-1].event
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := updateBatchInstallation(ctx, tx, req, last); err != nil {
		return nil, nil, "", "", err
	}
	persisted, err := persistPreparedEvents(ctx, tx, req, prepared, strictCatalog, now)
	if err != nil {
		return nil, nil, "", "", err
	}
	if persisted.activated {
		if err := activateInstallation(ctx, tx, req, persisted.firstProductEffectiveAt); err != nil {
			return nil, nil, "", "", err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, "", "", err
	}
	return persisted.accepted, persisted.duplicates, last.AppVersion, last.BuildNumber, nil
}

func updateBatchInstallation(ctx context.Context, tx pgx.Tx, req *contracts.BatchIngestRequest, last contracts.Event) error {
	_, err := tx.Exec(ctx, `
		UPDATE installations
		SET last_seen_at = clock_timestamp(),
		    last_app_version = $3, last_build_number = $4, last_sdk_version = $5
		WHERE project_id = $1 AND install_id = $2
	`, req.ProjectID, req.InstallID, last.AppVersion, last.BuildNumber, req.SDK.Version)
	return err
}

func persistPreparedEvents(ctx context.Context, tx pgx.Tx, req *contracts.BatchIngestRequest, prepared []preparedEvent, strictCatalog bool, now time.Time) (persistedEvents, error) {
	result := persistedEvents{}
	for _, pe := range prepared {
		inserted, err := insertPreparedEvent(ctx, tx, req, pe, now)
		if err != nil {
			return persistedEvents{}, err
		}
		if !inserted {
			result.duplicates = append(result.duplicates, pe.event.EventID)
			continue
		}
		result.accepted = append(result.accepted, pe.event.EventID)
		result.activated = true
		if err := trackProductEvent(ctx, tx, req.ProjectID, pe, strictCatalog, now, &result); err != nil {
			return persistedEvents{}, err
		}
	}
	return result, nil
}

func insertPreparedEvent(ctx context.Context, tx pgx.Tx, req *contracts.BatchIngestRequest, pe preparedEvent, now time.Time) (bool, error) {
	properties, err := json.Marshal(pe.event.Properties)
	if err != nil {
		return false, err
	}
	var insertedID string
	err = tx.QueryRow(ctx, `
		INSERT INTO events (
			project_id, event_id, install_id, session_id, sequence, session_elapsed_ms,
			name, event_kind, occurred_at_client, sent_at_client, received_at, effective_at,
			clock_skew_ms, time_quality, app_version, build_number, platform, os_version,
			device_class, locale, timezone_offset_minutes, properties
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22
		)
		ON CONFLICT (project_id, event_id) DO NOTHING
		RETURNING event_id
	`, req.ProjectID, pe.event.EventID, req.InstallID, pe.event.SessionID, pe.event.Sequence, pe.event.SessionElapsedMs,
		pe.event.Name, pe.kind, parseTimestamp(pe.event.OccurredAtClient), parseTimestamp(req.SentAtClient), now, pe.effectiveAt,
		now.Sub(parseTimestamp(req.SentAtClient)).Milliseconds(), pe.quality, pe.event.AppVersion, pe.event.BuildNumber,
		pe.event.Platform, pe.event.OSVersion, pe.event.DeviceClass, pe.event.Locale, pe.event.TimezoneOffsetMinutes, properties,
	).Scan(&insertedID)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func trackProductEvent(ctx context.Context, tx pgx.Tx, projectID string, pe preparedEvent, strictCatalog bool, now time.Time, result *persistedEvents) error {
	if pe.kind != "product" {
		return nil
	}
	if result.firstProductEffectiveAt == nil || pe.effectiveAt.Before(*result.firstProductEffectiveAt) {
		t := pe.effectiveAt
		result.firstProductEffectiveAt = &t
	}
	if strictCatalog {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO event_catalog (project_id, name, kind, first_seen_at, last_seen_at)
		VALUES ($1, $2, 'product', $3, $3)
		ON CONFLICT (project_id, name) DO UPDATE SET last_seen_at = $3
	`, projectID, pe.event.Name, now)
	return err
}

func activateInstallation(ctx context.Context, tx pgx.Tx, req *contracts.BatchIngestRequest, firstProductEffectiveAt *time.Time) error {
	_, err := tx.Exec(ctx, `
		UPDATE installations
		SET activated_at = COALESCE(activated_at, clock_timestamp()),
		    first_product_event_at = COALESCE(first_product_event_at, $3)
		WHERE project_id = $1 AND install_id = $2
	`, req.ProjectID, req.InstallID, firstProductEffectiveAt)
	return err
}

func (s *Service) recordBatchStats(ctx context.Context, req *contracts.BatchIngestRequest, accepted, duplicates, rejected int) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO ingestion_stats (project_id, install_id, accepted_count, duplicate_count, rejected_count)
		VALUES ($1, $2, $3, $4, $5)
	`, req.ProjectID, req.InstallID, accepted, duplicates, rejected)
	return err
}

// recordRejectionStats aggregates this batch's rejections by (name, code)
// and upserts day-bucketed counters (section: Phase 4 catalog governance)
// so the catalog can show which events are failing and how often — the
// events table itself never stores rejections at all.
func (s *Service) recordRejectionStats(ctx context.Context, projectID string, rejected []contracts.RejectedEvent, now time.Time) error {
	if len(rejected) == 0 {
		return nil
	}
	type key struct{ name, code string }
	counts := map[key]int64{}
	for _, r := range rejected {
		counts[key{name: r.Name, code: r.Code}]++
	}
	day := now.UTC().Format("2006-01-02")
	for k, count := range counts {
		if _, err := s.Pool.Exec(ctx, `
			INSERT INTO event_rejection_stats (project_id, name, code, day, count, last_seen_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (project_id, name, code, day)
			DO UPDATE SET count = event_rejection_stats.count + EXCLUDED.count, last_seen_at = EXCLUDED.last_seen_at
		`, projectID, k.name, k.code, day, count, now); err != nil {
			return err
		}
	}
	return nil
}

// recordDriftStats aggregates this batch's undeclared-property
// observations by (event name, property key) and upserts day-bucketed
// counters, mirroring recordRejectionStats.
func (s *Service) recordDriftStats(ctx context.Context, projectID string, drifts []driftObservation, now time.Time) error {
	if len(drifts) == 0 {
		return nil
	}
	type key struct{ name, propertyKey string }
	counts := map[key]int64{}
	for _, d := range drifts {
		for _, k := range d.keys {
			counts[key{name: d.name, propertyKey: k}]++
		}
	}
	day := now.UTC().Format("2006-01-02")
	for k, count := range counts {
		if _, err := s.Pool.Exec(ctx, `
			INSERT INTO event_property_drift (project_id, name, property_key, day, count, last_seen_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (project_id, name, property_key, day)
			DO UPDATE SET count = event_property_drift.count + EXCLUDED.count, last_seen_at = EXCLUDED.last_seen_at
		`, projectID, k.name, k.propertyKey, day, count, now); err != nil {
			return err
		}
	}
	return nil
}
