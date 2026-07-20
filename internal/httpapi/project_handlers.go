package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
	"github.com/ForkHorizon/Mortris/internal/sdktest"
)

type managedProject struct {
	ID              string     `json:"id"`
	DisplayName     string     `json:"display_name"`
	Environment     string     `json:"environment"`
	RetentionDays   int        `json:"retention_days"`
	StrictCatalog   bool       `json:"strict_catalog"`
	Enabled         bool       `json:"enabled"`
	ArchivedAt      *time.Time `json:"archived_at,omitempty"`
	SDKTestEnabled  bool       `json:"sdk_test_enabled"`
	SDKTestScenario string     `json:"sdk_test_scenario"`
}

type projectCreateRequest struct {
	DisplayName    string `json:"display_name"`
	Environment    string `json:"environment"`
	RetentionDays  int    `json:"retention_days"`
	StrictCatalog  *bool  `json:"strict_catalog"`
	SDKTestEnabled bool   `json:"sdk_test_enabled"`
}

type projectUpdateRequest struct {
	DisplayName   *string `json:"display_name"`
	Environment   *string `json:"environment"`
	RetentionDays *int    `json:"retention_days"`
	StrictCatalog *bool   `json:"strict_catalog"`
}

func (s *Server) handleProjectsList(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	if err := requireOwner(sess); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	archived := r.URL.Query().Get("archived") == "true"
	rows, err := s.ReaderPool.Query(r.Context(), `
		SELECT id, display_name, environment, retention_days, strict_catalog, enabled, archived_at, sdk_test_enabled, sdk_test_scenario
		FROM projects WHERE (archived_at IS NOT NULL) = $1 ORDER BY display_name, id
	`, archived)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	defer rows.Close()
	projects := []managedProject{}
	for rows.Next() {
		var project managedProject
		if err := rows.Scan(&project.ID, &project.DisplayName, &project.Environment, &project.RetentionDays, &project.StrictCatalog, &project.Enabled, &project.ArchivedAt, &project.SDKTestEnabled, &project.SDKTestScenario); err != nil {
			s.fail(w, r, requestID, start, err)
			return
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleProjectCreate(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	if err := requireOwner(sess); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	var req projectCreateRequest
	if err := decodeRequest(w, r, &req); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	req.DisplayName, req.Environment = strings.TrimSpace(req.DisplayName), strings.TrimSpace(req.Environment)
	if req.DisplayName == "" || req.Environment == "" || req.RetentionDays < 1 || req.RetentionDays > 3650 {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "display_name, environment, and retention_days (1-3650) are required"))
		return
	}
	if req.SDKTestEnabled && req.Environment != "test" {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "SDK test controls require environment=test"))
		return
	}
	projectID, err := generatedID("project")
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	strictCatalog := true
	if req.StrictCatalog != nil {
		strictCatalog = *req.StrictCatalog
	}
	var sdkToken string
	var sdkHash []byte
	if req.SDKTestEnabled {
		sdkToken, err = generatedID("sdk_test")
		if err != nil {
			s.fail(w, r, requestID, start, err)
			return
		}
		hash := sha256.Sum256([]byte(sdkToken))
		sdkHash = hash[:]
	}
	project := managedProject{ID: projectID, DisplayName: req.DisplayName, Environment: req.Environment, RetentionDays: req.RetentionDays, StrictCatalog: strictCatalog, Enabled: true, SDKTestEnabled: req.SDKTestEnabled}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	_, err = tx.Exec(r.Context(), `
		INSERT INTO projects (id, environment, display_name, retention_days, strict_catalog, sdk_test_enabled, sdk_test_token_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, project.ID, project.Environment, project.DisplayName, project.RetentionDays, project.StrictCatalog, project.SDKTestEnabled, sdkHash)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'project_created', jsonb_build_object('project_id',$2::text,'sdk_test',$3::boolean))`, sess.AdminUserID, project.ID, project.SDKTestEnabled)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	response := map[string]any{"project": project}
	if sdkToken != "" {
		response["sdk_test_token"] = sdkToken // shown once; only its hash is retained.
	}
	writeJSON(w, http.StatusCreated, response)
	s.logRequest(r, requestID, http.StatusCreated, start, map[string]any{"project_id": project.ID})
}

