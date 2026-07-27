package analytics

import (
	"context"
	"errors"
	"testing"
)

func TestInterpretQueryRequiresClient(t *testing.T) {
	_, err := InterpretQuery(context.Background(), nil, "how many level_end events this week?")
	if !errors.Is(err, ErrAIUnavailable) {
		t.Fatalf("expected ErrAIUnavailable, got %v", err)
	}
}

func TestNLQueryEndpointsAllowlist(t *testing.T) {
	for _, name := range []string{"overview", "event_counts", "recent_events", "funnel", "retention"} {
		if !NLQueryEndpoints[name] {
			t.Errorf("expected %q to be allowlisted", name)
		}
	}
	if NLQueryEndpoints["catalog"] || NLQueryEndpoints["drop_table"] || NLQueryEndpoints[""] {
		t.Error("allowlist must reject anything not one of the five read-only endpoints")
	}
}
