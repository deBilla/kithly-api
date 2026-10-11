package card

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts all video card endpoints onto the provided router group.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, auth gin.HandlerFunc) {
	g := rg.Group("/video-card")

	// Public endpoints (no auth).
	g.GET("/share/:shareToken", h.GetByShareToken)
	g.GET("/:id/public-details", h.PublicDetails)

	// Authenticated endpoints.
	a := g.Group("", auth)
	a.POST("", h.Create)
	a.GET("/dashboard", h.Dashboard)
	a.GET("/memories", h.Memories)
	a.GET("/:id/full-details", h.FullDetails)
	a.GET("/:id/status", h.GetStatus)
	a.GET("/:id/download", h.GetDownloadURL)
	a.GET("/:id/activity", h.GetActivity)
	a.PATCH("/:id", h.Update)
	a.PATCH("/:id/overlay", h.UpdateOverlay)
	a.POST("/:id/cover/upload-url", h.CoverUploadURL)
	a.PUT("/:id/cover", h.SetCover)
	a.DELETE("/:id/cover", h.ClearCover)
	a.PATCH("/:id/archive", h.Archive)
	a.PATCH("/:id/unarchive", h.Unarchive)
	a.POST("/:id/finalize", h.Finalize)
	a.POST("/:id/preview", h.GeneratePreview)
	a.POST("/:id/generate-wish", h.GenerateWish)
	a.DELETE("/:id", h.Delete)
	a.GET("/:id/collaborators", h.ListCollaborators)
	a.POST("/:id/collaborators", h.AddCollaborator)
	a.DELETE("/:id/collaborators/:collaboratorId", h.RemoveCollaborator)
}
