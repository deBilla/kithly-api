package musictheme

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

// Repository defines the data access contract for music themes.
type Repository interface {
	List(ctx context.Context) ([]*Entity, error)
	GetByID(ctx context.Context, id string) (*Entity, error)
	Create(ctx context.Context, p CreateParams) (*Entity, error)
	Update(ctx context.Context, id string, p UpdateParams) (*Entity, error)
	Delete(ctx context.Context, id string) error
}

// CreateParams holds validated data for creating a music theme.
type CreateParams struct {
	Label     string
	Emoji     string
	AudioURL  string
	Category  string
	IsActive  bool
	SortOrder int
}

// UpdateParams holds validated data for a partial music theme update.
type UpdateParams struct {
	Label     *string
	Emoji     *string
	AudioURL  *string
	Category  *string
	IsActive  *bool
	SortOrder *int32
}

type repository struct {
	q *sqlcdb.Queries
}

// NewRepository constructs a Repository backed by the given pgx pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{q: sqlcdb.New(pool)}
}

func (r *repository) List(ctx context.Context) ([]*Entity, error) {
	rows, err := r.q.ListMusicThemes(ctx)
	if err != nil {
		return nil, fmt.Errorf("musictheme.repository.List: %w", err)
	}
	out := make([]*Entity, len(rows))
	for i, row := range rows {
		out[i] = rowToEntity(row)
	}
	return out, nil
}

func (r *repository) GetByID(ctx context.Context, id string) (*Entity, error) {
	row, err := r.q.GetMusicThemeByID(ctx, pghelper.UUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("music theme")
		}
		return nil, fmt.Errorf("musictheme.repository.GetByID: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Create(ctx context.Context, p CreateParams) (*Entity, error) {
	row, err := r.q.CreateMusicTheme(ctx, sqlcdb.CreateMusicThemeParams{
		Label:     p.Label,
		Emoji:     p.Emoji,
		AudioUrl:  p.AudioURL,
		Category:  pghelper.TextVal(p.Category),
		IsActive:  p.IsActive,
		SortOrder: int32(p.SortOrder),
	})
	if err != nil {
		return nil, fmt.Errorf("musictheme.repository.Create: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Update(ctx context.Context, id string, p UpdateParams) (*Entity, error) {
	row, err := r.q.UpdateMusicTheme(ctx, sqlcdb.UpdateMusicThemeParams{
		ID:        pghelper.UUID(id),
		Label:     pghelper.Text(p.Label),
		Emoji:     pghelper.Text(p.Emoji),
		AudioUrl:  pghelper.Text(p.AudioURL),
		Category:  pghelper.Text(p.Category),
		IsActive:  pghelper.Bool(p.IsActive),
		SortOrder: pghelper.Int4(p.SortOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("music theme")
		}
		return nil, fmt.Errorf("musictheme.repository.Update: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Delete(ctx context.Context, id string) error {
	if err := r.q.DeleteMusicTheme(ctx, pghelper.UUID(id)); err != nil {
		return fmt.Errorf("musictheme.repository.Delete: %w", err)
	}
	return nil
}

func rowToEntity(row sqlcdb.MusicTheme) *Entity {
	return &Entity{
		ID:        pghelper.StringFromUUID(row.ID),
		Label:     row.Label,
		Emoji:     row.Emoji,
		AudioURL:  row.AudioUrl,
		Category:  pghelper.StringFromText(row.Category),
		IsActive:  row.IsActive,
		SortOrder: int(row.SortOrder),
		CreatedAt: pghelper.TimeFromTimestamptz(row.CreatedAt),
		UpdatedAt: pghelper.TimeFromTimestamptz(row.UpdatedAt),
	}
}
