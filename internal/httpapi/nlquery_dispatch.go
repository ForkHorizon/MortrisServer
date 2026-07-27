package httpapi

import (
	"context"
	"net/url"

	"github.com/ForkHorizon/Mortris/internal/analytics"
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

// dispatchNLQuery runs the model-selected endpoint+params through the
// exact same validation and query functions as the corresponding manual
// handler in analytics_handlers.go — same catalog allowlisting, same
// date-range cap, same everything.
func (s *Server) dispatchNLQuery(ctx context.Context, endpoint, projectID string, values url.Values) (any, error) {
	switch endpoint {
	case "overview":
		return s.nlQueryOverview(ctx, projectID, values)
	case "event_counts":
		return s.nlQueryEventCounts(ctx, projectID, values)
	case "recent_events":
		return s.nlQueryRecentEvents(ctx, projectID, values)
	case "funnel":
		return s.nlQueryFunnel(ctx, projectID, values)
	case "retention":
		return s.nlQueryRetention(ctx, projectID, values)
	default:
		// Unreachable: analytics.InterpretQuery already allowlists endpoint
		// names against this same set before returning.
		return nil, apierr.New(400, contracts.CodeInvalidRequest, "unknown endpoint: "+endpoint)
	}
}

func (s *Server) nlQueryOverview(ctx context.Context, projectID string, values url.Values) (any, error) {
	from, to, err := analytics.ParseDateRange(values)
	if err != nil {
		return nil, err
	}
	loc, err := analytics.ParseTimezone(values)
	if err != nil {
		return nil, err
	}
	return analytics.GetOverview(ctx, s.ReaderPool, projectID, from, to, loc)
}

func (s *Server) nlQueryEventCounts(ctx context.Context, projectID string, values url.Values) (any, error) {
	from, to, err := analytics.ParseDateRange(values)
	if err != nil {
		return nil, err
	}
	loc, err := analytics.ParseTimezone(values)
	if err != nil {
		return nil, err
	}
	filter, err := analytics.ParseEventExplorerFilter(ctx, s.ReaderPool, projectID, values)
	if err != nil {
		return nil, err
	}
	return analytics.GetEventExplorer(ctx, s.ReaderPool, projectID, from, to, loc, filter)
}

func (s *Server) nlQueryRecentEvents(ctx context.Context, projectID string, values url.Values) (any, error) {
	filter, err := analytics.ParseRecentEventsFilter(values)
	if err != nil {
		return nil, err
	}
	return analytics.GetRecentEvents(ctx, s.ReaderPool, projectID, filter)
}

func (s *Server) nlQueryFunnel(ctx context.Context, projectID string, values url.Values) (any, error) {
	from, to, err := analytics.ParseDateRange(values)
	if err != nil {
		return nil, err
	}
	steps, err := analytics.ParseFunnelSteps(ctx, s.ReaderPool, projectID, values)
	if err != nil {
		return nil, err
	}
	window, err := analytics.ParseCompletionWindow(values)
	if err != nil {
		return nil, err
	}
	return analytics.GetFunnel(ctx, s.ReaderPool, projectID, steps, from, to, window)
}

func (s *Server) nlQueryRetention(ctx context.Context, projectID string, values url.Values) (any, error) {
	from, to, err := analytics.ParseDateRange(values)
	if err != nil {
		return nil, err
	}
	loc, err := analytics.ParseTimezone(values)
	if err != nil {
		return nil, err
	}
	return analytics.GetRetention(ctx, s.ReaderPool, projectID, from, to, loc)
}
