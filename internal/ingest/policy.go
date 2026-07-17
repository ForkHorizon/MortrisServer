package ingest

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/contracts"
)

// DefaultPolicy applies when no client_policy_rules row matches (section
// 5.5). 21600s (6h) matches the plan's own registration-response example.
var DefaultPolicy = contracts.ClientPolicy{
	Mode:             contracts.PolicyModeActive,
	NextCheckSeconds: 21600,
	DiscardPending:   false,
}

// matchPolicyRow returns the most specific enabled client_policy_rules row
// for the given project/environment/app/build/SDK combination (section
// 5.5): "the most specific enabled rule wins", where specificity is the
// count of non-wildcard (non-NULL) match fields. matched is false when no
// rule applied and DefaultPolicy was used instead.
func matchPolicyRow(ctx context.Context, pool *pgxpool.Pool, projectID, environment, appVersion, buildNumber, sdkVersion string) (policy contracts.ClientPolicy, reason string, matched bool, err error) {
	row := pool.QueryRow(ctx, `
		SELECT mode, next_check_seconds, discard_pending, reason
		FROM client_policy_rules
		WHERE project_id = $1
		  AND enabled
		  AND (environment  IS NULL OR environment  = $2)
		  AND (app_version  IS NULL OR app_version  = $3)
		  AND (build_number IS NULL OR build_number = $4)
		  AND (sdk_version  IS NULL OR sdk_version  = $5)
		ORDER BY
			(environment  IS NOT NULL)::int +
			(app_version  IS NOT NULL)::int +
			(build_number IS NOT NULL)::int +
			(sdk_version  IS NOT NULL)::int DESC
		LIMIT 1
	`, projectID, environment, appVersion, buildNumber, sdkVersion)

	err = row.Scan(&policy.Mode, &policy.NextCheckSeconds, &policy.DiscardPending, &reason)
	if err == pgx.ErrNoRows {
		return DefaultPolicy, "", false, nil
	}
	if err != nil {
		return contracts.ClientPolicy{}, "", false, err
	}
	return policy, reason, true, nil
}

// MatchPolicy is matchPolicyRow without install-scoped audit logging, for
// callers that don't have (or don't want to log against) an install_id.
func MatchPolicy(ctx context.Context, pool *pgxpool.Pool, projectID, environment, appVersion, buildNumber, sdkVersion string) (contracts.ClientPolicy, error) {
	policy, _, _, err := matchPolicyRow(ctx, pool, projectID, environment, appVersion, buildNumber, sdkVersion)
	return policy, err
}

// MatchPolicyAudited matches the policy and, when an operator-configured
// rule actually applied (not the passive default), records the decision
// and its reason in admin_audit_log (section 5.5). The passive default
// case is intentionally not logged — it fires on every request and would
// drown out actual policy decisions.
func MatchPolicyAudited(ctx context.Context, pool *pgxpool.Pool, projectID, installID, environment, appVersion, buildNumber, sdkVersion string) (contracts.ClientPolicy, error) {
	policy, reason, matched, err := matchPolicyRow(ctx, pool, projectID, environment, appVersion, buildNumber, sdkVersion)
	if err != nil {
		return contracts.ClientPolicy{}, err
	}
	if matched {
		_, err := pool.Exec(ctx, `
			INSERT INTO admin_audit_log (admin_user_id, action, detail)
			VALUES (NULL, 'client_policy_applied', jsonb_build_object(
				'project_id', $1::text, 'install_id', $2::text, 'mode', $3::text, 'reason', $4::text
			))
		`, projectID, installID, policy.Mode, reason)
		if err != nil {
			return contracts.ClientPolicy{}, err
		}
	}
	return policy, nil
}
