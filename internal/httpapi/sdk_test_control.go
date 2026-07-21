package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/ForkHorizon/Mortris/internal/sdktest"
)

// sdkTestScenario reads central-dashboard controls from the database. The
// legacy controller remains as a fallback only for the already-deployed
// isolated staging service until it is intentionally retired.
func (s *Server) sdkTestScenario(ctx context.Context, r *http.Request, projectID string) sdktest.Scenario {
	var tokenHash []byte
	var scenario string
	err := s.Pool.QueryRow(ctx, `
		SELECT sdk_test_token_hash, sdk_test_scenario
		FROM projects
		WHERE id = $1 AND environment = 'test' AND sdk_test_enabled AND archived_at IS NULL
	`, projectID).Scan(&tokenHash, &scenario)
	if err == nil && len(tokenHash) > 0 {
		actual := sha256.Sum256([]byte(r.Header.Get(sdktest.HeaderToken)))
		if subtle.ConstantTimeCompare(actual[:], tokenHash) == 1 && knownSDKScenario(sdktest.Scenario(scenario)) {
			return sdktest.Scenario(scenario)
		}
		return ""
	}
	if err != nil && err != pgx.ErrNoRows {
		s.Log.Warn("read SDK test scenario", "error", err)
	}
	return s.SDKTest.Scenario(r, projectID)
}
