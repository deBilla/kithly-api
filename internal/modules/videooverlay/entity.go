// Package videooverlay implements the video overlay reference data module.
package videooverlay

import (
	"encoding/json"
	"time"

	"github.com/kithly/kithly-ws-go/pkg/s3"
)

// Entity represents a visual overlay template that can be applied to a video card.
type Entity struct {
	ID           string
	Label        string
	ResourceURL  string // bare S3 key
	ThumbnailURL string // bare S3 key, may be empty
	OverlayType  string
	Category     string
	Config       json.RawMessage
	IsActive     bool
	SortOrder    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// View is the snake_case representation returned to API clients.
type View struct {
	ID           string          `json:"id"`
	Label        string          `json:"label"`
	ResourceURL  string          `json:"resource_url"`
	ThumbnailURL string          `json:"thumbnail_url"`
	OverlayType  string          `json:"overlay_type"`
	Category     string          `json:"category"`
	Config       json.RawMessage `json:"config"`
	IsActive     bool            `json:"is_active"`
	SortOrder    int             `json:"sort_order"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

// ToView converts the entity to its API representation with CDN-prefixed URLs.
func (e *Entity) ToView() View {
	return View{
		ID:           e.ID,
		Label:        e.Label,
		ResourceURL:  s3.CloudFrontURL(e.ResourceURL),
		ThumbnailURL: s3.CloudFrontURL(e.ThumbnailURL),
		OverlayType:  e.OverlayType,
		Category:     e.Category,
		Config:       e.Config,
		IsActive:     e.IsActive,
		SortOrder:    e.SortOrder,
		CreatedAt:    e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    e.UpdatedAt.Format(time.RFC3339),
	}
}
