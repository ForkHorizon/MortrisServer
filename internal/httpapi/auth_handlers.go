package httpapi

import (
	"net/http"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	requestID := newRequestID()
	start := time.Now()

	data, err := readBody(w, r)
	if err != nil {
		status := writeError(w, s.Log, requestID, badRequest(err))
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	var req loginRequest
	if err := decodeJSONStrict(data, &req); err != nil {
		status := writeError(w, s.Log, requestID, decodeErr(err))
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	result, err := adminauth.Login(r.Context(), s.Pool, s.LoginThrottle, req.Username, req.Password, sourceIP(r))
	if err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	adminauth.SetAuthCookies(w, result.SessionToken, result.CSRFToken, result.ExpiresAt)
	writeSession(w, result.Session, map[string]any{"expires_at": result.ExpiresAt.UTC().Format("2006-01-02T15:04:05.000Z")})
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	if err := adminauth.CheckCSRF(r); err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	cookie, _ := r.Cookie(adminauth.SessionCookieName)
	if err := adminauth.Logout(r.Context(), s.Pool, cookie.Value); err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	adminauth.ClearAuthCookies(w)
	w.WriteHeader(http.StatusNoContent)
	s.logRequest(r, requestID, http.StatusNoContent, start, nil)
}

func (s *Server) handleSessionInfo(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()
	writeSession(w, *sess, nil)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func writeSession(w http.ResponseWriter, sess adminauth.Session, extra map[string]any) {
	result := map[string]any{
		"username":    sess.Username,
		"email":       sess.Email,
		"role":        sess.Role,
		"projects":    sess.Projects,
		"project_ids": sess.ProjectIDs,
	}
	for key, value := range extra {
		result[key] = value
	}
	writeJSON(w, http.StatusOK, result)
}
