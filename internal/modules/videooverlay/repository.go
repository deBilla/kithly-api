package videooverlay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/core/pghelper"
	"github.com/kithly/kithly-ws-go/internal/core/sqlcdb"
)

// Repository defines the data access contract for video overlays.
type Repository interface {
	List(ctx context.Context) ([]*Entity, error)
	GetByID(ctx context.Context, id string) (*Entity, error)
	Create(ctx context.Context, p CreateParams) (*Entity, error)
	Update(ctx context.Context, id string, p UpdateParams) (*Entity, error)
	Delete(ctx context.Context, id string) error
}

// CreateParams holds validated data for creating a video overlay.
type CreateParams struct {
	Label        string
	ResourceURL  string
	ThumbnailURL string
	OverlayType  string
	Category     string
	Config       json.RawMessage
	IsActive     bool
	SortOrder    int
}

// UpdateParams holds validated data for a partial overlay update.
type UpdateParams struct {
	Label        *string
	ResourceURL  *string
	ThumbnailURL *string
	OverlayType  *string
	Category     *string
	Config       json.RawMessage
	IsActive     *bool
	SortOrder    *int32
}

type repository struct {
	q *sqlcdb.Queries
}

// NewRepository constructs a Repository backed by the given pgx pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{q: sqlcdb.New(pool)}
}

func (r *repository) List(ctx context.Context) ([]*Entity, error) {
	rows, err := r.q.ListVideoOverlays(ctx)
	if err != nil {
		return nil, fmt.Errorf("videooverlay.repository.List: %w", err)
	}
	out := make([]*Entity, len(rows))
	for i, row := range rows {
		out[i] = rowToEntity(row)
	}
	return out, nil
}

func (r *repository) GetByID(ctx context.Context, id string) (*Entity, error) {
	row, err := r.q.GetVideoOverlayByID(ctx, pghelper.UUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("video overlay")
		}
		return nil, fmt.Errorf("videooverlay.repository.GetByID: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Create(ctx context.Context, p CreateParams) (*Entity, error) {
	cfg := []byte(p.Config)
	if cfg == nil {
		cfg = []byte("{}")
	}
	row, err := r.q.CreateVideoOverlay(ctx, sqlcdb.CreateVideoOverlayParams{
		Label:        p.Label,
		ResourceUrl:  p.ResourceURL,
		ThumbnailUrl: pghelper.TextVal(p.ThumbnailURL),
		OverlayType:  p.OverlayType,
		Category:     pghelper.TextVal(p.Category),
		Config:       cfg,
		IsActive:     p.IsActive,
		SortOrder:    int32(p.SortOrder),
	})
	if err != nil {
		return nil, fmt.Errorf("videooverlay.repository.Create: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Update(ctx context.Context, id string, p UpdateParams) (*Entity, error) {
	row, err := r.q.UpdateVideoOverlay(ctx, sqlcdb.UpdateVideoOverlayParams{
		ID:           pghelper.UUID(id),
		Label:        pghelper.Text(p.Label),
		ResourceUrl:  pghelper.Text(p.ResourceURL),
		ThumbnailUrl: pghelper.Text(p.ThumbnailURL),
		OverlayType:  pghelper.Text(p.OverlayType),
		Category:     pghelper.Text(p.Category),
		Config:       []byte(p.Config),
		IsActive:     pghelper.Bool(p.IsActive),
		SortOrder:    pghelper.Int4(p.SortOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("video overlay")
		}
		return nil, fmt.Errorf("videooverlay.repository.Update: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Delete(ctx context.Context, id string) error {
	if err := r.q.DeleteVideoOverlay(ctx, pghelper.UUID(id)); err != nil {
		return fmt.Errorf("videooverlay.repository.Delete: %w", err)
	}
	return nil
}

func rowToEntity(row sqlcdb.VideoOverlay) *Entity {
	return &Entity{
		ID:           pghelper.StringFromUUID(row.ID),
		Label:        row.Label,
		ResourceURL:  row.ResourceUrl,
		ThumbnailURL: pghelper.StringFromText(row.ThumbnailUrl),
		OverlayType:  row.OverlayType,
		Category:     pghelper.StringFromText(row.Category),
		Config:       json.RawMessage(row.Config),
		IsActive:     row.IsActive,
		SortOrder:    int(row.SortOrder),
		CreatedAt:    pghelper.TimeFromTimestamptz(row.CreatedAt),
		UpdatedAt:    pghelper.TimeFromTimestamptz(row.UpdatedAt),
	}
}
