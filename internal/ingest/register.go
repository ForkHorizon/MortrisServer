package ingest

import (
	"context"

	"github.com/ForkHorizon/Mortris/internal/contracts"
)

// Register implements the atomic registration behavior of section 5.2:
// unknown ID inserts, a matching-credential retry returns the same
// success, a mismatched credential is a permanent 409 conflict, and an
// invalid or over-quota request creates no row.
func (s *Service) Register(ctx context.Context, req *contracts.RegisterRequest, sourceIP string) (*contracts.RegisterResponse, error) {
	if err := s.admitRegistration(req, sourceIP); err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	environment, err := registrationEnvironment(ctx, tx, req.ProjectID)
	if err != nil {
		return nil, err
	}
	if err := s.createOrVerifyInstallation(ctx, tx, req); err != nil {
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
