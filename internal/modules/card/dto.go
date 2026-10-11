package card

import "encoding/json"

// CreateRequest is the request body for POST /video-card/.
type CreateRequest struct {
	Title           string  `json:"title"            binding:"required"`
	Occasion        string  `json:"occasion"         binding:"required"`
	Description     string  `json:"description"`
	MusicThemeID    *string `json:"music_theme_id"`
	VideoOverlayID  *string `json:"video_overlay_id"`
	MaxContributors int     `json:"max_contributors"`
}

// UpdateRequest is the request body for PATCH /video-card/:id.
type UpdateRequest struct {
	Title           *string `json:"title"`
	Occasion        *string `json:"occasion"`
	Description     *string `json:"description"`
	MusicThemeID    *string `json:"music_theme_id"`
	VideoOverlayID  *string `json:"video_overlay_id"`
	MaxContributors *int32  `json:"max_contributors"`
	IsLocked        *bool   `json:"is_locked"`
}

// UpdateOverlayRequest is the request body for PATCH /video-card/:id/overlay.
// video_overlay_id may be null to clear the overlay.
type UpdateOverlayRequest struct {
	VideoOverlayID *string `json:"video_overlay_id"`
}

// CoverUploadURLRequest is the request body for POST /video-card/:id/cover/upload-url.
// kind "photo" is the original picture (kept for re-editing); "image" is the
// rendered 1080x1920 cover frame the worker puts first in the video.
type CoverUploadURLRequest struct {
	Kind        string `json:"kind"         binding:"required,oneof=photo image"`
	ContentType string `json:"content_type" binding:"required"`
}

// CoverUploadURLView carries a presigned S3 PUT URL. Field names match the
// contribution upload endpoint so the app can share its upload code.
type CoverUploadURLView struct {
	UploadURL string `json:"uploadUrl"`
	Key       string `json:"key"`
}

// SetCoverRequest is the request body for PUT /video-card/:id/cover.
// Keys must come from the cover upload-url endpoint for the same card.
type SetCoverRequest struct {
	PhotoKey string          `json:"photo_key"`
	ImageKey string          `json:"image_key" binding:"required"`
	Config   json.RawMessage `json:"config"`
}

// AddCollaboratorRequest is the request body for POST /video-card/:id/collaborators.
type AddCollaboratorRequest struct {
	Email string `json:"email" binding:"required,email"`
	Name  string `json:"name"`
}

// GenerateWishRequest is the request body for POST /video-card/:id/generate-wish.
type GenerateWishRequest struct {
	FromName string `json:"from_name" binding:"required"`
	ToName   string `json:"to_name"   binding:"required"`
	Occasion string `json:"occasion"`
	Tone     string `json:"tone"      binding:"required,oneof=heartfelt funny inspirational casual"`
}

// DashboardQuery holds validated query params for GET /video-card/dashboard.
type DashboardQuery struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

// MemoriesQuery holds validated query params for GET /video-card/memories.
type MemoriesQuery struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

// PaginationMeta is the pagination envelope included in list responses.
type PaginationMeta struct {
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Pages int64 `json:"pages"`
}

// CollaboratorView is the API representation of a video_card_collaborator row.
type CollaboratorView struct {
	ID          string `json:"id"`
	VideoCardID string `json:"video_card_id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	InvitedAt   string `json:"invited_at"`
}
