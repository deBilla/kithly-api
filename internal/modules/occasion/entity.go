// Package occasion implements the occasion reference data module.
package occasion

import (
	"encoding/json"
	"time"
)

// Entity represents an occasion type (e.g., "birthday", "wedding").
type Entity struct {
	ID              string
	Label           string
	Emoji           string
	ContributorTips json.RawMessage
	MessageTemplate string
	SortOrder       int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// View is the snake_case representation returned to API clients.
type View struct {
	ID              string          `json:"id"`
	Label           string          `json:"label"`
	Emoji           string          `json:"emoji"`
	ContributorTips json.RawMessage `json:"contributor_tips"`
	MessageTemplate string          `json:"message_template"`
	SortOrder       int             `json:"sort_order"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

// ToView converts the entity to its API representation.
func (e *Entity) ToView() View {
	return View{
		ID:              e.ID,
		Label:           e.Label,
		Emoji:           e.Emoji,
		ContributorTips: e.ContributorTips,
		MessageTemplate: e.MessageTemplate,
		SortOrder:       e.SortOrder,
		CreatedAt:       e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       e.UpdatedAt.Format(time.RFC3339),
	}
}
