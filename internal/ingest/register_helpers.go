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

func (s *Service) admitRegistration(req *contracts.RegisterRequest, sourceIP string) error {
	if !s.regIPLimiter.Allow(sourceIP) {
		return apierr.WithRetryAfter(429, contracts.CodeRateLimited, "registration rate limit exceeded for source IP", time.Minute)
	}
	if err := req.Validate(); err != nil {
		ve := err.(*contracts.ValidationError)
		return apierr.New(400, ve.Code, ve.Message)
	}
	if !s.regProjectLimiter.Allow(req.ProjectID) {
		return apierr.WithRetryAfter(429, contracts.CodeRateLimited, "registration rate limit exceeded for project", time.Minute)
	}
	return nil
}

func registrationEnvironment(ctx context.Context, tx pgx.Tx, projectID string) (string, error) {
	var environment string
	var enabled bool
	err := tx.QueryRow(ctx, `SELECT environment, enabled FROM projects WHERE id = $1`, projectID).Scan(&environment, &enabled)
	if err == pgx.ErrNoRows {
		return "", apierr.New(400, contracts.CodeInvalidRequest, "unknown project_id")
	}
	if err != nil {
		return "", err
	}
	if !enabled {
		return "", apierr.New(400, contracts.CodeInvalidRequest, "project is disabled")
	}
	return environment, nil
}

func (s *Service) createOrVerifyInstallation(ctx context.Context, tx pgx.Tx, req *contracts.RegisterRequest) error {
	credentialBytes, _ := base64.RawURLEncoding.DecodeString(req.InstallationCredential)
	credentialHash := sha256.Sum256(credentialBytes)
	var existingHash []byte
	err := tx.QueryRow(ctx, `
		SELECT credential_hash FROM installations
		WHERE project_id = $1 AND install_id = $2
		FOR UPDATE
	`, req.ProjectID, req.InstallID).Scan(&existingHash)
	switch err {
	case nil:
		if subtle.ConstantTimeCompare(existingHash, credentialHash[:]) != 1 {
			return apierr.New(409, contracts.CodeInstallConflict, "install_id already registered with a different credential")
		}
		return nil
	case pgx.ErrNoRows:
		return s.insertInstallation(ctx, tx, req, credentialHash[:])
	default:
		return err
	}
}

func (s *Service) insertInstallation(ctx context.Context, tx pgx.Tx, req *contracts.RegisterRequest, credentialHash []byte) error {
	var count int64
	err := tx.QueryRow(ctx, `
		INSERT INTO daily_registration_counters (project_id, day, count)
		VALUES ($1, (clock_timestamp() AT TIME ZONE 'UTC')::date, 1)
		ON CONFLICT (project_id, day) DO UPDATE
			SET count = daily_registration_counters.count + 1
		RETURNING count
	`, req.ProjectID).Scan(&count)
	if err != nil {
		return err
	}
	if count > s.DailyRegistrationCap {
		return apierr.WithRetryAfter(429, contracts.CodeRateLimited, "project daily registration cap exceeded", time.Hour)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO installations
			(project_id, install_id, credential_hash, last_app_version, last_build_number, last_sdk_version)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, req.ProjectID, req.InstallID, credentialHash, req.AppVersion, req.BuildNumber, req.SDKVersion)
	return err
}
