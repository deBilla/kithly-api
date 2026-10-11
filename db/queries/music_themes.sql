-- name: GetMusicThemeByID :one
SELECT * FROM music_themes WHERE id = $1 LIMIT 1;

-- name: ListMusicThemes :many
SELECT * FROM music_themes WHERE is_active = true ORDER BY sort_order ASC, created_at ASC;

-- name: CreateMusicTheme :one
INSERT INTO music_themes (id, label, emoji, audio_url, category, is_active, sort_order)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateMusicTheme :one
UPDATE music_themes
SET label      = COALESCE(sqlc.narg('label'), label),
    emoji      = COALESCE(sqlc.narg('emoji'), emoji),
    audio_url  = COALESCE(sqlc.narg('audio_url'), audio_url),
    category   = COALESCE(sqlc.narg('category'), category),
    is_active  = COALESCE(sqlc.narg('is_active'), is_active),
    sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteMusicTheme :exec
DELETE FROM music_themes WHERE id = $1;
