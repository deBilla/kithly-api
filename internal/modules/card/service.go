package card

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
	"github.com/kithly/kithly-ws-go/internal/core/sqlcdb"
	"github.com/kithly/kithly-ws-go/internal/modules/transcoderclient"
	"github.com/kithly/kithly-ws-go/pkg/ai"
	"github.com/kithly/kithly-ws-go/pkg/s3"
)

const (
	defaultMaxContributors = 50
	defaultLimit           = 20

	coverUploadExpiry  = 15 * time.Minute
	maxCoverConfigSize = 4096
)

// coverContentTypes maps allowed upload types to file extensions, per kind.
// The rendered image is limited to formats the worker treats as images.
var coverContentTypes = map[string]map[string]string{
	"photo": {"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/heic": ".heic", "image/heif": ".heif"},
	"image": {"image/jpeg": ".jpg", "image/png": ".png"},
}

// CoverUploader issues presigned S3 upload URLs. Satisfied by *s3.Client.
type CoverUploader interface {
	PresignedUploadURL(ctx context.Context, key, contentType string, expiresIn time.Duration) (string, error)
}

// ProcessingEnqueuer abstracts job creation for video processing.
// Implemented by the processing job service to avoid a circular dependency.
type ProcessingEnqueuer interface {
	CreateAndEnqueue(ctx context.Context, cardID string, isPreview bool) (string, error)
}

// LatestJobInfo holds the processing-job fields required by the status endpoint.
type LatestJobInfo struct {
	Status   string
	Metadata json.RawMessage
}

// JobReader reads the latest processing job for a card.
// Implemented by processingjob.Service via an adapter in cmd/api/main.go
// to avoid a circular package dependency.
type JobReader interface {
	GetLatestByCard(ctx context.Context, cardID string) (*LatestJobInfo, error)
}

// RawContrib holds contribution data needed by the activity and full-details endpoints.
type RawContrib struct {
	ID                string
	VideoCardID       string
	ContributorUserID string
	ContributorName   string
	ContributorEmail  string
	VideoURL          string // bare S3 key
	ThumbnailURL      string // bare S3 key
	HLSPlaylistURL    string // bare S3 key
	DurationSeconds   float64
	Status            string
	DisplayOrder      int
	CreatedAt         time.Time
}

// ContribLister lists contributions for a card.
// Implemented by contribution.Service via an adapter in cmd/api/main.go.
type ContribLister interface {
	ListByCard(ctx context.Context, cardID string) ([]*RawContrib, error)
}

// Service defines the business logic contract for video cards.
type Service interface {
	GetByID(ctx context.Context, id, userID, email string) (*Entity, error)
	GetFullDetails(ctx context.Context, id, userID, email string) (*FullDetailsView, error)
	GetPublicByID(ctx context.Context, id string) (*Entity, error)
	GetByShareToken(ctx context.Context, token string) (*Entity, error)
	Dashboard(ctx context.Context, userID, email string, page, limit int) ([]*Entity, error)
	Memories(ctx context.Context, userID, email string, page, limit int) ([]*Entity, int64, error)
	Create(ctx context.Context, req CreateRequest, userID string) (*Entity, error)
	Update(ctx context.Context, id, userID, email string, req UpdateRequest) (*Entity, error)
	UpdateOverlay(ctx context.Context, id, userID, email string, req UpdateOverlayRequest) (*Entity, error)
	Finalize(ctx context.Context, id, userID, email string) error
	GeneratePreview(ctx context.Context, id, userID, email string) error
	Archive(ctx context.Context, id, userID, email string) error
	Unarchive(ctx context.Context, id, userID, email string) error
	Delete(ctx context.Context, id, userID, email string) error
	GetStatus(ctx context.Context, id, userID, email string) (*StatusView, error)
	GetDownloadURL(ctx context.Context, id, userID, email string) (*DownloadURLView, error)
	GetActivity(ctx context.Context, id, userID, email string) ([]ActivityEntry, error)
	ListCollaborators(ctx context.Context, cardID, userID, email string) ([]*CollaboratorEntity, error)
	AddCollaborator(ctx context.Context, cardID, userID, email string, req AddCollaboratorRequest) (*CollaboratorEntity, error)
	RemoveCollaborator(ctx context.Context, cardID, collaboratorID, userID, email string) error
	GenerateWish(ctx context.Context, cardID, userID, email string, req GenerateWishRequest) (string, error)
	CoverUploadURL(ctx context.Context, cardID, userID, email string, req CoverUploadURLRequest) (*CoverUploadURLView, error)
	SetCover(ctx context.Context, cardID, userID, email string, req SetCoverRequest) (*Entity, error)
	ClearCover(ctx context.Context, cardID, userID, email string) (*Entity, error)
}

