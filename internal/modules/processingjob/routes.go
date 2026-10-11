package processingjob

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts processing job endpoints onto the provided router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, auth gin.HandlerFunc) {
	g := rg.Group("/video-processing-job", auth)
	g.GET("/latest", h.GetLatest)
	g.POST("/retry/:id", h.Retry)
}
