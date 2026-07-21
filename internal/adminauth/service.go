package adminauth

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/apierr"
)

// decoyHash lets Login run a real Argon2id verify even when the email
// doesn't exist, so "no such user" and "wrong password" take the same
// time and an attacker can't enumerate accounts by timing.
var decoyHash string

func init() {
	h, err := HashPassword("decoy-password-used-only-for-timing-safety")
	if err != nil {
		panic(err)
	}
	decoyHash = h
}

type Session struct {
	AdminUserID int64
	Username    string
	Email       string
	Role        string
	ProjectIDs  []string
	Projects    []Project
}

const (
	RoleOwner        = "owner"
	RoleMember       = "member"
	ProjectAdminRole = "project_admin"
	ViewerRole       = "viewer"
)

// Project is the project metadata and the caller's role returned to the
// dashboard. It deliberately contains no secrets or ingestion credentials.
type Project struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Environment string `json:"environment"`
	Role        string `json:"role"`
}

type LoginResult struct {
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
	Session      Session
}

// Login implements section 10.3: throttled, constant-time-ish credential
// check, new session + CSRF token on success, audit record either way.
func Login(ctx context.Context, pool *pgxpool.Pool, throttle *Throttle, identifier, password, sourceIP string) (*LoginResult, error) {
	identifier = strings.TrimSpace(identifier)
	if !throttle.Allow(identifier, sourceIP) {
		return nil, apierr.New(429, CodeTooManyAttempts, "too many login attempts")
	}

	var userID int64
	var passwordHash, role string
	var username, email *string
	var disabled bool
	err := pool.QueryRow(ctx, `
		SELECT id, password_hash, role, disabled, username, email
		FROM admin_users
		WHERE lower(COALESCE(username, '')) = lower($1) OR lower(COALESCE(email, '')) = lower($1)
		ORDER BY id LIMIT 1
	`, identifier).Scan(&userID, &passwordHash, &role, &disabled, &username, &email)

	found := err == nil
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	if !found {
		VerifyPassword(decoyHash, password) // constant-time-ish: do the work anyway
		auditLoginFailure(ctx, pool, identifier, "no such user")
		return nil, apierr.New(401, CodeInvalidCredentials, "invalid username or password")
	}
	if disabled {
		auditLoginFailure(ctx, pool, identifier, "account disabled")
		return nil, apierr.New(401, CodeInvalidCredentials, "invalid username or password")
	}
	if !VerifyPassword(passwordHash, password) {
		auditLoginFailure(ctx, pool, identifier, "wrong password")
		return nil, apierr.New(401, CodeInvalidCredentials, "invalid username or password")
	}

	projects, err := loadProjects(ctx, pool, userID, role)
	if err != nil {
		return nil, err
	}

	sessionToken, sessionHash := newToken()
	csrfToken, _ := newToken()
	expiresAt := time.Now().UTC().Add(AbsoluteTimeout)

	if _, err := pool.Exec(ctx, `
		INSERT INTO admin_sessions (admin_user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, sessionHash, expiresAt); err != nil {
		return nil, err
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO admin_audit_log (admin_user_id, action, detail)
		VALUES ($1, 'login_success', jsonb_build_object('identifier', $2::text))
	`, userID, identifier); err != nil {
		return nil, err
	}

	return &LoginResult{
		SessionToken: sessionToken,
		CSRFToken:    csrfToken,
		ExpiresAt:    expiresAt,
		Session:      newSession(userID, stringValue(username, email), stringValue(email, username), role, projects),
	}, nil
}

func auditLoginFailure(ctx context.Context, pool *pgxpool.Pool, email, reason string) {
	_, _ = pool.Exec(ctx, `
		INSERT INTO admin_audit_log (admin_user_id, action, detail)
		VALUES (NULL, 'login_failure', jsonb_build_object('email', $1::text, 'reason', $2::text))
	`, email, reason)
}

