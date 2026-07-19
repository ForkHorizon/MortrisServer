package httpapi

import (
	"io/fs"
	"net/http"
	"path"
	"time"

	"github.com/ForkHorizon/Mortris/dashboard"
)

// dashboardFS returns the embedded frontend rooted at dist/ (not
// dashboard/dist/), so file paths match URL paths directly.
func dashboardFS() fs.FS {
	sub, err := fs.Sub(dashboard.DistFS, "dist")
	if err != nil {
		// Only fails if the embed directive itself was wrong — a build-time
		// bug, not a runtime condition to handle gracefully.
		panic(err)
	}
	return sub
}

// handleDashboard serves the embedded React SPA (section 13.1). Any path
// that isn't an actual built asset falls back to index.html so
// client-side routing (react-router) owns everything under it — the same
// pattern as every other embedded-SPA Go server. If the frontend hasn't
// been built yet (dist/ has only its tracked .gitkeep — see
// dashboard/embed.go), this serves a plain explanation instead of a
// confusing 404.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	requestID := newRequestID()
	start := time.Now()

	clean := path.Clean(r.URL.Path)
	if clean != "/" {
		if f, err := s.dashboardFS.Open(clean[1:]); err == nil {
			_ = f.Close()
			s.dashboardFileServer.ServeHTTP(w, r)
			s.logRequest(r, requestID, http.StatusOK, start, nil)
			return
		}
	}

	data, err := fs.ReadFile(s.dashboardFS, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = w.Write([]byte("dashboard frontend not built — run `npm run build` in dashboard/ (or `make build`)\n"))
		s.logRequest(r, requestID, http.StatusNotImplemented, start, nil)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}
