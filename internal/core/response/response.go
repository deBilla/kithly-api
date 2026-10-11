// Package response provides standardised JSON envelope helpers for all API
// responses. Handlers must use these helpers — never construct the envelope
// map inline.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// successEnvelope matches the TypeScript SuccessResponse shape exactly:
//
//	{ "success": true, "status": "success", "payload": { ... } }
type successEnvelope struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
	Payload any    `json:"payload,omitempty"`
}

// errorEnvelope matches the TypeScript sendErrorResponse shape exactly:
//
//	{ "success": false, "status": "error", "payload": "message", "errorCode": "CODE" }
type errorEnvelope struct {
	Success   bool   `json:"success"`
	Status    string `json:"status"`
	Payload   string `json:"payload"`
	ErrorCode string `json:"errorCode"`
}

// OK writes a 200 success response with the given data payload.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, successEnvelope{Success: true, Status: "success", Payload: data})
}

// Created writes a 201 success response.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, successEnvelope{Success: true, Status: "success", Payload: data})
}

// NoContent writes a 204 with no body.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Fail writes an error response using the given HTTP status, code, and message.
func Fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorEnvelope{
		Success:   false,
		Status:    "error",
		Payload:   message,
		ErrorCode: code,
	})
}
