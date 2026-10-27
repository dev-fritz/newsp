// Package apperrors defines semantic application errors with stable codes.
// Services return them; handlers translate them into HTTP responses.
package apperrors

import (
	"errors"
	"fmt"
	"time"
)

// Kind classifies an error so the HTTP layer can choose a status code.
type Kind int

// Error kinds, mapped to HTTP status codes in the handlers package.
const (
	KindInternal Kind = iota
	KindValidation
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindUnprocessable
	KindRateLimited
	KindUnavailable
	KindExternal
)

// Error is a safe-to-expose application error.
type Error struct {
	Kind    Kind
	Code    string            // stable UPPER_SNAKE_CASE code the frontend switches on
	Message string            // human-readable, safe to show
	Fields  map[string]string // per-field validation errors
	Err     error             // wrapped cause, never sent to the client
	// RetryAfter is sent as the Retry-After header on rate-limited responses.
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

// Wrap attaches an underlying cause to a copy of the error.
func (e *Error) Wrap(err error) *Error {
	c := *e
	c.Err = err
	return &c
}

// As extracts an *Error from err, if any.
func As(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// Validation builds a 400 error with optional per-field messages.
func Validation(message string, fields map[string]string) *Error {
	return &Error{Kind: KindValidation, Code: "VALIDATION_FAILED", Message: message, Fields: fields}
}

// Unauthorized builds a 401 error.
func Unauthorized(code, message string) *Error {
	return &Error{Kind: KindUnauthorized, Code: code, Message: message}
}

// Forbidden builds a 403 error.
func Forbidden(code, message string) *Error {
	return &Error{Kind: KindForbidden, Code: code, Message: message}
}

// NotFound builds a 404 error.
func NotFound(code, message string) *Error {
	return &Error{Kind: KindNotFound, Code: code, Message: message}
}

// Conflict builds a 409 error.
func Conflict(code, message string) *Error {
	return &Error{Kind: KindConflict, Code: code, Message: message}
}

// Unprocessable builds a 422 error for violated business rules.
func Unprocessable(code, message string) *Error {
	return &Error{Kind: KindUnprocessable, Code: code, Message: message}
}

// RateLimited builds a 429 error telling the client when to retry.
func RateLimited(retryAfter time.Duration) *Error {
	return &Error{Kind: KindRateLimited, Code: "RATE_LIMITED", Message: "too many requests, try again later", RetryAfter: retryAfter}
}

// Unavailable builds a 503 error.
func Unavailable(code, message string) *Error {
	return &Error{Kind: KindUnavailable, Code: code, Message: message}
}

// External builds a 502 error for failing upstream dependencies.
func External(code, message string, err error) *Error {
	return &Error{Kind: KindExternal, Code: code, Message: message, Err: err}
}
