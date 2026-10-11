package card

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/core/response"
	"github.com/kithly/kithly-ws-go/internal/middleware"
)

// Handler holds the HTTP handlers for the video card module.
type Handler struct {
	service Service
}

// NewHandler constructs a Handler with the given Service.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func userContext(c *gin.Context) (userID, email string) {
	return c.GetString(middleware.ContextKeyUserID), c.GetString(middleware.ContextKeyUserEmail)
}

// Create handles POST /video-card/.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID, _ := userContext(c)
	card, err := h.service.Create(c.Request.Context(), req, userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, card.ToOwnerView())
}

// Memories handles GET /video-card/memories.
func (h *Handler) Memories(c *gin.Context) {
	var q MemoriesQuery
	_ = c.ShouldBindQuery(&q)
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	userID, email := userContext(c)
	cards, _, err := h.service.Memories(c.Request.Context(), userID, email, q.Page, q.Limit)
	if err != nil {
		_ = c.Error(err)
		return
	}
	views := make([]OwnerView, len(cards))
	for i, card := range cards {
		views[i] = card.ToOwnerView()
	}
	// TypeScript returns payload as a flat card array — pagination is dropped
	// by sendSuccessResponse and the mobile app expects payload to be the array.
	response.OK(c, views)
}

// Dashboard handles GET /video-card/dashboard.
func (h *Handler) Dashboard(c *gin.Context) {
	var q DashboardQuery
	_ = c.ShouldBindQuery(&q)
	if q.Limit <= 0 {
		q.Limit = 20
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	userID, email := userContext(c)
	cards, err := h.service.Dashboard(c.Request.Context(), userID, email, q.Page, q.Limit)
	if err != nil {
		_ = c.Error(err)
		return
	}
	views := make([]OwnerView, len(cards))
	for i, card := range cards {
		views[i] = card.ToOwnerView()
	}
	response.OK(c, views)
}

// FullDetails handles GET /video-card/:id/full-details.
func (h *Handler) FullDetails(c *gin.Context) {
	userID, email := userContext(c)
	view, err := h.service.GetFullDetails(c.Request.Context(), c.Param("id"), userID, email)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, view)
}

// PublicDetails handles GET /video-card/:id/public-details.
func (h *Handler) PublicDetails(c *gin.Context) {
	card, err := h.service.GetPublicByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, card.ToWatchView())
}

// GetByShareToken handles GET /video-card/share/:shareToken.
func (h *Handler) GetByShareToken(c *gin.Context) {
	card, err := h.service.GetByShareToken(c.Request.Context(), c.Param("shareToken"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, card.ToPublicView())
}

// Update handles PATCH /video-card/:id.
func (h *Handler) Update(c *gin.Context) {
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID, email := userContext(c)
	card, err := h.service.Update(c.Request.Context(), c.Param("id"), userID, email, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, card.ToOwnerView())
}

// UpdateOverlay handles PATCH /video-card/:id/overlay.
func (h *Handler) UpdateOverlay(c *gin.Context) {
	var req UpdateOverlayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID, email := userContext(c)
	card, err := h.service.UpdateOverlay(c.Request.Context(), c.Param("id"), userID, email, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, card.ToOwnerView())
}

// CoverUploadURL handles POST /video-card/:id/cover/upload-url.
func (h *Handler) CoverUploadURL(c *gin.Context) {
	var req CoverUploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID, email := userContext(c)
	view, err := h.service.CoverUploadURL(c.Request.Context(), c.Param("id"), userID, email, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, view)
}

// SetCover handles PUT /video-card/:id/cover.
func (h *Handler) SetCover(c *gin.Context) {
	var req SetCoverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID, email := userContext(c)
	card, err := h.service.SetCover(c.Request.Context(), c.Param("id"), userID, email, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, card.ToOwnerView())
}

// ClearCover handles DELETE /video-card/:id/cover.
func (h *Handler) ClearCover(c *gin.Context) {
	userID, email := userContext(c)
	card, err := h.service.ClearCover(c.Request.Context(), c.Param("id"), userID, email)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, card.ToOwnerView())
}

