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

// Register implements the atomic registration behavior of section 5.2:
// unknown ID inserts, a matching-credential retry returns the same
// success, a mismatched credential is a permanent 409 conflict, and an
// invalid or over-quota request creates no row.
func (s *Service) Register(ctx context.Context, req *contracts.RegisterRequest, sourceIP string) (*contracts.RegisterResponse, error) {
	if !s.regIPLimiter.Allow(sourceIP) {
		return nil, apierr.WithRetryAfter(429, contracts.CodeRateLimited, "registration rate limit exceeded for source IP", time.Minute)
	}
	if err := req.Validate(); err != nil {
		ve := err.(*contracts.ValidationError)
		return nil, apierr.New(400, ve.Code, ve.Message)
	}
	if !s.regProjectLimiter.Allow(req.ProjectID) {
		return nil, apierr.WithRetryAfter(429, contracts.CodeRateLimited, "registration rate limit exceeded for project", time.Minute)
	}

	// Format already checked by req.Validate(); the decode here cannot fail.
	credentialBytes, _ := base64.RawURLEncoding.DecodeString(req.InstallationCredential)
	credentialHash := sha256.Sum256(credentialBytes)

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) // no-op once committed

	var environment string
	var enabled bool
	err = tx.QueryRow(ctx, `SELECT environment, enabled FROM projects WHERE id = $1`, req.ProjectID).Scan(&environment, &enabled)
	if err == pgx.ErrNoRows {
		return nil, apierr.New(400, contracts.CodeInvalidRequest, "unknown project_id")
	}
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, apierr.New(400, contracts.CodeInvalidRequest, "project is disabled")
	}

	var existingHash []byte
	err = tx.QueryRow(ctx, `
		SELECT credential_hash FROM installations
		WHERE project_id = $1 AND install_id = $2
		FOR UPDATE
	`, req.ProjectID, req.InstallID).Scan(&existingHash)

	switch {
	case err == nil:
		if subtle.ConstantTimeCompare(existingHash, credentialHash[:]) != 1 {
			// Never replace the stored credential (section 5.3).
			return nil, apierr.New(409, contracts.CodeInstallConflict, "install_id already registered with a different credential")
		}
		// Matching retry: a lost-response retry of the same registration.
		// Nothing to change — fall through to commit and respond.

	case err == pgx.ErrNoRows:
		var count int64
		err = tx.QueryRow(ctx, `
			INSERT INTO daily_registration_counters (project_id, day, count)
			VALUES ($1, (clock_timestamp() AT TIME ZONE 'UTC')::date, 1)
			ON CONFLICT (project_id, day) DO UPDATE
				SET count = daily_registration_counters.count + 1
			RETURNING count
		`, req.ProjectID).Scan(&count)
		if err != nil {
			return nil, err
		}
		if count > s.DailyRegistrationCap {
			// Rolling back releases the reserved counter slot for the
			// next attempt today (section 6: "raise it before a
			// campaign", not silently reject forever).
			return nil, apierr.WithRetryAfter(429, contracts.CodeRateLimited, "project daily registration cap exceeded", time.Hour)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO installations
				(project_id, install_id, credential_hash, last_app_version, last_build_number, last_sdk_version)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, req.ProjectID, req.InstallID, credentialHash[:], req.AppVersion, req.BuildNumber, req.SDKVersion)
		if err != nil {
			return nil, err
		}

	default:
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	policy, err := MatchPolicyAudited(ctx, s.Pool, req.ProjectID, req.InstallID, environment, req.AppVersion, req.BuildNumber, req.SDKVersion)
	if err != nil {
		return nil, err
	}

	return &contracts.RegisterResponse{
		ServerTime:         nowRFC3339Millis(),
		InstallationStatus: "registered",
		ClientPolicy:       policy,
	}, nil
}
