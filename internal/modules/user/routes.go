package user

import (
	"github.com/gin-gonic/gin"
	"github.com/kithly/kithly-ws-go/internal/middleware"
)

// RegisterRoutes mounts all user endpoints onto the provided router group.
// admin guards the user listing, which exposes every account.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, auth, admin gin.HandlerFunc) {
	g := rg.Group("/user")

	// Unauthenticated endpoints.
	g.POST("", h.Register)
	g.POST("/login", h.Login)
	g.POST("/social-login", h.SocialLogin)
	g.POST("/email-login", h.EmailLogin)
	g.POST("/guest", h.CreateGuest)

	// Authenticated endpoints.
	authenticated := g.Group("", auth)
	authenticated.POST("/convert-guest", h.ConvertGuest)
	authenticated.GET("/me", h.GetMe)
	authenticated.PUT("/me", h.UpdateMe)
	authenticated.PATCH("", h.UpdateMe)
	authenticated.POST("/device-token", h.RegisterDeviceToken)
	authenticated.GET("/:id", h.GetOne)
	authenticated.DELETE("/:id", h.Delete) // self only; enforced in the handler

	g.GET("", admin, h.List)

	_ = middleware.ContextKeyUserID // ensure package is imported
}
