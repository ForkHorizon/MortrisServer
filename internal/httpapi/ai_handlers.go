package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/analytics"
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

// handleAnomalies and handleDigest are the Phase 5 AI-layer analytics
// handlers — split out of analytics_handlers.go so that file stays
// focused on the plain, always-available dashboard queries.

func (s *Server) handleAnomalies(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	loc, err := analytics.ParseTimezone(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetAnomalies(r.Context(), s.ReaderPool, projectID, loc)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleDigest(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	loc, err := analytics.ParseTimezone(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetDigest(r.Context(), s.ReaderPool, s.Anthropic, projectID, loc)
	if err != nil {
		if errors.Is(err, analytics.ErrAIUnavailable) {
			s.fail(w, r, requestID, start, apierr.New(503, contracts.CodeAIUnavailable, err.Error()))
			return
		}
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}
