package card

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/core/pghelper"
	"github.com/kithly/kithly-ws-go/internal/core/sqlcdb"
)

// Repository defines the data access contract for video cards and their collaborators.
type Repository interface {
	GetByID(ctx context.Context, id string) (*Entity, error)
	GetByShareToken(ctx context.Context, token string) (*Entity, error)
	ListByOrganizer(ctx context.Context, userID string, limit, offset int) ([]*Entity, error)
	ListShared(ctx context.Context, email string, limit, offset int) ([]*Entity, error)
	CountByOrganizer(ctx context.Context, userID string) (int64, error)
	Create(ctx context.Context, p CreateCardParams) (*Entity, error)
	Update(ctx context.Context, id string, p UpdateCardParams) (*Entity, error)
	UpdateOverlay(ctx context.Context, id, overlayID string) (*Entity, error)
	// CountIncludedContributions returns card ID -> included contribution
	// count; cards with none are absent from the map.
	CountIncludedContributions(ctx context.Context, cardIDs []string) (map[string]int, error)
	// UpdateCover sets the cover; empty strings and nil config clear it.
	UpdateCover(ctx context.Context, id, photoKey, imageKey string, config []byte) (*Entity, error)
	UpdateVideoURLs(ctx context.Context, id string, p UpdateURLsParams) error
	Lock(ctx context.Context, id string) error
	UpdateStatus(ctx context.Context, id, status string) error
	Archive(ctx context.Context, id string) error
	Unarchive(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error

	// Memory methods.
	ListMemories(ctx context.Context, userID, email string, limit, offset int) ([]*Entity, error)
	CountMemories(ctx context.Context, userID, email string) (int64, error)

	// Collaborator methods.
	ListCollaborators(ctx context.Context, cardID string) ([]*CollaboratorEntity, error)
	CollaboratorExists(ctx context.Context, cardID, email string) (bool, error)
	AddCollaborator(ctx context.Context, cardID, email, name string, userID *string) (*CollaboratorEntity, error)
	RemoveCollaborator(ctx context.Context, collaboratorID string) error
}

// CollaboratorEntity represents a video card collaborator row.
type CollaboratorEntity struct {
	ID          string
	VideoCardID string
	Email       string
	Name        string
	UserID      string
	InvitedAt   time.Time
}

// CreateCardParams holds validated data for card creation.
type CreateCardParams struct {
	OrganizerID     string
	Title           string
	Occasion        string
	Description     string
	ShareToken      string
	MusicThemeID    *string
	VideoOverlayID  *string
	MaxContributors int
}

// UpdateCardParams holds validated data for a partial card update.
type UpdateCardParams struct {
	Title           *string
	Occasion        *string
	Description     *string
	MusicThemeID    *string
	VideoOverlayID  *string
	MaxContributors *int32
}

// UpdateURLsParams holds all nullable URL fields that can be updated after processing.
type UpdateURLsParams struct {
	FinalVideoURL          *string
	PreviewURL             *string
	ThumbnailURL           *string
	FinalHLSPlaylistURL    *string
	PreviewHLSPlaylistURL  *string
	PreviewStatus          *string
	LastPreviewGeneratedAt *time.Time
	ProcessedAt            *time.Time
	FinalDuration          *float64
}

type repository struct {
	q *sqlcdb.Queries
}

// NewRepository constructs a Repository backed by the given pgx pool.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{q: sqlcdb.New(pool)}
}

