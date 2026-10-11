package contribution

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts all video contribution endpoints onto the provided router group.
// optionalAuth identifies organizers who upload their own clips while still
// admitting anonymous contributors.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, auth, optionalAuth gin.HandlerFunc) {
	g := rg.Group("/video-contribution")

	// Contributors may not have accounts, so these accept anonymous callers;
	// the service limits anonymous uploads to cards that are still collecting.
	g.POST("/upload-url", optionalAuth, h.GetUploadURL)
	g.POST("", optionalAuth, h.ConfirmUpload)

	// Authenticated endpoints.
	a := g.Group("", auth)
	a.GET("", h.List)
	a.PUT("/batch-reorder", h.BatchReorder)
	a.PUT("/:id", h.Update)
	a.DELETE("/:id", h.Delete)
}
