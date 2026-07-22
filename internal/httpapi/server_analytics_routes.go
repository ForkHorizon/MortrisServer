package httpapi

import "net/http"

func (s *Server) registerAnalyticsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/analytics/overview", s.requireSession(s.handleOverview))
	mux.HandleFunc("GET /api/v1/analytics/events", s.requireSession(s.handleEventExplorer))
	mux.HandleFunc("GET /api/v1/analytics/funnel", s.requireSession(s.handleFunnel))
	mux.HandleFunc("GET /api/v1/analytics/retention", s.requireSession(s.handleRetention))
	mux.HandleFunc("GET /api/v1/analytics/installations/{id}", s.requireSession(s.handleInstallationTimeline))
	mux.HandleFunc("GET /api/v1/analytics/catalog", s.requireSession(s.handleCatalog))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/diagnostics", s.requireSession(s.handleGameplayDiagnostics))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/players", s.requireSession(s.handleGameplayPlayers))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/attempts/{id}", s.requireSession(s.handleGameplayAttempt))
}
