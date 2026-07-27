package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/analytics"
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

type nlQueryRequest struct {
	Question string `json:"question"`
	Project  string `json:"project"`
}

// handleNLQuery (Phase 5 #4) is the only handler that lets an LLM pick
// its own parameters. It's safe by construction: analytics.InterpretQuery
// only ever returns one of the five allowlisted endpoint names plus the
// existing named filter fields — dispatchNLQuery below runs those through
// the identical Parse*/Get* pipeline every manual dashboard filter uses,
// so nothing Claude produces ever reaches SQL directly.
func (s *Server) handleNLQuery(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	data, err := readBody(w, r)
	if err != nil {
		s.fail(w, r, requestID, start, badRequest(err))
		return
	}
	var req nlQueryRequest
	if err := decodeJSONStrict(data, &req); err != nil {
		s.fail(w, r, requestID, start, decodeErr(err))
		return
	}
	if req.Question == "" {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "question is required"))
		return
	}
	if req.Project == "" || !sess.HasProjectAccess(req.Project) {
		s.fail(w, r, requestID, start, apierr.New(403, adminauth.CodeForbiddenProject, "not scoped to this project"))
		return
	}

	intent, err := analytics.InterpretQuery(r.Context(), s.Anthropic, req.Question)
	if err != nil {
		if errors.Is(err, analytics.ErrAIUnavailable) {
			s.fail(w, r, requestID, start, apierr.New(503, contracts.CodeAIUnavailable, err.Error()))
			return
		}
		s.fail(w, r, requestID, start, err)
		return
	}

	values := nlQueryValues(intent.Params)
	result, err := s.dispatchNLQuery(r.Context(), intent.Endpoint, req.Project, values)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"endpoint":           intent.Endpoint,
		"interpreted_params": values,
		"result":             result,
	})
	s.logRequest(r, requestID, http.StatusOK, start, map[string]any{"endpoint": intent.Endpoint})
}

func nlQueryValues(p analytics.NLQueryParams) url.Values {
	v := url.Values{}
	setIf := func(key string, val *string) {
		if val != nil && *val != "" {
			v.Set(key, *val)
		}
	}
	setIf("from", p.From)
	setIf("to", p.To)
	setIf("timezone", p.Timezone)
	setIf("name", p.Name)
	setIf("app_version", p.AppVersion)
	setIf("build_number", p.BuildNumber)
	setIf("platform", p.Platform)
	setIf("property_key", p.PropertyKey)
	setIf("property_value", p.PropertyValue)
	setIf("install_id", p.InstallID)
	setIf("steps", p.Steps)
	if p.Limit != nil {
		v.Set("limit", strconv.Itoa(*p.Limit))
	}
	if p.WindowSeconds != nil {
		v.Set("window_seconds", strconv.Itoa(*p.WindowSeconds))
	}
	return v
}

// dispatchNLQuery runs the model-selected endpoint+params through the
// exact same validation and query functions as the corresponding manual
// handler in analytics_handlers.go — same catalog allowlisting, same
// date-range cap, same everything.
func (s *Server) dispatchNLQuery(ctx context.Context, endpoint, projectID string, values url.Values) (any, error) {
	switch endpoint {
	case "overview":
		from, to, err := analytics.ParseDateRange(values)
		if err != nil {
			return nil, err
		}
		loc, err := analytics.ParseTimezone(values)
		if err != nil {
			return nil, err
		}
		return analytics.GetOverview(ctx, s.ReaderPool, projectID, from, to, loc)

	case "event_counts":
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

	case "recent_events":
		filter, err := analytics.ParseRecentEventsFilter(values)
		if err != nil {
			return nil, err
		}
		return analytics.GetRecentEvents(ctx, s.ReaderPool, projectID, filter)

	case "funnel":
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

	case "retention":
		from, to, err := analytics.ParseDateRange(values)
		if err != nil {
			return nil, err
		}
		loc, err := analytics.ParseTimezone(values)
		if err != nil {
			return nil, err
		}
		return analytics.GetRetention(ctx, s.ReaderPool, projectID, from, to, loc)

	default:
		// Unreachable: analytics.InterpretQuery already allowlists endpoint
		// names against the same set before returning.
		return nil, apierr.New(400, contracts.CodeInvalidRequest, "unknown endpoint: "+endpoint)
	}
}
