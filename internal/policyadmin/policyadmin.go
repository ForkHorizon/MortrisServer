// Package policyadmin implements the kill-switch administration API
// (Phase S3: "policy administration") — creating and removing the
// client_policy_rules rows that internal/ingest.MatchPolicy reads.
// Distinct from that package: this is an admin-authenticated write path
// through the writer pool/role, not an SDK-facing read.
package policyadmin

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

type Rule struct {
	ID               int64     `json:"id"`
	ProjectID        string    `json:"project_id"`
	Environment      *string   `json:"environment,omitempty"`
	AppVersion       *string   `json:"app_version,omitempty"`
	BuildNumber      *string   `json:"build_number,omitempty"`
	SDKVersion       *string   `json:"sdk_version,omitempty"`
	Mode             string    `json:"mode"`
	NextCheckSeconds int       `json:"next_check_seconds"`
	DiscardPending   bool      `json:"discard_pending"`
	Reason           string    `json:"reason"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func List(ctx context.Context, pool *pgxpool.Pool, projectID string) ([]Rule, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, project_id, environment, app_version, build_number, sdk_version,
		       mode, next_check_seconds, discard_pending, reason, enabled, created_at, updated_at
		FROM client_policy_rules
		WHERE project_id = $1
		ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// []Rule{}, not nil — encoding/json emits null for a nil slice, and
	// the dashboard renders that empty-rules response every time a
	// project has no active kill-switch rules (the common case).
	rules := []Rule{}
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Environment, &r.AppVersion, &r.BuildNumber, &r.SDKVersion,
			&r.Mode, &r.NextCheckSeconds, &r.DiscardPending, &r.Reason, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

type CreateInput struct {
	ProjectID        string
	Environment      *string
	AppVersion       *string
	BuildNumber      *string
	SDKVersion       *string
	Mode             string
	NextCheckSeconds int
	DiscardPending   bool
	Reason           string
}

var validModes = map[string]bool{
	contracts.PolicyModeActive:            true,
	contracts.PolicyModePauseUpload:       true,
	contracts.PolicyModeDisableCollection: true,
}

func (in CreateInput) Validate() error {
	if in.ProjectID == "" {
		return apierr.New(400, contracts.CodeInvalidRequest, "project_id is required")
	}
	if !validModes[in.Mode] {
		return apierr.New(400, contracts.CodeInvalidRequest, "mode must be active, pause_upload, or disable_collection")
	}
	if in.NextCheckSeconds <= 0 {
		return apierr.New(400, contracts.CodeInvalidRequest, "next_check_seconds must be positive")
	}
	if in.Reason == "" {
		return apierr.New(400, contracts.CodeInvalidRequest, "reason is required — this is an audited kill-switch action")
	}
	return nil
}

// Create inserts a rule and audit-logs it (section 5.5: "Store the last
// policy decision and its reason in the admin audit log" — this is the
// decision being made, distinct from internal/ingest's per-request log of
// a decision being applied).
func Create(ctx context.Context, pool *pgxpool.Pool, adminUserID int64, in CreateInput) (*Rule, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var r Rule
	err = tx.QueryRow(ctx, `
		INSERT INTO client_policy_rules
			(project_id, environment, app_version, build_number, sdk_version, mode, next_check_seconds, discard_pending, reason, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true)
		RETURNING id, project_id, environment, app_version, build_number, sdk_version,
		          mode, next_check_seconds, discard_pending, reason, enabled, created_at, updated_at
	`, in.ProjectID, in.Environment, in.AppVersion, in.BuildNumber, in.SDKVersion,
		in.Mode, in.NextCheckSeconds, in.DiscardPending, in.Reason,
	).Scan(&r.ID, &r.ProjectID, &r.Environment, &r.AppVersion, &r.BuildNumber, &r.SDKVersion,
		&r.Mode, &r.NextCheckSeconds, &r.DiscardPending, &r.Reason, &r.Enabled, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_log (admin_user_id, action, detail)
		VALUES ($1, 'policy_rule_created', jsonb_build_object(
			'rule_id', $2::bigint, 'project_id', $3::text, 'mode', $4::text, 'reason', $5::text
		))
	`, adminUserID, r.ID, r.ProjectID, r.Mode, r.Reason); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &r, nil
}

// Delete removes a rule scoped to projectID (defense in depth — the
// caller must already have checked session project access, but this
// keeps the query itself from ever touching another project's rule) and
// audit-logs it.
func Delete(ctx context.Context, pool *pgxpool.Pool, adminUserID int64, projectID string, ruleID int64) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var mode string
	err = tx.QueryRow(ctx, `
		DELETE FROM client_policy_rules WHERE id = $1 AND project_id = $2 RETURNING mode
	`, ruleID, projectID).Scan(&mode)
	if err == pgx.ErrNoRows {
		return apierr.New(404, "not_found", "policy rule not found")
	}
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_log (admin_user_id, action, detail)
		VALUES ($1, 'policy_rule_deleted', jsonb_build_object('rule_id', $2::bigint, 'project_id', $3::text, 'mode', $4::text))
	`, adminUserID, ruleID, projectID, mode); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
