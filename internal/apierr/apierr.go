// Package apierr is the typed error every HTTP handler returns, so
// internal/httpapi can render contracts.ErrorResponse consistently
// without each handler duplicating status-code/Retry-After logic.
package apierr

import (
	"fmt"
	"time"
)

type Error struct {
	Status     int
	Code       string
	Message    string
	RetryAfter time.Duration // 0 means no Retry-After header
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func WithRetryAfter(status int, code, message string, retryAfter time.Duration) *Error {
	return &Error{Status: status, Code: code, Message: message, RetryAfter: retryAfter}
}
