// Package processingjob implements the video processing job domain.
package processingjob

import (
	"encoding/json"
	"time"
)

// Entity represents a video_processing_jobs row.
type Entity struct {
	ID           string
	VideoCardID  string
	Status       string
	ErrorMessage string
	Attempts     int
	Metadata     json.RawMessage
	CreatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	RetriedJobID string
}

// IsPreview reads the is_preview flag from the job metadata.
// Metadata keys are snake_case (stored that way by the worker).
func (e *Entity) IsPreview() bool {
	if e.Metadata == nil {
		return false
	}
	var meta struct {
		IsPreview bool `json:"is_preview"`
	}
	if err := json.Unmarshal(e.Metadata, &meta); err != nil {
		return false
	}
	return meta.IsPreview
}

// View is the snake_case representation returned to API clients.
type View struct {
	ID           string          `json:"id"`
	VideoCardID  string          `json:"video_card_id"`
	Status       string          `json:"status"`
	ErrorMessage string          `json:"error_message"`
	Attempts     int             `json:"attempts"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    string          `json:"created_at"`
}

// ToView converts the entity to its API representation.
func (e *Entity) ToView() View {
	return View{
		ID:           e.ID,
		VideoCardID:  e.VideoCardID,
		Status:       e.Status,
		ErrorMessage: e.ErrorMessage,
		Attempts:     e.Attempts,
		Metadata:     e.Metadata,
		CreatedAt:    e.CreatedAt.Format(time.RFC3339),
	}
}
