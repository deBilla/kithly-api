package musictheme

// CreateRequest is the validated request body for POST /music-theme/.
type CreateRequest struct {
	Label     string `json:"label"      binding:"required"`
	Emoji     string `json:"emoji"      binding:"required"`
	AudioURL  string `json:"audio_url"  binding:"required"`
	Category  string `json:"category"`
	IsActive  bool   `json:"is_active"`
	SortOrder int    `json:"sort_order"`
}

// UpdateRequest is the validated request body for PUT /music-theme/.
// All fields are optional — only non-zero values are applied.
type UpdateRequest struct {
	ID        string  `json:"id"         binding:"required"`
	Label     *string `json:"label"`
	Emoji     *string `json:"emoji"`
	AudioURL  *string `json:"audio_url"`
	Category  *string `json:"category"`
	IsActive  *bool   `json:"is_active"`
	SortOrder *int32  `json:"sort_order"`
}
