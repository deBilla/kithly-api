-- name: GetProcessingJobByID :one
SELECT * FROM video_processing_jobs WHERE id = $1 LIMIT 1;

-- name: GetLatestProcessingJob :one
SELECT * FROM video_processing_jobs
WHERE video_card_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateProcessingJob :one
INSERT INTO video_processing_jobs (id, video_card_id, status, metadata)
VALUES (gen_random_uuid(), $1, 'queued', $2::json)
RETURNING *;

-- name: UpdateProcessingJobStatus :exec
UPDATE video_processing_jobs
SET status        = $2,
    error_message = COALESCE(sqlc.narg('error_message'), error_message),
    started_at    = COALESCE(sqlc.narg('started_at'), started_at),
    completed_at  = COALESCE(sqlc.narg('completed_at'), completed_at),
    attempts      = attempts + 1
WHERE id = $1;

-- name: UpdateProcessingJobMetadata :exec
UPDATE video_processing_jobs SET metadata = $2::json WHERE id = $1;

-- name: MarkProcessingJobRetried :exec
UPDATE video_processing_jobs
SET status = 'retried', retried_job_id = $2, updated_at = now()
WHERE id = $1;
