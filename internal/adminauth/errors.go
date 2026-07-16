package adminauth

// Dashboard-API-specific error codes, rendered through the same
// contracts.ErrorResponse shape as the SDK endpoints (see docs/errors.md's
// "Dashboard API errors" section) but namespaced separately since they
// describe operator-auth failures, not SDK wire-contract violations.
const (
	CodeInvalidCredentials = "invalid_credentials"
	CodeSessionInvalid     = "session_invalid"
	CodeSessionExpired     = "session_expired"
	CodeCSRFMismatch       = "csrf_mismatch"
	CodeForbiddenProject   = "forbidden_project"
	CodeForbiddenRole      = "forbidden_role"
	CodeTooManyAttempts    = "too_many_attempts"
)
