package analytics

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

const (
	defaultRecentEventsLimit = 100
	maxRecentEventsLimit     = 500
)

type RecentEventsFilter struct {
	Name             *string
	Platform         *string
	AppVersion       *string
	InstallID        *string
	BeforeReceivedAt *time.Time
	BeforeEventID    *string
	Limit            int
}

// ParseRecentEventsFilter reads the Live Event Feed's filters and its
// keyset pagination cursor. Unlike ParseEventExplorerFilter, name is not
// checked against the event catalog — this is a raw debug tail, and an
// unrecognized or mistyped name is exactly the kind of thing it exists to
// surface.
func ParseRecentEventsFilter(q url.Values) (RecentEventsFilter, error) {
	f := RecentEventsFilter{
		Name:       optional(q, "name"),
		Platform:   optional(q, "platform"),
		AppVersion: optional(q, "app_version"),
		InstallID:  optional(q, "install_id"),
		Limit:      defaultRecentEventsLimit,
	}

	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return f, apierr.New(400, contracts.CodeInvalidRequest, "limit must be a positive integer")
		}
		if n > maxRecentEventsLimit {
			n = maxRecentEventsLimit
		}
		f.Limit = n
	}

	before := optional(q, "before_received_at")
	beforeID := optional(q, "before_event_id")
	if (before == nil) != (beforeID == nil) {
		return f, apierr.New(400, contracts.CodeInvalidRequest, "before_received_at and before_event_id must be given together")
	}
	if before != nil {
		t, err := time.Parse(time.RFC3339, *before)
		if err != nil {
			return f, apierr.New(400, contracts.CodeInvalidRequest, "invalid before_received_at: must be RFC 3339")
		}
		f.BeforeReceivedAt = &t
		f.BeforeEventID = beforeID
	}

	return f, nil
}

type RecentEvent struct {
	EventID     string          `json:"event_id"`
	Name        string          `json:"name"`
	EventKind   string          `json:"event_kind"`
	InstallID   string          `json:"install_id"`
	Platform    string          `json:"platform"`
	AppVersion  string          `json:"app_version"`
	BuildNumber string          `json:"build_number"`
	ReceivedAt  time.Time       `json:"received_at"`
	EffectiveAt time.Time       `json:"effective_at"`
	TimeQuality string          `json:"time_quality"`
	Properties  json.RawMessage `json:"properties"`
}

type RecentEventsResult struct {
	Events               []RecentEvent `json:"events"`
	NextBeforeReceivedAt *string       `json:"next_before_received_at,omitempty"`
	NextBeforeEventID    *string       `json:"next_before_event_id,omitempty"`
}

// GetRecentEvents implements the Live Event Feed (Phase 1a): the newest
// events for a project, newest-first, keyset-paginated on received_at so a
// "Load older" page is a cheap index range scan rather than an OFFSET.
func GetRecentEvents(ctx context.Context, pool *pgxpool.Pool, projectID string, f RecentEventsFilter) (*RecentEventsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := pool.Query(ctx, `
		SELECT event_id, name, event_kind, install_id::text, platform, app_version, build_number,
		       received_at, effective_at, time_quality, properties
		FROM events
		WHERE project_id = $1
		  AND ($2::text IS NULL OR name = $2)
		  AND ($3::text IS NULL OR platform = $3)
		  AND ($4::text IS NULL OR app_version = $4)
		  AND ($5::text IS NULL OR install_id::text = $5)
		  AND ($6::timestamptz IS NULL OR (received_at, event_id) < ($6, $7))
		ORDER BY received_at DESC, event_id DESC
		LIMIT $8
	`, projectID, f.Name, f.Platform, f.AppVersion, f.InstallID, f.BeforeReceivedAt, f.BeforeEventID, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Events starts as []RecentEvent{}, not nil — encoding/json emits null
	// for a nil slice, which crashes a naive frontend list render on a
	// brand-new project with no events yet.
	result := &RecentEventsResult{Events: []RecentEvent{}}
	for rows.Next() {
		var e RecentEvent
		if err := rows.Scan(&e.EventID, &e.Name, &e.EventKind, &e.InstallID, &e.Platform, &e.AppVersion, &e.BuildNumber,
			&e.ReceivedAt, &e.EffectiveAt, &e.TimeQuality, &e.Properties); err != nil {
			return nil, err
		}
		result.Events = append(result.Events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(result.Events) == f.Limit {
		last := result.Events[len(result.Events)-1]
		receivedAt := last.ReceivedAt.Format(time.RFC3339Nano)
		eventID := last.EventID
		result.NextBeforeReceivedAt = &receivedAt
		result.NextBeforeEventID = &eventID
	}

	return result, nil
}
