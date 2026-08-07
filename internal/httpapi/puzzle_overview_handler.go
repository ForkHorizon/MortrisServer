package httpapi

import (
	"net/http"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/analytics"
)

func (s *Server) handlePuzzleOverview(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, from, to, scope, err := s.gameplayRangeAndScope(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	result, err := analytics.GetPuzzleOverview(r.Context(), s.ReaderPool, projectID, from, to, scope)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}
