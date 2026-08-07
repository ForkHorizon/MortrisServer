package httpapi

import "net/http"

func (s *Server) registerAnalyticsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/analytics/overview", s.requireSession(s.handleOverview))
	mux.HandleFunc("GET /api/v1/analytics/events", s.requireSession(s.handleEventExplorer))
	mux.HandleFunc("GET /api/v1/analytics/events/property-values", s.requireSession(s.handlePropertyValues))
	mux.HandleFunc("GET /api/v1/analytics/events/recent", s.requireSession(s.handleRecentEvents))
	mux.HandleFunc("GET /api/v1/analytics/funnel", s.requireSession(s.handleFunnel))
	mux.HandleFunc("GET /api/v1/analytics/retention", s.requireSession(s.handleRetention))
	mux.HandleFunc("GET /api/v1/analytics/anomalies", s.requireSession(s.handleAnomalies))
	mux.HandleFunc("GET /api/v1/analytics/digest", s.requireSession(s.handleDigest))
	mux.HandleFunc("POST /api/v1/analytics/query", s.requireSession(s.handleNLQuery))
	mux.HandleFunc("GET /api/v1/analytics/installations/{id}", s.requireSession(s.handleInstallationTimeline))
	mux.HandleFunc("GET /api/v1/analytics/catalog", s.requireSession(s.handleCatalog))
	mux.HandleFunc("GET /api/v1/analytics/dimensions", s.requireSession(s.handleDimensions))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/diagnostics", s.requireSession(s.handleGameplayDiagnostics))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/players", s.requireSession(s.handleGameplayPlayers))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/overview", s.requireSession(s.handlePuzzleOverview))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/quality", s.requireSession(s.handlePuzzleQuality))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/tester-impact", s.requireSession(s.handlePuzzleTesterImpact))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/attempts/{id}", s.requireSession(s.handleGameplayAttempt))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/houses", s.requireSession(s.handlePuzzleHouses))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/houses/{city}/{house}", s.requireSession(s.handlePuzzleHouseDetail))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/houses/{city}/{house}/art", s.requireSession(s.handlePuzzleHouseArt))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/houses/{city}/{house}/drops", s.requireSession(s.handlePuzzleDrops))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/houses/{city}/{house}/funnel", s.requireSession(s.handlePuzzleWaveFunnel))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/houses/{city}/{house}/attempts", s.requireSession(s.handlePuzzleAttempts))
	mux.HandleFunc("GET /api/v1/analytics/gameplay/attempts/{id}/replay", s.requireSession(s.handlePuzzleReplay))
}
