-- name: GetVideoOverlayByID :one
SELECT * FROM video_overlays WHERE id = $1 LIMIT 1;

-- name: ListVideoOverlays :many
SELECT * FROM video_overlays WHERE is_active = true ORDER BY sort_order ASC, created_at ASC;

-- name: CreateVideoOverlay :one
INSERT INTO video_overlays (id, label, resource_url, thumbnail_url, overlay_type, category, config, is_active, sort_order)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateVideoOverlay :one
UPDATE video_overlays
SET label         = COALESCE(sqlc.narg('label'), label),
    resource_url  = COALESCE(sqlc.narg('resource_url'), resource_url),
    thumbnail_url = COALESCE(sqlc.narg('thumbnail_url'), thumbnail_url),
    overlay_type  = COALESCE(sqlc.narg('overlay_type'), overlay_type),
    category      = COALESCE(sqlc.narg('category'), category),
    config        = COALESCE(sqlc.narg('config'), config),
    is_active     = COALESCE(sqlc.narg('is_active'), is_active),
    sort_order    = COALESCE(sqlc.narg('sort_order'), sort_order),
    updated_at    = now()
WHERE id = $1
RETURNING *;

-- name: DeleteVideoOverlay :exec
DELETE FROM video_overlays WHERE id = $1;
