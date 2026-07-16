package analytics

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

const (
	minFunnelSteps         = 2
	maxFunnelSteps         = 5
	defaultFunnelWindow    = time.Hour
	maxFunnelWindow        = 24 * time.Hour
	maxFunnelEventsFetched = 500_000 // safety valve, not a correctness boundary — see Truncated below
)

// ParseFunnelSteps validates "steps" (comma-separated, 2-5, no
// duplicates) against the project's product event catalog — funnel
// steps are "product event names" (section 9), never system events.
func ParseFunnelSteps(ctx context.Context, pool *pgxpool.Pool, projectID string, q url.Values) ([]string, error) {
	raw := q.Get("steps")
	if raw == "" {
		return nil, apierr.New(400, contracts.CodeInvalidRequest, "steps is required")
	}
	steps := strings.Split(raw, ",")
	seen := make(map[string]bool, len(steps))
	for i := range steps {
		steps[i] = strings.TrimSpace(steps[i])
		if seen[steps[i]] {
			return nil, apierr.New(400, contracts.CodeInvalidRequest, "duplicate step name: "+steps[i])
		}
		seen[steps[i]] = true
	}
	if len(steps) < minFunnelSteps || len(steps) > maxFunnelSteps {
		return nil, apierr.New(400, contracts.CodeInvalidRequest, "steps must have 2 to 5 items")
	}
	for _, name := range steps {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_catalog WHERE project_id = $1 AND name = $2 AND kind = 'product')`, projectID, name).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, apierr.New(400, contracts.CodeInvalidRequest, "unknown product event name: "+name)
		}
	}
	return steps, nil
}

// ParseCompletionWindow reads "window_seconds", defaulting to 1 hour and
// capping at 24 hours — the plan leaves the default unspecified, so this
// is a documented choice, not a spec value.
func ParseCompletionWindow(q url.Values) (time.Duration, error) {
	raw := q.Get("window_seconds")
	if raw == "" {
		return defaultFunnelWindow, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, apierr.New(400, contracts.CodeInvalidRequest, "window_seconds must be a positive integer")
	}
	window := time.Duration(n) * time.Second
	if window > maxFunnelWindow {
		return 0, apierr.New(400, contracts.CodeInvalidRequest, "window_seconds cannot exceed 86400 (24h)")
	}
	return window, nil
}

type FunnelStep struct {
	Name                   string  `json:"name"`
	Count                  int64   `json:"count"`
	ConversionFromFirst    float64 `json:"conversion_from_first"`
	ConversionFromPrevious float64 `json:"conversion_from_previous"`
}

type FunnelResult struct {
	Steps                   []FunnelStep `json:"steps"`
	CompletionWindowSeconds int64        `json:"completion_window_seconds"`
	Truncated               bool         `json:"truncated"`
}

type funnelEvent struct {
	InstallID   string
	Name        string
	EffectiveAt time.Time
}

// GetFunnel implements section 10.2 #3 / section 9's funnel definition:
// ordered 2-5 product-event steps, each step's event required within
// window of the first step's event, using the first qualifying
// occurrence for a repeated step (the rule chosen and documented in
// docs/metrics.md).
func GetFunnel(ctx context.Context, pool *pgxpool.Pool, projectID string, steps []string, from, to time.Time, window time.Duration) (*FunnelResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	rows, err := pool.Query(ctx, `
		SELECT install_id, name, effective_at
		FROM events
		WHERE project_id = $1 AND event_kind = 'product' AND name = ANY($2)
		  AND effective_at >= $3 AND effective_at <= $4::timestamptz + make_interval(secs => $5)
		ORDER BY install_id, effective_at
		LIMIT $6
	`, projectID, steps, from, to, window.Seconds(), maxFunnelEventsFetched+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []funnelEvent
	for rows.Next() {
		var e funnelEvent
		if err := rows.Scan(&e.InstallID, &e.Name, &e.EffectiveAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	truncated := len(events) > maxFunnelEventsFetched
	if truncated {
		events = events[:maxFunnelEventsFetched]
	}

	counts := make([]int64, len(steps))
	stepIndex := map[string]int{}
	for i, s := range steps {
		stepIndex[s] = i
	}

	i := 0
	for i < len(events) {
		install := events[i].InstallID
		wantStep := 0
		var firstAt time.Time
		for i < len(events) && events[i].InstallID == install {
			e := events[i]
			if idx, ok := stepIndex[e.Name]; ok && idx == wantStep {
				if wantStep == 0 {
					if !e.EffectiveAt.Before(from) && e.EffectiveAt.Before(to) {
						firstAt = e.EffectiveAt
						counts[0]++
						wantStep = 1
					}
				} else if !e.EffectiveAt.After(firstAt.Add(window)) {
					counts[wantStep]++
					wantStep++
				}
			}
			i++
		}
	}

	result := &FunnelResult{CompletionWindowSeconds: int64(window.Seconds()), Truncated: truncated}
	for i, name := range steps {
		fs := FunnelStep{Name: name, Count: counts[i]}
		if counts[0] > 0 {
			fs.ConversionFromFirst = float64(counts[i]) / float64(counts[0])
		}
		if i > 0 && counts[i-1] > 0 {
			fs.ConversionFromPrevious = float64(counts[i]) / float64(counts[i-1])
		} else if i == 0 {
			fs.ConversionFromPrevious = 1
		}
		result.Steps = append(result.Steps, fs)
	}
	return result, nil
}
