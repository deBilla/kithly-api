package user

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

// Repository defines the data access contract for users.
type Repository interface {
	FindByID(ctx context.Context, id string) (*Entity, error)
	FindByEmail(ctx context.Context, email string) (*Entity, error)
	FindByFirebaseUID(ctx context.Context, uid string) (*Entity, error)
	FindByGuestToken(ctx context.Context, token string) (*Entity, error)
	LinkFirebaseUID(ctx context.Context, id, uid string) (*Entity, error)
	FindEmailByID(ctx context.Context, id string) (string, error)
	List(ctx context.Context) ([]*Entity, error)
	Create(ctx context.Context, p CreateParams) (*Entity, error)
	Update(ctx context.Context, id string, p UpdateParams) (*Entity, error)
	UpdateFCMToken(ctx context.Context, id, token string) error
	UpdateLastLogin(ctx context.Context, id string) error
	ConvertGuest(ctx context.Context, id string, p ConvertGuestParams) (*Entity, error)
	Delete(ctx context.Context, id string) error
	CountCards(ctx context.Context, userID string) (int64, error)
}

// CreateParams holds validated data for inserting a new user.
type CreateParams struct {
	Email          *string
	PasswordHash   *string
	Name           string
	EmailVerified  bool
	Avatar         string
	MaxCards       int
	IsGuest        bool
	GuestToken     *string
	FirebaseUID    *string
	GuestExpiresAt *string // ISO8601 or nil
}

// UpdateParams holds validated data for a partial user update.
type UpdateParams struct {
	Name          *string
	Email         *string
	Avatar        *string
	PasswordHash  *string
	EmailVerified *bool
}

// ConvertGuestParams holds data for converting a guest account to a full account.
type ConvertGuestParams struct {
	Email        string
	PasswordHash string
	Name         string
}

type repository struct {
	q *sqlcdb.Queries
}

// NewRepository constructs a Repository backed by the given pgx pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{q: sqlcdb.New(pool)}
}

