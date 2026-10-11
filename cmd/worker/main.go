// Command worker runs the video processing worker.
// It consumes "video-processing" asynq tasks and compiles video contributions
// into a final MP4 using FFmpeg, then enqueues the result for HLS transcoding.
package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	"github.com/kithly/kithly-ws-go/internal/modules/processingjob"
	"github.com/kithly/kithly-ws-go/internal/modules/transcoderclient"
	pkgs3 "github.com/kithly/kithly-ws-go/pkg/s3"
)

//go:embed assets
var assetFiles embed.FS

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

	asynqClient := redisclient.NewClient(cfg)
	defer asynqClient.Close() //nolint:errcheck

	tc := transcoderclient.New(asynqClient)
	queries := sqlcdb.New(pool)

	// Extract embedded assets (overlays, RNN model) to a temp directory once at startup.
	// If extraction fails the worker continues but skips watermarks and audio denoising.
	assetDir, err := extractAssets(assetFiles)
	if err != nil {
		logger.Warn("failed to extract embedded assets; overlays/denoising will be skipped", zap.Error(err))
		assetDir = ""
	}
	if assetDir != "" {
		defer os.RemoveAll(assetDir) //nolint:errcheck
	}

	handler := &videoProcessor{
		logger:   logger,
		queries:  queries,
		s3:       s3Client,
		tc:       tc,
		assetDir: assetDir,
	}

	srv := redisclient.NewServer(cfg, 2, map[string]int{
		processingjob.QueueName: 1,
	})
	mux := asynq.NewServeMux()
	mux.HandleFunc(processingjob.TaskTypeName, handler.Process)

	logger.Info("video processing worker starting")
	if err := srv.Run(mux); err != nil {
		logger.Fatal("worker error", zap.Error(err))
	}
}

// extractAssets writes the embedded asset files to a temporary directory.
// The caller must os.RemoveAll the returned path when done.
func extractAssets(embedded embed.FS) (string, error) {
	dir, err := os.MkdirTemp("", "kithly-assets-*")
	if err != nil {
		return "", fmt.Errorf("extractAssets: mkdir: %w", err)
	}
	entries, err := fs.ReadDir(embedded, "assets")
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("extractAssets: readdir: %w", err)
	}
	for _, e := range entries {
		data, err := embedded.ReadFile("assets/" + e.Name())
		if err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("extractAssets: read %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0644); err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("extractAssets: write %s: %w", e.Name(), err)
		}
	}
	return dir, nil
}

// videoProcessor implements the FFmpeg-based video compilation logic.
type videoProcessor struct {
	logger   *zap.Logger
	queries  *sqlcdb.Queries
	s3       *pkgs3.Client
	tc       transcoderclient.Client
	assetDir string // path to extracted embedded assets; empty if unavailable
}

