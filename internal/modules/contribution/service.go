package contribution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/modules/transcoderclient"
)

const (
	presignExpiry = 15 * time.Minute

	// maxUploadBytes rejects oversized uploads when the client reports a size.
	// A presigned PUT cannot enforce size itself; see GetUploadURL.
	maxUploadBytes = 500 << 20

	defaultMaxContributors = 50
)

// allowedImageTypes excludes SVG and other formats that could carry script
// when served back from the CDN.
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/heic": true,
	"image/heif": true,
	"image/webp": true,
}

// CardInfo is the subset of a card needed to decide whether it accepts uploads.
type CardInfo struct {
	Status          string
	MaxContributors int
}

// CardGate answers card-level questions for this module. Implemented in
// cmd/api/main.go over the card repository, reusing card.VerifyAccess so the
// organizer/collaborator rule is defined once.
type CardGate interface {
	// CheckAccess returns nil if the user may manage the card.
	CheckAccess(ctx context.Context, cardID, userID, email string) error
	GetCardInfo(ctx context.Context, cardID string) (*CardInfo, error)
}

// ContributionNotifier sends push notifications triggered by contribution events.
// Implemented in cmd/api/main.go as an adapter over the card repo, user repo, and firebase client.
type ContributionNotifier interface {
	NotifyNewContribution(ctx context.Context, cardID, contributorName string) error
}

// Service defines the business logic contract for video contributions.
//
// userID and email are empty for anonymous contributors on the upload
// endpoints; every other method requires card access.
type Service interface {
	GetUploadURL(ctx context.Context, req UploadURLRequest, userID, email string) (*UploadURLResponse, error)
	ConfirmUpload(ctx context.Context, req ConfirmUploadRequest, userID, email string) (*Entity, error)
	// ListByCard is for internal callers that have already authorised access.
	ListByCard(ctx context.Context, cardID string) ([]*Entity, error)
	List(ctx context.Context, cardID, userID, email string) ([]*Entity, error)
	BatchReorder(ctx context.Context, req BatchReorderRequest, userID, email string) error
	Update(ctx context.Context, id, userID, email string, req UpdateRequest) (*Entity, error)
	Delete(ctx context.Context, id, userID, email string) error
}

type service struct {
	repo       Repository
	cards      CardGate
	s3         Presigner
	transcoder transcoderclient.Client
	notifier   ContributionNotifier // optional; nil disables push notifications
}

// Presigner issues presigned S3 upload URLs. Satisfied by *s3.Client.
type Presigner interface {
	PresignedUploadURL(ctx context.Context, key, contentType string, expiresIn time.Duration) (string, error)
}

// NewService constructs a Service.
func NewService(repo Repository, cards CardGate, s3Client Presigner, tc transcoderclient.Client, notifier ContributionNotifier) Service {
	return &service{repo: repo, cards: cards, s3: s3Client, transcoder: tc, notifier: notifier}
}

// keyPrefix is where uploads for a card live; ConfirmUpload only accepts keys
// under it so nobody can attach another card's (or arbitrary) objects.
func keyPrefix(cardID string) string {
	return "contributions/" + cardID + "/"
}

func validateContentType(ct string) error {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if strings.HasPrefix(ct, "video/") || allowedImageTypes[ct] {
		return nil
	}
	return apperr.BadRequest("unsupported content type")
}

// checkCanContribute lets the card's organizers upload at any time, and
// anyone else until the card expires and while it is not full. Finished and
// rendering cards stay open because organizers re-render after late messages;
// a clip arriving mid-render simply joins the next render.
func (s *service) checkCanContribute(ctx context.Context, cardID, userID, email string) error {
	info, err := s.cards.GetCardInfo(ctx, cardID)
	if err != nil {
		return err
	}
	if userID != "" && s.cards.CheckAccess(ctx, cardID, userID, email) == nil {
		return nil
	}
	if info.Status == "expired" {
		return apperr.Forbidden("this card is no longer accepting messages")
	}
	limit := info.MaxContributors
	if limit <= 0 {
		limit = defaultMaxContributors
	}
	existing, err := s.repo.ListByCard(ctx, cardID)
	if err != nil {
		return fmt.Errorf("contribution.service.checkCanContribute: %w", err)
	}
	if len(existing) >= limit {
		return apperr.LimitExceeded("this card has reached its message limit")
	}
	return nil
}

