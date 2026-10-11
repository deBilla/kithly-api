package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/core/response"
	"github.com/kithly/kithly-ws-go/internal/middleware"
)

// Handler holds the HTTP handlers for the user module.
type Handler struct {
	service Service
}

// NewHandler constructs a Handler with the given Service.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Register handles POST /user/.
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	req.IDToken = middleware.BearerToken(c)
	u, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, gin.H{"user": u.ToView()})
}

// Login handles POST /user/login.
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	u, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, AuthResponse{User: u.ToView()})
}

// SocialLogin handles POST /user/social-login.
func (h *Handler) SocialLogin(c *gin.Context) {
	var req SocialLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	req.IDToken = middleware.BearerToken(c)
	u, err := h.service.SocialLogin(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, AuthResponse{User: u.ToView()})
}

// EmailLogin handles POST /user/email-login.
func (h *Handler) EmailLogin(c *gin.Context) {
	var req EmailLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	req.IDToken = middleware.BearerToken(c)
	u, err := h.service.EmailLogin(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, AuthResponse{User: u.ToView()})
}

// CreateGuest handles POST /user/guest.
func (h *Handler) CreateGuest(c *gin.Context) {
	u, err := h.service.CreateGuest(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	// The guest token is the guest's only credential; the client must store
	// it and send it as X-Guest-Token.
	response.Created(c, AuthResponse{User: u.ToView(), Token: u.GuestToken})
}

// ConvertGuest handles POST /user/convert-guest.
func (h *Handler) ConvertGuest(c *gin.Context) {
	var req ConvertGuestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	req.IDToken = middleware.BearerToken(c)
	userID := c.GetString(middleware.ContextKeyUserID)
	u, err := h.service.ConvertGuest(c.Request.Context(), userID, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"user": u.ToView()})
}

// GetMe handles GET /user/me.
func (h *Handler) GetMe(c *gin.Context) {
	userID := c.GetString(middleware.ContextKeyUserID)
	u, err := h.service.GetByID(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"user": u.ToView()})
}

// UpdateMe handles PUT /user/me.
func (h *Handler) UpdateMe(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID := c.GetString(middleware.ContextKeyUserID)
	u, err := h.service.UpdateProfile(c.Request.Context(), userID, req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, gin.H{"user": u.ToView()})
}

// RegisterDeviceToken handles POST /user/device-token.
func (h *Handler) RegisterDeviceToken(c *gin.Context) {
	var req RegisterDeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperr.BadRequest(err.Error()))
		return
	}
	userID := c.GetString(middleware.ContextKeyUserID)
	if err := h.service.RegisterDeviceToken(c.Request.Context(), userID, req); err != nil {
		_ = c.Error(err)
		return
	}
	response.NoContent(c)
}

// List handles GET /user/ (admin).
func (h *Handler) List(c *gin.Context) {
	users, err := h.service.List(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	views := make([]View, len(users))
	for i, u := range users {
		views[i] = u.ToView()
	}
	response.OK(c, views)
}

// GetOne handles GET /user/:id.
func (h *Handler) GetOne(c *gin.Context) {
	if !isSelf(c) {
		_ = c.Error(apperr.Forbidden("cannot read another user"))
		return
	}
	u, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, u.ToView())
}

// Delete handles DELETE /user/:id.
func (h *Handler) Delete(c *gin.Context) {
	if !isSelf(c) {
		_ = c.Error(apperr.Forbidden("cannot delete another user"))
		return
	}
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		_ = c.Error(err)
		return
	}
	c.Status(http.StatusNoContent)
}

// isSelf reports whether the :id path parameter is the authenticated user.
func isSelf(c *gin.Context) bool {
	id := c.Param("id")
	return id != "" && id == c.GetString(middleware.ContextKeyUserID)
}
