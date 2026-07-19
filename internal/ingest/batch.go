package ingest

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
	"github.com/ForkHorizon/Mortris/internal/diskstate"
)

const timestampLayout = "2006-01-02T15:04:05.000Z"

// parseTimestamp parses a value already confirmed to match
// internal/contracts' timestamp pattern — callers only use it after
// Validate()/ValidateEvent() succeeded, so the error case is unreachable.
func parseTimestamp(s string) time.Time {
	t, _ := time.Parse(timestampLayout, s)
	return t
}

func plausible(t, receivedAt time.Time) bool {
	lower := receivedAt.Add(-8 * 24 * time.Hour)
	upper := receivedAt.Add(5 * time.Minute)
	return !t.Before(lower) && !t.After(upper)
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// effectiveTime implements the clock-quality rule from section 8.4.
func effectiveTime(occurredAtClient, receivedAt time.Time, skew time.Duration) (time.Time, string) {
	if absDuration(skew) <= 5*time.Minute && plausible(occurredAtClient, receivedAt) {
		return occurredAtClient, "client"
	}
	adjusted := occurredAtClient.Add(skew)
	if plausible(adjusted, receivedAt) {
		return adjusted, "batch_adjusted"
	}
	return receivedAt, "untrusted"
}

type preparedEvent struct {
	event       contracts.Event
	kind        string
	effectiveAt time.Time
	quality     string
}

// Batch implements section 5.4/8.4/7: envelope + per-event validation,
// bearer authentication, event_kind + clock-quality assignment, one
// transaction for all valid events (accept/duplicate via ON CONFLICT DO
// NOTHING), and activation bookkeeping — response only built after commit.
func (s *Service) Batch(ctx context.Context, req *contracts.BatchIngestRequest, decodeRejections []contracts.RejectedEvent, bearerToken, sourceIP string) (*contracts.BatchIngestResponse, error) {
	if s.diskState() == diskstate.Rejecting {
		return nil, apierr.WithRetryAfter(503, contracts.CodeServerStoragePressure, "server is rejecting ingestion under disk pressure", 5*time.Minute)
	}
	if !s.ingestIPLimiter.Allow(sourceIP) {
		return nil, apierr.WithRetryAfter(429, contracts.CodeRateLimited, "ingestion rate limit exceeded for source IP", time.Second)
	}
	if err := req.Validate(); err != nil {
		ve := err.(*contracts.ValidationError)
		return nil, apierr.New(400, ve.Code, ve.Message)
	}
	if !s.ingestProjectLimiter.Allow(req.ProjectID) {
		return nil, apierr.WithRetryAfter(429, contracts.CodeRateLimited, "ingestion rate limit exceeded for project", time.Second)
	}
	if !s.ingestInstallLimiter.Allow(req.ProjectID + "/" + req.InstallID) {
		return nil, apierr.WithRetryAfter(429, contracts.CodeRateLimited, "ingestion rate limit exceeded for installation", time.Minute)
	}

	if bearerToken == "" {
		return nil, apierr.New(401, contracts.CodeUnauthorized, "missing bearer credential")
	}
	credentialBytes, err := base64.RawURLEncoding.DecodeString(bearerToken)
	if err != nil || len(credentialBytes) != 32 {
		return nil, apierr.New(401, contracts.CodeUnauthorized, "malformed bearer credential")
	}
	providedHash := sha256.Sum256(credentialBytes)

	var storedHash []byte
	var environment string
	var strictCatalog, enabled bool
	err = s.Pool.QueryRow(ctx, `
		SELECT i.credential_hash, p.environment, p.strict_catalog, p.enabled
		FROM installations i JOIN projects p ON p.id = i.project_id
		WHERE i.project_id = $1 AND i.install_id = $2
	`, req.ProjectID, req.InstallID).Scan(&storedHash, &environment, &strictCatalog, &enabled)
	if err == pgx.ErrNoRows {
		return nil, apierr.New(401, contracts.CodeUnauthorized, "unknown installation")
	}
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, apierr.New(400, contracts.CodeInvalidRequest, "project is disabled")
	}
	if subtle.ConstantTimeCompare(storedHash, providedHash[:]) != 1 {
		return nil, apierr.New(401, contracts.CodeUnauthorized, "credential does not match installation")
	}

	now := time.Now().UTC()
	sentAtClient := parseTimestamp(req.SentAtClient)
	skew := now.Sub(sentAtClient)
	skewMs := skew.Milliseconds()

	rejected := append([]contracts.RejectedEvent{}, decodeRejections...)
	var prepared []preparedEvent

	for i := range req.Events {
		e := req.Events[i]
		if err := contracts.ValidateEvent(&e); err != nil {
			ve := err.(*contracts.ValidationError)
			rejected = append(rejected, contracts.RejectedEvent{EventID: e.EventID, Code: ve.Code})
			continue
		}

		kind := "product"
		if contracts.ReservedSystemEvents[e.Name] {
			kind = "system"
		}

		if kind == "product" && strictCatalog {
			var exists bool
			if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_catalog WHERE project_id = $1 AND name = $2)`, req.ProjectID, e.Name).Scan(&exists); err != nil {
				return nil, err
			}
			if !exists {
				rejected = append(rejected, contracts.RejectedEvent{EventID: e.EventID, Code: contracts.CodeUnknownEvent})
				continue
			}
		}

		effectiveAt, quality := effectiveTime(parseTimestamp(e.OccurredAtClient), now, skew)
		prepared = append(prepared, preparedEvent{event: e, kind: kind, effectiveAt: effectiveAt, quality: quality})
	}

	var accepted, duplicates []string
	var policyAppVersion, policyBuildNumber string

	if len(prepared) > 0 {
		last := prepared[len(prepared)-1].event
		policyAppVersion, policyBuildNumber = last.AppVersion, last.BuildNumber

		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)

		if _, err := tx.Exec(ctx, `
			UPDATE installations
			SET last_seen_at = clock_timestamp(),
			    last_app_version = $3, last_build_number = $4, last_sdk_version = $5
			WHERE project_id = $1 AND install_id = $2
		`, req.ProjectID, req.InstallID, last.AppVersion, last.BuildNumber, req.SDK.Version); err != nil {
			return nil, err
		}

		var activated bool
		var firstProductEffectiveAt *time.Time
		for _, pe := range prepared {
			propsJSON, err := json.Marshal(pe.event.Properties)
			if err != nil {
				return nil, err
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
			`,
				req.ProjectID, pe.event.EventID, req.InstallID, pe.event.SessionID, pe.event.Sequence, pe.event.SessionElapsedMs,
				pe.event.Name, pe.kind, parseTimestamp(pe.event.OccurredAtClient), sentAtClient, now, pe.effectiveAt,
				skewMs, pe.quality, pe.event.AppVersion, pe.event.BuildNumber, pe.event.Platform, pe.event.OSVersion,
				pe.event.DeviceClass, pe.event.Locale, pe.event.TimezoneOffsetMinutes, propsJSON,
			).Scan(&insertedID)

			if err == pgx.ErrNoRows {
				duplicates = append(duplicates, pe.event.EventID)
				continue
			}
			if err != nil {
				return nil, err
			}
			accepted = append(accepted, insertedID)
			activated = true

			if pe.kind == "product" {
				if firstProductEffectiveAt == nil || pe.effectiveAt.Before(*firstProductEffectiveAt) {
					t := pe.effectiveAt
					firstProductEffectiveAt = &t
				}
				if !strictCatalog {
					// Auto-discover unknown product events in development
					// mode instead of rejecting (section 7).
					if _, err := tx.Exec(ctx, `
						INSERT INTO event_catalog (project_id, name, kind, first_seen_at, last_seen_at)
						VALUES ($1, $2, 'product', $3, $3)
						ON CONFLICT (project_id, name) DO UPDATE SET last_seen_at = $3
					`, req.ProjectID, pe.event.Name, now); err != nil {
						return nil, err
					}
				}
			}
		}

		if activated {
			if _, err := tx.Exec(ctx, `
				UPDATE installations
				SET activated_at = COALESCE(activated_at, clock_timestamp()),
				    first_product_event_at = COALESCE(first_product_event_at, $3)
				WHERE project_id = $1 AND install_id = $2
			`, req.ProjectID, req.InstallID, firstProductEffectiveAt); err != nil {
				return nil, err
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}

	policy, err := MatchPolicyAudited(ctx, s.Pool, req.ProjectID, req.InstallID, environment, policyAppVersion, policyBuildNumber, req.SDK.Version)
	if err != nil {
		return nil, err
	}

	// Durable record for parity-report (section 11) — the events table
	// alone can't answer "how many duplicates/rejections happened",
	// since neither is ever inserted there.
	if _, err := s.Pool.Exec(ctx, `
		INSERT INTO ingestion_stats (project_id, install_id, accepted_count, duplicate_count, rejected_count)
		VALUES ($1, $2, $3, $4, $5)
	`, req.ProjectID, req.InstallID, len(accepted), len(duplicates), len(rejected)); err != nil {
		return nil, err
	}

	return &contracts.BatchIngestResponse{
		ServerTime:   nowRFC3339Millis(),
		Accepted:     accepted,
		Duplicates:   duplicates,
		Rejected:     rejected,
		ClientPolicy: policy,
	}, nil
}
