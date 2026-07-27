package analytics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
)

// NLQueryParams mirrors the query-string filters already accepted by the
// five read-only analytics endpoints. Claude only ever fills in these
// named, typed fields — it has no path to raw SQL.
type NLQueryParams struct {
	From          *string `json:"from,omitempty"`
	To            *string `json:"to,omitempty"`
	Timezone      *string `json:"timezone,omitempty"`
	Name          *string `json:"name,omitempty"`
	AppVersion    *string `json:"app_version,omitempty"`
	BuildNumber   *string `json:"build_number,omitempty"`
	Platform      *string `json:"platform,omitempty"`
	PropertyKey   *string `json:"property_key,omitempty"`
	PropertyValue *string `json:"property_value,omitempty"`
	InstallID     *string `json:"install_id,omitempty"`
	Limit         *int    `json:"limit,omitempty"`
	Steps         *string `json:"steps,omitempty"`
	WindowSeconds *int    `json:"window_seconds,omitempty"`
}

type NLQueryIntent struct {
	Endpoint string        `json:"endpoint"`
	Params   NLQueryParams `json:"params"`
}

// NLQueryEndpoints allowlists which existing read-only endpoint Claude may
// route to — the same five exposed to the MCP server (Phase 5 #1).
var NLQueryEndpoints = map[string]bool{
	"overview":      true,
	"event_counts":  true,
	"recent_events": true,
	"funnel":        true,
	"retention":     true,
}

var nlQuerySchema = map[string]any{
	"type":     "object",
	"required": []string{"endpoint", "params"},
	"properties": map[string]any{
		"endpoint": map[string]any{
			"type": "string",
			"enum": []string{"overview", "event_counts", "recent_events", "funnel", "retention"},
		},
		"params": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"from":           map[string]any{"type": "string", "description": "RFC3339 start time"},
				"to":             map[string]any{"type": "string", "description": "RFC3339 end time"},
				"timezone":       map[string]any{"type": "string", "description": "IANA timezone"},
				"name":           map[string]any{"type": "string", "description": "one cataloged event name"},
				"app_version":    map[string]any{"type": "string"},
				"build_number":   map[string]any{"type": "string"},
				"platform":       map[string]any{"type": "string"},
				"property_key":   map[string]any{"type": "string"},
				"property_value": map[string]any{"type": "string"},
				"install_id":     map[string]any{"type": "string"},
				"limit":          map[string]any{"type": "integer", "description": "recent_events only, max 500"},
				"steps":          map[string]any{"type": "string", "description": "funnel only: 2-5 comma-separated event names, in order"},
				"window_seconds": map[string]any{"type": "integer", "description": "funnel only"},
			},
			"additionalProperties": false,
		},
	},
	"additionalProperties": false,
}

// InterpretQuery turns a free-text question into a structured intent:
// which read-only analytics endpoint to call and which of its existing,
// allowlisted filters to set. This is the only thing Claude decides —
// the caller (httpapi.handleNLQuery) runs the resulting params through
// the exact same Parse*/Get* functions the dashboard's manual filter
// fields use, so a bad or ungrounded guess just gets today's ordinary
// validation error instead of behaving specially (Phase 5 #4: "never raw
// SQL, safe by construction").
func InterpretQuery(ctx context.Context, client *anthropic.Client, question string) (*NLQueryIntent, error) {
	if client == nil {
		return nil, ErrAIUnavailable
	}

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     "claude-opus-5",
		MaxTokens: 1024,
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortLow,
			Format: anthropic.JSONOutputFormatParam{Schema: nlQuerySchema},
		},
		System: []anthropic.TextBlockParam{{
			Text: "You translate an analytics question into exactly one call against a fixed set of " +
				"read-only endpoints: overview (project-wide totals and daily trend), event_counts " +
				"(per-event counts and trend, optionally filtered by name/app_version/build_number/" +
				"platform/one property), recent_events (live tail of raw events, optionally filtered), " +
				"funnel (2-5 step conversion over cataloged product events), retention (D1/D7/D30 " +
				"cohorts). Only set params that endpoint actually accepts — steps and window_seconds are " +
				"funnel-only, limit is recent_events-only. Dates are RFC3339; if the question doesn't " +
				"name a range, omit from/to entirely and the default (trailing 7 days) applies. If the " +
				"question doesn't map cleanly to one of these endpoints, pick the closest fit.",
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(question)),
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return nil, fmt.Errorf("query interpretation declined")
	}

	var text string
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text += t.Text
		}
	}

	var intent NLQueryIntent
	if err := json.Unmarshal([]byte(text), &intent); err != nil {
		return nil, fmt.Errorf("could not parse model output: %w", err)
	}
	if !NLQueryEndpoints[intent.Endpoint] {
		return nil, fmt.Errorf("model selected an unknown endpoint: %q", intent.Endpoint)
	}
	return &intent, nil
}
