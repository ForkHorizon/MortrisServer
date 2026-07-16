package ingest

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

// Policy implements the lightweight authenticated policy probe (section
// 5.5): same bearer auth as batch ingestion, but no events and therefore
// no transaction.
func (s *Service) Policy(ctx context.Context, req *contracts.PolicyRequest, bearerToken, sourceIP string) (*contracts.PolicyResponse, error) {
	if !s.ingestIPLimiter.Allow(sourceIP) {
		return nil, apierr.WithRetryAfter(429, contracts.CodeRateLimited, "ingestion rate limit exceeded for source IP", time.Second)
	}
	if err := req.Validate(); err != nil {
		ve := err.(*contracts.ValidationError)
		return nil, apierr.New(400, ve.Code, ve.Message)
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
	var enabled bool
	err = s.Pool.QueryRow(ctx, `
		SELECT i.credential_hash, p.environment, p.enabled
		FROM installations i JOIN projects p ON p.id = i.project_id
		WHERE i.project_id = $1 AND i.install_id = $2
	`, req.ProjectID, req.InstallID).Scan(&storedHash, &environment, &enabled)
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

	policy, err := MatchPolicyAudited(ctx, s.Pool, req.ProjectID, req.InstallID, environment, req.AppVersion, req.BuildNumber, req.SDK.Version)
	if err != nil {
		return nil, err
	}

	if _, err := s.Pool.Exec(ctx, `UPDATE installations SET last_seen_at = clock_timestamp() WHERE project_id = $1 AND install_id = $2`, req.ProjectID, req.InstallID); err != nil {
		return nil, err
	}

	return &contracts.PolicyResponse{
		ServerTime:   nowRFC3339Millis(),
		ClientPolicy: policy,
	}, nil
}
