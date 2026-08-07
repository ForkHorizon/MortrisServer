package httpapi

import (
	"net/http"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/analytics"
)

// handlePuzzleTesterImpact is Stage 3's "compact tester-impact summary"
// (docs/puzzle-analytics-remaining-plan.md section 6): what developer
// traffic did in this range, shown next to the natural verdict rather
// than blended into it.
func (s *Server) handlePuzzleTesterImpact(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, from, to, err := s.gameplayRange(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	result, err := analytics.GetPuzzleTesterImpact(r.Context(), s.ReaderPool, projectID, from, to)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}