type service struct {
	repo       Repository
	enqueuer   ProcessingEnqueuer
	jobReader  JobReader
	contribs   ContribLister
	ai         *ai.Client
	transcoder transcoderclient.Client
	s3         *s3.Client
	uploads    CoverUploader
}

// NewService constructs a Service.
func NewService(
	repo Repository,
	enqueuer ProcessingEnqueuer,
	jobReader JobReader,
	contribs ContribLister,
	aiClient *ai.Client,
	tc transcoderclient.Client,
	s3Client *s3.Client,
) Service {
	return &service{
		repo:       repo,
		enqueuer:   enqueuer,
		jobReader:  jobReader,
		contribs:   contribs,
		ai:         aiClient,
		transcoder: tc,
		s3:         s3Client,
		uploads:    s3Client,
	}
}

// verifyCardAccess checks that the user is either the organizer or a collaborator.
// Returns the card entity on success, or a Forbidden/NotFound error.
func (s *service) verifyCardAccess(ctx context.Context, cardID, userID, email string) (*Entity, error) {
	return VerifyAccess(ctx, s.repo, cardID, userID, email)
}

// VerifyAccess is the single definition of who may manage a card: its
// organizer, or a collaborator invited by email. Exported so other modules
// (contributions, processing jobs) enforce the same rule via adapters.
func VerifyAccess(ctx context.Context, repo Repository, cardID, userID, email string) (*Entity, error) {
	if userID == "" {
		return nil, apperr.Forbidden("access denied")
	}

	card, err := repo.GetByID(ctx, cardID)
	if err != nil {
		return nil, err
	}

	if card.OrganizerID == userID {
		card.IsCollaborator = false
		return card, nil
	}

	if email != "" {
		exists, err := repo.CollaboratorExists(ctx, cardID, email)
		if err != nil {
			return nil, err
		}
		if exists {
			card.IsCollaborator = true
			return card, nil
		}
	}

	return nil, apperr.Forbidden("access denied")
}

// verifyOrganizerAccess is like verifyCardAccess but also requires organizer role.
func (s *service) verifyOrganizerAccess(ctx context.Context, cardID, userID, email string) (*Entity, error) {
	card, err := s.verifyCardAccess(ctx, cardID, userID, email)
	if err != nil {
		return nil, err
	}
	if card.IsCollaborator {
		return nil, apperr.Forbidden("only the organizer can perform this action")
	}
	return card, nil
}

func (s *service) GetByID(ctx context.Context, id, userID, email string) (*Entity, error) {
	return s.verifyCardAccess(ctx, id, userID, email)
}

func (s *service) GetFullDetails(ctx context.Context, id, userID, email string) (*FullDetailsView, error) {
	card, err := s.verifyCardAccess(ctx, id, userID, email)
	if err != nil {
		return nil, err
	}

	raw, err := s.contribs.ListByCard(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("card.service.GetFullDetails: %w", err)
	}

	contribs := make([]ContribView, len(raw))
	for i, c := range raw {
		dur := c.DurationSeconds
		if dur < 0.5 {
			dur = 0
		}
		contribs[i] = ContribView{
			ID:                c.ID,
			VideoCardID:       c.VideoCardID,
			ContributorUserID: c.ContributorUserID,
			ContributorName:   c.ContributorName,
			ContributorEmail:  c.ContributorEmail,
			VideoURL:          s3.CloudFrontURL(c.VideoURL),
			ThumbnailURL:      s3.CloudFrontURL(c.ThumbnailURL),
			HLSPlaylistURL:    s3.CloudFrontURL(c.HLSPlaylistURL),
			DurationSeconds:   dur,
			Status:            c.Status,
			DisplayOrder:      c.DisplayOrder,
			CreatedAt:         c.CreatedAt.Format(time.RFC3339),
		}
	}

	return &FullDetailsView{
		Card:          card.ToOwnerView(),
		Contributions: contribs,
	}, nil
}

