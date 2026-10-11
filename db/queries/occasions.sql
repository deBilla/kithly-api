-- name: GetOccasionByID :one
SELECT * FROM occasions WHERE id = $1 LIMIT 1;

-- name: ListOccasions :many
SELECT * FROM occasions ORDER BY sort_order ASC, label ASC;

-- name: CreateOccasion :one
INSERT INTO occasions (id, label, emoji, contributor_tips, message_template, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateOccasion :one
UPDATE occasions
SET label            = COALESCE(sqlc.narg('label'), label),
    emoji            = COALESCE(sqlc.narg('emoji'), emoji),
    contributor_tips = COALESCE(sqlc.narg('contributor_tips'), contributor_tips),
    message_template = COALESCE(sqlc.narg('message_template'), message_template),
    sort_order       = COALESCE(sqlc.narg('sort_order'), sort_order),
    updated_at       = now()
WHERE id = $1
RETURNING *;

-- name: DeleteOccasion :exec
DELETE FROM occasions WHERE id = $1;
