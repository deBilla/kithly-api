package videooverlay

import "encoding/json"

// CreateRequest is the validated request body for POST /video-overlay/.
type CreateRequest struct {
	Label        string          `json:"label"         binding:"required"`
	ResourceURL  string          `json:"resource_url"  binding:"required"`
	ThumbnailURL string          `json:"thumbnail_url"`
	OverlayType  string          `json:"overlay_type"  binding:"required"`
	Category     string          `json:"category"`
	Config       json.RawMessage `json:"config"`
	IsActive     bool            `json:"is_active"`
	SortOrder    int             `json:"sort_order"`
}

// UpdateRequest is the validated request body for PUT /video-overlay/.
type UpdateRequest struct {
	ID           string          `json:"id"            binding:"required"`
	Label        *string         `json:"label"`
	ResourceURL  *string         `json:"resource_url"`
	ThumbnailURL *string         `json:"thumbnail_url"`
	OverlayType  *string         `json:"overlay_type"`
	Category     *string         `json:"category"`
	Config       json.RawMessage `json:"config"`
	IsActive     *bool           `json:"is_active"`
	SortOrder    *int32          `json:"sort_order"`
}
