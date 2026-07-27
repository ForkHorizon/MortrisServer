package httpapi

import (
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
// existing named filter fields — dispatchNLQuery (nlquery_dispatch.go)
// runs those through the identical Parse*/Get* pipeline every manual
// dashboard filter uses, so nothing Claude produces ever reaches SQL
// directly.
func (s *Server) handleNLQuery(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	req, err := decodeNLQueryRequest(w, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := validateNLQueryRequest(sess, req); err != nil {
		s.fail(w, r, requestID, start, err)
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

func decodeNLQueryRequest(w http.ResponseWriter, r *http.Request) (nlQueryRequest, error) {
	data, err := readBody(w, r)
	if err != nil {
		return nlQueryRequest{}, badRequest(err)
	}
	var req nlQueryRequest
	if err := decodeJSONStrict(data, &req); err != nil {
		return nlQueryRequest{}, decodeErr(err)
	}
	return req, nil
}

func validateNLQueryRequest(sess *adminauth.Session, req nlQueryRequest) error {
	if req.Question == "" {
		return apierr.New(400, contracts.CodeInvalidRequest, "question is required")
	}
	if req.Project == "" || !sess.HasProjectAccess(req.Project) {
		return apierr.New(403, adminauth.CodeForbiddenProject, "not scoped to this project")
	}
	return nil
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
