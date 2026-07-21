package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

type projectMember struct {
	ID         int64  `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email,omitempty"`
	AccessRole string `json:"access_role"`
	Disabled   bool   `json:"disabled"`
}

type memberCreateRequest struct {
	AccountID  *int64 `json:"account_id"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Email      string `json:"email"`
	AccessRole string `json:"access_role"`
}

type memberUpdateRequest struct {
	AccessRole *string `json:"access_role"`
	Password   *string `json:"password"`
}

func (s *Server) handleProjectMembersList(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID := r.PathValue("id")
	if err := requireProjectAdmin(sess, projectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	members, err := s.listProjectMembers(r.Context(), projectID)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
	s.logRequest(r, requestID, http.StatusOK, start, map[string]any{"project_id": projectID})
}

func (s *Server) listProjectMembers(ctx context.Context, projectID string) ([]projectMember, error) {
	rows, err := s.ReaderPool.Query(ctx, `
		SELECT u.id, COALESCE(u.username, u.email, ''), COALESCE(u.email, ''), up.access_role, u.disabled
		FROM admin_user_projects up JOIN admin_users u ON u.id = up.admin_user_id
		WHERE up.project_id = $1 ORDER BY lower(COALESCE(u.username, u.email, '')), u.id
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []projectMember{}
	for rows.Next() {
		var member projectMember
		if err := rows.Scan(&member.ID, &member.Username, &member.Email, &member.AccessRole, &member.Disabled); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (s *Server) handleProjectMemberCreate(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID := r.PathValue("id")
	if err := requireProjectAdmin(sess, projectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	var req memberCreateRequest
	if err := decodeRequest(w, r, &req); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if req.AccessRole == "" {
		req.AccessRole = adminauth.ViewerRole
	}
	accountID, err := s.createProjectMember(r.Context(), sess, projectID, req)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"account_id": accountID, "project_id": projectID, "access_role": req.AccessRole})
	s.logRequest(r, requestID, http.StatusCreated, start, map[string]any{"project_id": projectID, "account_id": accountID})
}

func (s *Server) createProjectMember(ctx context.Context, sess *adminauth.Session, projectID string, req memberCreateRequest) (int64, error) {
	if req.AccessRole == "" {
		req.AccessRole = adminauth.ViewerRole
	}
	if req.AccessRole != adminauth.ViewerRole && req.AccessRole != adminauth.ProjectAdminRole {
		return 0, apierr.New(400, contracts.CodeInvalidRequest, "access_role must be project_admin or viewer")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	accountID, err := projectMemberAccountID(ctx, tx, sess, req)
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO admin_user_projects (admin_user_id, project_id, access_role) VALUES ($1,$2,$3) ON CONFLICT (admin_user_id, project_id) DO UPDATE SET access_role = EXCLUDED.access_role`, accountID, projectID, req.AccessRole)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'project_member_granted', jsonb_build_object('project_id',$2::text,'account_id',$3::text,'access_role',$4::text))`, sess.AdminUserID, projectID, accountID, req.AccessRole)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	return accountID, err
}

func projectMemberAccountID(ctx context.Context, tx pgx.Tx, sess *adminauth.Session, req memberCreateRequest) (int64, error) {
	if req.AccountID == nil {
		return createNamedAccount(ctx, tx, req.Username, req.Password, req.Email)
	}
	if !sess.IsOwner() {
		return 0, apierr.New(403, adminauth.CodeForbiddenRole, "only the owner may assign an existing account")
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM admin_users WHERE id = $1)`, *req.AccountID).Scan(&exists); err != nil {
		return 0, err
	}
	if !exists {
		return 0, apierr.New(400, contracts.CodeInvalidRequest, "account not found")
	}
	return *req.AccountID, nil
}

func (s *Server) handleProjectMemberDelete(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID := r.PathValue("id")
	if err := requireProjectAdmin(sess, projectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	accountID, err := strconv.ParseInt(r.PathValue("accountID"), 10, 64)
	if err != nil {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "invalid account id"))
		return
	}
	result, err := s.Pool.Exec(r.Context(), `DELETE FROM admin_user_projects WHERE project_id = $1 AND admin_user_id = $2`, projectID, accountID)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if result.RowsAffected() == 0 {
		s.fail(w, r, requestID, start, apierr.New(404, contracts.CodeInvalidRequest, "project membership not found"))
		return
	}
	_, _ = s.Pool.Exec(r.Context(), `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'project_member_revoked', jsonb_build_object('project_id',$2::text,'account_id',$3::text))`, sess.AdminUserID, projectID, accountID)
	w.WriteHeader(http.StatusNoContent)
	s.logRequest(r, requestID, http.StatusNoContent, start, map[string]any{"project_id": projectID, "account_id": accountID})
}

func (s *Server) handleProjectMemberUpdate(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID := r.PathValue("id")
	if err := requireProjectAdmin(sess, projectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	accountID, err := strconv.ParseInt(r.PathValue("accountID"), 10, 64)
	if err != nil {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "invalid account id"))
		return
	}
	var req memberUpdateRequest
	if err := decodeRequest(w, r, &req); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := s.updateProjectMember(r.Context(), sess, projectID, accountID, req); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.logRequest(r, requestID, http.StatusNoContent, start, map[string]any{"project_id": projectID, "account_id": accountID})
}

func (s *Server) updateProjectMember(ctx context.Context, sess *adminauth.Session, projectID string, accountID int64, req memberUpdateRequest) error {
	if err := validateMemberUpdate(req); err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = canChangeProjectMember(ctx, tx, sess, projectID, accountID); err == nil {
		err = applyMemberUpdate(ctx, tx, projectID, accountID, req)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'project_member_updated', jsonb_build_object('project_id',$2::text,'account_id',$3::text))`, sess.AdminUserID, projectID, accountID)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	return err
}

