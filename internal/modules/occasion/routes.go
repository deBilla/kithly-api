package occasion

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts all occasion endpoints onto the provided router group.
// Reads are public reference data; writes require admin.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, admin gin.HandlerFunc) {
	g := rg.Group("/occasion")
	g.GET("", h.List)
	g.GET("/:id", h.GetOne)
	g.POST("", admin, h.Create)
	g.PUT("", admin, h.Update)
	g.DELETE("/:id", admin, h.Delete)
}
