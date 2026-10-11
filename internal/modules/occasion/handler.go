package occasion

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/core/response"
)

// Handler holds the HTTP handlers for the occasion module.
type Handler struct{ service Service }

// NewHandler constructs a Handler with the given Service.
func NewHandler(service Service) *Handler { return &Handler{service: service} }

// List handles GET /occasion/.
func (h *Handler) List(c *gin.Context) {
	occasions, err := h.service.List(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	views := make([]View, len(occasions))
	for i, o := range occasions {
		views[i] = o.ToView()
	}
	response.OK(c, views)
}

// GetOne handles GET /occasion/:id.
func (h *Handler) GetOne(c *gin.Context) {
	o, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, o.ToView())
}

// Create handles POST /occasion/.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	o, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, o.ToView())
}

// Update handles PUT /occasion/.
func (h *Handler) Update(c *gin.Context) {
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	o, err := h.service.Update(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, o.ToView())
}

// Delete handles DELETE /occasion/:id.
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}
