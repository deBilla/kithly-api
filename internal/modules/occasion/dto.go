package occasion

import "encoding/json"

// CreateRequest is the validated request body for POST /occasion/.
type CreateRequest struct {
	ID              string          `json:"id"               binding:"required"`
	Label           string          `json:"label"            binding:"required"`
	Emoji           string          `json:"emoji"`
	ContributorTips json.RawMessage `json:"contributor_tips"`
	MessageTemplate string          `json:"message_template" binding:"required"`
	SortOrder       int             `json:"sort_order"`
}

// UpdateRequest is the validated request body for PUT /occasion/.
type UpdateRequest struct {
	ID              string          `json:"id"               binding:"required"`
	Label           *string         `json:"label"`
	Emoji           *string         `json:"emoji"`
	ContributorTips json.RawMessage `json:"contributor_tips"`
	MessageTemplate *string         `json:"message_template"`
	SortOrder       *int32          `json:"sort_order"`
}
