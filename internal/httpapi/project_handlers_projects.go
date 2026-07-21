package httpapi

import (
	"context"
	"crypto/sha256"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

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
	projects, err := s.listProjects(r.Context(), r.URL.Query().Get("archived") == "true")
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": projects})
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) listProjects(ctx context.Context, archived bool) ([]managedProject, error) {
	rows, err := s.ReaderPool.Query(ctx, `
		SELECT id, display_name, environment, retention_days, strict_catalog, enabled, archived_at, sdk_test_enabled, sdk_test_scenario
		FROM projects WHERE (archived_at IS NOT NULL) = $1 ORDER BY display_name, id
	`, archived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []managedProject{}
	for rows.Next() {
		var project managedProject
		if err := rows.Scan(&project.ID, &project.DisplayName, &project.Environment, &project.RetentionDays, &project.StrictCatalog, &project.Enabled, &project.ArchivedAt, &project.SDKTestEnabled, &project.SDKTestScenario); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
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
	project, sdkToken, sdkHash, err := newManagedProject(req)
	if err == nil {
		err = s.createProject(r.Context(), sess.AdminUserID, project, sdkHash)
	}
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	response := map[string]any{"project": project}
	if sdkToken != "" {
		response["sdk_test_token"] = sdkToken
	}
	writeJSON(w, http.StatusCreated, response)
	s.logRequest(r, requestID, http.StatusCreated, start, map[string]any{"project_id": project.ID})
}

func newManagedProject(req projectCreateRequest) (managedProject, string, []byte, error) {
	req.DisplayName, req.Environment = strings.TrimSpace(req.DisplayName), strings.TrimSpace(req.Environment)
	if req.DisplayName == "" || req.Environment == "" || req.RetentionDays < 1 || req.RetentionDays > 3650 {
		return managedProject{}, "", nil, apierr.New(400, contracts.CodeInvalidRequest, "display_name, environment, and retention_days (1-3650) are required")
	}
	if req.SDKTestEnabled && req.Environment != "test" {
		return managedProject{}, "", nil, apierr.New(400, contracts.CodeInvalidRequest, "SDK test controls require environment=test")
	}
	projectID, err := generatedID("project")
	if err != nil {
		return managedProject{}, "", nil, err
	}
	strictCatalog := req.StrictCatalog == nil || *req.StrictCatalog
	project := managedProject{ID: projectID, DisplayName: req.DisplayName, Environment: req.Environment, RetentionDays: req.RetentionDays, StrictCatalog: strictCatalog, Enabled: true, SDKTestEnabled: req.SDKTestEnabled}
	if !req.SDKTestEnabled {
		return project, "", nil, nil
	}
	token, err := generatedID("sdk_test")
	if err != nil {
		return managedProject{}, "", nil, err
	}
	hash := sha256.Sum256([]byte(token))
	return project, token, hash[:], nil
}

func (s *Server) createProject(ctx context.Context, ownerID int64, project managedProject, sdkHash []byte) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO projects (id, environment, display_name, retention_days, strict_catalog, sdk_test_enabled, sdk_test_token_hash)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, project.ID, project.Environment, project.DisplayName, project.RetentionDays, project.StrictCatalog, project.SDKTestEnabled, sdkHash)
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'project_created', jsonb_build_object('project_id',$2::text,'sdk_test',$3::boolean))`, ownerID, project.ID, project.SDKTestEnabled)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	return err
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
	project, err := s.updateProject(r.Context(), projectID, req)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	_, _ = s.Pool.Exec(r.Context(), `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'project_updated', jsonb_build_object('project_id',$2::text))`, sess.AdminUserID, projectID)
	writeJSON(w, http.StatusOK, map[string]any{"project": project})
	s.logRequest(r, requestID, http.StatusOK, start, map[string]any{"project_id": projectID})
}

func (s *Server) updateProject(ctx context.Context, projectID string, req projectUpdateRequest) (managedProject, error) {
	if req.DisplayName == nil && req.Environment == nil && req.RetentionDays == nil && req.StrictCatalog == nil {
		return managedProject{}, apierr.New(400, contracts.CodeInvalidRequest, "at least one setting is required")
	}
	if req.DisplayName != nil && strings.TrimSpace(*req.DisplayName) == "" || req.Environment != nil && strings.TrimSpace(*req.Environment) == "" || req.RetentionDays != nil && (*req.RetentionDays < 1 || *req.RetentionDays > 3650) {
		return managedProject{}, apierr.New(400, contracts.CodeInvalidRequest, "invalid project settings")
	}
	var project managedProject
	err := s.Pool.QueryRow(ctx, `
		UPDATE projects SET display_name = COALESCE($2, display_name), environment = COALESCE($3, environment),
		retention_days = COALESCE($4, retention_days), strict_catalog = COALESCE($5, strict_catalog), updated_at = clock_timestamp()
		WHERE id = $1 AND archived_at IS NULL
		RETURNING id, display_name, environment, retention_days, strict_catalog, enabled, archived_at, sdk_test_enabled, sdk_test_scenario
	`, projectID, trimmed(req.DisplayName), trimmed(req.Environment), req.RetentionDays, req.StrictCatalog).Scan(&project.ID, &project.DisplayName, &project.Environment, &project.RetentionDays, &project.StrictCatalog, &project.Enabled, &project.ArchivedAt, &project.SDKTestEnabled, &project.SDKTestScenario)
	if err == pgx.ErrNoRows {
		err = apierr.New(404, contracts.CodeInvalidRequest, "active project not found")
	}
	return project, err
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
	if err := s.purgeProject(r.Context(), projectID); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.logRequest(r, requestID, http.StatusNoContent, start, map[string]any{"project_id": projectID, "purged": true})
}

func (s *Server) purgeProject(ctx context.Context, projectID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var archived bool
	err = tx.QueryRow(ctx, `SELECT archived_at IS NOT NULL FROM projects WHERE id = $1 FOR UPDATE`, projectID).Scan(&archived)
	if err == pgx.ErrNoRows || !archived {
		return apierr.New(400, contracts.CodeInvalidRequest, "only an archived project may be permanently deleted")
	}
	if err != nil {
		return err
	}
	for _, query := range projectPurgeQueries {
		if _, err := tx.Exec(ctx, query, projectID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
