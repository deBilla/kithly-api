// Package errors defines sentinel errors and the AppError type used to
// communicate structured failures across all application layers.
// The global error middleware maps AppError values to HTTP status codes.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors — use errors.Is() to check for these across layers.
var (
	ErrNotFound      = errors.New("not found")
	ErrForbidden     = errors.New("forbidden")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrConflict      = errors.New("conflict")
	ErrLimitExceeded = errors.New("limit exceeded")
	ErrBadRequest    = errors.New("bad request")
	ErrInternal      = errors.New("internal server error")
)

// AppError wraps a sentinel error with a human-readable message and an
// optional machine-readable code for the API response body.
type AppError struct {
	// Code is the machine-readable error code returned in the API response.
	Code string
	// Message is the human-readable description returned to the client.
	Message string
	// Err is the underlying sentinel error used for errors.Is checks.
	Err error
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// HTTPStatus maps the wrapped sentinel error to an HTTP status code.
func (e *AppError) HTTPStatus() int {
	switch {
	case errors.Is(e.Err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(e.Err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(e.Err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(e.Err, ErrConflict):
		return http.StatusConflict
	case errors.Is(e.Err, ErrLimitExceeded):
		return http.StatusUnprocessableEntity
	case errors.Is(e.Err, ErrBadRequest):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// New constructs an AppError with an explicit code and message.
func New(err error, code, message string) *AppError {
	return &AppError{Err: err, Code: code, Message: message}
}

// NotFound returns a standard 404 AppError.
func NotFound(resource string) *AppError {
	return New(ErrNotFound, "NOT_FOUND", fmt.Sprintf("%s not found", resource))
}

// Forbidden returns a standard 403 AppError.
func Forbidden(reason string) *AppError {
	return New(ErrForbidden, "FORBIDDEN", reason)
}

// Unauthorized returns a standard 401 AppError.
func Unauthorized(reason string) *AppError {
	return New(ErrUnauthorized, "UNAUTHORIZED", reason)
}

// LimitExceeded returns a 422 AppError for quota violations.
func LimitExceeded(reason string) *AppError {
	return New(ErrLimitExceeded, "LIMIT_EXCEEDED", reason)
}

// Conflict returns a 409 AppError.
func Conflict(reason string) *AppError {
	return New(ErrConflict, "CONFLICT", reason)
}

// BadRequest returns a 400 AppError for validation or input errors.
func BadRequest(reason string) *AppError {
	return New(ErrBadRequest, "BAD_REQUEST", reason)
}

// Internal returns a 500 AppError. Use sparingly — prefer specific errors.
func Internal(reason string) *AppError {
	return New(ErrInternal, "INTERNAL_ERROR", reason)
}