func (s *service) GetPublicByID(ctx context.Context, id string) (*Entity, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) GetByShareToken(ctx context.Context, token string) (*Entity, error) {
	return s.repo.GetByShareToken(ctx, token)
}

func (s *service) Dashboard(ctx context.Context, userID, email string, page, limit int) ([]*Entity, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	cards, err := s.repo.ListByOrganizer(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("card.service.Dashboard: %w", err)
	}
	for _, c := range cards {
		c.IsCollaborator = false
	}

	if email != "" {
		shared, err := s.repo.ListShared(ctx, email, limit, offset)
		if err != nil {
			return nil, fmt.Errorf("card.service.Dashboard: shared: %w", err)
		}
		for _, c := range shared {
			c.IsCollaborator = true
		}
		cards = append(cards, shared...)
	}

	// One grouped query for all cards rather than one per card.
	ids := make([]string, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	counts, err := s.repo.CountIncludedContributions(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("card.service.Dashboard: counts: %w", err)
	}
	for _, c := range cards {
		c.ContributionCount = counts[c.ID]
	}

	return cards, nil
}

func (s *service) Create(ctx context.Context, req CreateRequest, userID string) (*Entity, error) {
	maxContributors := req.MaxContributors
	if maxContributors <= 0 {
		maxContributors = defaultMaxContributors
	}

	shareToken := generateShareToken()

	card, err := s.repo.Create(ctx, CreateCardParams{
		OrganizerID:     userID,
		Title:           req.Title,
		Occasion:        req.Occasion,
		Description:     req.Description,
		ShareToken:      shareToken,
		MusicThemeID:    req.MusicThemeID,
		VideoOverlayID:  req.VideoOverlayID,
		MaxContributors: maxContributors,
	})
	if err != nil {
		return nil, fmt.Errorf("card.service.Create: %w", err)
	}
	return card, nil
}

func (s *service) Update(ctx context.Context, id, userID, email string, req UpdateRequest) (*Entity, error) {
	if _, err := s.verifyCardAccess(ctx, id, userID, email); err != nil {
		return nil, err
	}
	card, err := s.repo.Update(ctx, id, UpdateCardParams{
		Title:           req.Title,
		Occasion:        req.Occasion,
		Description:     req.Description,
		MusicThemeID:    req.MusicThemeID,
		VideoOverlayID:  req.VideoOverlayID,
		MaxContributors: req.MaxContributors,
	})
	if err != nil {
		return nil, fmt.Errorf("card.service.Update: %w", err)
	}
	return card, nil
}

func (s *service) UpdateOverlay(ctx context.Context, id, userID, email string, req UpdateOverlayRequest) (*Entity, error) {
	if _, err := s.verifyCardAccess(ctx, id, userID, email); err != nil {
		return nil, err
	}
	// Dereference pointer; nil means clear the overlay (empty string → NULL via pghelper.UUID).
	overlayID := ""
	if req.VideoOverlayID != nil {
		overlayID = *req.VideoOverlayID
	}
	card, err := s.repo.UpdateOverlay(ctx, id, overlayID)
	if err != nil {
		return nil, fmt.Errorf("card.service.UpdateOverlay: %w", err)
	}
	return card, nil
}

func (s *service) Finalize(ctx context.Context, id, userID, email string) error {
	if _, err := s.verifyCardAccess(ctx, id, userID, email); err != nil {
		return err
	}

	if err := s.repo.Lock(ctx, id); err != nil {
		return fmt.Errorf("card.service.Finalize: lock: %w", err)
	}

	if _, err := s.enqueuer.CreateAndEnqueue(ctx, id, false); err != nil {
		return fmt.Errorf("card.service.Finalize: enqueue: %w", err)
	}
	return nil
}

func (s *service) GeneratePreview(ctx context.Context, id, userID, email string) error {
	if _, err := s.verifyCardAccess(ctx, id, userID, email); err != nil {
		return err
	}

	if _, err := s.enqueuer.CreateAndEnqueue(ctx, id, true); err != nil {
		return fmt.Errorf("card.service.GeneratePreview: %w", err)
	}
	return nil
}

func (s *service) Archive(ctx context.Context, id, userID, email string) error {
	if _, err := s.verifyOrganizerAccess(ctx, id, userID, email); err != nil {
		return err
	}
	return s.repo.Archive(ctx, id)
}

func (s *service) Unarchive(ctx context.Context, id, userID, email string) error {
	if _, err := s.verifyOrganizerAccess(ctx, id, userID, email); err != nil {
		return err
	}
	return s.repo.Unarchive(ctx, id)
}

func (s *service) Delete(ctx context.Context, id, userID, email string) error {
	if _, err := s.verifyOrganizerAccess(ctx, id, userID, email); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

func (s *service) ListCollaborators(ctx context.Context, cardID, userID, email string) ([]*CollaboratorEntity, error) {
	if _, err := s.verifyCardAccess(ctx, cardID, userID, email); err != nil {
		return nil, err
	}
	return s.repo.ListCollaborators(ctx, cardID)
}

func (s *service) AddCollaborator(ctx context.Context, cardID, userID, email string, req AddCollaboratorRequest) (*CollaboratorEntity, error) {
	if _, err := s.verifyCardAccess(ctx, cardID, userID, email); err != nil {
		return nil, err
	}

	exists, err := s.repo.CollaboratorExists(ctx, cardID, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflict("collaborator already added")
	}

	return s.repo.AddCollaborator(ctx, cardID, req.Email, req.Name, nil)
}

func (s *service) RemoveCollaborator(ctx context.Context, cardID, collaboratorID, userID, email string) error {
	if _, err := s.verifyOrganizerAccess(ctx, cardID, userID, email); err != nil {
		return err
	}
	return s.repo.RemoveCollaborator(ctx, collaboratorID)
}

func (s *service) Memories(ctx context.Context, userID, email string, page, limit int) ([]*Entity, int64, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	cards, err := s.repo.ListMemories(ctx, userID, email, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("card.service.Memories: %w", err)
	}
	for _, c := range cards {
		c.IsCollaborator = c.OrganizerID != userID
	}

	total, err := s.repo.CountMemories(ctx, userID, email)
	if err != nil {
		return nil, 0, fmt.Errorf("card.service.Memories: count: %w", err)
	}
	return cards, total, nil
}

func (s *service) GetStatus(ctx context.Context, id, userID, email string) (*StatusView, error) {
	card, err := s.verifyCardAccess(ctx, id, userID, email)
	if err != nil {
		return nil, err
	}

	view := &StatusView{
		Status:        card.Status,
		FinalVideoURL: s3.CloudFrontURL(card.FinalVideoURL),
		PreviewURL:    s3.CloudFrontURL(card.PreviewURL),
		PreviewStatus: card.PreviewStatus,
		Metadata:      nil,
	}

	if job, err := s.jobReader.GetLatestByCard(ctx, id); err == nil && job != nil {
		view.Status = job.Status
		view.Metadata = job.Metadata
	}

	return view, nil
}

func (s *service) GetDownloadURL(ctx context.Context, id, userID, email string) (*DownloadURLView, error) {
	card, err := s.verifyCardAccess(ctx, id, userID, email)
	if err != nil {
		return nil, err
	}
	if card.Status != "completed" || card.FinalVideoURL == "" {
		return nil, apperr.NotFound("video is not ready for download")
	}

	url, err := s.s3.PresignGetURL(ctx, card.FinalVideoURL, time.Hour)
	if err != nil {
		return nil, fmt.Errorf("card.service.GetDownloadURL: %w", err)
	}

	return &DownloadURLView{
		DownloadURL: url,
		ExpiresIn:   3600,
		ExpiresAt:   time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	}, nil
}

func (s *service) GetActivity(ctx context.Context, id, userID, email string) ([]ActivityEntry, error) {
	if _, err := s.verifyCardAccess(ctx, id, userID, email); err != nil {
		return nil, err
	}

	contribs, err := s.contribs.ListByCard(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("card.service.GetActivity: %w", err)
	}

	entries := make([]ActivityEntry, len(contribs))
	for i, c := range contribs {
		entries[i] = ActivityEntry{
			Type:            "contribution_added",
			ContributorName: c.ContributorName,
			Message:         c.ContributorName + " added a video to the card",
			CreatedAt:       c.CreatedAt.Format(time.RFC3339),
			ContributionID:  c.ID,
			Status:          c.Status,
		}
	}
	return entries, nil
}

func (s *service) GenerateWish(ctx context.Context, cardID, userID, email string, req GenerateWishRequest) (string, error) {
	card, err := s.verifyCardAccess(ctx, cardID, userID, email)
	if err != nil {
		return "", err
	}
	occasion := req.Occasion
	if occasion == "" {
		occasion = card.Occasion
	}
	wish, err := s.ai.GenerateWish(ctx, ai.WishInput{
		Tone:     req.Tone,
		FromName: req.FromName,
		ToName:   req.ToName,
		Occasion: occasion,
	})
	if err != nil {
		return "", fmt.Errorf("card.service.GenerateWish: %w", err)
	}
	return wish, nil
}

// generateShareToken creates a 10-character alphanumeric share token.
func generateShareToken() string {
	id := uuid.New().String()
	// Use first 10 hex chars from UUID (no dashes).
	clean := ""
	for _, c := range id {
		if c != '-' {
			clean += string(c)
		}
		if len(clean) == 10 {
			break
		}
	}
	// Ensure we always have length 10 by padding if needed (shouldn't happen).
	for len(clean) < 10 {
		clean += "x"
	}
	return clean
}

// Ensure sqlcdb is imported for CardStatus type used elsewhere.
var _ = sqlcdb.CardStatusCollecting

// Ensure time is used.
var _ = time.Now

// coverKeyPrefix scopes cover objects to their card, so SetCover can reject
// keys that belong to another card or to arbitrary bucket objects.
func coverKeyPrefix(cardID, kind string) string {
	return "covers/" + cardID + "/" + kind + "_"
}

func validCoverKey(key, cardID, kind string) bool {
	return strings.HasPrefix(key, coverKeyPrefix(cardID, kind)) && !strings.Contains(key, "..")
}

// editableForCover rejects changes while the video is being compiled; the
// worker reads the cover when the job starts.
func editableForCover(card *Entity) error {
	if card.Status == "processing" {
		return apperr.Conflict("the video is being created; try again when it is ready")
	}
	return nil
}

func (s *service) CoverUploadURL(ctx context.Context, cardID, userID, email string, req CoverUploadURLRequest) (*CoverUploadURLView, error) {
	card, err := s.verifyCardAccess(ctx, cardID, userID, email)
	if err != nil {
		return nil, err
	}
	if err := editableForCover(card); err != nil {
		return nil, err
	}
	ext, ok := coverContentTypes[req.Kind][strings.ToLower(req.ContentType)]
	if !ok {
		return nil, apperr.BadRequest("unsupported content type for cover " + req.Kind)
	}

	key := coverKeyPrefix(cardID, req.Kind) + uuid.New().String() + ext
	url, err := s.uploads.PresignedUploadURL(ctx, key, strings.ToLower(req.ContentType), coverUploadExpiry)
	if err != nil {
		return nil, fmt.Errorf("card.service.CoverUploadURL: %w", err)
	}
	return &CoverUploadURLView{UploadURL: url, Key: key}, nil
}

func (s *service) SetCover(ctx context.Context, cardID, userID, email string, req SetCoverRequest) (*Entity, error) {
	card, err := s.verifyCardAccess(ctx, cardID, userID, email)
	if err != nil {
		return nil, err
	}
	if err := editableForCover(card); err != nil {
		return nil, err
	}
	if !validCoverKey(req.ImageKey, cardID, "image") {
		return nil, apperr.BadRequest("invalid image_key")
	}
	if req.PhotoKey != "" && !validCoverKey(req.PhotoKey, cardID, "photo") {
		return nil, apperr.BadRequest("invalid photo_key")
	}

	var config []byte
	if len(req.Config) > 0 && string(req.Config) != "null" {
		if len(req.Config) > maxCoverConfigSize {
			return nil, apperr.BadRequest("config is too large")
		}
		var obj map[string]any
		if err := json.Unmarshal(req.Config, &obj); err != nil {
			return nil, apperr.BadRequest("config must be a JSON object")
		}
		config = req.Config
	}

	updated, err := s.repo.UpdateCover(ctx, cardID, req.PhotoKey, req.ImageKey, config)
	if err != nil {
		return nil, fmt.Errorf("card.service.SetCover: %w", err)
	}
	return updated, nil
}

func (s *service) ClearCover(ctx context.Context, cardID, userID, email string) (*Entity, error) {
	card, err := s.verifyCardAccess(ctx, cardID, userID, email)
	if err != nil {
		return nil, err
	}
	if err := editableForCover(card); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateCover(ctx, cardID, "", "", nil)
	if err != nil {
		return nil, fmt.Errorf("card.service.ClearCover: %w", err)
	}
	return updated, nil
}