// updateStatus is a helper that writes a status transition to the DB and logs
// any error rather than propagating it — a status update failure should never
// mask the underlying job error.
func (p *videoProcessor) updateStatus(ctx context.Context, jobID pgtype.UUID, status sqlcdb.JobStatus, errMsg *string, started, completed *time.Time) {
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

// Process handles a single video processing task.
func (p *videoProcessor) Process(ctx context.Context, t *asynq.Task) error {
	var payload processingjob.VideoJobPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("worker.Process: unmarshal payload: %w", err)
	}

	jobID := payload.JobID
	p.logger.Info("processing job", zap.String("job_id", jobID))

	// Load job from DB.
	job, err := p.queries.GetProcessingJobByID(ctx, pghelper.UUID(jobID))
	if err != nil {
		return fmt.Errorf("worker.Process: get job: %w", err)
	}

	// Determine mode from metadata before defining fail, so the closure can reference isPreview.
	isPreview := false
	if job.Metadata != nil {
		var meta struct {
			IsPreview bool `json:"is_preview"`
		}
		if err := json.Unmarshal(job.Metadata, &meta); err == nil {
			isPreview = meta.IsPreview
		}
	}

	// Mark the job as processing so callers can observe progress.
	// On any subsequent failure, mark the job failed and reset card state so users can retry.
	startedAt := time.Now()
	p.updateStatus(ctx, job.ID, sqlcdb.JobStatusProcessing, nil, &startedAt, nil)
	fail := func(err error) error {
		msg := err.Error()
		p.updateStatus(ctx, job.ID, sqlcdb.JobStatusFailed, &msg, nil, nil)
		// Reset card state so the organiser is not permanently locked out.
		if isPreview {
			if dbErr := p.queries.SetCardPreviewFailed(ctx, job.VideoCardID); dbErr != nil {
				p.logger.Error("failed to set card preview_status=failed", zap.Error(dbErr))
			}
		} else {
			if dbErr := p.queries.UnlockCardAfterFailure(ctx, job.VideoCardID); dbErr != nil {
				p.logger.Error("failed to unlock card after failure", zap.Error(dbErr))
			}
		}
		return err
	}

	cardID := pghelper.StringFromUUID(job.VideoCardID)

	// Load card.
	card, err := p.queries.GetCardByID(ctx, job.VideoCardID)
	if err != nil {
		return fail(fmt.Errorf("worker.Process: get card: %w", err))
	}

	// Load included contributions.
	allContributions, err := p.queries.ListIncludedContributionsByCard(ctx, job.VideoCardID)
	if err != nil {
		return fail(fmt.Errorf("worker.Process: list contributions: %w", err))
	}

	// Reject contributions with no meaningful duration (broken uploads).
	// Images are still accepted — they carry a duration and are rendered as a
	// still-frame loop by FFmpeg (matching the TypeScript worker behaviour).
	// Contributions without any duration fall back to 3 s (matching TS fallback).
	var contributions []sqlcdb.VideoContribution
	for _, c := range allContributions {
		if c.DurationSeconds.Valid && c.DurationSeconds.Float64 >= 0.5 {
			contributions = append(contributions, c)
		}
	}
	if len(contributions) == 0 {
		return fail(fmt.Errorf("worker.Process: no usable contributions (all %d had no valid duration) for card %s", len(allContributions), cardID))
	}

	// Create temp working directory.
	tmpDir, err := os.MkdirTemp("", "kithly-worker-*")
	if err != nil {
		return fail(fmt.Errorf("worker.Process: create temp dir: %w", err))
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	// Download contributions and probe audio.
	compInputs := make([]compInput, 0, len(contributions))
	for i, contrib := range contributions {
		ext := filepath.Ext(contrib.VideoUrl)
		if ext == "" {
			ext = ".mp4"
		}
		localPath := filepath.Join(tmpDir, fmt.Sprintf("input_%d%s", i, ext))
		if err := p.downloadFile(ctx, contrib.VideoUrl, localPath); err != nil {
			return fail(fmt.Errorf("worker.Process: download contribution %s: %w", contrib.ID, err))
		}
		localPath, isImg, err := resolveStillImage(contrib.VideoUrl, localPath)
		if err != nil {
			return fail(fmt.Errorf("worker.Process: contribution %s: %w", pghelper.StringFromUUID(contrib.ID), err))
		}
		hasAud := false
		if !isImg {
			hasAud = probeHasAudio(localPath)
			if !hasAud {
				p.logger.Warn("contribution has no audio stream; silence will be used",
					zap.String("contribution_id", pghelper.StringFromUUID(contrib.ID)))
			}
		}
		compInputs = append(compInputs, compInput{
			path:     localPath,
			isImage:  isImg,
			hasAudio: hasAud,
			duration: contrib.DurationSeconds.Float64,
		})
	}

	// Prepend the cover frame when the card has one. It is optional polish, so
	// a missing or broken cover is logged and skipped rather than failing a
	// video people have already recorded messages for.
	if card.CoverImageUrl.Valid && card.CoverImageUrl.String != "" {
		coverKey := card.CoverImageUrl.String
		ext := filepath.Ext(coverKey)
		if ext == "" {
			ext = ".jpg"
		}
		coverPath := filepath.Join(tmpDir, "cover"+ext)
		if err := p.downloadFile(ctx, coverKey, coverPath); err != nil {
			p.logger.Warn("failed to download cover image; continuing without it",
				zap.String("card_id", cardID), zap.String("cover_key", coverKey), zap.Error(err))
		} else {
			compInputs = append([]compInput{{
				path:     coverPath,
				isImage:  true,
				duration: coverDurationSeconds,
			}}, compInputs...)
		}
	}

	// Download music theme to disk (matching TS worker behaviour).
	// audio_url may be stored as a bare S3 key or as a full HTTPS URL; handle both.
	// A failure here is non-fatal — the job continues without music.
	var musicPath string
	if card.MusicThemeID.Valid {
		theme, err := p.queries.GetMusicThemeByID(ctx, card.MusicThemeID)
		if err == nil && theme.AudioUrl != "" {
			key := theme.AudioUrl
			if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
				if u, parseErr := url.Parse(key); parseErr == nil {
					key = strings.TrimPrefix(u.Path, "/")
				}
			}
			ext := filepath.Ext(key)
			if ext == "" || len(ext) > 5 {
				ext = ".mp3"
			}
			musicLocal := filepath.Join(tmpDir, "music"+ext)
			if dlErr := p.downloadFile(ctx, key, musicLocal); dlErr != nil {
				p.logger.Warn("failed to download background music; continuing without audio",
					zap.String("audio_url", theme.AudioUrl),
					zap.Error(dlErr),
				)
			} else {
				musicPath = musicLocal
			}
		}
	}

	// Compile with FFmpeg.
	outputPath := filepath.Join(tmpDir, "output.mp4")
	if err := p.compile(compInputs, musicPath, outputPath, isPreview); err != nil {
		return fail(fmt.Errorf("worker.Process: compile: %w", err))
	}

	// Upload output to S3.
	fileType := "preview"
	if !isPreview {
		fileType = "final"
	}
	s3Key := fmt.Sprintf("video-cards/%s/%s_%s.mp4", cardID, fileType, jobID)

	f, err := os.Open(outputPath)
	if err != nil {
		return fail(fmt.Errorf("worker.Process: open output: %w", err))
	}
	defer f.Close()

	if err := p.s3.UploadFile(ctx, s3Key, "video/mp4", f); err != nil {
		return fail(fmt.Errorf("worker.Process: upload: %w", err))
	}

	// Update job metadata with output URL.
	metaKey := "preview_video_url"
	if !isPreview {
		metaKey = "final_video_url"
	}
	newMeta, _ := json.Marshal(map[string]interface{}{
		"is_preview": isPreview,
		metaKey:      s3Key,
	})
	if err := p.queries.UpdateProcessingJobMetadata(ctx, sqlcdb.UpdateProcessingJobMetadataParams{
		ID:       job.ID,
		Metadata: string(newMeta),
	}); err != nil {
		p.logger.Warn("failed to update job metadata", zap.Error(err))
	}

	// Enqueue transcoding job.
	if isPreview {
		if err := p.tc.EnqueueCardPreview(ctx, transcoderclient.CardTranscodeInput{
			JobID:     jobID,
			CardID:    cardID,
			SourceURL: s3Key,
		}); err != nil {
			return fail(fmt.Errorf("worker.Process: enqueue preview transcoding: %w", err))
		}
	} else {
		if err := p.tc.EnqueueCardCompilation(ctx, transcoderclient.CardTranscodeInput{
			JobID:     jobID,
			CardID:    cardID,
			SourceURL: s3Key,
		}); err != nil {
			return fail(fmt.Errorf("worker.Process: enqueue compilation transcoding: %w", err))
		}
	}

	// completedAt := time.Now()
	// p.updateStatus(ctx, job.ID, sqlcdb.JobStatusCompleted, nil, nil, &completedAt)
	p.logger.Info("job completed", zap.String("job_id", jobID), zap.String("s3_key", s3Key))
	return nil
}

