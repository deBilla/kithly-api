package contribution

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/core/pghelper"
	"github.com/kithly/kithly-ws-go/internal/core/sqlcdb"
)

// Repository defines the data access contract for video contributions.
type Repository interface {
	GetByID(ctx context.Context, id string) (*Entity, error)
	ListByCard(ctx context.Context, cardID string) ([]*Entity, error)
	Create(ctx context.Context, p CreateParams) (*Entity, error)
	Update(ctx context.Context, id string, p UpdateParams) (*Entity, error)
	UpdateOrder(ctx context.Context, id string, order int) error
	Delete(ctx context.Context, id string) error
}

// CreateParams holds validated data for creating a contribution.
type CreateParams struct {
	VideoCardID       string
	ContributorUserID *string
	ContributorName   string
	ContributorEmail  string
	VideoURL          string
	DurationSeconds   *float64
}

// UpdateParams holds validated data for a partial contribution update.
type UpdateParams struct {
	Status       *string
	DisplayOrder *int32
}

type repository struct {
	q *sqlcdb.Queries
}

// NewRepository constructs a Repository backed by the given pgx pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{q: sqlcdb.New(pool)}
}

func (r *repository) GetByID(ctx context.Context, id string) (*Entity, error) {
	row, err := r.q.GetContributionByID(ctx, pghelper.UUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("contribution")
		}
		return nil, fmt.Errorf("contribution.repository.GetByID: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) ListByCard(ctx context.Context, cardID string) ([]*Entity, error) {
	rows, err := r.q.ListContributionsByCard(ctx, pghelper.UUID(cardID))
	if err != nil {
		return nil, fmt.Errorf("contribution.repository.ListByCard: %w", err)
	}
	out := make([]*Entity, len(rows))
	for i, row := range rows {
		out[i] = rowToEntity(row)
	}
	return out, nil
}

func (r *repository) Create(ctx context.Context, p CreateParams) (*Entity, error) {
	row, err := r.q.CreateContribution(ctx, sqlcdb.CreateContributionParams{
		VideoCardID:       pghelper.UUID(p.VideoCardID),
		ContributorUserID: pghelper.NullUUID(p.ContributorUserID),
		ContributorName:   p.ContributorName,
		ContributorEmail:  pghelper.TextVal(p.ContributorEmail),
		VideoUrl:          p.VideoURL,
		DurationSeconds:   pghelper.Float8(p.DurationSeconds),
	})
	if err != nil {
		return nil, fmt.Errorf("contribution.repository.Create: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Update(ctx context.Context, id string, p UpdateParams) (*Entity, error) {
	row, err := r.q.UpdateContribution(ctx, sqlcdb.UpdateContributionParams{
		ID:           pghelper.UUID(id),
		Status:       pghelper.Text(p.Status),
		DisplayOrder: pghelper.Int4(p.DisplayOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("contribution")
		}
		return nil, fmt.Errorf("contribution.repository.Update: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) UpdateOrder(ctx context.Context, id string, order int) error {
	return r.q.UpdateContributionOrder(ctx, sqlcdb.UpdateContributionOrderParams{
		ID:           pghelper.UUID(id),
		DisplayOrder: int32(order),
	})
}

func (r *repository) Delete(ctx context.Context, id string) error {
	return r.q.DeleteContribution(ctx, pghelper.UUID(id))
}

func rowToEntity(row sqlcdb.VideoContribution) *Entity {
	return &Entity{
		ID:                pghelper.StringFromUUID(row.ID),
		VideoCardID:       pghelper.StringFromUUID(row.VideoCardID),
		ContributorUserID: pghelper.StringFromUUID(row.ContributorUserID),
		ContributorName:   row.ContributorName,
		ContributorEmail:  pghelper.StringFromText(row.ContributorEmail),
		VideoURL:          row.VideoUrl,
		ThumbnailURL:      pghelper.StringFromText(row.ThumbnailUrl),
		HLSPlaylistURL:    pghelper.StringFromText(row.HlsPlaylistUrl),
		OriginalURL:       pghelper.StringFromText(row.OriginalUrl),
		DurationSeconds:   pghelper.Float64FromFloat8(row.DurationSeconds),
		Status:            row.Status,
		DisplayOrder:      int(row.DisplayOrder),
		CreatedAt:         pghelper.TimeFromTimestamptz(row.CreatedAt),
		UpdatedAt:         pghelper.TimeFromTimestamptz(row.UpdatedAt),
	}
}
