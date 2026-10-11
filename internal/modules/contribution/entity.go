// Package contribution implements the video contribution domain.
package contribution

import (
	"time"

	"github.com/kithly/kithly-ws-go/pkg/s3"
)

// Entity represents a single video or image contribution to a video card.
type Entity struct {
	ID                string
	VideoCardID       string
	ContributorUserID string
	ContributorName   string
	ContributorEmail  string
	VideoURL          string  // bare S3 key
	ThumbnailURL      string  // bare S3 key
	HLSPlaylistURL    string  // bare S3 key
	OriginalURL       string  // bare S3 key
	DurationSeconds   float64 // guard: values < 0.5 treated as unset
	Status            string
	DisplayOrder      int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// View is the snake_case representation returned to API clients.
type View struct {
	ID                string  `json:"id"`
	VideoCardID       string  `json:"video_card_id"`
	ContributorUserID string  `json:"contributor_user_id"`
	ContributorName   string  `json:"contributor_name"`
	ContributorEmail  string  `json:"contributor_email"`
	VideoURL          string  `json:"video_url"`
	ThumbnailURL      string  `json:"thumbnail_url"`
	HLSPlaylistURL    string  `json:"hls_playlist_url"`
	DurationSeconds   float64 `json:"duration_seconds"`
	Status            string  `json:"status"`
	DisplayOrder      int     `json:"display_order"`
	CreatedAt         string  `json:"created_at"`
}

// ToView converts the entity to its API representation with CDN-prefixed URLs.
// DurationSeconds values below 0.5 are treated as unset and returned as 0.
func (e *Entity) ToView() View {
	dur := e.DurationSeconds
	if dur < 0.5 {
		dur = 0
	}
	return View{
		ID:                e.ID,
		VideoCardID:       e.VideoCardID,
		ContributorUserID: e.ContributorUserID,
		ContributorName:   e.ContributorName,
		ContributorEmail:  e.ContributorEmail,
		VideoURL:          s3.CloudFrontURL(e.VideoURL),
		ThumbnailURL:      s3.CloudFrontURL(e.ThumbnailURL),
		HLSPlaylistURL:    s3.CloudFrontURL(e.HLSPlaylistURL),
		DurationSeconds:   dur,
		Status:            e.Status,
		DisplayOrder:      e.DisplayOrder,
		CreatedAt:         e.CreatedAt.Format(time.RFC3339),
	}
}