// Logout revokes the session and audit-logs it. Revoking a token that
// doesn't exist or is already revoked is not an error — logout is
// idempotent.
func Logout(ctx context.Context, pool *pgxpool.Pool, sessionToken string) error {
	var adminUserID int64
	err := pool.QueryRow(ctx, `
		UPDATE admin_sessions SET revoked_at = clock_timestamp()
		WHERE token_hash = $1 AND revoked_at IS NULL
		RETURNING admin_user_id
	`, hashToken(sessionToken)).Scan(&adminUserID)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO admin_audit_log (admin_user_id, action, detail)
		VALUES ($1, 'logout', '{}')
	`, adminUserID)
	return err
}

// ValidateSession checks the session token against both expiry rules
// (section 10.3: inactivity and absolute) and refreshes last_seen_at on
// success.
func ValidateSession(ctx context.Context, pool *pgxpool.Pool, sessionToken string) (*Session, error) {
	var adminUserID int64
	var role string
	var username, email *string
	var disabled bool
	var expiresAt, lastSeenAt time.Time
	var revokedAt *time.Time

	err := pool.QueryRow(ctx, `
		SELECT s.admin_user_id, s.expires_at, s.last_seen_at, s.revoked_at, u.username, u.email, u.role, u.disabled
		FROM admin_sessions s JOIN admin_users u ON u.id = s.admin_user_id
		WHERE s.token_hash = $1
	`, hashToken(sessionToken)).Scan(&adminUserID, &expiresAt, &lastSeenAt, &revokedAt, &username, &email, &role, &disabled)
	if err == pgx.ErrNoRows {
		return nil, apierr.New(401, CodeSessionInvalid, "invalid session")
	}
	if err != nil {
		return nil, err
	}
	if revokedAt != nil || disabled {
		return nil, apierr.New(401, CodeSessionInvalid, "invalid session")
	}

	now := time.Now().UTC()
	if now.After(expiresAt) || now.Sub(lastSeenAt) > IdleTimeout {
		return nil, apierr.New(401, CodeSessionExpired, "session expired")
	}

	if _, err := pool.Exec(ctx, `UPDATE admin_sessions SET last_seen_at = clock_timestamp() WHERE token_hash = $1`, hashToken(sessionToken)); err != nil {
		return nil, err
	}

	projects, err := loadProjects(ctx, pool, adminUserID, role)
	if err != nil {
		return nil, err
	}

	return ptrSession(newSession(adminUserID, stringValue(username, email), stringValue(email, username), role, projects)), nil
}

func ptrSession(s Session) *Session { return &s }

func newSession(id int64, username, email, role string, projects []Project) Session {
	ids := make([]string, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.ID)
	}
	return Session{AdminUserID: id, Username: username, Email: email, Role: role, ProjectIDs: ids, Projects: projects}
}

func stringValue(primary, fallback *string) string {
	if primary != nil {
		return *primary
	}
	if fallback != nil {
		return *fallback
	}
	return ""
}

func loadProjects(ctx context.Context, pool *pgxpool.Pool, adminUserID int64, globalRole string) ([]Project, error) {
	query := `
		SELECT p.id, p.display_name, p.environment,
		       CASE WHEN $2 = 'owner' THEN 'project_admin' ELSE up.access_role END
		FROM projects p
		LEFT JOIN admin_user_projects up ON up.project_id = p.id AND up.admin_user_id = $1
		WHERE p.archived_at IS NULL AND ($2 = 'owner' OR up.admin_user_id IS NOT NULL)
		ORDER BY p.display_name, p.id
	`
	rows, err := pool.Query(ctx, query, adminUserID, globalRole)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.ID, &project.DisplayName, &project.Environment, &project.Role); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

// HasProjectAccess reports whether the session may view projectID
// (section 10.3: "each scoped to explicit projects").
func (s *Session) HasProjectAccess(projectID string) bool {
	for _, id := range s.ProjectIDs {
		if id == projectID {
			return true
		}
	}
	return false
}

func (s *Session) ProjectRole(projectID string) string {
	if s.Role == RoleOwner {
		return ProjectAdminRole
	}
	for _, project := range s.Projects {
		if project.ID == projectID {
			return project.Role
		}
	}
	return ""
}

func (s *Session) CanManageProject(projectID string) bool {
	return s.Role == RoleOwner || s.ProjectRole(projectID) == ProjectAdminRole
}

func (s *Session) IsOwner() bool { return s.Role == RoleOwner }
