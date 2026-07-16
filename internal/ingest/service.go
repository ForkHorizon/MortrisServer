// Package ingest implements the registration, batch-ingestion, and
// client-policy-probe business logic (server_implementation_plan.md
// section 5). It is HTTP-agnostic — internal/httpapi decodes requests and
// renders these results/errors as JSON.
package ingest

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/diskstate"
	"github.com/ForkHorizon/Mortris/internal/ratelimit"
)

// DiskStateSource reports the current disk-pressure state (section 12).
// Batch ingestion gates on it; nil means "no gating" (Normal), which is
// what CLI tools and tests get by default.
type DiskStateSource interface {
	Get() diskstate.State
}

// Service holds everything the three SDK-facing endpoints need: the
// writer pool and the rate limiters from section 6.
type Service struct {
	Pool *pgxpool.Pool
	Disk DiskStateSource

	regIPLimiter      *ratelimit.Limiter
	regProjectLimiter *ratelimit.Limiter

	ingestInstallLimiter *ratelimit.Limiter
	ingestProjectLimiter *ratelimit.Limiter
	ingestIPLimiter      *ratelimit.Limiter

	// DailyRegistrationCap is the "prototype safety cap" from section 6 —
	// raise it deliberately before a launch, per the plan, rather than
	// discovering it during one.
	DailyRegistrationCap int64
}

func (s *Service) diskState() diskstate.State {
	if s.Disk == nil {
		return diskstate.Normal
	}
	return s.Disk.Get()
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		Pool: pool,

		regIPLimiter: ratelimit.NewLimiter(ratelimit.PerMinute(120), 240),
		// project/minute and ingestion-IP/second have no stated burst in
		// section 6's table; 2x rate mirrors the ratio the plan did state
		// for registration-per-IP (burst 240 = 2x rate 120/min).
		regProjectLimiter: ratelimit.NewLimiter(ratelimit.PerMinute(600), 1200),

		ingestInstallLimiter: ratelimit.NewLimiter(ratelimit.PerMinute(12), 30),
		ingestProjectLimiter: ratelimit.NewLimiter(30, 100),
		ingestIPLimiter:      ratelimit.NewLimiter(100, 200),

		DailyRegistrationCap: 10000,
	}
}
