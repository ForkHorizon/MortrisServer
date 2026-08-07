package httpapi

// The house-view read endpoints: the house wall, one house's blocks and
// metrics, its art, and its drop map. Split out of
// puzzle_gravity_handlers.go, which was at its 300-line ceiling.

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/analytics"
	"github.com/ForkHorizon/Mortris/internal/apierr"
)

func (s *Server) handlePuzzleHouses(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, from, to, scope, err := s.gameplayRangeAndScope(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	result, err := analytics.GetPuzzleHouses(r.Context(), s.ReaderPool, projectID, from, to, scope)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handlePuzzleHouseDetail(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, from, to, scope, err := s.gameplayRangeAndScope(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	cityID, houseID, err := pathCityHouse(r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	result, err := analytics.GetPuzzleHouse(r.Context(), s.ReaderPool, projectID, cityID, houseID, from, to, scope)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

// handlePuzzleWaveFunnel is Stage 4's wave staircase (docs/puzzle-analytics-remaining-plan.md
// section 7).
func (s *Server) handlePuzzleWaveFunnel(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, from, to, scope, err := s.gameplayRangeAndScope(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	cityID, houseID, err := pathCityHouse(r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	result, err := analytics.GetPuzzleWaveFunnel(r.Context(), s.ReaderPool, projectID, cityID, houseID, from, to, scope)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handlePuzzleDrops(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, from, to, scope, err := s.gameplayRangeAndScope(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	cityID, houseID, err := pathCityHouse(r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	var blockID *int
	if raw := r.URL.Query().Get("block_id"); raw != "" {
		parsed, convErr := strconv.Atoi(raw)
		if convErr != nil {
			s.fail(w, r, requestID, start, apierr.New(400, "invalid_request", "block_id must be an integer"))
			return
		}
		blockID = &parsed
	}
	result, err := analytics.GetPuzzleDrops(r.Context(), s.ReaderPool, projectID, cityID, houseID, blockID, from, to, scope)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

// handlePuzzleHouseArt serves the assembled house picture the diagram is
// drawn over. Content-Type comes from a strict allowlist checked at
// import, never from the stored row alone.
func (s *Server) handlePuzzleHouseArt(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	cityID, houseID, err := pathCityHouse(r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	revision := r.URL.Query().Get("revision")
	art, err := analytics.GetPuzzleHouseArt(r.Context(), s.ReaderPool, projectID, revision, cityID, houseID)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if art == nil {
		s.fail(w, r, requestID, start, apierr.New(404, "invalid_request", "no art stored for this house and revision"))
		return
	}
	w.Header().Set("Content-Type", art.MediaType)
	// A revision is immutable, so its art can never change under a client.
	// Private, because this is project-scoped content behind a session.
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(art.Image)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func pathCityHouse(r *http.Request) (int, int, error) {
	cityID, err := strconv.Atoi(r.PathValue("city"))
	if err != nil {
		return 0, 0, apierr.New(400, "invalid_request", "city must be an integer")
	}
	houseID, err := strconv.Atoi(r.PathValue("house"))
	if err != nil {
		return 0, 0, apierr.New(400, "invalid_request", "house must be an integer")
	}
	return cityID, houseID, nil
}

func (s *Server) handlePuzzleAttempts(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, from, to, err := s.gameplayRange(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	cityID, houseID, err := pathCityHouse(r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	result, err := analytics.GetPuzzleAttempts(r.Context(), s.ReaderPool, projectID, cityID, houseID, from, to)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"attempts": result})
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

// handlePuzzleReplay exposes one player's raw step-by-step path through a
// house, so it carries the same project-admin guard as the existing raw
// attempt timeline rather than plain project access.
func (s *Server) handlePuzzleReplay(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
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
	result, err := analytics.GetPuzzleReplay(r.Context(), s.ReaderPool, projectID, r.PathValue("id"))
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

// gameplayRangeAndScope is gameplayRange plus the Stage 3 traffic scope
// (docs/puzzle-analytics-remaining-plan.md section 6) every Puzzle
// aggregate endpoint must consistently accept, defaulting to natural_only
// so a caller that omits it still gets the only scope safe to show as a
// verdict.
func (s *Server) gameplayRangeAndScope(sess *adminauth.Session, r *http.Request) (string, time.Time, time.Time, analytics.TrafficScope, error) {
	projectID, from, to, err := s.gameplayRange(sess, r)
	if err != nil {
		return "", time.Time{}, time.Time{}, "", err
	}
	scope, err := analytics.ParseTrafficScope(r.URL.Query())
	if err != nil {
		return "", time.Time{}, time.Time{}, "", err
	}
	return projectID, from, to, scope, nil
}
