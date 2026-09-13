package httpapi

import (
	"net/http"
	"regexp"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/analytics"
	"github.com/ForkHorizon/Mortris/internal/apierr"
)

var (
	validUUIDPattern  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	validBuildPattern = regexp.MustCompile(`^[0-9a-zA-Z._-]{1,64}$`)
)

func validateBuildNumberParam(build string) (*string, error) {
	if build == "" {
		return nil, nil
	}
	if !validBuildPattern.MatchString(build) {
		return nil, apierr.New(400, "invalid_request", "invalid build_number format")
	}
	return &build, nil
}

func validateInstallIDParam(id string) error {
	if !validUUIDPattern.MatchString(id) {
		return apierr.New(400, "invalid_request", "invalid install_id UUID format")
	}
	return nil
}

func (s *Server) handlePuzzleDevices(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	from, to, err := analytics.ParseDateRange(r.URL.Query())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	buildNumber, err := validateBuildNumberParam(r.URL.Query().Get("build_number"))
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetPuzzleDevices(r.Context(), s.ReaderPool, projectID, from, to, buildNumber)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handlePuzzleDeviceMemoryTimeline(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	installID := r.PathValue("id")
	if installID == "" {
		installID = r.URL.Query().Get("install_id")
	}
	if err := validateInstallIDParam(installID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	result, err := analytics.GetPuzzleMemoryTimeline(r.Context(), s.ReaderPool, projectID, installID)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}
