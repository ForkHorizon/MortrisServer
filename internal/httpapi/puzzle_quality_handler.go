package httpapi

import (
	"net/http"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/analytics"
)

// handlePuzzleQuality is the Stage 2 data-quality control plane (docs/puzzle-analytics-remaining-plan.md
// section 5): whether the selected project/date range/build is safe to
// draw a house-difficulty conclusion from.
func (s *Server) handlePuzzleQuality(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, from, to, err := s.gameplayRange(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	var build *string
	if b := r.URL.Query().Get("build"); b != "" {
		build = &b
	}
	result, err := analytics.GetPuzzleQuality(r.Context(), s.ReaderPool, projectID, from, to, build)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}
