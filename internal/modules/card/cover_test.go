package card

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
)

// coverRepo implements the Repository methods the cover flow uses; the
// embedded interface makes any other call panic, flagging unexpected use.
type coverRepo struct {
	Repository
	card  *Entity
	saved struct {
		photo, image string
		config       []byte
		called       bool
	}
}

func (r *coverRepo) GetByID(_ context.Context, id string) (*Entity, error) {
	if r.card == nil || r.card.ID != id {
		return nil, apperr.NotFound("card")
	}
	c := *r.card
	return &c, nil
}

func (r *coverRepo) CollaboratorExists(context.Context, string, string) (bool, error) {
	return false, nil
}

func (r *coverRepo) UpdateCover(_ context.Context, _ string, photo, image string, config []byte) (*Entity, error) {
	r.saved.photo, r.saved.image, r.saved.config, r.saved.called = photo, image, config, true
	c := *r.card
	c.CoverPhotoURL, c.CoverImageURL, c.CoverConfig = photo, image, config
	return &c, nil
}

type fakeUploader struct{ lastKey, lastType string }

func (u *fakeUploader) PresignedUploadURL(_ context.Context, key, ct string, _ time.Duration) (string, error) {
	u.lastKey, u.lastType = key, ct
	return "https://s3.example/" + key, nil
}

func newCoverService(status string) (*service, *coverRepo, *fakeUploader) {
	repo := &coverRepo{card: &Entity{ID: "card1", OrganizerID: "owner", Status: status}}
	up := &fakeUploader{}
	return &service{repo: repo, uploads: up}, repo, up
}

func isStatus(err error, sentinel error) bool {
	var ae *apperr.AppError
	return errors.As(err, &ae) && errors.Is(ae.Err, sentinel)
}

func TestCoverUploadURL(t *testing.T) {
	tests := []struct {
		name, user, status, kind, ct string
		wantExt                      string
		wantErr                      error
	}{
		{name: "photo jpeg", user: "owner", status: "collecting", kind: "photo", ct: "image/jpeg", wantExt: ".jpg"},
		{name: "photo heic", user: "owner", status: "collecting", kind: "photo", ct: "image/heic", wantExt: ".heic"},
		{name: "rendered image png", user: "owner", status: "completed", kind: "image", ct: "image/png", wantExt: ".png"},
		{name: "rendered image cannot be heic", user: "owner", status: "collecting", kind: "image", ct: "image/heic", wantErr: apperr.ErrBadRequest},
		{name: "svg rejected", user: "owner", status: "collecting", kind: "photo", ct: "image/svg+xml", wantErr: apperr.ErrBadRequest},
		{name: "stranger", user: "stranger", status: "collecting", kind: "photo", ct: "image/jpeg", wantErr: apperr.ErrForbidden},
		{name: "while processing", user: "owner", status: "processing", kind: "photo", ct: "image/jpeg", wantErr: apperr.ErrConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, up := newCoverService(tt.status)
			view, err := svc.CoverUploadURL(context.Background(), "card1", tt.user, "", CoverUploadURLRequest{Kind: tt.kind, ContentType: tt.ct})
			if tt.wantErr != nil {
				assert.True(t, isStatus(err, tt.wantErr), "got %v", err)
				return
			}
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(view.Key, "covers/card1/"+tt.kind+"_"), view.Key)
			assert.True(t, strings.HasSuffix(view.Key, tt.wantExt), view.Key)
			assert.Equal(t, view.Key, up.lastKey)
		})
	}
}

func TestSetCover(t *testing.T) {
	const img = "covers/card1/image_abc.jpg"
	const photo = "covers/card1/photo_abc.jpg"
	tests := []struct {
		name    string
		status  string
		req     SetCoverRequest
		wantErr error
	}{
		{name: "image, photo and config", status: "collecting", req: SetCoverRequest{ImageKey: img, PhotoKey: photo, Config: json.RawMessage(`{"headline":"Happy Birthday Dad!"}`)}},
		{name: "image only", status: "completed", req: SetCoverRequest{ImageKey: img}},
		{name: "image key from another card", status: "collecting", req: SetCoverRequest{ImageKey: "covers/card2/image_abc.jpg"}, wantErr: apperr.ErrBadRequest},
		{name: "photo key passed as image", status: "collecting", req: SetCoverRequest{ImageKey: photo}, wantErr: apperr.ErrBadRequest},
		{name: "arbitrary bucket object", status: "collecting", req: SetCoverRequest{ImageKey: "video-cards/card1/final.mp4"}, wantErr: apperr.ErrBadRequest},
		{name: "traversal", status: "collecting", req: SetCoverRequest{ImageKey: "covers/card1/image_../../x.jpg"}, wantErr: apperr.ErrBadRequest},
		{name: "config not an object", status: "collecting", req: SetCoverRequest{ImageKey: img, Config: json.RawMessage(`"hi"`)}, wantErr: apperr.ErrBadRequest},
		{name: "config too large", status: "collecting", req: SetCoverRequest{ImageKey: img, Config: json.RawMessage(`{"a":"` + strings.Repeat("x", maxCoverConfigSize) + `"}`)}, wantErr: apperr.ErrBadRequest},
		{name: "while processing", status: "processing", req: SetCoverRequest{ImageKey: img}, wantErr: apperr.ErrConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, _ := newCoverService(tt.status)
			card, err := svc.SetCover(context.Background(), "card1", "owner", "", tt.req)
			if tt.wantErr != nil {
				assert.True(t, isStatus(err, tt.wantErr), "got %v", err)
				assert.False(t, repo.saved.called)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.req.ImageKey, repo.saved.image)
			assert.Equal(t, tt.req.PhotoKey, repo.saved.photo)

			view := card.ToOwnerView()
			assert.True(t, strings.HasSuffix(view.CoverImageURL, tt.req.ImageKey))
			assert.Equal(t, tt.req.PhotoKey, view.CoverPhotoKey)
			_, err = json.Marshal(view)
			require.NoError(t, err)
		})
	}
}

func TestClearCover(t *testing.T) {
	svc, repo, _ := newCoverService("collecting")
	card, err := svc.ClearCover(context.Background(), "card1", "owner", "")
	require.NoError(t, err)
	assert.True(t, repo.saved.called)
	assert.Empty(t, repo.saved.image)

	// A card without a cover must still serialise (cover_config: null).
	b, err := json.Marshal(card.ToOwnerView())
	require.NoError(t, err)
	assert.Contains(t, string(b), `"cover_config":null`)
}

func TestWatchView_IncludesVideoURLsOnlyForWatchPage(t *testing.T) {
	e := &Entity{
		ID: "card1", Title: "Dad", Status: "collecting", PreviewStatus: "ready",
		PreviewURL: "video-cards/card1/preview.mp4", PreviewHLSPlaylistURL: "video-cards/card1/hls/index.m3u8",
		FinalHLSPlaylistURL: "video-cards/card1/final/index.m3u8",
	}

	watch, err := json.Marshal(e.ToWatchView())
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(watch, &got))
	// Flat JSON (embedded struct), as the watch page reads it.
	assert.Equal(t, "card1", got["id"])
	assert.Equal(t, "ready", got["preview_status"])
	assert.True(t, strings.HasSuffix(got["preview_url"].(string), "video-cards/card1/preview.mp4"))
	assert.True(t, strings.HasSuffix(got["preview_hls_playlist_url"].(string), "index.m3u8"))

	// The share-token (contributor) view must not expose previews.
	public, err := json.Marshal(e.ToPublicView())
	require.NoError(t, err)
	assert.NotContains(t, string(public), "preview")
}
