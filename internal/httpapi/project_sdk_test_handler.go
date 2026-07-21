package httpapi

import (
	"net/http"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
	"github.com/ForkHorizon/Mortris/internal/sdktest"
)

type sdkTestControlRequest struct {
	Scenario string `json:"scenario"`
}

func (s *Server) handleSDKTestControl(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	if err := requireOwner(sess); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	var req sdkTestControlRequest
	if err := decodeRequest(w, r, &req); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if req.Scenario != "" && !knownSDKScenario(sdktest.Scenario(req.Scenario)) {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "unknown SDK test scenario"))
		return
	}
	projectID := r.PathValue("id")
	result, err := s.Pool.Exec(r.Context(), `UPDATE projects SET sdk_test_scenario = $2, updated_at = clock_timestamp() WHERE id = $1 AND environment = 'test' AND sdk_test_enabled AND archived_at IS NULL`, projectID, req.Scenario)
	if err != nil || result.RowsAffected() != 1 {
		if err == nil {
			err = apierr.New(400, contracts.CodeInvalidRequest, "active SDK test project not found")
		}
		s.fail(w, r, requestID, start, err)
		return
	}
	_, _ = s.Pool.Exec(r.Context(), `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'sdk_test_scenario_updated', jsonb_build_object('project_id',$2::text,'scenario',$3::text))`, sess.AdminUserID, projectID, req.Scenario)
	writeJSON(w, http.StatusOK, map[string]any{"project_id": projectID, "scenario": req.Scenario})
	s.logRequest(r, requestID, http.StatusOK, start, map[string]any{"project_id": projectID, "scenario": req.Scenario})
}

func knownSDKScenario(scenario sdktest.Scenario) bool {
	switch scenario {
	case sdktest.LostAcknowledgement, sdktest.UnauthorizedOnce, sdktest.PayloadTooLarge, sdktest.RateLimited, sdktest.PolicyActive, sdktest.PolicyPauseUpload, sdktest.PolicyDisable:
		return true
	default:
		return false
	}
}