func (s *Server) handleProjectGet(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	projectID := r.PathValue("id")
	if err := requireProjectAdmin(sess, projectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	var project managedProject
	err := s.ReaderPool.QueryRow(r.Context(), `
		SELECT id, display_name, environment, retention_days, strict_catalog, enabled, archived_at, sdk_test_enabled, sdk_test_scenario
		FROM projects WHERE id = $1 AND archived_at IS NULL
	`, projectID).Scan(&project.ID, &project.DisplayName, &project.Environment, &project.RetentionDays, &project.StrictCatalog, &project.Enabled, &project.ArchivedAt, &project.SDKTestEnabled, &project.SDKTestScenario)
	if err == pgx.ErrNoRows {
		err = apierr.New(404, contracts.CodeInvalidRequest, "active project not found")
	}
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": project})
	s.logRequest(r, requestID, http.StatusOK, start, map[string]any{"project_id": projectID})
}

func (s *Server) handleProjectUpdate(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
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
	var req projectUpdateRequest
	if err := decodeRequest(w, r, &req); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if req.DisplayName == nil && req.Environment == nil && req.RetentionDays == nil && req.StrictCatalog == nil {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "at least one setting is required"))
		return
	}
	if req.DisplayName != nil && strings.TrimSpace(*req.DisplayName) == "" || req.Environment != nil && strings.TrimSpace(*req.Environment) == "" || req.RetentionDays != nil && (*req.RetentionDays < 1 || *req.RetentionDays > 3650) {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "invalid project settings"))
		return
	}
	var project managedProject
	err := s.Pool.QueryRow(r.Context(), `
		UPDATE projects SET display_name = COALESCE($2, display_name), environment = COALESCE($3, environment),
		retention_days = COALESCE($4, retention_days), strict_catalog = COALESCE($5, strict_catalog), updated_at = clock_timestamp()
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, display_name, environment, retention_days, strict_catalog, enabled, archived_at, sdk_test_enabled, sdk_test_scenario
	`, projectID, trimmed(req.DisplayName), trimmed(req.Environment), req.RetentionDays, req.StrictCatalog).Scan(&project.ID, &project.DisplayName, &project.Environment, &project.RetentionDays, &project.StrictCatalog, &project.Enabled, &project.ArchivedAt, &project.SDKTestEnabled, &project.SDKTestScenario)
	if err == pgx.ErrNoRows {
		err = apierr.New(404, contracts.CodeInvalidRequest, "active project not found")
	}
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	_, _ = s.Pool.Exec(r.Context(), `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'project_updated', jsonb_build_object('project_id',$2::text))`, sess.AdminUserID, projectID)
	writeJSON(w, http.StatusOK, map[string]any{"project": project})
	s.logRequest(r, requestID, http.StatusOK, start, map[string]any{"project_id": projectID})
}

func (s *Server) handleProjectArchive(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	s.handleProjectArchiveState(w, r, sess, true)
}

func (s *Server) handleProjectRestore(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	s.handleProjectArchiveState(w, r, sess, false)
}

func (s *Server) handleProjectArchiveState(w http.ResponseWriter, r *http.Request, sess *adminauth.Session, archive bool) {
	requestID, start := newRequestID(), time.Now()
	if err := requireOwner(sess); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	projectID := r.PathValue("id")
	result, err := s.Pool.Exec(r.Context(), `
		UPDATE projects SET archived_at = CASE WHEN $2 THEN clock_timestamp() ELSE NULL END, enabled = NOT $2, updated_at = clock_timestamp()
		WHERE id = $1 AND (($2 AND archived_at IS NULL) OR (NOT $2 AND archived_at IS NOT NULL))
	`, projectID, archive)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if result.RowsAffected() != 1 {
		s.fail(w, r, requestID, start, apierr.New(404, contracts.CodeInvalidRequest, "project is not in the requested state"))
		return
	}
	action := "project_restored"
	if archive {
		action = "project_archived"
	}
	_, _ = s.Pool.Exec(r.Context(), `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, $2, jsonb_build_object('project_id',$3::text))`, sess.AdminUserID, action, projectID)
	w.WriteHeader(http.StatusNoContent)
	s.logRequest(r, requestID, http.StatusNoContent, start, map[string]any{"project_id": projectID})
}

