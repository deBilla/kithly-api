package musictheme

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/core/response"
)

// Handler holds the HTTP handlers for the music theme module.
type Handler struct {
	service Service
}

// NewHandler constructs a Handler with the given Service dependency.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// List handles GET /music-theme/ — returns all active music themes.
func (h *Handler) List(c *gin.Context) {
	themes, err := h.service.List(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	views := make([]View, len(themes))
	for i, t := range themes {
		views[i] = t.ToView()
	}
	response.OK(c, views)
}

// GetOne handles GET /music-theme/:id.
func (h *Handler) GetOne(c *gin.Context) {
	theme, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, theme.ToView())
}

// Create handles POST /music-theme/.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	theme, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, theme.ToView())
}

// Update handles PUT /music-theme/.
func (h *Handler) Update(c *gin.Context) {
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	theme, err := h.service.Update(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, theme.ToView())
}

// Delete handles DELETE /music-theme/:id.
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}
