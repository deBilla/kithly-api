package processingjob

import (
	"github.com/gin-gonic/gin"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/core/response"
	"github.com/kithly/kithly-ws-go/internal/middleware"
)

// Handler holds the HTTP handlers for the processing job module.
type Handler struct{ service Service }

// NewHandler constructs a Handler with the given Service.
func NewHandler(service Service) *Handler { return &Handler{service: service} }

// GetLatest handles GET /video-processing-job/latest?video_card_id=.
func (h *Handler) GetLatest(c *gin.Context) {
	cardID := c.Query("video_card_id")
	if cardID == "" {
		_ = c.Error(apperr.BadRequest("video_card_id query param is required"))
		return
	}
	userID, email := c.GetString(middleware.ContextKeyUserID), c.GetString(middleware.ContextKeyUserEmail)
	job, err := h.service.GetLatestForUser(c.Request.Context(), cardID, userID, email)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, job.ToView())
}

// Retry handles POST /video-processing-job/retry/:id.
func (h *Handler) Retry(c *gin.Context) {
	userID, email := c.GetString(middleware.ContextKeyUserID), c.GetString(middleware.ContextKeyUserEmail)
	job, err := h.service.Retry(c.Request.Context(), c.Param("id"), userID, email)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, job.ToView())
}
