package occasion

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

// Repository defines the data access contract for occasions.
type Repository interface {
	List(ctx context.Context) ([]*Entity, error)
	GetByID(ctx context.Context, id string) (*Entity, error)
	Create(ctx context.Context, p CreateParams) (*Entity, error)
	Update(ctx context.Context, id string, p UpdateParams) (*Entity, error)
	Delete(ctx context.Context, id string) error
}

// CreateParams holds validated data for creating an occasion.
type CreateParams struct {
	ID              string
	Label           string
	Emoji           string
	ContributorTips json.RawMessage
	MessageTemplate string
	SortOrder       int
}

// UpdateParams holds validated data for a partial occasion update.
type UpdateParams struct {
	Label           *string
	Emoji           *string
	ContributorTips json.RawMessage
	MessageTemplate *string
	SortOrder       *int32
}

type repository struct {
	q *sqlcdb.Queries
}

// NewRepository constructs a Repository backed by the given pgx pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{q: sqlcdb.New(pool)}
}

func (r *repository) List(ctx context.Context) ([]*Entity, error) {
	rows, err := r.q.ListOccasions(ctx)
	if err != nil {
		return nil, fmt.Errorf("occasion.repository.List: %w", err)
	}
	out := make([]*Entity, len(rows))
	for i, row := range rows {
		out[i] = rowToEntity(row)
	}
	return out, nil
}

func (r *repository) GetByID(ctx context.Context, id string) (*Entity, error) {
	row, err := r.q.GetOccasionByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("occasion")
		}
		return nil, fmt.Errorf("occasion.repository.GetByID: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Create(ctx context.Context, p CreateParams) (*Entity, error) {
	tips := []byte(p.ContributorTips)
	if tips == nil {
		tips = []byte("[]")
	}
	row, err := r.q.CreateOccasion(ctx, sqlcdb.CreateOccasionParams{
		ID:              p.ID,
		Label:           p.Label,
		Emoji:           pghelper.TextVal(p.Emoji),
		ContributorTips: tips,
		MessageTemplate: p.MessageTemplate,
		SortOrder:       int32(p.SortOrder),
	})
	if err != nil {
		return nil, fmt.Errorf("occasion.repository.Create: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Update(ctx context.Context, id string, p UpdateParams) (*Entity, error) {
	row, err := r.q.UpdateOccasion(ctx, sqlcdb.UpdateOccasionParams{
		ID:              id,
		Label:           pghelper.Text(p.Label),
		Emoji:           pghelper.Text(p.Emoji),
		ContributorTips: []byte(p.ContributorTips),
		MessageTemplate: pghelper.Text(p.MessageTemplate),
		SortOrder:       pghelper.Int4(p.SortOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("occasion")
		}
		return nil, fmt.Errorf("occasion.repository.Update: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Delete(ctx context.Context, id string) error {
	if err := r.q.DeleteOccasion(ctx, id); err != nil {
		return fmt.Errorf("occasion.repository.Delete: %w", err)
	}
	return nil
}

func rowToEntity(row sqlcdb.Occasion) *Entity {
	return &Entity{
		ID:              row.ID,
		Label:           row.Label,
		Emoji:           pghelper.StringFromText(row.Emoji),
		ContributorTips: json.RawMessage(row.ContributorTips),
		MessageTemplate: row.MessageTemplate,
		SortOrder:       int(row.SortOrder),
		CreatedAt:       pghelper.TimeFromTimestamptz(row.CreatedAt),
		UpdatedAt:       pghelper.TimeFromTimestamptz(row.UpdatedAt),
	}
}
