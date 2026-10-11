// Package card implements the video card domain — the central entity of the
// Kithly platform.
package card

import (
	"encoding/json"
	"time"

	"github.com/kithly/kithly-ws-go/pkg/s3"
)

// Entity represents a video card with all its associated metadata.
type Entity struct {
	ID                     string
	OrganizerID            string
	Title                  string
	Occasion               string
	Description            string
	ShareToken             string
	Status                 string
	MusicThemeID           string
	VideoOverlayID         string
	MaxContributors        int
	IsLocked               bool
	FinalVideoURL          string
	PreviewURL             string
	PreviewStatus          string
	LastPreviewGeneratedAt *time.Time
	FinalHLSPlaylistURL    string
	PreviewHLSPlaylistURL  string
	ThumbnailURL           string
	FinalDuration          float64
	IsArchived             bool
	CreatedAt              time.Time
	ProcessedAt            *time.Time
	ExpiresAt              *time.Time
	UpdatedAt              time.Time

	// Cover frame. The URL fields hold bare S3 keys; CoverConfig is the app's
	// editor state, stored opaquely so the cover can be re-edited.
	CoverPhotoURL string
	CoverImageURL string
	CoverConfig   json.RawMessage

	// ContributionCount is the number of included contributions. Only the
	// dashboard fills it in; elsewhere it is zero.
	ContributionCount int

	// IsCollaborator is set at query time to indicate the requesting user's role.
	// false = organizer, true = co-organizer/collaborator.
	IsCollaborator bool
}

// OwnerView is the full card view for organizers and collaborators.
type OwnerView struct {
	ID                    string  `json:"id"`
	OrganizerID           string  `json:"organizer_id"`
	Title                 string  `json:"title"`
	Occasion              string  `json:"occasion"`
	Description           string  `json:"description"`
	ShareToken            string  `json:"share_token"`
	Status                string  `json:"status"`
	MusicThemeID          string  `json:"music_theme_id"`
	VideoOverlayID        string  `json:"video_overlay_id"`
	MaxContributors       int     `json:"max_contributors"`
	IsLocked              bool    `json:"is_locked"`
	FinalVideoURL         string  `json:"final_video_url"`
	PreviewURL            string  `json:"preview_url"`
	PreviewStatus         string  `json:"preview_status"`
	FinalHLSPlaylistURL   string  `json:"final_hls_playlist_url"`
	PreviewHLSPlaylistURL string  `json:"preview_hls_playlist_url"`
	ThumbnailURL          string  `json:"thumbnail_url"`
	FinalDuration         float64 `json:"final_duration"`
	IsArchived            bool    `json:"is_archived"`
	IsCollaborator        bool    `json:"is_collaborator"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`

	CoverPhotoURL string          `json:"cover_photo_url"`
	CoverImageURL string          `json:"cover_image_url"`
	CoverConfig   json.RawMessage `json:"cover_config"`
	// CoverPhotoKey lets the editor re-save a cover without re-uploading the photo.
	CoverPhotoKey string `json:"cover_photo_key"`
	// ContributionCount is the number of included messages (dashboard only).
	ContributionCount int `json:"contribution_count"`
}

// PublicView is the minimal card view for unauthenticated share-link access.
type PublicView struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Occasion       string `json:"occasion"`
	Description    string `json:"description"`
	ShareToken     string `json:"share_token"`
	Status         string `json:"status"`
	FinalVideoURL  string `json:"final_video_url"`
	ThumbnailURL   string `json:"thumbnail_url"`
	FinalDuration  float64 `json:"final_duration"`
	// CoverPhotoURL lets the invite page show who the video is for.
	CoverPhotoURL string `json:"cover_photo_url"`
}

