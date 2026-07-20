package httpapi

import (
	"net/http"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

// requireSession validates the session cookie and passes the resolved
// *adminauth.Session to next; on failure it renders the error itself and
// next is never called.
func (s *Server) requireSession(next func(w http.ResponseWriter, r *http.Request, sess *adminauth.Session)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		start := time.Now()

		cookie, err := r.Cookie(adminauth.SessionCookieName)
		if err != nil || cookie.Value == "" {
			status := writeError(w, s.Log, requestID, apierr.New(401, adminauth.CodeSessionInvalid, "missing session cookie"))
			s.logRequest(r, requestID, status, start, nil)
			return
		}

		sess, err := adminauth.ValidateSession(r.Context(), s.Pool, cookie.Value)
		if err != nil {
			status := writeError(w, s.Log, requestID, err)
			s.logRequest(r, requestID, status, start, nil)
			return
		}

		next(w, r, sess)
	}
}

// requireProjectAccess reads the required "project" query parameter and
// checks the session is scoped to it (section 10.3).
func requireProjectAccess(sess *adminauth.Session, r *http.Request) (string, error) {
	projectID := r.URL.Query().Get("project")
	if projectID == "" {
		return "", apierr.New(400, contracts.CodeInvalidRequest, "project query parameter is required")
	}
	if !sess.HasProjectAccess(projectID) {
		return "", apierr.New(403, adminauth.CodeForbiddenProject, "not scoped to this project")
	}
	return projectID, nil
}

// requireAdminRole gates the installation timeline (section 10.2 #5:
// "admin-only") and policy administration (kill-switch) endpoints —
// viewer sessions can read analytics but never see raw per-installation
// event history or change collection policy.
func requireProjectAdmin(sess *adminauth.Session, projectID string) error {
	if !sess.CanManageProject(projectID) {
		return apierr.New(403, adminauth.CodeForbiddenRole, "project administrator role required")
	}
	return nil
}

func requireOwner(sess *adminauth.Session) error {
	if !sess.IsOwner() {
		return apierr.New(403, adminauth.CodeForbiddenRole, "owner role required")
	}
	return nil
}
