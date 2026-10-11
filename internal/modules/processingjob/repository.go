package processingjob

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

// Repository defines the data access contract for video processing jobs.
type Repository interface {
	GetByID(ctx context.Context, id string) (*Entity, error)
	GetLatestByCard(ctx context.Context, cardID string) (*Entity, error)
	Create(ctx context.Context, cardID string, metadata json.RawMessage) (*Entity, error)
	UpdateStatus(ctx context.Context, id, status string, errMsg *string) error
}

type repository struct {
	q *sqlcdb.Queries
}

// NewRepository constructs a Repository backed by the given pgx pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{q: sqlcdb.New(pool)}
}

func (r *repository) GetByID(ctx context.Context, id string) (*Entity, error) {
	row, err := r.q.GetProcessingJobByID(ctx, pghelper.UUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("processing job")
		}
		return nil, fmt.Errorf("processingjob.repository.GetByID: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) GetLatestByCard(ctx context.Context, cardID string) (*Entity, error) {
	row, err := r.q.GetLatestProcessingJob(ctx, pghelper.UUID(cardID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("processing job")
		}
		return nil, fmt.Errorf("processingjob.repository.GetLatestByCard: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Create(ctx context.Context, cardID string, metadata json.RawMessage) (*Entity, error) {
	row, err := r.q.CreateProcessingJob(ctx, sqlcdb.CreateProcessingJobParams{
		VideoCardID: pghelper.UUID(cardID),
		Metadata:    string(metadata),
	})
	if err != nil {
		return nil, fmt.Errorf("processingjob.repository.Create: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) UpdateStatus(ctx context.Context, id, status string, errMsg *string) error {
	return r.q.UpdateProcessingJobStatus(ctx, sqlcdb.UpdateProcessingJobStatusParams{
		ID:           pghelper.UUID(id),
		Status:       sqlcdb.JobStatus(status),
		ErrorMessage: pghelper.Text(errMsg),
	})
}

func rowToEntity(row sqlcdb.VideoProcessingJob) *Entity {
	e := &Entity{
		ID:          pghelper.StringFromUUID(row.ID),
		VideoCardID: pghelper.StringFromUUID(row.VideoCardID),
		Status:      string(row.Status),
		ErrorMessage: pghelper.StringFromText(row.ErrorMessage),
		Attempts:    int(row.Attempts),
		Metadata:    json.RawMessage(row.Metadata),
		CreatedAt:   pghelper.TimeFromTimestamptz(row.CreatedAt),
		RetriedJobID: pghelper.StringFromUUID(row.RetriedJobID),
	}
	if row.StartedAt.Valid {
		t := pghelper.TimeFromTimestamptz(row.StartedAt)
		e.StartedAt = &t
	}
	if row.CompletedAt.Valid {
		t := pghelper.TimeFromTimestamptz(row.CompletedAt)
		e.CompletedAt = &t
	}
	return e
}
