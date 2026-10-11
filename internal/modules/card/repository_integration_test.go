//go:build integration

package card

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kithly/kithly-ws-go/internal/core/db"
)

// TestUpdateCover_RealPostgres runs the cover write against a real database
// through db.Connect, so it uses the production pgx settings (simple protocol
// for PgBouncer). A fake repository cannot catch encoding bugs like a []byte
// parameter being sent as bytea into a jsonb column.
//
//	TEST_DATABASE_URL=postgres://... go test -tags integration ./internal/modules/card/
func TestUpdateCover_RealPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	var organizerID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO users (name) VALUES ('cover-test') RETURNING id::text`).Scan(&organizerID))
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, organizerID) })

	repo := NewRepository(pool)
	card, err := repo.Create(ctx, CreateCardParams{
		OrganizerID: organizerID, Title: "Dad", Occasion: "birthday", ShareToken: generateShareToken(),
	})
	require.NoError(t, err)

	config := []byte(`{"version":1,"style":"polaroid","eyebrow":"Happy Birthday","headline":"Papa Penguin","subline":"With love ❤️"}`)
	photo := "covers/" + card.ID + "/photo_a.jpg"
	image := "covers/" + card.ID + "/image_a.jpg"

	saved, err := repo.UpdateCover(ctx, card.ID, photo, image, config)
	require.NoError(t, err)
	assert.Equal(t, photo, saved.CoverPhotoURL)
	assert.Equal(t, image, saved.CoverImageURL)
	assert.JSONEq(t, string(config), string(saved.CoverConfig))

	// Round-trip through a fresh read and the API view.
	got, err := repo.GetByID(ctx, card.ID)
	require.NoError(t, err)
	assert.JSONEq(t, string(config), string(got.CoverConfig))
	_, err = json.Marshal(got.ToOwnerView())
	require.NoError(t, err)

	// Clearing stores SQL NULLs.
	cleared, err := repo.UpdateCover(ctx, card.ID, "", "", nil)
	require.NoError(t, err)
	assert.Empty(t, cleared.CoverImageURL)
	assert.Empty(t, cleared.CoverPhotoURL)
	assert.Empty(t, cleared.CoverConfig)
}

// TestCountIncludedContributions_RealPostgres checks the uuid[] parameter
// under the production simple-protocol settings, and that only included
// contributions are counted.
func TestCountIncludedContributions_RealPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	var organizerID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO users (name) VALUES ('count-test') RETURNING id::text`).Scan(&organizerID))
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, organizerID) })

	repo := NewRepository(pool)
	mk := func() string {
		c, err := repo.Create(ctx, CreateCardParams{OrganizerID: organizerID, Title: "x", Occasion: "birthday", ShareToken: generateShareToken()})
		require.NoError(t, err)
		return c.ID
	}
	busy, empty := mk(), mk()
	for _, status := range []string{"included", "included", "excluded", "uploaded"} {
		_, err := pool.Exec(ctx, `INSERT INTO video_contributions (video_card_id, contributor_name, video_url, status) VALUES ($1, 'n', 'k', $2)`, busy, status)
		require.NoError(t, err)
	}

	counts, err := repo.CountIncludedContributions(ctx, []string{busy, empty})
	require.NoError(t, err)
	assert.Equal(t, map[string]int{busy: 2}, counts)

	none, err := repo.CountIncludedContributions(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}
