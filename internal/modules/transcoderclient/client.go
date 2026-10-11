// Package transcoderclient provides the asynq task producer for the
// video transcoding queue. It is used by both the API server (to enqueue
// contribution ingest jobs) and the video processing worker (to enqueue
// card-level transcoding after compilation).
package transcoderclient

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const (
	// QueueName is the asynq queue consumed by the transcoder worker.
	QueueName = "video-transcoding"

	// TaskTypeName is the asynq task type registered in the transcoder worker.
	TaskTypeName = "transcode"

	// Job type constants mirror the TypeScript VideoTranscoderJobType enum.
	JobTypeContributionIngest = "contribution_ingest"
	JobTypeCardPreview        = "card_preview"
	JobTypeCardCompilation    = "card_compilation"
)

// Payload mirrors the TypeScript VideoTranscodeQueuePayload struct.
// Keys are snake_case because the DB metadata field stores them this way
// and the worker reads them directly.
type Payload struct {
	JobID                 string                 `json:"job_id"`
	PayloadVersion        int                    `json:"payload_version"`
	ResourceType          string                 `json:"resource_type"`
	VideoCardID           string                 `json:"video_card_id"`
	ResourceID            string                 `json:"resource_id"`
	SourceURL             string                 `json:"source_url"`
	JobType               string                 `json:"job_type"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
	CompilationVersionKey string                 `json:"compilation_version_key,omitempty"`
}

// ContributionIngestInput holds the parameters for enqueuing a contribution ingest job.
type ContributionIngestInput struct {
	JobID          string
	VideoCardID    string
	ContributionID string
	SourceURL      string
	Metadata       map[string]interface{}
}

// CardTranscodeInput holds the parameters for enqueuing a card-level transcoding job.
type CardTranscodeInput struct {
	JobID                 string
	CardID                string
	SourceURL             string
	Metadata              map[string]interface{}
	CompilationVersionKey string
}

// Client is the asynq task producer interface for the transcoding queue.
type Client interface {
	EnqueueContributionIngest(ctx context.Context, input ContributionIngestInput) error
	EnqueueCardPreview(ctx context.Context, input CardTranscodeInput) error
	EnqueueCardCompilation(ctx context.Context, input CardTranscodeInput) error
}

type client struct {
	asynq *asynq.Client
}

// New constructs a Client using the given asynq client.
func New(asynqClient *asynq.Client) Client {
	return &client{asynq: asynqClient}
}

func (c *client) EnqueueContributionIngest(ctx context.Context, input ContributionIngestInput) error {
	p := Payload{
		JobID:          input.JobID,
		PayloadVersion: 1,
		ResourceType:   "contribution",
		VideoCardID:    input.VideoCardID,
		ResourceID:     input.ContributionID,
		SourceURL:      input.SourceURL,
		JobType:        JobTypeContributionIngest,
		Metadata:       input.Metadata,
	}
	return c.enqueue(ctx, p)
}

func (c *client) EnqueueCardPreview(ctx context.Context, input CardTranscodeInput) error {
	p := Payload{
		JobID:          input.JobID,
		PayloadVersion: 1,
		ResourceType:   "video_card",
		VideoCardID:    input.CardID,
		ResourceID:     input.CardID,
		SourceURL:      input.SourceURL,
		JobType:        JobTypeCardPreview,
		Metadata:       input.Metadata,
	}
	return c.enqueue(ctx, p)
}

func (c *client) EnqueueCardCompilation(ctx context.Context, input CardTranscodeInput) error {
	p := Payload{
		JobID:                 input.JobID,
		PayloadVersion:        1,
		ResourceType:          "video_card",
		VideoCardID:           input.CardID,
		ResourceID:            input.CardID,
		SourceURL:             input.SourceURL,
		JobType:               JobTypeCardCompilation,
		Metadata:              input.Metadata,
		CompilationVersionKey: input.CompilationVersionKey,
	}
	return c.enqueue(ctx, p)
}

func (c *client) enqueue(ctx context.Context, p Payload) error {
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("transcoderclient.enqueue: marshal: %w", err)
	}
	task := asynq.NewTask(TaskTypeName, data,
		asynq.Queue(QueueName),
		asynq.MaxRetry(3),
	)
	if _, err := c.asynq.EnqueueContext(ctx, task); err != nil {
		return fmt.Errorf("transcoderclient.enqueue: %w", err)
	}
	return nil
}
