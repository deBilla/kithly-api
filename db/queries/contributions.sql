-- name: GetContributionByID :one
SELECT * FROM video_contributions WHERE id = $1 LIMIT 1;

-- name: ListContributionsByCard :many
SELECT * FROM video_contributions
WHERE video_card_id = $1
ORDER BY display_order ASC, created_at ASC;

-- name: ListIncludedContributionsByCard :many
SELECT * FROM video_contributions
WHERE video_card_id = $1 AND status = 'included'
ORDER BY display_order ASC, created_at ASC;

-- name: CreateContribution :one
INSERT INTO video_contributions (
    id, video_card_id, contributor_user_id, contributor_name,
    contributor_email, video_url, duration_seconds, display_order
) VALUES (
    gen_random_uuid(), $1, $2, $3, $4, $5, $6,
    COALESCE((SELECT MAX(display_order) + 1 FROM video_contributions WHERE video_card_id = $1), 0)
) RETURNING *;

-- name: UpdateContribution :one
UPDATE video_contributions
SET status         = COALESCE(sqlc.narg('status'), status),
    display_order  = COALESCE(sqlc.narg('display_order'), display_order),
    updated_at     = now()
WHERE id = $1
RETURNING *;

-- name: UpdateContributionURLs :exec
UPDATE video_contributions
SET hls_playlist_url = COALESCE(sqlc.narg('hls_playlist_url'), hls_playlist_url),
    thumbnail_url    = COALESCE(sqlc.narg('thumbnail_url'), thumbnail_url),
    original_url     = COALESCE(sqlc.narg('original_url'), original_url),
    updated_at       = now()
WHERE id = $1;

-- name: UpdateContributionOrder :exec
UPDATE video_contributions SET display_order = $2, updated_at = now() WHERE id = $1;

-- name: DeleteContribution :exec
DELETE FROM video_contributions WHERE id = $1;

-- name: CountIncludedContributions :one
SELECT COUNT(*) FROM video_contributions
WHERE video_card_id = $1 AND status = 'included';

-- name: CountIncludedContributionsByCards :many
-- Per-card count of contributions that will appear in the video, for the
-- dashboard. Cards with none are simply absent from the result.
SELECT video_card_id, COUNT(*)::int AS included_count
FROM video_contributions
WHERE video_card_id = ANY(sqlc.arg('card_ids')::uuid[])
  AND status = 'included'
GROUP BY video_card_id;
