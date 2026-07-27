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

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	from, to, err := analytics.ParseDateRange(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	loc, err := analytics.ParseTimezone(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetOverview(r.Context(), s.ReaderPool, projectID, from, to, loc)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleEventExplorer(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	from, to, err := analytics.ParseDateRange(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	loc, err := analytics.ParseTimezone(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	filter, err := analytics.ParseEventExplorerFilter(r.Context(), s.ReaderPool, projectID, r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetEventExplorer(r.Context(), s.ReaderPool, projectID, from, to, loc, filter)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handlePropertyValues(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	from, to, err := analytics.ParseDateRange(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	filter, err := analytics.ParsePropertyValuesFilter(r.Context(), s.ReaderPool, projectID, r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetPropertyValues(r.Context(), s.ReaderPool, projectID, filter, from, to)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleRecentEvents(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	filter, err := analytics.ParseRecentEventsFilter(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetRecentEvents(r.Context(), s.ReaderPool, projectID, filter)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleFunnel(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	from, to, err := analytics.ParseDateRange(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	steps, err := analytics.ParseFunnelSteps(r.Context(), s.ReaderPool, projectID, r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	window, err := analytics.ParseCompletionWindow(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetFunnel(r.Context(), s.ReaderPool, projectID, steps, from, to, window)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleRetention(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	from, to, err := analytics.ParseDateRange(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	loc, err := analytics.ParseTimezone(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetRetention(r.Context(), s.ReaderPool, projectID, from, to, loc)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

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

func (s *Server) handleInstallationTimeline(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := requireProjectAdmin(sess, projectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetInstallationTimeline(r.Context(), s.ReaderPool, projectID, r.PathValue("id"))
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	from, to, err := analytics.ParseDateRange(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	loc, err := analytics.ParseTimezone(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetCatalog(r.Context(), s.ReaderPool, projectID, from, to, loc)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleSystemHealth(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	result, err := analytics.GetSystemHealth(r.Context(), s.Pool, s.ReaderPool, s.currentDiskState(), sess.ProjectIDs)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}
