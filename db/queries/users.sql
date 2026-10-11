-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserByFirebaseUID :one
SELECT * FROM users WHERE firebase_uid = $1 LIMIT 1;

-- name: GetUserByGuestToken :one
SELECT * FROM users WHERE guest_token = $1 LIMIT 1;

-- name: GetEmailByID :one
SELECT email FROM users WHERE id = $1 LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at DESC;

-- name: CreateUser :one
INSERT INTO users (id, email, password_hash, name, email_verified, avatar, max_cards, is_guest, guest_token, firebase_uid, guest_expires_at)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET name = COALESCE(sqlc.narg('name'), name),
    email = COALESCE(sqlc.narg('email'), email),
    avatar = COALESCE(sqlc.narg('avatar'), avatar),
    password_hash = COALESCE(sqlc.narg('password_hash'), password_hash),
    email_verified = COALESCE(sqlc.narg('email_verified'), email_verified),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserFCMToken :exec
UPDATE users SET fcm_token = $2, updated_at = now() WHERE id = $1;

-- name: UpdateUserLastLogin :exec
UPDATE users SET last_login_at = now(), updated_at = now() WHERE id = $1;

-- name: ConvertGuestToUser :one
UPDATE users
SET is_guest = false,
    guest_token = NULL,
    guest_expires_at = NULL,
    email = $2,
    password_hash = $3,
    name = $4,
    email_verified = false,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: CountUserCards :one
SELECT COUNT(*) FROM video_cards WHERE organizer_id = $1;

-- name: LinkUserFirebaseUID :one
-- Attaches a verified Firebase UID to an existing account (e.g. one created
-- before the API stored UIDs), so later logins resolve by UID directly.
UPDATE users SET firebase_uid = $2, updated_at = now() WHERE id = $1
RETURNING *;
