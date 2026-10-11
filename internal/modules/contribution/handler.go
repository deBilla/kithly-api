package contribution

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/core/response"
	"github.com/kithly/kithly-ws-go/internal/middleware"
)

// caller returns the authenticated user, or empty strings for anonymous
// contributors on the optionally-authenticated upload routes.
func caller(c *gin.Context) (userID, email string) {
	return c.GetString(middleware.ContextKeyUserID), c.GetString(middleware.ContextKeyUserEmail)
}

// Handler holds the HTTP handlers for the video contribution module.
type Handler struct{ service Service }

// NewHandler constructs a Handler with the given Service.
func NewHandler(service Service) *Handler { return &Handler{service: service} }

// GetUploadURL handles POST /video-contribution/upload-url.
func (h *Handler) GetUploadURL(c *gin.Context) {
	var req UploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID, email := caller(c)
	result, err := h.service.GetUploadURL(c.Request.Context(), req, userID, email)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, result)
}

// ConfirmUpload handles POST /video-contribution/.
func (h *Handler) ConfirmUpload(c *gin.Context) {
	var req ConfirmUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID, email := caller(c)
	contrib, err := h.service.ConfirmUpload(c.Request.Context(), req, userID, email)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, contrib.ToView())
}

// List handles GET /video-contribution/ (requires ?video_card_id=).
func (h *Handler) List(c *gin.Context) {
	cardID := c.Query("video_card_id")
	if cardID == "" {
		_ = c.Error(apperr.BadRequest("video_card_id query param is required"))
		return
	}
	userID, email := caller(c)
	contribs, err := h.service.List(c.Request.Context(), cardID, userID, email)
	if err != nil {
		_ = c.Error(err)
		return
	}
	views := make([]View, len(contribs))
	for i, contrib := range contribs {
		views[i] = contrib.ToView()
	}
	response.OK(c, views)
}

// BatchReorder handles PUT /video-contribution/batch-reorder.
func (h *Handler) BatchReorder(c *gin.Context) {
	var req BatchReorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID, email := caller(c)
	if err := h.service.BatchReorder(c.Request.Context(), req, userID, email); err != nil {
		_ = c.Error(err)
		return
	}
	response.NoContent(c)
}

// Update handles PUT /video-contribution/:id.
func (h *Handler) Update(c *gin.Context) {
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID, email := caller(c)
	contrib, err := h.service.Update(c.Request.Context(), c.Param("id"), userID, email, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, contrib.ToView())
}

// Delete handles DELETE /video-contribution/:id.
func (h *Handler) Delete(c *gin.Context) {
	userID, email := caller(c)
	if err := h.service.Delete(c.Request.Context(), c.Param("id"), userID, email); err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}