// ToOwnerView converts the entity to its full owner/collaborator API representation.
func (e *Entity) ToOwnerView() OwnerView {
	return OwnerView{
		ID:                    e.ID,
		OrganizerID:           e.OrganizerID,
		Title:                 e.Title,
		Occasion:              e.Occasion,
		Description:           e.Description,
		ShareToken:            e.ShareToken,
		Status:                e.Status,
		MusicThemeID:          e.MusicThemeID,
		VideoOverlayID:        e.VideoOverlayID,
		MaxContributors:       e.MaxContributors,
		IsLocked:              e.IsLocked,
		FinalVideoURL:         s3.CloudFrontURL(e.FinalVideoURL),
		PreviewURL:            s3.CloudFrontURL(e.PreviewURL),
		PreviewStatus:         e.PreviewStatus,
		FinalHLSPlaylistURL:   s3.CloudFrontURL(e.FinalHLSPlaylistURL),
		PreviewHLSPlaylistURL: s3.CloudFrontURL(e.PreviewHLSPlaylistURL),
		ThumbnailURL:          s3.CloudFrontURL(e.ThumbnailURL),
		FinalDuration:         e.FinalDuration,
		IsArchived:            e.IsArchived,
		IsCollaborator:        e.IsCollaborator,
		CreatedAt:             e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             e.UpdatedAt.Format(time.RFC3339),
		CoverPhotoURL:         s3.CloudFrontURL(e.CoverPhotoURL),
		CoverImageURL:         s3.CloudFrontURL(e.CoverImageURL),
		CoverConfig:           coverConfigOrNull(e.CoverConfig),
		CoverPhotoKey:         e.CoverPhotoURL,
		ContributionCount:     e.ContributionCount,
	}
}

// coverConfigOrNull keeps the JSON field valid when no cover is set;
// an empty json.RawMessage would otherwise fail to marshal.
func coverConfigOrNull(c json.RawMessage) json.RawMessage {
	if len(c) == 0 {
		return json.RawMessage("null")
	}
	return c
}

// StatusView is the response for GET /video-card/:id/status.
type StatusView struct {
	Status        string          `json:"status"`
	FinalVideoURL string          `json:"final_video_url"`
	PreviewURL    string          `json:"preview_url"`
	PreviewStatus string          `json:"preview_status"`
	Metadata      json.RawMessage `json:"metadata"`
}

// DownloadURLView is the response for GET /video-card/:id/download.
type DownloadURLView struct {
	DownloadURL string `json:"download_url"`
	ExpiresIn   int    `json:"expires_in"`
	ExpiresAt   string `json:"expires_at"`
}

// ContribView is the contribution representation returned inside full-details.
// Field names match the TypeScript toTransformedObject() output.
type ContribView struct {
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

// FullDetailsView is the response for GET /video-card/:id/full-details.
type FullDetailsView struct {
	Card          OwnerView     `json:"card"`
	Contributions []ContribView `json:"contributions"`
}

// ActivityEntry is a single item in the card activity log.
type ActivityEntry struct {
	Type            string `json:"type"`
	ContributorName string `json:"contributor_name"`
	Message         string `json:"message"`
	CreatedAt       string `json:"created_at"`
	ContributionID  string `json:"contribution_id"`
	Status          string `json:"status"`
}

// WatchView is returned by GET /video-card/:id/public-details, which backs the
// shareable watch page (kithly.app/preview/:id, /final/:id). It adds the video
// URLs to PublicView. It is only reachable by card ID, never by share token,
// so contributors holding the invite link cannot watch other messages early.
type WatchView struct {
	PublicView
	PreviewStatus         string `json:"preview_status"`
	PreviewURL            string `json:"preview_url"`
	PreviewHLSPlaylistURL string `json:"preview_hls_playlist_url"`
	FinalHLSPlaylistURL   string `json:"final_hls_playlist_url"`
}

// ToWatchView converts the entity to the watch-page representation.
func (e *Entity) ToWatchView() WatchView {
	return WatchView{
		PublicView:            e.ToPublicView(),
		PreviewStatus:         e.PreviewStatus,
		PreviewURL:            s3.CloudFrontURL(e.PreviewURL),
		PreviewHLSPlaylistURL: s3.CloudFrontURL(e.PreviewHLSPlaylistURL),
		FinalHLSPlaylistURL:   s3.CloudFrontURL(e.FinalHLSPlaylistURL),
	}
}

// ToPublicView converts the entity to the minimal public representation.
func (e *Entity) ToPublicView() PublicView {
	return PublicView{
		ID:            e.ID,
		Title:         e.Title,
		Occasion:      e.Occasion,
		Description:   e.Description,
		ShareToken:    e.ShareToken,
		Status:        e.Status,
		FinalVideoURL: s3.CloudFrontURL(e.FinalVideoURL),
		ThumbnailURL:  s3.CloudFrontURL(e.ThumbnailURL),
		FinalDuration: e.FinalDuration,
		CoverPhotoURL: s3.CloudFrontURL(e.CoverPhotoURL),
	}
}
