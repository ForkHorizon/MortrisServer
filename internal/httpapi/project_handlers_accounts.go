package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
)

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
	if err := rows.Err(); err != nil {
		s.fail(w, r, requestID, start, err)
		return
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
	id, err := s.createAccount(r.Context(), sess.AdminUserID, req)
	if err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"account_id": id})
	s.logRequest(r, requestID, http.StatusCreated, start, map[string]any{"account_id": id})
}

func (s *Server) createAccount(ctx context.Context, ownerID int64, req accountCreateRequest) (int64, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, err := createNamedAccount(ctx, tx, req.Username, req.Password, req.Email)
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'account_created', jsonb_build_object('account_id',$2::text))`, ownerID, id)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	return id, err
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
	if err := s.updateAccount(r.Context(), sess.AdminUserID, id, req); err != nil {
		s.fail(w, r, requestID, start, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	s.logRequest(r, requestID, http.StatusNoContent, start, map[string]any{"account_id": id})
}

func (s *Server) updateAccount(ctx context.Context, ownerID int64, id int64, req accountUpdateRequest) error {
	if req.Password == nil && req.Disabled == nil {
		return apierr.New(400, contracts.CodeInvalidRequest, "password or disabled is required")
	}
	hash, err := accountPasswordHash(req.Password)
	if err != nil {
		return err
	}
	result, err := s.Pool.Exec(ctx, `UPDATE admin_users SET password_hash = COALESCE($2, password_hash), disabled = COALESCE($3, disabled) WHERE id = $1`, id, hash, req.Disabled)
	if err != nil || result.RowsAffected() != 1 {
		if err == nil {
			err = apierr.New(404, contracts.CodeInvalidRequest, "account not found")
		}
		return err
	}
	if req.Password != nil || req.Disabled != nil && *req.Disabled {
		_, _ = s.Pool.Exec(ctx, `UPDATE admin_sessions SET revoked_at = clock_timestamp() WHERE admin_user_id = $1 AND revoked_at IS NULL`, id)
	}
	_, _ = s.Pool.Exec(ctx, `INSERT INTO admin_audit_log (admin_user_id, action, detail) VALUES ($1, 'account_updated', jsonb_build_object('account_id',$2::text))`, ownerID, id)
	return nil
}

func accountPasswordHash(password *string) (*string, error) {
	if password == nil {
		return nil, nil
	}
	if len(*password) < 8 {
		return nil, apierr.New(400, contracts.CodeInvalidRequest, "password must be at least 8 characters")
	}
	hash, err := adminauth.HashPassword(*password)
	if err != nil {
		return nil, err
	}
	return &hash, nil
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
	return id, err
}
