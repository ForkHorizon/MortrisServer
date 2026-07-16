package httpapi

import (
	"net/http"
	"time"
)

// logRequest emits one structured line per request (section 13.4): request
// ID, route, status, latency, and any handler-specific fields (e.g. batch
// accepted/duplicate/rejected counts). Never logs headers, bodies, or
// property values — extra is caller-controlled and must stay that way.
func (s *Server) logRequest(r *http.Request, requestID string, status int, start time.Time, extra map[string]any) {
	args := []any{
		"request_id", requestID,
		"method", r.Method,
		"route", r.Pattern,
		"status", status,
		"latency_ms", time.Since(start).Milliseconds(),
	}
	for k, v := range extra {
		args = append(args, k, v)
	}
	s.Log.Info("request", args...)
}
