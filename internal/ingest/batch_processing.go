package ingest

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
	"github.com/ForkHorizon/Mortris/internal/diskstate"
)

func (s *Service) admitBatch(req *contracts.BatchIngestRequest, sourceIP string) error {
	if s.diskState() == diskstate.Rejecting {
		return apierr.WithRetryAfter(503, contracts.CodeServerStoragePressure, "server is rejecting ingestion under disk pressure", 5*time.Minute)
	}
	if !s.ingestIPLimiter.Allow(sourceIP) {
		return apierr.WithRetryAfter(429, contracts.CodeRateLimited, "ingestion rate limit exceeded for source IP", time.Second)
	}
	if err := req.Validate(); err != nil {
		ve := err.(*contracts.ValidationError)
		return apierr.New(400, ve.Code, ve.Message)
	}
	if !s.ingestProjectLimiter.Allow(req.ProjectID) {
		return apierr.WithRetryAfter(429, contracts.CodeRateLimited, "ingestion rate limit exceeded for project", time.Second)
	}
	if !s.ingestInstallLimiter.Allow(req.ProjectID + "/" + req.InstallID) {
		return apierr.WithRetryAfter(429, contracts.CodeRateLimited, "ingestion rate limit exceeded for installation", time.Minute)
	}
	return nil
}

func (s *Service) authorizeBatch(ctx context.Context, req *contracts.BatchIngestRequest, bearerToken string) (string, bool, error) {
	providedHash, err := batchCredentialHash(bearerToken)
	if err != nil {
		return "", false, err
	}
	var storedHash []byte
	var environment string
	var strictCatalog, enabled bool
	err = s.Pool.QueryRow(ctx, `
		SELECT i.credential_hash, p.environment, p.strict_catalog, p.enabled
		FROM installations i JOIN projects p ON p.id = i.project_id
		WHERE i.project_id = $1 AND i.install_id = $2
	`, req.ProjectID, req.InstallID).Scan(&storedHash, &environment, &strictCatalog, &enabled)
	if err == pgx.ErrNoRows {
		return "", false, apierr.New(401, contracts.CodeUnauthorized, "unknown installation")
	}
	if err != nil {
		return "", false, err
	}
	if !enabled {
		return "", false, apierr.New(400, contracts.CodeInvalidRequest, "project is disabled")
	}
	if subtle.ConstantTimeCompare(storedHash, providedHash[:]) != 1 {
		return "", false, apierr.New(401, contracts.CodeUnauthorized, "credential does not match installation")
	}
	return environment, strictCatalog, nil
}

func batchCredentialHash(bearerToken string) ([sha256.Size]byte, error) {
	if bearerToken == "" {
		return [sha256.Size]byte{}, apierr.New(401, contracts.CodeUnauthorized, "missing bearer credential")
	}
	credential, err := base64.RawURLEncoding.DecodeString(bearerToken)
	if err != nil || len(credential) != sha256.Size {
		return [sha256.Size]byte{}, apierr.New(401, contracts.CodeUnauthorized, "malformed bearer credential")
	}
	return sha256.Sum256(credential), nil
}

func (s *Service) prepareBatch(ctx context.Context, req *contracts.BatchIngestRequest, decodeRejections []contracts.RejectedEvent, strictCatalog bool, now time.Time) ([]preparedEvent, []contracts.RejectedEvent, error) {
	rejected := append([]contracts.RejectedEvent{}, decodeRejections...)
	prepared := make([]preparedEvent, 0, len(req.Events))
	skew := now.Sub(parseTimestamp(req.SentAtClient))
	for i := range req.Events {
		pe, rejection, err := s.prepareEvent(ctx, req.ProjectID, req.Events[i], strictCatalog, now, skew)
		if err != nil {
			return nil, nil, err
		}
		if rejection != nil {
			rejected = append(rejected, *rejection)
			continue
		}
		prepared = append(prepared, pe)
	}
	return prepared, rejected, nil
}

func (s *Service) prepareEvent(ctx context.Context, projectID string, event contracts.Event, strictCatalog bool, now time.Time, skew time.Duration) (preparedEvent, *contracts.RejectedEvent, error) {
	if err := contracts.ValidateEvent(&event); err != nil {
		ve := err.(*contracts.ValidationError)
		return preparedEvent{}, &contracts.RejectedEvent{EventID: event.EventID, Code: ve.Code}, nil
	}
	kind := "product"
	if contracts.ReservedSystemEvents[event.Name] {
		kind = "system"
	}
	if kind == "product" && strictCatalog {
		allowed, known, err := s.catalogProperties(ctx, projectID, event.Name)
		if err != nil {
			return preparedEvent{}, nil, err
		}
		if !known {
			return preparedEvent{}, &contracts.RejectedEvent{EventID: event.EventID, Code: contracts.CodeUnknownEvent}, nil
		}
		// Empty property declarations retain the original strict-name-only
		// behavior for older projects. A declared list makes the event schema
		// strict as well, which is what the gravity playtest needs.
		if len(allowed) > 0 {
			for key := range event.Properties {
				if !allowed[key] {
					return preparedEvent{}, &contracts.RejectedEvent{EventID: event.EventID, Code: contracts.CodeInvalidPropertyKey}, nil
				}
			}
		}
	}
	effectiveAt, quality := effectiveTime(parseTimestamp(event.OccurredAtClient), now, skew)
	return preparedEvent{event: event, kind: kind, effectiveAt: effectiveAt, quality: quality}, nil, nil
}

func (s *Service) catalogProperties(ctx context.Context, projectID, name string) (map[string]bool, bool, error) {
	var properties []struct {
		Name string `json:"name"`
	}
	err := s.Pool.QueryRow(ctx, `SELECT properties FROM event_catalog WHERE project_id = $1 AND name = $2`, projectID, name).Scan(&properties)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	allowed := make(map[string]bool, len(properties))
	for _, property := range properties {
		if property.Name != "" {
			allowed[property.Name] = true
		}
	}
	return allowed, true, nil
}