func (r *repository) FindByID(ctx context.Context, id string) (*Entity, error) {
	row, err := r.q.GetUserByID(ctx, pghelper.UUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("user")
		}
		return nil, fmt.Errorf("user.repository.FindByID: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) FindByEmail(ctx context.Context, email string) (*Entity, error) {
	row, err := r.q.GetUserByEmail(ctx, pghelper.TextVal(email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("user")
		}
		return nil, fmt.Errorf("user.repository.FindByEmail: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) FindByFirebaseUID(ctx context.Context, uid string) (*Entity, error) {
	row, err := r.q.GetUserByFirebaseUID(ctx, pghelper.TextVal(uid))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("user")
		}
		return nil, fmt.Errorf("user.repository.FindByFirebaseUID: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) FindByGuestToken(ctx context.Context, token string) (*Entity, error) {
	row, err := r.q.GetUserByGuestToken(ctx, pghelper.TextVal(token))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("user")
		}
		return nil, fmt.Errorf("user.repository.FindByGuestToken: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) LinkFirebaseUID(ctx context.Context, id, uid string) (*Entity, error) {
	row, err := r.q.LinkUserFirebaseUID(ctx, sqlcdb.LinkUserFirebaseUIDParams{
		ID:          pghelper.UUID(id),
		FirebaseUid: pghelper.TextVal(uid),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("user")
		}
		return nil, fmt.Errorf("user.repository.LinkFirebaseUID: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) FindEmailByID(ctx context.Context, id string) (string, error) {
	email, err := r.q.GetEmailByID(ctx, pghelper.UUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperr.NotFound("user")
		}
		return "", fmt.Errorf("user.repository.FindEmailByID: %w", err)
	}
	return pghelper.StringFromText(email), nil
}

func (r *repository) List(ctx context.Context) ([]*Entity, error) {
	rows, err := r.q.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("user.repository.List: %w", err)
	}
	out := make([]*Entity, len(rows))
	for i, row := range rows {
		out[i] = rowToEntity(row)
	}
	return out, nil
}

func (r *repository) Create(ctx context.Context, p CreateParams) (*Entity, error) {
	row, err := r.q.CreateUser(ctx, sqlcdb.CreateUserParams{
		Email:          pghelper.Text(p.Email),
		PasswordHash:   pghelper.Text(p.PasswordHash),
		Name:           pghelper.TextVal(p.Name),
		EmailVerified:  p.EmailVerified,
		Avatar:         pghelper.TextVal(p.Avatar),
		MaxCards:       int32(p.MaxCards),
		IsGuest:        p.IsGuest,
		GuestToken:     pghelper.Text(p.GuestToken),
		FirebaseUid:    pghelper.Text(p.FirebaseUID),
		GuestExpiresAt: pghelper.Timestamptz(nil),
	})
	if err != nil {
		return nil, fmt.Errorf("user.repository.Create: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Update(ctx context.Context, id string, p UpdateParams) (*Entity, error) {
	row, err := r.q.UpdateUser(ctx, sqlcdb.UpdateUserParams{
		ID:            pghelper.UUID(id),
		Name:          pghelper.Text(p.Name),
		Email:         pghelper.Text(p.Email),
		Avatar:        pghelper.Text(p.Avatar),
		PasswordHash:  pghelper.Text(p.PasswordHash),
		EmailVerified: pghelper.Bool(p.EmailVerified),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("user")
		}
		return nil, fmt.Errorf("user.repository.Update: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) UpdateFCMToken(ctx context.Context, id, token string) error {
	if err := r.q.UpdateUserFCMToken(ctx, sqlcdb.UpdateUserFCMTokenParams{
		ID:       pghelper.UUID(id),
		FcmToken: pghelper.TextVal(token),
	}); err != nil {
		return fmt.Errorf("user.repository.UpdateFCMToken: %w", err)
	}
	return nil
}

func (r *repository) UpdateLastLogin(ctx context.Context, id string) error {
	if err := r.q.UpdateUserLastLogin(ctx, pghelper.UUID(id)); err != nil {
		return fmt.Errorf("user.repository.UpdateLastLogin: %w", err)
	}
	return nil
}

func (r *repository) ConvertGuest(ctx context.Context, id string, p ConvertGuestParams) (*Entity, error) {
	row, err := r.q.ConvertGuestToUser(ctx, sqlcdb.ConvertGuestToUserParams{
		ID:           pghelper.UUID(id),
		Email:        pghelper.TextVal(p.Email),
		PasswordHash: pghelper.TextVal(p.PasswordHash),
		Name:         pghelper.TextVal(p.Name),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("user")
		}
		return nil, fmt.Errorf("user.repository.ConvertGuest: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Delete(ctx context.Context, id string) error {
	if err := r.q.DeleteUser(ctx, pghelper.UUID(id)); err != nil {
		return fmt.Errorf("user.repository.Delete: %w", err)
	}
	return nil
}

func (r *repository) CountCards(ctx context.Context, userID string) (int64, error) {
	count, err := r.q.CountUserCards(ctx, pghelper.UUID(userID))
	if err != nil {
		return 0, fmt.Errorf("user.repository.CountCards: %w", err)
	}
	return count, nil
}

func rowToEntity(row sqlcdb.User) *Entity {
	e := &Entity{
		ID:            pghelper.StringFromUUID(row.ID),
		Email:         pghelper.StringFromText(row.Email),
		PasswordHash:  pghelper.StringFromText(row.PasswordHash),
		Name:          pghelper.StringFromText(row.Name),
		Avatar:        pghelper.StringFromText(row.Avatar),
		EmailVerified: row.EmailVerified,
		IsGuest:       row.IsGuest,
		GuestToken:    pghelper.StringFromText(row.GuestToken),
		FirebaseUID:   pghelper.StringFromText(row.FirebaseUid),
		FCMToken:      pghelper.StringFromText(row.FcmToken),
		MaxCards:      int(row.MaxCards),
		CreatedAt:     pghelper.TimeFromTimestamptz(row.CreatedAt),
		UpdatedAt:     pghelper.TimeFromTimestamptz(row.UpdatedAt),
	}
	if row.LastLoginAt.Valid {
		t := pghelper.TimeFromTimestamptz(row.LastLoginAt)
		e.LastLoginAt = &t
	}
	if row.GuestExpiresAt.Valid {
		t := pghelper.TimeFromTimestamptz(row.GuestExpiresAt)
		e.GuestExpiresAt = &t
	}
	return e
}
