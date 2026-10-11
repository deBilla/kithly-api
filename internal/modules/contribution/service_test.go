package contribution

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/modules/transcoderclient"
)

type fakeRepo struct {
	contribs []*Entity
	deleted  []string
	ordered  map[string]int
}

func (r *fakeRepo) GetByID(_ context.Context, id string) (*Entity, error) {
	for _, c := range r.contribs {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, apperr.NotFound("contribution")
}

func (r *fakeRepo) ListByCard(_ context.Context, cardID string) ([]*Entity, error) {
	var out []*Entity
	for _, c := range r.contribs {
		if c.VideoCardID == cardID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *fakeRepo) Create(_ context.Context, p CreateParams) (*Entity, error) {
	c := &Entity{ID: "new", VideoCardID: p.VideoCardID, ContributorName: p.ContributorName}
	if p.ContributorUserID != nil {
		c.ContributorUserID = *p.ContributorUserID
	}
	r.contribs = append(r.contribs, c)
	return c, nil
}

func (r *fakeRepo) Update(ctx context.Context, id string, _ UpdateParams) (*Entity, error) {
	return r.GetByID(ctx, id)
}

func (r *fakeRepo) UpdateOrder(_ context.Context, id string, order int) error {
	if r.ordered == nil {
		r.ordered = map[string]int{}
	}
	r.ordered[id] = order
	return nil
}

func (r *fakeRepo) Delete(_ context.Context, id string) error {
	r.deleted = append(r.deleted, id)
	return nil
}

// fakeGate: "owner" manages every card; everyone else manages none.
type fakeGate struct {
	cards map[string]*CardInfo
}

func (g *fakeGate) CheckAccess(_ context.Context, cardID, userID, _ string) error {
	if _, ok := g.cards[cardID]; !ok {
		return apperr.NotFound("card")
	}
	if userID == "owner" {
		return nil
	}
	return apperr.Forbidden("access denied")
}

func (g *fakeGate) GetCardInfo(_ context.Context, cardID string) (*CardInfo, error) {
	if info, ok := g.cards[cardID]; ok {
		return info, nil
	}
	return nil, apperr.NotFound("card")
}

type fakePresigner struct{}

func (fakePresigner) PresignedUploadURL(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	return "https://s3.example/" + key, nil
}

type fakeTranscoder struct{}

func (fakeTranscoder) EnqueueContributionIngest(context.Context, transcoderclient.ContributionIngestInput) error {
	return nil
}
func (fakeTranscoder) EnqueueCardPreview(context.Context, transcoderclient.CardTranscodeInput) error {
	return nil
}
func (fakeTranscoder) EnqueueCardCompilation(context.Context, transcoderclient.CardTranscodeInput) error {
	return nil
}

func newTestService() (*service, *fakeRepo) {
	repo := &fakeRepo{contribs: []*Entity{
		{ID: "c1", VideoCardID: "open"},
		{ID: "c2", VideoCardID: "open"},
		{ID: "x1", VideoCardID: "full"},
		{ID: "x2", VideoCardID: "full"},
	}}
	gate := &fakeGate{cards: map[string]*CardInfo{
		"open":      {Status: "collecting"},
		"rendering": {Status: "processing"},
		"done":      {Status: "completed"},
		"expired":   {Status: "expired"},
		"full":      {Status: "collecting", MaxContributors: 2},
	}}
	return &service{repo: repo, cards: gate, s3: fakePresigner{}, transcoder: fakeTranscoder{}}, repo
}

func isStatus(err error, sentinel error) bool {
	var ae *apperr.AppError
	return errors.As(err, &ae) && errors.Is(ae.Err, sentinel)
}

func TestGetUploadURL(t *testing.T) {
	tests := []struct {
		name    string
		card    string
		ct      string
		size    int64
		user    string
		wantErr error
	}{
		{name: "anonymous on a collecting card", card: "open", ct: "video/mp4"},
		{name: "images are accepted", card: "open", ct: "image/jpeg"},
		{name: "svg is rejected", card: "open", ct: "image/svg+xml", wantErr: apperr.ErrBadRequest},
		{name: "arbitrary types are rejected", card: "open", ct: "application/zip", wantErr: apperr.ErrBadRequest},
		{name: "oversized file", card: "open", ct: "video/mp4", size: maxUploadBytes + 1, wantErr: apperr.ErrLimitExceeded},
		{name: "unknown card", card: "ghost", ct: "video/mp4", wantErr: apperr.ErrNotFound},
		{name: "anonymous while the video renders", card: "rendering", ct: "video/mp4"},
		{name: "anonymous on a finished card", card: "done", ct: "video/mp4"},
		{name: "anonymous on an expired card", card: "expired", ct: "video/mp4", wantErr: apperr.ErrForbidden},
		{name: "anonymous on a full card", card: "full", ct: "video/mp4", wantErr: apperr.ErrLimitExceeded},
		{name: "organizer may add to a finished card", card: "done", ct: "image/jpeg", user: "owner"},
		{name: "organizer may add to a full card", card: "full", ct: "video/mp4", user: "owner"},
		{name: "organizer may add to an expired card", card: "expired", ct: "video/mp4", user: "owner"},
		{name: "signed-in non-organizer follows contributor rules", card: "expired", ct: "video/mp4", user: "stranger", wantErr: apperr.ErrForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService()
			res, err := svc.GetUploadURL(context.Background(), UploadURLRequest{
				VideoCardID: tt.card, ContentType: tt.ct, FileSize: tt.size,
			}, tt.user, "")
			if tt.wantErr != nil {
				assert.True(t, isStatus(err, tt.wantErr), "got %v", err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, res.S3Key, "contributions/"+tt.card+"/")
		})
	}
}

func TestConfirmUpload(t *testing.T) {
	tests := []struct {
		name    string
		card    string
		key     string
		user    string
		wantErr error
	}{
		{name: "key under the card", card: "open", key: "contributions/open/a/original_b"},
		{name: "key from another card", card: "open", key: "contributions/done/a/original_b", wantErr: apperr.ErrBadRequest},
		{name: "arbitrary key", card: "open", key: "private/secret.mp4", wantErr: apperr.ErrBadRequest},
		{name: "path traversal", card: "open", key: "contributions/open/../done/x", wantErr: apperr.ErrBadRequest},
		{name: "finished card", card: "done", key: "contributions/done/a/b"},
		{name: "expired card", card: "expired", key: "contributions/expired/a/b", wantErr: apperr.ErrForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService()
			_, err := svc.ConfirmUpload(context.Background(), ConfirmUploadRequest{
				VideoCardID: tt.card, S3Key: tt.key, ContributorName: "Aunt May",
			}, tt.user, "")
			if tt.wantErr != nil {
				assert.True(t, isStatus(err, tt.wantErr), "got %v", err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestConfirmUpload_RecordsSignedInContributor(t *testing.T) {
	svc, _ := newTestService()
	c, err := svc.ConfirmUpload(context.Background(), ConfirmUploadRequest{
		VideoCardID: "open", S3Key: "contributions/open/a/b", ContributorName: "Me",
	}, "owner", "")
	require.NoError(t, err)
	assert.Equal(t, "owner", c.ContributorUserID)
}

func TestManagementRequiresCardAccess(t *testing.T) {
	ctx := context.Background()

	t.Run("list", func(t *testing.T) {
		svc, _ := newTestService()
		_, err := svc.List(ctx, "open", "stranger", "")
		assert.True(t, isStatus(err, apperr.ErrForbidden))
		got, err := svc.List(ctx, "open", "owner", "")
		require.NoError(t, err)
		assert.Len(t, got, 2)
	})

	t.Run("delete", func(t *testing.T) {
		svc, repo := newTestService()
		err := svc.Delete(ctx, "c1", "stranger", "")
		assert.True(t, isStatus(err, apperr.ErrForbidden))
		assert.Empty(t, repo.deleted)
		require.NoError(t, svc.Delete(ctx, "c1", "owner", ""))
		assert.Equal(t, []string{"c1"}, repo.deleted)
	})

	t.Run("update", func(t *testing.T) {
		svc, _ := newTestService()
		status := "excluded"
		_, err := svc.Update(ctx, "c1", "stranger", "", UpdateRequest{Status: &status})
		assert.True(t, isStatus(err, apperr.ErrForbidden))
	})

	t.Run("reorder rejects contributions from other cards", func(t *testing.T) {
		svc, repo := newTestService()
		err := svc.BatchReorder(ctx, BatchReorderRequest{VideoCardID: "open", IDs: []string{"c1", "x1"}}, "owner", "")
		assert.True(t, isStatus(err, apperr.ErrBadRequest))
		assert.Empty(t, repo.ordered)
	})

	t.Run("reorder own card", func(t *testing.T) {
		svc, repo := newTestService()
		require.NoError(t, svc.BatchReorder(ctx, BatchReorderRequest{VideoCardID: "open", IDs: []string{"c2", "c1"}}, "owner", ""))
		assert.Equal(t, map[string]int{"c2": 0, "c1": 1}, repo.ordered)
	})
}