func validateMemberUpdate(req memberUpdateRequest) error {
	if req.AccessRole == nil && req.Password == nil {
		return apierr.New(400, contracts.CodeInvalidRequest, "access_role or password is required")
	}
	if req.AccessRole != nil && *req.AccessRole != adminauth.ViewerRole && *req.AccessRole != adminauth.ProjectAdminRole {
		return apierr.New(400, contracts.CodeInvalidRequest, "access_role must be project_admin or viewer")
	}
	if req.Password != nil && len(*req.Password) < 8 {
		return apierr.New(400, contracts.CodeInvalidRequest, "password must be at least 8 characters")
	}
	return nil
}

func canChangeProjectMember(ctx context.Context, tx pgx.Tx, sess *adminauth.Session, projectID string, accountID int64) error {
	var globalRole string
	var membershipCount int
	err := tx.QueryRow(ctx, `
		SELECT u.role, COUNT(all_memberships.project_id)
		FROM admin_users u
		JOIN admin_user_projects current_membership ON current_membership.admin_user_id = u.id AND current_membership.project_id = $2
		LEFT JOIN admin_user_projects all_memberships ON all_memberships.admin_user_id = u.id
		WHERE u.id = $1 GROUP BY u.role
	`, accountID, projectID).Scan(&globalRole, &membershipCount)
	if err == pgx.ErrNoRows {
		return apierr.New(404, contracts.CodeInvalidRequest, "project membership not found")
	}
	if err != nil {
		return err
	}
	if !sess.IsOwner() && (globalRole != adminauth.RoleMember || membershipCount != 1) {
		return apierr.New(403, adminauth.CodeForbiddenRole, "a project admin may only reset a login exclusive to this project")
	}
	return nil
}

func applyMemberUpdate(ctx context.Context, tx pgx.Tx, projectID string, accountID int64, req memberUpdateRequest) error {
	if req.AccessRole != nil {
		if _, err := tx.Exec(ctx, `UPDATE admin_user_projects SET access_role = $3 WHERE project_id = $1 AND admin_user_id = $2`, projectID, accountID, *req.AccessRole); err != nil {
			return err
		}
	}
	if req.Password == nil {
		return nil
	}
	hash, err := adminauth.HashPassword(*req.Password)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE admin_users SET password_hash = $2 WHERE id = $1`, accountID, hash); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE admin_sessions SET revoked_at = clock_timestamp() WHERE admin_user_id = $1 AND revoked_at IS NULL`, accountID)
	return err
}
