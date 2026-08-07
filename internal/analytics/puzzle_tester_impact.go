package analytics

// PuzzleTesterImpact is Stage 3's "compact tester-impact summary" (docs/puzzle-analytics-remaining-plan.md
// section 6, "Dashboard behavior"): what developer/tester traffic did in
// this range, shown next to — never blended into — the natural verdict.

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PuzzleTesterCommandCount struct {
	Command string `json:"command"`
	Result  string `json:"result"`
	Count   int64  `json:"count"`
}

type PuzzleTesterImpact struct {
	DeveloperActions  int64 `json:"developer_actions"`
	AffectedHouseRuns int64 `json:"affected_house_runs"`
	AffectedAttempts  int64 `json:"affected_attempts"`

	Commands []PuzzleTesterCommandCount `json:"commands"`

	// Resets are commands whose name starts with "Reset" (the client's own
	// split, PuzzleAnalyticsService.cs: progress_origin becomes
	// developer_reset vs developer_modified); ProgressCompletions is every
	// other progress-changing command.
	Resets              int64 `json:"resets"`
	ProgressCompletions int64 `json:"progress_completions"`
	Failures            int64 `json:"failures"`
	Noops               int64 `json:"noops"`

	// A developer_progress_mutated event should always carry both hashes;
	// Missing is a coverage/quality signal, not expected to be nonzero.
	MutationsWithBeforeAfter    int64 `json:"mutations_with_before_after"`
	MutationsMissingBeforeAfter int64 `json:"mutations_missing_before_after"`
}

func GetPuzzleTesterImpact(ctx context.Context, pool *pgxpool.Pool, projectID string, from, to time.Time) (*PuzzleTesterImpact, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	impact := &PuzzleTesterImpact{Commands: []PuzzleTesterCommandCount{}}
	if err := loadTesterActionCounts(ctx, pool, impact, projectID, from, to); err != nil {
		return nil, err
	}
	if err := loadTesterAffectedRuns(ctx, pool, impact, projectID, from, to); err != nil {
		return nil, err
	}
	if err := loadTesterCommandBreakdown(ctx, pool, impact, projectID, from, to); err != nil {
		return nil, err
	}
	if err := loadTesterResetsAndCompletions(ctx, pool, impact, projectID, from, to); err != nil {
		return nil, err
	}
	return impact, nil
}

func loadTesterActionCounts(ctx context.Context, pool *pgxpool.Pool, impact *PuzzleTesterImpact, projectID string, from, to time.Time) error {
	return pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT properties->>'developer_action_id')
		FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND name='developer_command_started'
	`, projectID, from, to).Scan(&impact.DeveloperActions)
}

// loadTesterAffectedRuns reuses the same classification views every other
// Stage 3 query joins against, rather than re-deriving "developer
// touched" here.
func loadTesterAffectedRuns(ctx context.Context, pool *pgxpool.Pool, impact *PuzzleTesterImpact, projectID string, from, to time.Time) error {
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM puzzle_house_run_classification
		WHERE project_id=$1 AND NOT fully_natural AND last_event_at>=$2 AND last_event_at<$3
	`, projectID, from, to).Scan(&impact.AffectedHouseRuns); err != nil {
		return err
	}
	return pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM puzzle_wave_attempt_classification
		WHERE project_id=$1 AND NOT fully_natural AND last_event_at>=$2 AND last_event_at<$3
	`, projectID, from, to).Scan(&impact.AffectedAttempts)
}

func loadTesterCommandBreakdown(ctx context.Context, pool *pgxpool.Pool, impact *PuzzleTesterImpact, projectID string, from, to time.Time) error {
	rows, err := pool.Query(ctx, `
		SELECT COALESCE(properties->>'developer_command',''), name, COUNT(*)
		FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3
		  AND name IN ('developer_command_completed','developer_command_failed','developer_command_noop')
		GROUP BY 1,2 ORDER BY 3 DESC
	`, projectID, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var c PuzzleTesterCommandCount
		var name string
		if err := rows.Scan(&c.Command, &name, &c.Count); err != nil {
			return err
		}
		c.Result = name
		impact.Commands = append(impact.Commands, c)
		switch name {
		case "developer_command_failed":
			impact.Failures += c.Count
		case "developer_command_noop":
			impact.Noops += c.Count
		}
	}
	return rows.Err()
}

// loadTesterResetsAndCompletions counts progress-changing commands
// (progress_changed=true on their completed event) split by the client's
// own Reset-prefix convention, and separately checks that every
// developer_progress_mutated event carries both state hashes.
func loadTesterResetsAndCompletions(ctx context.Context, pool *pgxpool.Pool, impact *PuzzleTesterImpact, projectID string, from, to time.Time) error {
	if err := pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE properties->>'developer_command' LIKE 'Reset%'),
		  COUNT(*) FILTER (WHERE properties->>'developer_command' NOT LIKE 'Reset%')
		FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3
		  AND name='developer_command_completed' AND (properties->>'progress_changed')::boolean IS TRUE
	`, projectID, from, to).Scan(&impact.Resets, &impact.ProgressCompletions); err != nil {
		return err
	}
	return pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE COALESCE(properties->>'before_state_hash','')<>'' AND COALESCE(properties->>'after_state_hash','')<>''),
		  COUNT(*) FILTER (WHERE COALESCE(properties->>'before_state_hash','')='' OR COALESCE(properties->>'after_state_hash','')='')
		FROM events
		WHERE project_id=$1 AND effective_at>=$2 AND effective_at<$3 AND name='developer_progress_mutated'
	`, projectID, from, to).Scan(&impact.MutationsWithBeforeAfter, &impact.MutationsMissingBeforeAfter)
}