// Finalize handles POST /video-card/:id/finalize.
func (h *Handler) Finalize(c *gin.Context) {
	userID, email := userContext(c)
	if err := h.service.Finalize(c.Request.Context(), c.Param("id"), userID, email); err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"message": "card finalisation queued"})
}

// GeneratePreview handles POST /video-card/:id/preview.
func (h *Handler) GeneratePreview(c *gin.Context) {
	userID, email := userContext(c)
	if err := h.service.GeneratePreview(c.Request.Context(), c.Param("id"), userID, email); err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"message": "preview generation queued"})
}

// Archive handles PATCH /video-card/:id/archive.
func (h *Handler) Archive(c *gin.Context) {
	id := c.Param("id")
	userID, email := userContext(c)
	if err := h.service.Archive(c.Request.Context(), id, userID, email); err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"id": id, "is_archived": true})
}

// Unarchive handles PATCH /video-card/:id/unarchive.
func (h *Handler) Unarchive(c *gin.Context) {
	id := c.Param("id")
	userID, email := userContext(c)
	if err := h.service.Unarchive(c.Request.Context(), id, userID, email); err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"id": id, "is_archived": false})
}

// Delete handles DELETE /video-card/:id.
func (h *Handler) Delete(c *gin.Context) {
	userID, email := userContext(c)
	if err := h.service.Delete(c.Request.Context(), c.Param("id"), userID, email); err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListCollaborators handles GET /video-card/:id/collaborators.
func (h *Handler) ListCollaborators(c *gin.Context) {
	userID, email := userContext(c)
	collabs, err := h.service.ListCollaborators(c.Request.Context(), c.Param("id"), userID, email)
	if err != nil {
		_ = c.Error(err)
		return
	}
	views := make([]CollaboratorView, len(collabs))
	for i, col := range collabs {
		views[i] = CollaboratorView{
			ID:          col.ID,
			VideoCardID: col.VideoCardID,
			Email:       col.Email,
			Name:        col.Name,
			InvitedAt:   col.InvitedAt.Format(time.RFC3339),
		}
	}
	response.OK(c, views)
}

// AddCollaborator handles POST /video-card/:id/collaborators.
func (h *Handler) AddCollaborator(c *gin.Context) {
	var req AddCollaboratorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID, email := userContext(c)
	col, err := h.service.AddCollaborator(c.Request.Context(), c.Param("id"), userID, email, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, CollaboratorView{
		ID:          col.ID,
		VideoCardID: col.VideoCardID,
		Email:       col.Email,
		Name:        col.Name,
		InvitedAt:   col.InvitedAt.Format(time.RFC3339),
	})
}

// RemoveCollaborator handles DELETE /video-card/:id/collaborators/:collaboratorId.
func (h *Handler) RemoveCollaborator(c *gin.Context) {
	userID, email := userContext(c)
	if err := h.service.RemoveCollaborator(
		c.Request.Context(),
		c.Param("id"),
		c.Param("collaboratorId"),
		userID, email,
	); err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"success": true})
}

// GetStatus handles GET /video-card/:id/status.
func (h *Handler) GetStatus(c *gin.Context) {
	userID, email := userContext(c)
	status, err := h.service.GetStatus(c.Request.Context(), c.Param("id"), userID, email)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, status)
}

// GetDownloadURL handles GET /video-card/:id/download.
func (h *Handler) GetDownloadURL(c *gin.Context) {
	userID, email := userContext(c)
	dl, err := h.service.GetDownloadURL(c.Request.Context(), c.Param("id"), userID, email)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, dl)
}

// GetActivity handles GET /video-card/:id/activity.
func (h *Handler) GetActivity(c *gin.Context) {
	userID, email := userContext(c)
	entries, err := h.service.GetActivity(c.Request.Context(), c.Param("id"), userID, email)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, entries)
}

// GenerateWish handles POST /video-card/:id/generate-wish.
func (h *Handler) GenerateWish(c *gin.Context) {
	var req GenerateWishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID, email := userContext(c)
	wish, err := h.service.GenerateWish(c.Request.Context(), c.Param("id"), userID, email, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"message": wish})
}
