// Command transcoder runs the HLS transcoding worker.
// It consumes "video-transcoding" asynq tasks, encodes each MP4 into an
// adaptive-bitrate HLS ladder packaged as CMAF (fMP4 segments), and uploads
// the result to S3.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"
	"go.uber.org/zap"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kithly/kithly-ws-go/internal/core/config"
	"github.com/kithly/kithly-ws-go/internal/core/db"
	"github.com/kithly/kithly-ws-go/internal/core/pghelper"
	redisclient "github.com/kithly/kithly-ws-go/internal/core/redis"
	"github.com/kithly/kithly-ws-go/internal/core/sqlcdb"
	"github.com/kithly/kithly-ws-go/internal/modules/transcoderclient"
	"github.com/kithly/kithly-ws-go/pkg/abr"
	"github.com/kithly/kithly-ws-go/pkg/firebase"
	pkgs3 "github.com/kithly/kithly-ws-go/pkg/s3"
)

// transcodingCompletedEvent is published to Redis after a successful transcoding job.
type transcodingCompletedEvent struct {
	JobID        string `json:"job_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	JobType      string `json:"job_type"`
	VideoCardID  string `json:"video_card_id"`
	HLSKey       string `json:"hls_key"`
	ThumbnailKey string `json:"thumbnail_key"`
}

const completedChannel = "video.transcoding.completed"

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("logger init: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("config load", zap.Error(err))
	}

	pool, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("db connect", zap.Error(err))
	}
	defer pool.Close()

	s3Client, err := pkgs3.New(context.Background(), cfg)
	if err != nil {
		logger.Fatal("s3 init", zap.Error(err))
	}

	fbClient, err := firebase.New(context.Background(), cfg.FirebaseServiceAccountBase64)
	if err != nil {
		logger.Fatal("firebase init", zap.Error(err))
	}

	queries := sqlcdb.New(pool)
	handler := &transcoderHandler{
		logger:   logger,
		queries:  queries,
		s3:       s3Client,
		cfg:      cfg,
		packager: abr.NewPackager(abr.ExecRunner{}, abr.DefaultLadder),
		notifier: &organizerNotifier{store: queries, push: fbClient},
	}

	srv := redisclient.NewServer(cfg, 2, map[string]int{
		transcoderclient.QueueName: 1,
	})
	mux := asynq.NewServeMux()
	mux.HandleFunc(transcoderclient.TaskTypeName, handler.Process)

	logger.Info("transcoder worker starting")
	if err := srv.Run(mux); err != nil {
		logger.Fatal("worker error", zap.Error(err))
	}
}

type transcoderHandler struct {
	logger   *zap.Logger
	queries  *sqlcdb.Queries
	s3       *pkgs3.Client
	cfg      *config.Config
	packager abr.Packager
	notifier finalVideoNotifier // optional; nil disables push notifications
}

// updateStatus is a helper that writes a status transition to the DB and logs
// any error rather than propagating it — a status update failure should never
// mask the underlying job error.
func (p *transcoderHandler) updateStatus(ctx context.Context, jobID pgtype.UUID, status sqlcdb.JobStatus, errMsg *string, started, completed *time.Time) {
	if err := p.queries.UpdateProcessingJobStatus(ctx, sqlcdb.UpdateProcessingJobStatusParams{
		ID:           jobID,
		Status:       status,
		ErrorMessage: pghelper.Text(errMsg),
		StartedAt:    pghelper.Timestamptz(started),
		CompletedAt:  pghelper.Timestamptz(completed),
	}); err != nil {
		p.logger.Error("failed to update job status", zap.String("status", string(status)), zap.Error(err))
	}
}

// Process handles a single transcoding task.
func (h *transcoderHandler) Process(ctx context.Context, t *asynq.Task) error {
	var payload transcoderclient.Payload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("transcoder.Process: unmarshal: %w", err)
	}

	h.logger.Info("transcoding job",
		zap.String("job_id", payload.JobID),
		zap.String("type", payload.JobType),
	)

	tmpDir, err := os.MkdirTemp("", "kithly-transcoder-*")
	if err != nil {
		return fmt.Errorf("transcoder.Process: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download source MP4.
	sourcePath := filepath.Join(tmpDir, "source.mp4")
	if err := h.downloadFile(ctx, payload.SourceURL, sourcePath); err != nil {
		return fmt.Errorf("transcoder.Process: download: %w", err)
	}

	ts := time.Now().UnixMilli()

	var basePrefix string
	switch payload.JobType {
	case transcoderclient.JobTypeContributionIngest:
		basePrefix = fmt.Sprintf("contributions/%s/%s", payload.VideoCardID, payload.ResourceID)
	default:
		basePrefix = fmt.Sprintf("video-cards/%s/%s", payload.VideoCardID, payload.JobType)
	}

	// Encode the ABR ladder.
	hlsDir := filepath.Join(tmpDir, "hls")
	if err := os.MkdirAll(hlsDir, 0755); err != nil {
		return fmt.Errorf("transcoder.Process: mkdir hls: %w", err)
	}
	result, err := h.packager.Package(ctx, sourcePath, hlsDir, encodeOptions(payload.JobType))
	if err != nil {
		return fmt.Errorf("transcoder.Process: package ABR: %w", err)
	}

	// Extract thumbnail.
	thumbPath := filepath.Join(tmpDir, "thumbnail.jpg")
	if err := h.extractThumbnail(sourcePath, thumbPath); err != nil {
		h.logger.Warn("thumbnail extraction failed", zap.Error(err))
	}

	// Upload HLS files to S3.
	hlsPrefix := fmt.Sprintf("%s/hls_%d", basePrefix, ts)
	hlsKey := hlsPrefix + "/" + result.MasterPlaylist
	if err := h.uploadDir(ctx, hlsDir, hlsPrefix); err != nil {
		return fmt.Errorf("transcoder.Process: upload HLS: %w", err)
	}

	// Upload thumbnail.
	thumbKey := fmt.Sprintf("%s/thumbnail_%d.jpg", basePrefix, ts)
	if thumbFile, err := os.Open(thumbPath); err == nil {
		defer thumbFile.Close()
		_ = h.s3.UploadFile(ctx, thumbKey, "image/jpeg", thumbFile)
	}

	// Store job completion in DB.
	meta, _ := json.Marshal(map[string]interface{}{
		"source_path": payload.SourceURL,
		"output": map[string]string{
			"hls_url":       hlsKey,
			"thumbnail_url": thumbKey,
		},
	})
	_ = h.queries.UpdateTranscodingJobMetadata(ctx, sqlcdb.UpdateTranscodingJobMetadataParams{
		ID:       pghelper.UUID(payload.JobID),
		Metadata: string(meta),
	})

	// Update resource URLs in DB based on job type.
	h.updateResourceURLs(ctx, payload, hlsKey, thumbKey)
	completedAt := time.Now()
	h.updateStatus(ctx, pghelper.UUID(payload.JobID), sqlcdb.JobStatusCompleted, nil, nil, &completedAt)

	// Only the final video: previews are watched in-app as they finish.
	// A failed push must not fail (and so retry) a finished transcode.
	if payload.JobType == transcoderclient.JobTypeCardCompilation && h.notifier != nil {
		if err := h.notifier.NotifyFinalVideoReady(ctx, payload.VideoCardID); err != nil {
			h.logger.Warn("final video push failed", zap.String("card_id", payload.VideoCardID), zap.Error(err))
		}
	}

	h.logger.Info("transcoding complete",
		zap.String("job_id", payload.JobID),
		zap.String("hls_key", hlsKey),
		zap.Int("renditions", len(result.Renditions)),
	)
	return nil
}

// encodeOptions trades encode speed against quality per job type. Previews
// and contribution ingests are on the user's critical path and are replaced or
// re-compiled later; the final compilation is what recipients keep watching.
func encodeOptions(jobType string) abr.Options {
	opts := abr.DefaultOptions()
	switch jobType {
	case transcoderclient.JobTypeCardPreview:
		opts.Preset = "veryfast"
	case transcoderclient.JobTypeContributionIngest:
		opts.Preset = "fast"
	}
	return opts
}

func (h *transcoderHandler) extractThumbnail(input, output string) error {
	return ffmpeg.Input(input, ffmpeg.KwArgs{"ss": "5"}).
		Output(output, ffmpeg.KwArgs{"vframes": "1", "q:v": "2"}).
		OverWriteOutput().ErrorToStdOut().Run()
}

func (h *transcoderHandler) updateResourceURLs(ctx context.Context, payload transcoderclient.Payload, hlsKey, thumbKey string) {
	now := pghelper.TimestamptzVal(time.Now())

	switch payload.JobType {
	case transcoderclient.JobTypeContributionIngest:
		_ = h.queries.UpdateContributionURLs(ctx, sqlcdb.UpdateContributionURLsParams{
			ID:             pghelper.UUID(payload.ResourceID),
			HlsPlaylistUrl: pghelper.TextVal(hlsKey),
			ThumbnailUrl:   pghelper.TextVal(thumbKey),
		})
	case transcoderclient.JobTypeCardPreview:
		_ = h.queries.UpdateCardVideoURLs(ctx, sqlcdb.UpdateCardVideoURLsParams{
			ID:                     pghelper.UUID(payload.VideoCardID),
			PreviewUrl:             pghelper.TextVal(payload.SourceURL),
			PreviewHlsPlaylistUrl:  pghelper.TextVal(hlsKey),
			ThumbnailUrl:           pghelper.TextVal(thumbKey),
			PreviewStatus:          pghelper.TextVal("ready"),
			LastPreviewGeneratedAt: now,
			// Leave other fields unchanged.
			FinalVideoUrl:       pgtype.Text{},
			FinalHlsPlaylistUrl: pgtype.Text{},
			ProcessedAt:         pgtype.Timestamptz{},
			FinalDuration:       pgtype.Float8{},
		})
	case transcoderclient.JobTypeCardCompilation:
		_ = h.queries.UpdateCardVideoURLs(ctx, sqlcdb.UpdateCardVideoURLsParams{
			ID:                  pghelper.UUID(payload.VideoCardID),
			FinalVideoUrl:       pghelper.TextVal(payload.SourceURL),
			FinalHlsPlaylistUrl: pghelper.TextVal(hlsKey),
			ThumbnailUrl:        pghelper.TextVal(thumbKey),
			ProcessedAt:         now,
			// Leave other fields unchanged.
			PreviewUrl:             pgtype.Text{},
			PreviewHlsPlaylistUrl:  pgtype.Text{},
			PreviewStatus:          pgtype.Text{},
			LastPreviewGeneratedAt: pgtype.Timestamptz{},
			FinalDuration:          pgtype.Float8{},
		})
		_ = h.queries.UpdateCardStatus(ctx, sqlcdb.UpdateCardStatusParams{
			ID:     pghelper.UUID(payload.VideoCardID),
			Status: sqlcdb.CardStatusCompleted,
		})
	}
}

func (h *transcoderHandler) downloadFile(ctx context.Context, key, destPath string) error {
	r, err := h.s3.DownloadFile(ctx, key)
	if err != nil {
		return err
	}
	defer r.Close()
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

// uploadDir uploads every file under localDir (recursively, since each ABR
// rendition lives in its own sub-directory) to the given s3Prefix, preserving
// relative paths so the playlists' relative URIs resolve.
func (h *transcoderHandler) uploadDir(ctx context.Context, localDir, s3Prefix string) error {
	return filepath.WalkDir(localDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		s3Key := s3Prefix + "/" + filepath.ToSlash(rel)

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := h.s3.UploadFile(ctx, s3Key, abr.ContentType(rel), f); err != nil {
			return fmt.Errorf("upload %s: %w", s3Key, err)
		}
		return nil
	})
}
