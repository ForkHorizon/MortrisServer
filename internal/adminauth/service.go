package adminauth

import (
	"context"
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
	Email       string
	Role        string
	ProjectIDs  []string
}

type LoginResult struct {
	SessionToken string
	CSRFToken    string
	ExpiresAt    time.Time
	Session      Session
}

// Login implements section 10.3: throttled, constant-time-ish credential
// check, new session + CSRF token on success, audit record either way.
func Login(ctx context.Context, pool *pgxpool.Pool, throttle *Throttle, email, password, sourceIP string) (*LoginResult, error) {
	if !throttle.Allow(email, sourceIP) {
		return nil, apierr.New(429, CodeTooManyAttempts, "too many login attempts")
	}

	var userID int64
	var passwordHash, role string
	var disabled bool
	err := pool.QueryRow(ctx, `SELECT id, password_hash, role, disabled FROM admin_users WHERE email = $1`, email).
		Scan(&userID, &passwordHash, &role, &disabled)

	found := err == nil
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	if !found {
		VerifyPassword(decoyHash, password) // constant-time-ish: do the work anyway
		auditLoginFailure(ctx, pool, email, "no such user")
		return nil, apierr.New(401, CodeInvalidCredentials, "invalid email or password")
	}
	if disabled {
		auditLoginFailure(ctx, pool, email, "account disabled")
		return nil, apierr.New(401, CodeInvalidCredentials, "invalid email or password")
	}
	if !VerifyPassword(passwordHash, password) {
		auditLoginFailure(ctx, pool, email, "wrong password")
		return nil, apierr.New(401, CodeInvalidCredentials, "invalid email or password")
	}

	projectIDs, err := loadProjectIDs(ctx, pool, userID)
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
		VALUES ($1, 'login_success', jsonb_build_object('email', $2::text))
	`, userID, email); err != nil {
		return nil, err
	}

	return &LoginResult{
		SessionToken: sessionToken,
		CSRFToken:    csrfToken,
		ExpiresAt:    expiresAt,
		Session:      Session{AdminUserID: userID, Email: email, Role: role, ProjectIDs: projectIDs},
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
	var email, role string
	var disabled bool
	var expiresAt, lastSeenAt time.Time
	var revokedAt *time.Time

	err := pool.QueryRow(ctx, `
		SELECT s.admin_user_id, s.expires_at, s.last_seen_at, s.revoked_at, u.email, u.role, u.disabled
		FROM admin_sessions s JOIN admin_users u ON u.id = s.admin_user_id
		WHERE s.token_hash = $1
	`, hashToken(sessionToken)).Scan(&adminUserID, &expiresAt, &lastSeenAt, &revokedAt, &email, &role, &disabled)
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

	projectIDs, err := loadProjectIDs(ctx, pool, adminUserID)
	if err != nil {
		return nil, err
	}

	return &Session{AdminUserID: adminUserID, Email: email, Role: role, ProjectIDs: projectIDs}, nil
}

func loadProjectIDs(ctx context.Context, pool *pgxpool.Pool, adminUserID int64) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT project_id FROM admin_user_projects WHERE admin_user_id = $1 ORDER BY project_id`, adminUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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