func (r *repository) GetByID(ctx context.Context, id string) (*Entity, error) {
	row, err := r.q.GetCardByID(ctx, pghelper.UUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("card")
		}
		return nil, fmt.Errorf("card.repository.GetByID: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) GetByShareToken(ctx context.Context, token string) (*Entity, error) {
	row, err := r.q.GetCardByShareToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("card")
		}
		return nil, fmt.Errorf("card.repository.GetByShareToken: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) ListByOrganizer(ctx context.Context, userID string, limit, offset int) ([]*Entity, error) {
	rows, err := r.q.ListCardsByOrganizer(ctx, sqlcdb.ListCardsByOrganizerParams{
		OrganizerID: pghelper.UUID(userID),
		Limit:       int32(limit),
		Offset:      int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("card.repository.ListByOrganizer: %w", err)
	}
	return rowsToEntities(rows), nil
}

func (r *repository) ListShared(ctx context.Context, email string, limit, offset int) ([]*Entity, error) {
	rows, err := r.q.ListSharedCards(ctx, sqlcdb.ListSharedCardsParams{
		Email:  email,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("card.repository.ListShared: %w", err)
	}
	return rowsToEntities(rows), nil
}

func (r *repository) CountByOrganizer(ctx context.Context, userID string) (int64, error) {
	count, err := r.q.CountCardsByOrganizer(ctx, pghelper.UUID(userID))
	if err != nil {
		return 0, fmt.Errorf("card.repository.CountByOrganizer: %w", err)
	}
	return count, nil
}

func (r *repository) ListMemories(ctx context.Context, userID, email string, limit, offset int) ([]*Entity, error) {
	rows, err := r.q.ListMemoryCards(ctx, sqlcdb.ListMemoryCardsParams{
		OrganizerID: pghelper.UUID(userID),
		Column2:     email,
		Limit:       int32(limit),
		Offset:      int32(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("card.repository.ListMemories: %w", err)
	}
	return rowsToEntities(rows), nil
}

func (r *repository) CountMemories(ctx context.Context, userID, email string) (int64, error) {
	count, err := r.q.CountMemoryCards(ctx, sqlcdb.CountMemoryCardsParams{
		OrganizerID: pghelper.UUID(userID),
		Column2:     email,
	})
	if err != nil {
		return 0, fmt.Errorf("card.repository.CountMemories: %w", err)
	}
	return count, nil
}

func (r *repository) Create(ctx context.Context, p CreateCardParams) (*Entity, error) {
	row, err := r.q.CreateCard(ctx, sqlcdb.CreateCardParams{
		OrganizerID:     pghelper.UUID(p.OrganizerID),
		Title:           p.Title,
		Occasion:        p.Occasion,
		Description:     pghelper.TextVal(p.Description),
		ShareToken:      p.ShareToken,
		MusicThemeID:    pghelper.NullUUID(p.MusicThemeID),
		VideoOverlayID:  pghelper.NullUUID(p.VideoOverlayID),
		MaxContributors: int32(p.MaxContributors),
	})
	if err != nil {
		return nil, fmt.Errorf("card.repository.Create: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) Update(ctx context.Context, id string, p UpdateCardParams) (*Entity, error) {
	row, err := r.q.UpdateCard(ctx, sqlcdb.UpdateCardParams{
		ID:              pghelper.UUID(id),
		Title:           pghelper.Text(p.Title),
		Occasion:        pghelper.Text(p.Occasion),
		Description:     pghelper.Text(p.Description),
		MusicThemeID:    pghelper.NullUUID(p.MusicThemeID),
		VideoOverlayID:  pghelper.NullUUID(p.VideoOverlayID),
		MaxContributors: pghelper.Int4(p.MaxContributors),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("card")
		}
		return nil, fmt.Errorf("card.repository.Update: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) CountIncludedContributions(ctx context.Context, cardIDs []string) (map[string]int, error) {
	counts := make(map[string]int, len(cardIDs))
	if len(cardIDs) == 0 {
		return counts, nil
	}
	ids := make([]pgtype.UUID, len(cardIDs))
	for i, id := range cardIDs {
		ids[i] = pghelper.UUID(id)
	}
	rows, err := r.q.CountIncludedContributionsByCards(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("card.repository.CountIncludedContributions: %w", err)
	}
	for _, row := range rows {
		counts[pghelper.StringFromUUID(row.VideoCardID)] = int(row.IncludedCount)
	}
	return counts, nil
}

func (r *repository) UpdateCover(ctx context.Context, id, photoKey, imageKey string, config []byte) (*Entity, error) {
	row, err := r.q.UpdateCardCover(ctx, sqlcdb.UpdateCardCoverParams{
		ID:            pghelper.UUID(id),
		CoverPhotoUrl: nullableText(photoKey),
		CoverImageUrl: nullableText(imageKey),
		CoverConfig:   nullableText(string(config)),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("card")
		}
		return nil, fmt.Errorf("card.repository.UpdateCover: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) UpdateOverlay(ctx context.Context, id, overlayID string) (*Entity, error) {
	row, err := r.q.UpdateCardOverlay(ctx, sqlcdb.UpdateCardOverlayParams{
		ID:             pghelper.UUID(id),
		VideoOverlayID: pghelper.UUID(overlayID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.NotFound("card")
		}
		return nil, fmt.Errorf("card.repository.UpdateOverlay: %w", err)
	}
	return rowToEntity(row), nil
}

func (r *repository) UpdateVideoURLs(ctx context.Context, id string, p UpdateURLsParams) error {
	if err := r.q.UpdateCardVideoURLs(ctx, sqlcdb.UpdateCardVideoURLsParams{
		ID:                     pghelper.UUID(id),
		FinalVideoUrl:          pghelper.Text(p.FinalVideoURL),
		PreviewUrl:             pghelper.Text(p.PreviewURL),
		ThumbnailUrl:           pghelper.Text(p.ThumbnailURL),
		FinalHlsPlaylistUrl:    pghelper.Text(p.FinalHLSPlaylistURL),
		PreviewHlsPlaylistUrl:  pghelper.Text(p.PreviewHLSPlaylistURL),
		PreviewStatus:          pghelper.Text(p.PreviewStatus),
		LastPreviewGeneratedAt: pghelper.Timestamptz(p.LastPreviewGeneratedAt),
		ProcessedAt:            pghelper.Timestamptz(p.ProcessedAt),
		FinalDuration:          pghelper.Float8(p.FinalDuration),
	}); err != nil {
		return fmt.Errorf("card.repository.UpdateVideoURLs: %w", err)
	}
	return nil
}

func (r *repository) Lock(ctx context.Context, id string) error {
	return r.q.LockCard(ctx, pghelper.UUID(id))
}

func (r *repository) UpdateStatus(ctx context.Context, id, status string) error {
	return r.q.UpdateCardStatus(ctx, sqlcdb.UpdateCardStatusParams{
		ID:     pghelper.UUID(id),
		Status: sqlcdb.CardStatus(status),
	})
}

func (r *repository) Archive(ctx context.Context, id string) error {
	return r.q.ArchiveCard(ctx, pghelper.UUID(id))
}

func (r *repository) Unarchive(ctx context.Context, id string) error {
	return r.q.UnarchiveCard(ctx, pghelper.UUID(id))
}

func (r *repository) Delete(ctx context.Context, id string) error {
	return r.q.DeleteCard(ctx, pghelper.UUID(id))
}

func (r *repository) ListCollaborators(ctx context.Context, cardID string) ([]*CollaboratorEntity, error) {
	rows, err := r.q.ListCollaborators(ctx, pghelper.UUID(cardID))
	if err != nil {
		return nil, fmt.Errorf("card.repository.ListCollaborators: %w", err)
	}
	out := make([]*CollaboratorEntity, len(rows))
	for i, row := range rows {
		out[i] = collabRowToEntity(row)
	}
	return out, nil
}

func (r *repository) CollaboratorExists(ctx context.Context, cardID, email string) (bool, error) {
	exists, err := r.q.CollaboratorExistsByCardAndEmail(ctx, sqlcdb.CollaboratorExistsByCardAndEmailParams{
		VideoCardID: pghelper.UUID(cardID),
		Email:       email,
	})
	if err != nil {
		return false, fmt.Errorf("card.repository.CollaboratorExists: %w", err)
	}
	return exists, nil
}

func (r *repository) AddCollaborator(ctx context.Context, cardID, email, name string, userID *string) (*CollaboratorEntity, error) {
	row, err := r.q.AddCollaborator(ctx, sqlcdb.AddCollaboratorParams{
		VideoCardID: pghelper.UUID(cardID),
		Email:       email,
		Name:        pghelper.TextVal(name),
		UserID:      pghelper.NullUUID(userID),
	})
	if err != nil {
		return nil, fmt.Errorf("card.repository.AddCollaborator: %w", err)
	}
	return collabRowToEntity(row), nil
}

func (r *repository) RemoveCollaborator(ctx context.Context, collaboratorID string) error {
	return r.q.RemoveCollaborator(ctx, pghelper.UUID(collaboratorID))
}

func rowToEntity(row sqlcdb.VideoCard) *Entity {
	e := &Entity{
		ID:              pghelper.StringFromUUID(row.ID),
		OrganizerID:     pghelper.StringFromUUID(row.OrganizerID),
		Title:           row.Title,
		Occasion:        row.Occasion,
		Description:     pghelper.StringFromText(row.Description),
		ShareToken:      row.ShareToken,
		Status:          string(row.Status),
		MusicThemeID:    pghelper.StringFromUUID(row.MusicThemeID),
		VideoOverlayID:  pghelper.StringFromUUID(row.VideoOverlayID),
		MaxContributors: int(row.MaxContributors),
		IsLocked:        row.IsLocked,
		FinalVideoURL:   pghelper.StringFromText(row.FinalVideoUrl),
		PreviewURL:      pghelper.StringFromText(row.PreviewUrl),
		PreviewStatus:   row.PreviewStatus,
		FinalHLSPlaylistURL:   pghelper.StringFromText(row.FinalHlsPlaylistUrl),
		PreviewHLSPlaylistURL: pghelper.StringFromText(row.PreviewHlsPlaylistUrl),
		ThumbnailURL:    pghelper.StringFromText(row.ThumbnailUrl),
		FinalDuration:   pghelper.Float64FromFloat8(row.FinalDuration),
		IsArchived:      row.IsArchived,
		CreatedAt:       pghelper.TimeFromTimestamptz(row.CreatedAt),
		UpdatedAt:       pghelper.TimeFromTimestamptz(row.UpdatedAt),
		CoverPhotoURL:   pghelper.StringFromText(row.CoverPhotoUrl),
		CoverImageURL:   pghelper.StringFromText(row.CoverImageUrl),
		CoverConfig:     row.CoverConfig,
	}
	if row.LastPreviewGeneratedAt.Valid {
		t := pghelper.TimeFromTimestamptz(row.LastPreviewGeneratedAt)
		e.LastPreviewGeneratedAt = &t
	}
	if row.ProcessedAt.Valid {
		t := pghelper.TimeFromTimestamptz(row.ProcessedAt)
		e.ProcessedAt = &t
	}
	if row.ExpiresAt.Valid {
		t := pghelper.TimeFromTimestamptz(row.ExpiresAt)
		e.ExpiresAt = &t
	}
	return e
}

func rowsToEntities(rows []sqlcdb.VideoCard) []*Entity {
	out := make([]*Entity, len(rows))
	for i, row := range rows {
		out[i] = rowToEntity(row)
	}
	return out
}

func collabRowToEntity(row sqlcdb.VideoCardCollaborator) *CollaboratorEntity {
	return &CollaboratorEntity{
		ID:          pghelper.StringFromUUID(row.ID),
		VideoCardID: pghelper.StringFromUUID(row.VideoCardID),
		Email:       row.Email,
		Name:        pghelper.StringFromText(row.Name),
		UserID:      pghelper.StringFromUUID(row.UserID),
		InvitedAt:   pghelper.TimeFromTimestamptz(row.InvitedAt),
	}
}

// nullableText maps "" to SQL NULL.
func nullableText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pghelper.TextVal(s)
}
