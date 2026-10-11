-- name: GetCardByID :one
SELECT * FROM video_cards WHERE id = $1 LIMIT 1;

-- name: GetCardByShareToken :one
SELECT * FROM video_cards WHERE share_token = $1 LIMIT 1;

-- name: ListCardsByOrganizer :many
SELECT * FROM video_cards
WHERE organizer_id = $1 AND is_archived = false
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListArchivedCardsByOrganizer :many
SELECT * FROM video_cards
WHERE organizer_id = $1 AND is_archived = true
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListSharedCards :many
SELECT vc.* FROM video_cards vc
INNER JOIN video_card_collaborators vcc ON vcc.video_card_id = vc.id
WHERE vcc.email = $1 AND vc.is_archived = false
ORDER BY vc.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountCardsByOrganizer :one
SELECT COUNT(*) FROM video_cards WHERE organizer_id = $1;

-- name: CreateCard :one
INSERT INTO video_cards (
    id, organizer_id, title, occasion, description, share_token,
    status, music_theme_id, video_overlay_id, max_contributors
) VALUES (
    gen_random_uuid(), $1, $2, $3, $4, $5,
    'collecting', $6, $7, $8
) RETURNING *;

-- name: UpdateCard :one
UPDATE video_cards
SET title            = COALESCE(sqlc.narg('title'), title),
    occasion         = COALESCE(sqlc.narg('occasion'), occasion),
    description      = COALESCE(sqlc.narg('description'), description),
    music_theme_id   = COALESCE(sqlc.narg('music_theme_id'), music_theme_id),
    video_overlay_id = COALESCE(sqlc.narg('video_overlay_id'), video_overlay_id),
    max_contributors = COALESCE(sqlc.narg('max_contributors'), max_contributors),
    updated_at       = now()
WHERE id = $1
RETURNING *;

-- name: UpdateCardOverlay :one
UPDATE video_cards
SET video_overlay_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: LockCard :exec
UPDATE video_cards SET is_locked = true, status = 'processing', updated_at = now() WHERE id = $1;

-- name: UnlockCardAfterFailure :exec
-- Resets a card that was locked for processing back to a submittable state.
-- Called by the worker when compilation fails so users can retry.
UPDATE video_cards SET is_locked = false, status = 'collecting', updated_at = now() WHERE id = $1;

-- name: SetCardPreviewFailed :exec
UPDATE video_cards SET preview_status = 'failed', updated_at = now() WHERE id = $1;

-- name: UpdateCardStatus :exec
UPDATE video_cards SET status = $2, updated_at = now() WHERE id = $1;

-- name: UpdateCardVideoURLs :exec
UPDATE video_cards
SET final_video_url = COALESCE(sqlc.narg('final_video_url'), final_video_url),
    preview_url     = COALESCE(sqlc.narg('preview_url'), preview_url),
    thumbnail_url   = COALESCE(sqlc.narg('thumbnail_url'), thumbnail_url),
    final_hls_playlist_url  = COALESCE(sqlc.narg('final_hls_playlist_url'), final_hls_playlist_url),
    preview_hls_playlist_url = COALESCE(sqlc.narg('preview_hls_playlist_url'), preview_hls_playlist_url),
    preview_status  = COALESCE(sqlc.narg('preview_status'), preview_status),
    last_preview_generated_at = COALESCE(sqlc.narg('last_preview_generated_at'), last_preview_generated_at),
    processed_at    = COALESCE(sqlc.narg('processed_at'), processed_at),
    final_duration  = COALESCE(sqlc.narg('final_duration'), final_duration),
    updated_at      = now()
WHERE id = $1;

-- name: ListMemoryCards :many
SELECT vc.* FROM video_cards vc
WHERE (
    vc.organizer_id = $1
    OR ($2::text != '' AND vc.id IN (
        SELECT video_card_id FROM video_card_collaborators WHERE email = $2
    ))
)
AND vc.status = 'completed'
AND vc.final_hls_playlist_url IS NOT NULL
ORDER BY vc.updated_at DESC
LIMIT $3 OFFSET $4;

-- name: CountMemoryCards :one
SELECT COUNT(*) FROM video_cards vc
WHERE (
    vc.organizer_id = $1
    OR ($2::text != '' AND vc.id IN (
        SELECT video_card_id FROM video_card_collaborators WHERE email = $2
    ))
)
AND vc.status = 'completed'
AND vc.final_hls_playlist_url IS NOT NULL;

-- name: ArchiveCard :exec
UPDATE video_cards SET is_archived = true, updated_at = now() WHERE id = $1;

-- name: UnarchiveCard :exec
UPDATE video_cards SET is_archived = false, updated_at = now() WHERE id = $1;

-- name: DeleteCard :exec
DELETE FROM video_cards WHERE id = $1;

-- Collaborators --

-- name: ListCollaborators :many
SELECT * FROM video_card_collaborators WHERE video_card_id = $1 ORDER BY invited_at ASC;

-- name: GetCollaboratorByID :one
SELECT * FROM video_card_collaborators WHERE id = $1 LIMIT 1;

-- name: CollaboratorExistsByCardAndEmail :one
SELECT EXISTS(
    SELECT 1 FROM video_card_collaborators
    WHERE video_card_id = $1 AND email = $2
) AS exists;

-- name: AddCollaborator :one
INSERT INTO video_card_collaborators (id, video_card_id, email, name, user_id)
VALUES (gen_random_uuid(), $1, $2, $3, $4)
RETURNING *;

-- name: RemoveCollaborator :exec
DELETE FROM video_card_collaborators WHERE id = $1;

-- name: UpdateCardCover :one
-- Sets or clears (all NULL) the cover frame.
-- cover_config is passed as text and cast here: the pool runs pgx in simple
-- protocol mode (PgBouncer), where a []byte parameter is sent as a bytea
-- literal that jsonb rejects.
UPDATE video_cards
SET cover_photo_url = $2,
    cover_image_url = $3,
    cover_config    = sqlc.narg('cover_config')::text::jsonb,
    updated_at      = now()
WHERE id = $1
RETURNING *;
