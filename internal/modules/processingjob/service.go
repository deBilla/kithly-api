package processingjob

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	apperr "github.com/kithly/kithly-ws-go/internal/core/errors"
)

const (
	// QueueName is the asynq queue consumed by the video processing worker.
	QueueName    = "video-processing"
	TaskTypeName = "process-video"
)

// VideoJobPayload is the task payload sent to the video processing worker.
type VideoJobPayload struct {
	JobID string `json:"job_id"`
}

// Service defines the business logic contract for processing jobs.
// It also implements card.ProcessingEnqueuer.
type Service interface {
	// GetLatestByCard is for internal callers that have already authorised access.
	GetLatestByCard(ctx context.Context, cardID string) (*Entity, error)
	GetLatestForUser(ctx context.Context, cardID, userID, email string) (*Entity, error)
	CreateAndEnqueue(ctx context.Context, cardID string, isPreview bool) (string, error)
	Retry(ctx context.Context, jobID, userID, email string) (*Entity, error)
}

// AccessChecker returns nil if the user may manage the card. Implemented in
// cmd/api/main.go via card.VerifyAccess.
type AccessChecker interface {
	CheckAccess(ctx context.Context, cardID, userID, email string) error
}

type service struct {
	repo   Repository
	asynq  *asynq.Client
	access AccessChecker
}

// NewService constructs a Service.
func NewService(repo Repository, asynqClient *asynq.Client, access AccessChecker) Service {
	return &service{repo: repo, asynq: asynqClient, access: access}
}

func (s *service) GetLatestForUser(ctx context.Context, cardID, userID, email string) (*Entity, error) {
	if err := s.access.CheckAccess(ctx, cardID, userID, email); err != nil {
		return nil, err
	}
	return s.repo.GetLatestByCard(ctx, cardID)
}

func (s *service) GetLatestByCard(ctx context.Context, cardID string) (*Entity, error) {
	return s.repo.GetLatestByCard(ctx, cardID)
}

// CreateAndEnqueue creates a processing job record and enqueues an asynq task.
// It implements card.ProcessingEnqueuer.
func (s *service) CreateAndEnqueue(ctx context.Context, cardID string, isPreview bool) (string, error) {
	meta, _ := json.Marshal(map[string]interface{}{
		"is_preview": isPreview,
	})

	job, err := s.repo.Create(ctx, cardID, json.RawMessage(meta))
	if err != nil {
		return "", fmt.Errorf("processingjob.service.CreateAndEnqueue: create: %w", err)
	}

	payload, _ := json.Marshal(VideoJobPayload{JobID: job.ID})
	task := asynq.NewTask(TaskTypeName, payload,
		asynq.Queue(QueueName),
		asynq.MaxRetry(3),
	)
	if _, err := s.asynq.EnqueueContext(ctx, task); err != nil {
		return "", fmt.Errorf("processingjob.service.CreateAndEnqueue: enqueue: %w", err)
	}

	return job.ID, nil
}

func (s *service) Retry(ctx context.Context, jobID, userID, email string) (*Entity, error) {
	old, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if err := s.access.CheckAccess(ctx, old.VideoCardID, userID, email); err != nil {
		return nil, err
	}
	if old.Status != "failed" {
		return nil, apperr.BadRequest("only failed jobs can be retried")
	}

	newJobID, err := s.CreateAndEnqueue(ctx, old.VideoCardID, old.IsPreview())
	if err != nil {
		return nil, fmt.Errorf("processingjob.service.Retry: %w", err)
	}

	return s.repo.GetByID(ctx, newJobID)
}
