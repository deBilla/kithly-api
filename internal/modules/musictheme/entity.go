// Package musictheme implements the music theme reference data module.
package musictheme

import (
	"time"

	"github.com/kithly/kithly-ws-go/pkg/s3"
)

// Entity represents a background music option available for video cards.
type Entity struct {
	ID        string
	Label     string
	Emoji     string
	AudioURL  string // bare S3 key — CloudFront prefix added in ToView
	Category  string
	IsActive  bool
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// View is the snake_case representation returned to API clients.
type View struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Emoji     string `json:"emoji"`
	AudioURL  string `json:"audio_url"`
	Category  string `json:"category"`
	IsActive  bool   `json:"is_active"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ToView converts the entity to its API representation, prepending the
// CloudFront CDN prefix to the audio URL.
func (e *Entity) ToView() View {
	return View{
		ID:        e.ID,
		Label:     e.Label,
		Emoji:     e.Emoji,
		AudioURL:  s3.CloudFrontURL(e.AudioURL),
		Category:  e.Category,
		IsActive:  e.IsActive,
		SortOrder: e.SortOrder,
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
		UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
	}
}
