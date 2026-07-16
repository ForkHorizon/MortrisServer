package httpapi

import (
	"net/http"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/analytics"
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

func (s *Server) handleInstallationTimeline(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	if err := requireAdminRole(sess); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
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

	result, err := analytics.GetCatalog(r.Context(), s.ReaderPool, projectID)
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