func (s *Server) handleProjectPurge(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	if err := requireOwner(sess); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	projectID := r.PathValue("id")
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var archived bool
	err = tx.QueryRow(r.Context(), `SELECT archived_at IS NOT NULL FROM projects WHERE id = $1 FOR UPDATE`, projectID).Scan(&archived)
	if err == pgx.ErrNoRows || !archived {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "only an archived project may be permanently deleted"))
		return
	}
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	for _, query := range []string{
		`DELETE FROM ingestion_stats WHERE project_id = $1`,
		`DELETE FROM daily_registration_counters WHERE project_id = $1`,
		`DELETE FROM client_policy_rules WHERE project_id = $1`,
		`DELETE FROM event_catalog WHERE project_id = $1`,
		`DELETE FROM events WHERE project_id = $1`,
		`DELETE FROM installations WHERE project_id = $1`,
		`DELETE FROM admin_user_projects WHERE project_id = $1`,
		`DELETE FROM admin_audit_log WHERE detail->>'project_id' = $1`,
		`DELETE FROM projects WHERE id = $1`,
	} {
		if _, err := tx.Exec(r.Context(), query, projectID); err != nil {
			s.fail(w, r, requestID, start, err)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.logRequest(r, requestID, http.StatusNoContent, start, map[string]any{"project_id": projectID, "purged": true})
}

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
	rows, err := s.ReaderPool.Query(r.Context(), `
		SELECT u.id, COALESCE(u.username, u.email, ''), COALESCE(u.email, ''), up.access_role, u.disabled
		FROM admin_user_projects up JOIN admin_users u ON u.id = up.admin_user_id
		WHERE up.project_id = $1 ORDER BY lower(COALESCE(u.username, u.email, '')), u.id
	`, projectID)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	defer rows.Close()
	members := []projectMember{}
	for rows.Next() {
		var member projectMember
		if err := rows.Scan(&member.ID, &member.Username, &member.Email, &member.AccessRole, &member.Disabled); err != nil {
			s.fail(w, r, requestID, start, err)
			return
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members})
	s.logRequest(r, requestID, http.StatusOK, start, map[string]any{"project_id": projectID})
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
	if req.AccessRole != adminauth.ViewerRole && req.AccessRole != adminauth.ProjectAdminRole {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "access_role must be project_admin or viewer"))
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var accountID int64
	if req.AccountID != nil {
		if !sess.IsOwner() {
			s.fail(w, r, requestID, start, apierr.New(403, adminauth.CodeForbiddenRole, "only the owner may assign an existing account"))
			return
		}
		accountID = *req.AccountID
		var exists bool
		if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM admin_users WHERE id = $1)`, accountID).Scan(&exists); err != nil || !exists {
			s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "account not found"))
			return
		}
	} else {
		accountID, err = createNamedAccount(r.Context(), tx, req.Username, req.Password, req.Email)
		if err != nil {
			s.fail(w, r, requestID, start, err)
			return
		}
	}
	_, err = tx.Exec(r.Context(), `
		INSERT INTO admin_user_projects (admin_user_id, project_id, access_role) VALUES ($1,$2,$3)
		ON CONFLICT (admin_user_id, project_id) DO UPDATE SET access_role = EXCLUDED.access_role
	`, accountID, projectID, req.AccessRole)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'project_member_granted', jsonb_build_object('project_id',$2::text,'account_id',$3::text,'access_role',$4::text))`, sess.AdminUserID, projectID, accountID, req.AccessRole)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"account_id": accountID, "project_id": projectID, "access_role": req.AccessRole})
	s.logRequest(r, requestID, http.StatusCreated, start, map[string]any{"project_id": projectID, "account_id": accountID})
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
	if req.AccessRole == nil && req.Password == nil {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "access_role or password is required"))
		return
	}
	if req.AccessRole != nil && *req.AccessRole != adminauth.ViewerRole && *req.AccessRole != adminauth.ProjectAdminRole {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "access_role must be project_admin or viewer"))
		return
	}
	if req.Password != nil && len(*req.Password) < 8 {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "password must be at least 8 characters"))
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var globalRole string
	var membershipCount int
	err = tx.QueryRow(r.Context(), `
		SELECT u.role, COUNT(all_memberships.project_id)
		FROM admin_users u
		JOIN admin_user_projects current_membership ON current_membership.admin_user_id = u.id AND current_membership.project_id = $2
		LEFT JOIN admin_user_projects all_memberships ON all_memberships.admin_user_id = u.id
		WHERE u.id = $1 GROUP BY u.role
	`, accountID, projectID).Scan(&globalRole, &membershipCount)
	if err == pgx.ErrNoRows {
		s.fail(w, r, requestID, start, apierr.New(404, contracts.CodeInvalidRequest, "project membership not found"))
		return
	}
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if !sess.IsOwner() && (globalRole != adminauth.RoleMember || membershipCount != 1) {
		s.fail(w, r, requestID, start, apierr.New(403, adminauth.CodeForbiddenRole, "a project admin may only reset a login exclusive to this project"))
		return
	}
	if req.AccessRole != nil {
		if _, err := tx.Exec(r.Context(), `UPDATE admin_user_projects SET access_role = $3 WHERE project_id = $1 AND admin_user_id = $2`, projectID, accountID, *req.AccessRole); err != nil {
			s.fail(w, r, requestID, start, err)
			return
		}
	}
	if req.Password != nil {
		hash, err := adminauth.HashPassword(*req.Password)
		if err != nil {
			s.fail(w, r, requestID, start, err)
			return
		}
		if _, err := tx.Exec(r.Context(), `UPDATE admin_users SET password_hash = $2 WHERE id = $1`, accountID, hash); err != nil {
			s.fail(w, r, requestID, start, err)
			return
		}
		if _, err := tx.Exec(r.Context(), `UPDATE admin_sessions SET revoked_at = clock_timestamp() WHERE admin_user_id = $1 AND revoked_at IS NULL`, accountID); err != nil {
			s.fail(w, r, requestID, start, err)
			return
		}
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'project_member_updated', jsonb_build_object('project_id',$2::text,'account_id',$3::text))`, sess.AdminUserID, projectID, accountID)
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.logRequest(r, requestID, http.StatusNoContent, start, map[string]any{"project_id": projectID, "account_id": accountID})
}

type accountRecord struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role"`
	Disabled bool   `json:"disabled"`
}

type accountCreateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type accountUpdateRequest struct {
	Password *string `json:"password"`
	Disabled *bool   `json:"disabled"`
}

func (s *Server) handleAccountsList(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	if err := requireOwner(sess); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	rows, err := s.ReaderPool.Query(r.Context(), `SELECT id, COALESCE(username, email, ''), COALESCE(email, ''), role, disabled FROM admin_users ORDER BY lower(COALESCE(username, email, '')), id`)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	defer rows.Close()
	accounts := []accountRecord{}
	for rows.Next() {
		var account accountRecord
		if err := rows.Scan(&account.ID, &account.Username, &account.Email, &account.Role, &account.Disabled); err != nil {
			s.fail(w, r, requestID, start, err)
			return
		}
		accounts = append(accounts, account)
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleAccountCreate(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	if err := requireOwner(sess); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	var req accountCreateRequest
	if err := decodeRequest(w, r, &req); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	id, err := createNamedAccount(r.Context(), tx, req.Username, req.Password, req.Email)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'account_created', jsonb_build_object('account_id',$2::text))`, sess.AdminUserID, id)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"account_id": id})
	s.logRequest(r, requestID, http.StatusCreated, start, map[string]any{"account_id": id})
}

func (s *Server) handleAccountUpdate(w http.ResponseWriter, r *http.Request, sess *adminauth.Session) {
	requestID, start := newRequestID(), time.Now()
	if err := requireOwner(sess); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if err := adminauth.CheckCSRF(r); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "invalid account id"))
		return
	}
	var req accountUpdateRequest
	if err := decodeRequest(w, r, &req); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	if req.Password == nil && req.Disabled == nil {
		s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "password or disabled is required"))
		return
	}
	var hash *string
	if req.Password != nil {
		if len(*req.Password) < 8 {
			s.fail(w, r, requestID, start, apierr.New(400, contracts.CodeInvalidRequest, "password must be at least 8 characters"))
			return
		}
		value, err := adminauth.HashPassword(*req.Password)
		if err != nil {
			s.fail(w, r, requestID, start, err)
			return
		}
		hash = &value
	}
	result, err := s.Pool.Exec(r.Context(), `UPDATE admin_users SET password_hash = COALESCE($2, password_hash), disabled = COALESCE($3, disabled) WHERE id = $1`, id, hash, req.Disabled)
	if err != nil || result.RowsAffected() != 1 {
		if err == nil {
			err = apierr.New(404, contracts.CodeInvalidRequest, "account not found")
		}
		s.fail(w, r, requestID, start, err)
		return
	}
	if req.Password != nil || req.Disabled != nil && *req.Disabled {
		_, _ = s.Pool.Exec(r.Context(), `UPDATE admin_sessions SET revoked_at = clock_timestamp() WHERE admin_user_id = $1 AND revoked_at IS NULL`, id)
	}
	_, _ = s.Pool.Exec(r.Context(), `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'account_updated', jsonb_build_object('account_id',$2::text))`, sess.AdminUserID, id)
	w.WriteHeader(http.StatusNoContent)
	s.logRequest(r, requestID, http.StatusNoContent, start, map[string]any{"account_id": id})
}

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

func createNamedAccount(ctx context.Context, tx pgx.Tx, username, password, email string) (int64, error) {
	username, email = strings.TrimSpace(username), strings.TrimSpace(email)
	if username == "" || len(username) > 80 || strings.ContainsAny(username, " \t\n") || len(password) < 8 {
		return 0, apierr.New(400, contracts.CodeInvalidRequest, "username without spaces and password of at least 8 characters are required")
	}
	hash, err := adminauth.HashPassword(password)
	if err != nil {
		return 0, err
	}
	var taken bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM admin_users
			WHERE lower(COALESCE(username, '')) = lower($1)
			   OR lower(COALESCE(email, '')) = lower($1)
			   OR ($2 <> '' AND (lower(COALESCE(username, '')) = lower($2) OR lower(COALESCE(email, '')) = lower($2)))
		)
	`, username, email).Scan(&taken); err != nil {
		return 0, err
	}
	if taken {
		return 0, apierr.New(400, contracts.CodeInvalidRequest, "username or email is already in use")
	}
	var optionalEmail any
	if email != "" {
		optionalEmail = email
	}
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO admin_users (username, email, password_hash, role) VALUES ($1,$2,$3,'member') RETURNING id`, username, optionalEmail, hash).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any) error {
	data, err := readBody(w, r)
	if err != nil {
		return badRequest(err)
	}
	if err := decodeJSONStrict(data, target); err != nil {
		return decodeErr(err)
	}
	return nil
}

func generatedID(prefix string) (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(bytes), nil
}

func trimmed(value *string) *string {
	if value == nil {
		return nil
	}
	result := strings.TrimSpace(*value)
	return &result
}

func knownSDKScenario(scenario sdktest.Scenario) bool {
	switch scenario {
	case sdktest.LostAcknowledgement, sdktest.UnauthorizedOnce, sdktest.PayloadTooLarge, sdktest.RateLimited, sdktest.PolicyActive, sdktest.PolicyPauseUpload, sdktest.PolicyDisable:
		return true
	default:
		return false
	}
}
