package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
	"github.com/ForkHorizon/Mortris/internal/policyadmin"
)

// handlePolicyList/Create/Delete implement the kill-switch administration
// API (Phase S3). List is a read (reader pool); Create/Delete mutate
// client_policy_rules and are admin-role-only, CSRF-checked, and go
// through the writer pool (section 8.1: policy administration is a
// service write, not a dashboard analytics query).

func (s *Server) handlePolicyList(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	rules, err := policyadmin.List(r.Context(), s.ReaderPool, projectID)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

type policyCreateRequest struct {
	ProjectID        string  `json:"project_id"`
	Environment      *string `json:"environment"`
	AppVersion       *string `json:"app_version"`
	BuildNumber      *string `json:"build_number"`
	SDKVersion       *string `json:"sdk_version"`
	Mode             string  `json:"mode"`
	NextCheckSeconds int     `json:"next_check_seconds"`
	DiscardPending   bool    `json:"discard_pending"`
	Reason           string  `json:"reason"`
}

func (s *Server) handlePolicyCreate(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	data, err := readBody(w, r)
	if err != nil {
		s.fail(w, r, requestID, start, badRequest(err))
		return
	}
	var req policyCreateRequest
	if err := decodeJSONStrict(data, &req); err != nil {
		s.fail(w, r, requestID, start, decodeErr(err))
		return
	}
	if err := requireProjectAdmin(sess, req.ProjectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	rule, err := policyadmin.Create(r.Context(), s.Pool, sess.AdminUserID, policyadmin.CreateInput{
		ProjectID:        req.ProjectID,
		Environment:      req.Environment,
		AppVersion:       req.AppVersion,
		BuildNumber:      req.BuildNumber,
		SDKVersion:       req.SDKVersion,
		Mode:             req.Mode,
		NextCheckSeconds: req.NextCheckSeconds,
		DiscardPending:   req.DiscardPending,
		Reason:           req.Reason,
	})
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	writeJSON(w, http.StatusCreated, rule)
	s.logRequest(r, requestID, http.StatusCreated, start, nil)
}

func (s *Server) handlePolicyDelete(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID := newRequestID()
	start := time.Now()

	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	projectID, err := requireProjectAccess(sess, r)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := requireProjectAdmin(sess, projectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	ruleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "invalid rule id"))
		return
	}

	if err := policyadmin.Delete(r.Context(), s.Pool, sess.AdminUserID, projectID, ruleID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	s.logRequest(r, requestID, http.StatusNoContent, start, nil)
}
