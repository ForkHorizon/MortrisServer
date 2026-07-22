package httpapi

import (
	"net/http"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/analytics"
	"github.com/ForkHorizon/Mortris/internal/apierr"
)

func (s *Server) handlePuzzleContentImport(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID := r.PathValue("id")
	if !sess.HasProjectAccess(projectID) {
		s.fail(w, r, requestID, start, apierr.New(403, adminauth.CodeForbiddenProject, "not scoped to this project"))
		return
	}
	if err := requireProjectAdmin(sess, projectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	var catalog analytics.PuzzleCatalog
	if err := decodeRequestWithLimits(w, r, &catalog, maxPuzzleCatalogBody, maxPuzzleCatalogBody); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := analytics.ImportPuzzleCatalog(r.Context(), s.Pool, projectID, catalog); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"content_revision": catalog.ContentRevision})
	s.logRequest(r, requestID, http.StatusCreated, start, nil)
}

func (s *Server) handleGameplayDiagnostics(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
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
	filter, err := analytics.ParseGameplayFilter(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	timezone := r.URL.Query().Get("timezone")
	if timezone == "" {
		timezone = "Europe/Madrid"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		s.fail(w, r, requestID, start, apierr.New(400, "invalid_request", "invalid timezone: "+timezone))
		return
	}
	result, err := analytics.GetGameplayDiagnostics(r.Context(), s.ReaderPool, projectID, from, to, loc, filter)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleGameplayAttempt(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := requireProjectAdmin(sess, projectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	result, err := analytics.GetGameplayAttempt(r.Context(), s.ReaderPool, projectID, r.PathValue("id"))
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleGameplayPlayers(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := requireProjectAdmin(sess, projectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	from, to, err := analytics.ParseDateRange(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	result, err := analytics.GetGameplayPlayers(r.Context(), s.ReaderPool, projectID, from, to)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}