// coverDurationSeconds is how long the cover frame is shown before the first
// message. Long enough to read the greeting, short enough not to drag.
const coverDurationSeconds = 3.0

// compInput carries per-contribution metadata needed to build the FFmpeg filter graph.
type compInput struct {
	path     string
	isImage  bool
	hasAudio bool
	duration float64 // seconds — used for image loop duration and anullsrc
}

// probeHasAudio runs ffprobe on filePath and returns true if an audio stream is present.
func probeHasAudio(filePath string) bool {
	data, err := ffmpeg.Probe(filePath)
	if err != nil {
		return false
	}
	var meta struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
		} `json:"streams"`
	}
	if err := json.Unmarshal([]byte(data), &meta); err != nil {
		return false
	}
	for _, s := range meta.Streams {
		if s.CodecType == "audio" {
			return true
		}
	}
	return false
}

// compile runs FFmpeg to stitch contributions into a single MP4, matching the
// TypeScript worker's filter graph:
//   - Per-contribution scale/pad to target resolution, setsar=1, yuv420p
//   - Per-contribution audio: anullsrc for silent/image inputs; arnndn+loudnorm for video with audio
//   - concat=v=1:a=1 to merge all segments
//   - Music sidechain compression/ducking when a music track is present
//   - Preview: kithly-preview.png watermark overlay
//   - Final: kithly-stamp.png overlay + 3-second black outro with k-logo.png
//
// Preview: 720×1280, ultrafast, CRF 28.
// Final:   1080×1920, slow,     CRF 23.
func (p *videoProcessor) compile(inputs []compInput, musicPath, output string, isPreview bool) error {
	width, height := 1080, 1920
	preset := "slow"
	crf := "23"
	if isPreview {
		width, height = 720, 1280
		preset = "ultrafast"
		crf = "28"
	}
	scaleRes := fmt.Sprintf("%d:%d", width, height)
	colorRes := fmt.Sprintf("%dx%d", width, height)

	// --- Build -i input arguments and track stream indices ---
	var args []string
	nextIdx := 0

	for _, inp := range inputs {
		if inp.isImage {
			args = append(args, "-loop", "1", "-t", fmt.Sprintf("%.3f", inp.duration))
		}
		args = append(args, "-i", inp.path)
		nextIdx++
	}

	musicIdx := -1
	if musicPath != "" {
		args = append(args, "-i", musicPath)
		musicIdx = nextIdx
		nextIdx++
	}

	previewPatternIdx, logoIdx, stampIdx := -1, -1, -1
	if p.assetDir != "" {
		if isPreview {
			pp := filepath.Join(p.assetDir, "kithly-preview.png")
			if _, err := os.Stat(pp); err == nil {
				args = append(args, "-i", pp)
				previewPatternIdx = nextIdx
				nextIdx++
			}
		} else {
			lp := filepath.Join(p.assetDir, "k-logo.png")
			sp := filepath.Join(p.assetDir, "kithly-stamp.png")
			if _, err := os.Stat(lp); err == nil {
				args = append(args, "-i", lp)
				logoIdx = nextIdx
				nextIdx++
			}
			if _, err := os.Stat(sp); err == nil {
				args = append(args, "-i", sp)
				stampIdx = nextIdx
				nextIdx++
			}
		}
	}

	// --- Build filter_complex ---

	// RNN model path for audio denoising. Colons must be escaped in FFmpeg option strings.
	rnnModel := ""
	if p.assetDir != "" {
		rp := filepath.Join(p.assetDir, "bd.rnn")
		if _, err := os.Stat(rp); err == nil {
			rnnModel = strings.ReplaceAll(rp, "\\", "/")
			rnnModel = strings.ReplaceAll(rnnModel, ":", "\\:")
		}
	}

	var filters []string
	var mainDur float64

	for i, inp := range inputs {
		// Video: scale to target resolution preserving aspect, pad, normalise SAR and colour space.
		filters = append(filters, fmt.Sprintf(
			"[%d:v]scale=%s:force_original_aspect_ratio=decrease,pad=%s:(ow-iw)/2:(oh-ih)/2:black,setsar=1,format=yuv420p[v%d]",
			i, scaleRes, scaleRes, i))

		// Audio: images and silent video get a silence generator; voiced video gets denoised + normalised.
		if inp.isImage || !inp.hasAudio {
			filters = append(filters, fmt.Sprintf(
				"anullsrc=channel_layout=stereo:sample_rate=48000:d=%.3f[a%d]",
				inp.duration, i))
		} else if rnnModel != "" {
			filters = append(filters, fmt.Sprintf(
				"[%d:a]aformat=sample_rates=48000:channel_layouts=stereo,arnndn=m='%s',loudnorm=I=-16:TP=-1.5:LRA=11[a%d]",
				i, rnnModel, i))
		} else {
			// RNN model unavailable — still normalise loudness.
			filters = append(filters, fmt.Sprintf(
				"[%d:a]aformat=sample_rates=48000:channel_layouts=stereo,loudnorm=I=-16:TP=-1.5:LRA=11[a%d]",
				i, i))
		}

		mainDur += inp.duration
	}

	// Concat all segments (video + audio).
	var concatIn strings.Builder
	for i := range inputs {
		fmt.Fprintf(&concatIn, "[v%d][a%d]", i, i)
	}
	filters = append(filters, fmt.Sprintf(
		"%sconcat=n=%d:v=1:a=1[v_concatenated][a_raw_concatenated]",
		concatIn.String(), len(inputs)))

	// Outro duration: 3 s black screen with logo, only for final when logo is available.
	outroDuration := 0.0
	if !isPreview && logoIdx >= 0 {
		outroDuration = 3.0
	}

	// Audio mixing: sidechain compression when music is present, otherwise pass-through.
	if musicIdx >= 0 {
		filters = append(filters, fmt.Sprintf(
			"[a_raw_concatenated]apad=pad_dur=%.1f,asplit=2[a_voice_clean][a_voice_trigger]", outroDuration+0.5))
		filters = append(filters, fmt.Sprintf(
			"[%d:a]aloop=loop=-1:size=2000000000,volume=0.4,aformat=sample_rates=48000[music_looped]", musicIdx))
		filters = append(filters,
			"[music_looped][a_voice_trigger]sidechaincompress=threshold=0.08:ratio=6:attack=20:release=300[music_ducked]")
		filters = append(filters, fmt.Sprintf(
			"[music_ducked]afade=t=out:st=%.3f:d=%.1f[music_faded]", mainDur, outroDuration))
		filters = append(filters,
			"[a_voice_clean][music_faded]amix=inputs=2:duration=first:dropout_transition=0[a_main]")
	} else {
		filters = append(filters, fmt.Sprintf(
			"[a_raw_concatenated]apad=pad_dur=%.1f,volume=1.0[a_main]", outroDuration+0.5))
	}

	// Video overlay / outro.
	videoMapLabel := "[v_concatenated]"
	if isPreview {
		if previewPatternIdx >= 0 {
			filters = append(filters, fmt.Sprintf(
				"[%d:v]scale=450:-1,format=rgba,colorchannelmixer=aa=0.7[preview_scaled]", previewPatternIdx))
			filters = append(filters,
				"[v_concatenated][preview_scaled]overlay=main_w-overlay_w-10:main_h-overlay_h-20[v_watermarked]")
			videoMapLabel = "[v_watermarked]"
		} else {
			// No watermark asset — pass through with a no-op null filter.
			filters = append(filters, "[v_concatenated]null[v_watermarked]")
			videoMapLabel = "[v_watermarked]"
		}
	} else if logoIdx >= 0 {
		watermarkSrc := "[v_concatenated]"
		if stampIdx >= 0 {
			filters = append(filters, fmt.Sprintf(
				"[%d:v]scale=450:-1,format=rgba,colorchannelmixer=aa=0.7[stamp_scaled]", stampIdx))
			filters = append(filters,
				"[v_concatenated][stamp_scaled]overlay=main_w-overlay_w-10:main_h-overlay_h-20[v_stamped]")
			watermarkSrc = "[v_stamped]"
		}
		filters = append(filters, fmt.Sprintf(
			"color=c=black:s=%s:d=%.1f[black_bg]", colorRes, outroDuration))
		filters = append(filters, fmt.Sprintf(
			"[%d:v]scale=500:-1[logo_centered]", logoIdx))
		filters = append(filters,
			"[black_bg][logo_centered]overlay=(W-w)/2:(H-h)/2[v_outro_visual]")
		filters = append(filters, fmt.Sprintf(
			"%s[v_outro_visual]concat=n=2:v=1:a=0[outv]", watermarkSrc))
		videoMapLabel = "[outv]"
	}

	// Assemble the full ffmpeg command.
	args = append(args,
		"-filter_complex", strings.Join(filters, "; "),
		"-map", videoMapLabel,
		"-map", "[a_main]",
		"-c:v", "libx264",
		"-preset", preset,
		"-crf", crf,
		"-c:a", "aac",
		"-b:a", "256k",
		"-ar", "48000",
		"-pix_fmt", "yuv420p",
		"-movflags", "+faststart",
		"-shortest",
		"-r", "30",
		"-y", output,
	)

	cmd := exec.Command("ffmpeg", args...) //nolint:gosec
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg compile: %w", err)
	}
	return nil
}

