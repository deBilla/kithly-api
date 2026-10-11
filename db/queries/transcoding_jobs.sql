-- name: GetTranscodingJobByID :one
SELECT * FROM video_transcoding_jobs WHERE id = $1 LIMIT 1;

-- name: GetTranscodingJobByResourceAndType :one
SELECT * FROM video_transcoding_jobs
WHERE resource_id = $1 AND job_type = $2
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateTranscodingJob :one
INSERT INTO video_transcoding_jobs (
    id, resource_type, resource_id, job_type, video_card_id, status, metadata
) VALUES (
    gen_random_uuid(), $1, $2, $3, $4, 'queued', $5::json
) RETURNING *;

-- name: UpdateTranscodingJobStatus :exec
UPDATE video_transcoding_jobs
SET status        = $2,
    error_message = COALESCE(sqlc.narg('error_message'), error_message),
    started_at    = COALESCE(sqlc.narg('started_at'), started_at),
    completed_at  = COALESCE(sqlc.narg('completed_at'), completed_at),
    attempts      = attempts + 1,
    updated_at    = now()
WHERE id = $1;

-- name: UpdateTranscodingJobMetadata :exec
UPDATE video_transcoding_jobs SET metadata = $2::json, updated_at = now() WHERE id = $1;
