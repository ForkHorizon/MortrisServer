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
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}
	from, to, err := analytics.ParseDateRange(r.URL.Query())
	if err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}
	loc, err := analytics.ParseTimezone(r.URL.Query())
	if err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	result, err := analytics.GetOverview(r.Context(), s.ReaderPool, projectID, from, to, loc)
	if err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
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
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}
	from, to, err := analytics.ParseDateRange(r.URL.Query())
	if err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}
	loc, err := analytics.ParseTimezone(r.URL.Query())
	if err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}
	filter, err := analytics.ParseEventExplorerFilter(r.Context(), s.ReaderPool, projectID, r.URL.Query())
	if err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	result, err := analytics.GetEventExplorer(r.Context(), s.ReaderPool, projectID, from, to, loc, filter)
	if err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}
