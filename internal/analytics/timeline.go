package analytics

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/apierr"
)

const maxTimelineEvents = 500

type TimelineEvent struct {
	EventID     string          `json:"event_id"`
	Name        string          `json:"name"`
	EventKind   string          `json:"event_kind"`
	EffectiveAt time.Time       `json:"effective_at"`
	TimeQuality string          `json:"time_quality"`
	Properties  json.RawMessage `json:"properties"`
}

type TimelineResult struct {
	InstallID    string          `json:"install_id"`
	RegisteredAt time.Time       `json:"registered_at"`
	ActivatedAt  *time.Time      `json:"activated_at,omitempty"`
	Events       []TimelineEvent `json:"events"`
	Truncated    bool            `json:"truncated"`
}

// GetInstallationTimeline implements section 10.2 #5: product and system
// event history for one anonymous install_id, admin-only (enforced by the
// caller, not here — this package has no notion of session role).
func GetInstallationTimeline(ctx context.Context, pool *pgxpool.Pool, projectID, installID string) (*TimelineResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	// Events starts as []TimelineEvent{}, not nil — encoding/json emits
	// null for a nil slice, which crashes a naive frontend list render on
	// the (unremarkable) case of a registered-but-not-yet-active install.
	result := &TimelineResult{InstallID: installID, Events: []TimelineEvent{}}
	err := pool.QueryRow(ctx, `
		SELECT registered_at, activated_at FROM installations
		WHERE project_id = $1 AND install_id = $2
	`, projectID, installID).Scan(&result.RegisteredAt, &result.ActivatedAt)
	if err == pgx.ErrNoRows {
		return nil, apierr.New(404, "not_found", "installation not found")
	}
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, `
		SELECT event_id, name, event_kind, effective_at, time_quality, properties
		FROM events
		WHERE project_id = $1 AND install_id = $2
		ORDER BY effective_at DESC
		LIMIT $3
	`, projectID, installID, maxTimelineEvents+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var e TimelineEvent
		if err := rows.Scan(&e.EventID, &e.Name, &e.EventKind, &e.EffectiveAt, &e.TimeQuality, &e.Properties); err != nil {
			return nil, err
		}
		result.Events = append(result.Events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(result.Events) > maxTimelineEvents {
		result.Events = result.Events[:maxTimelineEvents]
		result.Truncated = true
	}
	return result, nil
}