func (s *service) GetUploadURL(ctx context.Context, req UploadURLRequest, userID, email string) (*UploadURLResponse, error) {
	if err := validateContentType(req.ContentType); err != nil {
		return nil, err
	}
	if req.FileSize > maxUploadBytes {
		return nil, apperr.LimitExceeded("file is too large")
	}
	if err := s.checkCanContribute(ctx, req.VideoCardID, userID, email); err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s%s/original_%s",
		keyPrefix(req.VideoCardID), uuid.New().String(), uuid.New().String(),
	)

	url, err := s.s3.PresignedUploadURL(ctx, key, req.ContentType, presignExpiry)
	if err != nil {
		return nil, fmt.Errorf("contribution.service.GetUploadURL: %w", err)
	}

	return &UploadURLResponse{UploadURL: url, S3Key: key}, nil
}

func (s *service) ConfirmUpload(ctx context.Context, req ConfirmUploadRequest, userID, email string) (*Entity, error) {
	if !strings.HasPrefix(req.S3Key, keyPrefix(req.VideoCardID)) || strings.Contains(req.S3Key, "..") {
		return nil, apperr.BadRequest("invalid upload key")
	}
	if err := s.checkCanContribute(ctx, req.VideoCardID, userID, email); err != nil {
		return nil, err
	}

	var contributorUserID *string
	if userID != "" {
		contributorUserID = &userID
	}

	dur := req.DurationSeconds
	var durPtr *float64
	if dur >= 0.5 {
		durPtr = &dur
	}

	contrib, err := s.repo.Create(ctx, CreateParams{
		VideoCardID:       req.VideoCardID,
		ContributorUserID: contributorUserID,
		ContributorName:   req.ContributorName,
		ContributorEmail:  req.ContributorEmail,
		VideoURL:          req.S3Key,
		DurationSeconds:   durPtr,
	})
	if err != nil {
		return nil, fmt.Errorf("contribution.service.ConfirmUpload: %w", err)
	}

	// Enqueue transcoding job asynchronously; failure is non-fatal here.
	_ = s.transcoder.EnqueueContributionIngest(ctx, transcoderclient.ContributionIngestInput{
		JobID:          contrib.ID,
		VideoCardID:    req.VideoCardID,
		ContributionID: contrib.ID,
		SourceURL:      req.S3Key,
	})

	// Send push notification to the card organizer. Fire-and-forget — a
	// notification failure must never fail the contribution upload response.
	if s.notifier != nil {
		go func() {
			_ = s.notifier.NotifyNewContribution(context.Background(), req.VideoCardID, req.ContributorName)
		}()
	}

	return contrib, nil
}

func (s *service) ListByCard(ctx context.Context, cardID string) ([]*Entity, error) {
	return s.repo.ListByCard(ctx, cardID)
}

func (s *service) List(ctx context.Context, cardID, userID, email string) ([]*Entity, error) {
	if err := s.cards.CheckAccess(ctx, cardID, userID, email); err != nil {
		return nil, err
	}
	return s.repo.ListByCard(ctx, cardID)
}

func (s *service) BatchReorder(ctx context.Context, req BatchReorderRequest, userID, email string) error {
	if err := s.cards.CheckAccess(ctx, req.VideoCardID, userID, email); err != nil {
		return err
	}
	// Every ID must belong to the card the caller was authorised for.
	existing, err := s.repo.ListByCard(ctx, req.VideoCardID)
	if err != nil {
		return fmt.Errorf("contribution.service.BatchReorder: %w", err)
	}
	onCard := make(map[string]bool, len(existing))
	for _, c := range existing {
		onCard[c.ID] = true
	}
	for _, id := range req.IDs {
		if !onCard[id] {
			return apperr.BadRequest("contribution does not belong to this card")
		}
	}

	for i, id := range req.IDs {
		if err := s.repo.UpdateOrder(ctx, id, i); err != nil {
			return fmt.Errorf("contribution.service.BatchReorder: id %s: %w", id, err)
		}
	}
	return nil
}

func (s *service) Update(ctx context.Context, id, userID, email string, req UpdateRequest) (*Entity, error) {
	if req.Status == nil && req.DisplayOrder == nil {
		return nil, apperr.BadRequest("no fields provided for update")
	}
	if err := s.authoriseContribution(ctx, id, userID, email); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, UpdateParams{
		Status:       req.Status,
		DisplayOrder: req.DisplayOrder,
	})
}

func (s *service) Delete(ctx context.Context, id, userID, email string) error {
	if err := s.authoriseContribution(ctx, id, userID, email); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// authoriseContribution resolves the contribution's card and checks access to it.
func (s *service) authoriseContribution(ctx context.Context, id, userID, email string) error {
	contrib, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.cards.CheckAccess(ctx, contrib.VideoCardID, userID, email)
}
