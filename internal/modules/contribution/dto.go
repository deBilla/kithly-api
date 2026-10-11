package contribution

// UploadURLRequest is the request body for POST /video-contribution/upload-url.
type UploadURLRequest struct {
	VideoCardID string `json:"video_card_id" binding:"required"`
	ContentType string `json:"content_type"  binding:"required"`
	FileSize    int64  `json:"file_size"`
}

// UploadURLResponse contains the pre-signed S3 URL and the S3 key.
// Field names match the TypeScript API: uploadUrl and key.
type UploadURLResponse struct {
	UploadURL string `json:"uploadUrl"`
	S3Key     string `json:"key"`
}

// ConfirmUploadRequest is the request body for POST /video-contribution/.
type ConfirmUploadRequest struct {
	VideoCardID      string  `json:"video_card_id"      binding:"required"`
	S3Key            string  `json:"video_url"          binding:"required"`
	ContributorName  string  `json:"contributor_name"   binding:"required"`
	ContributorEmail string  `json:"contributor_email"`
	DurationSeconds  float64 `json:"duration_seconds"`
}

// UpdateRequest is the request body for PUT /video-contribution/:id.
type UpdateRequest struct {
	Status       *string `json:"status"`
	DisplayOrder *int32  `json:"display_order"`
}

// BatchReorderRequest is the request body for PUT /video-contribution/batch-reorder.
type BatchReorderRequest struct {
	VideoCardID string   `json:"video_card_id"     binding:"required"`
	IDs         []string `json:"contribution_ids"  binding:"required"`
}