// isImageURL returns true when the S3 key or URL has an image file extension.
func isImageURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp", ".gif"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// resolveStillImage decides whether a downloaded contribution is a still
// image. Upload keys carry no extension (contributions/<card>/<id>/original_<id>),
// so the key alone is not enough: the contents are sniffed as well. Otherwise
// photos and text cards are decoded as one-frame videos and vanish from the
// compiled video. A sniffed image is renamed to a matching extension so
// FFmpeg's image demuxer, which -loop requires, picks it up.
func resolveStillImage(key, localPath string) (string, bool, error) {
	if isImageURL(key) {
		return localPath, true, nil
	}
	imgExt := imageExtFromFile(localPath)
	if imgExt == "" {
		return localPath, false, nil
	}
	renamed := strings.TrimSuffix(localPath, filepath.Ext(localPath)) + imgExt
	if err := os.Rename(localPath, renamed); err != nil {
		return "", false, fmt.Errorf("rename image: %w", err)
	}
	return renamed, true, nil
}

// imageExtFromFile returns the extension for a JPEG, PNG, GIF or WebP file
// (the still formats the app uploads as contributions), or "" if the file
// is not one of them.
func imageExtFromFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck

	head := make([]byte, 12)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	switch {
	case bytes.HasPrefix(head, []byte{0xFF, 0xD8, 0xFF}):
		return ".jpg"
	case bytes.HasPrefix(head, []byte{0x89, 'P', 'N', 'G'}):
		return ".png"
	case bytes.HasPrefix(head, []byte("GIF8")):
		return ".gif"
	case len(head) == 12 && bytes.Equal(head[:4], []byte("RIFF")) && bytes.Equal(head[8:12], []byte("WEBP")):
		return ".webp"
	}
	return ""
}

func (p *videoProcessor) downloadFile(ctx context.Context, key, destPath string) error {
	r, err := p.s3.DownloadFile(ctx, key)
	if err != nil {
		return err
	}
	defer r.Close()

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", destPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write %s: %w", destPath, err)
	}
	return nil
}
