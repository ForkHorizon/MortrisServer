package ingest

import (
	"context"
	"time"

	"github.com/ForkHorizon/Mortris/internal/contracts"
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
	if err := s.admitBatch(req, sourceIP); err != nil {
		return nil, err
	}
	environment, strictCatalog, err := s.authorizeBatch(ctx, req, bearerToken)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	prepared, rejected, err := s.prepareBatch(ctx, req, decodeRejections, strictCatalog, now)
	if err != nil {
		return nil, err
	}
	accepted, duplicates, appVersion, buildNumber, err := s.persistBatch(ctx, req, prepared, strictCatalog, now)
	if err != nil {
		return nil, err
	}
	policy, err := MatchPolicyAudited(ctx, s.Pool, req.ProjectID, req.InstallID, environment, appVersion, buildNumber, req.SDK.Version)
	if err != nil {
		return nil, err
	}
	if err := s.recordBatchStats(ctx, req, len(accepted), len(duplicates), len(rejected)); err != nil {
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
