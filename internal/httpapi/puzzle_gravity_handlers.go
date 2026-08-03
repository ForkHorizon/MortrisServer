package httpapi

import (
	"net/http"
	"strconv"
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

// maxPuzzleGeometryBody lives here rather than in server.go's limit block
// because that file is at its 300-line ceiling, and this limit has exactly
// one consumer.
//
// Geometry is bulkier than the catalogue it describes — every block carries
// a silhouette, and the full 103-house export is around 4 MB. This ceiling
// stays below the production nginx client_max_body_size of 10m
// (deploy/nginx/mortris.conf), which would otherwise reject the upload
// before it ever reached this limit. Geometry is additive per block, so an
// export that outgrows this can be split across several uploads with no
// special handling.
const maxPuzzleGeometryBody = 8 * 1024 * 1024

// handlePuzzleGeometryUpload attaches block shapes to a revision that has
// already been imported. Same guard as the catalogue import — this writes
// content, so it is project-admin plus CSRF, never merely project-scoped.
func (s *Server) handlePuzzleGeometryUpload(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
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
	var geometry analytics.PuzzleGeometry
	if err := decodeRequestWithLimits(w, r, &geometry, maxPuzzleGeometryBody, maxPuzzleGeometryBody); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	// The revision in the path is authoritative: a payload claiming a
	// different one is a mismatched export, and silently trusting the body
	// would attach one revision's shapes to another's blocks.
	if revision := r.PathValue("revision"); revision != geometry.ContentRevision {
		s.fail(w, r, requestID, start, apierr.New(400, "invalid_request", "content_revision in the body does not match the path"))
		return
	}
	updated, err := analytics.ApplyPuzzleGeometry(r.Context(), s.Pool, projectID, geometry)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"blocks_updated": updated})
	s.logRequest(r, requestID, http.StatusOK, start, nil)
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

func (s *Server) handlePuzzleHouses(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, from, to, err := s.gameplayRange(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	result, err := analytics.GetPuzzleHouses(r.Context(), s.ReaderPool, projectID, from, to)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handlePuzzleHouseDetail(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, from, to, err := s.gameplayRange(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	cityID, err := strconv.Atoi(r.PathValue("city"))
	if err != nil {
		s.fail(w, r, requestID, start, apierr.New(400, "invalid_request", "city must be an integer"))
		return
	}
	houseID, err := strconv.Atoi(r.PathValue("house"))
	if err != nil {
		s.fail(w, r, requestID, start, apierr.New(400, "invalid_request", "house must be an integer"))
		return
	}
	result, err := analytics.GetPuzzleHouse(r.Context(), s.ReaderPool, projectID, cityID, houseID, from, to)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

// gameplayRange resolves the project and date range both house endpoints
// need, so neither can accidentally skip the project-access check.
func (s *Server) gameplayRange(sess *adminauth.Session, r *http.Request) (string, time.Time, time.Time, error) {
	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	from, to, err := analytics.ParseDateRange(r.URL.Query())
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	return projectID, from, to, nil
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
