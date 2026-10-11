package errors_test

import (
	"errors"
	"net/http"
	"testing"

	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/stretchr/testify/assert"
)

func TestAppError_HTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        *apperr.AppError
		wantStatus int
	}{
		{"not found", apperr.NotFound("card"), http.StatusNotFound},
		{"forbidden", apperr.Forbidden("access denied"), http.StatusForbidden},
		{"unauthorized", apperr.Unauthorized("missing token"), http.StatusUnauthorized},
		{"conflict", apperr.Conflict("already exists"), http.StatusConflict},
		{"limit exceeded", apperr.LimitExceeded("max cards reached"), http.StatusUnprocessableEntity},
		{"bad request", apperr.BadRequest("invalid input"), http.StatusBadRequest},
		{"internal", apperr.Internal("unexpected failure"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantStatus, tt.err.HTTPStatus())
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	err := apperr.NotFound("card")
	assert.True(t, errors.Is(err, apperr.ErrNotFound))
	assert.False(t, errors.Is(err, apperr.ErrForbidden))
}

func TestAppError_Error(t *testing.T) {
	err := apperr.NotFound("card")
	assert.Equal(t, "NOT_FOUND: card not found", err.Error())
}
