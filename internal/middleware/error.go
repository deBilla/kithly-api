// Package middleware contains Gin middleware functions used globally across
// the application.
package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/core/response"
)

// ErrorHandler is a Gin middleware that must be registered first (before all
// routes). It intercepts errors attached via c.Error() and writes a
// standardised JSON error response.
//
// Handlers should never write their own error responses — they call c.Error(err)
// and return. This middleware handles the rest.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		// Use the last error; earlier errors are treated as context.
		err := c.Errors.Last().Err

		var appErr *apperr.AppError
		if errors.As(err, &appErr) {
			response.Fail(c, appErr.HTTPStatus(), appErr.Code, appErr.Message)
			return
		}

		// Unknown error — do not leak internal details to the client.
		response.Fail(c, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
	}
}
