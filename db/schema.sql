-- Kithly database schema (read-only reference for sqlc).
-- DDL migrations live in db/migrations/ and are applied separately. Never run this file against production.
-- Keep in sync with prisma/schema.prisma in kithly-ws.

CREATE TYPE card_status AS ENUM ('collecting', 'processing', 'completed', 'expired');
CREATE TYPE job_status  AS ENUM ('queued', 'processing', 'completed', 'failed', 'retried');

CREATE TABLE users (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email            VARCHAR(255) UNIQUE,
    password_hash    TEXT,
    name             VARCHAR(100),
    email_verified   BOOLEAN     NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    avatar           TEXT        DEFAULT '❤️',
    fcm_token        TEXT,
    last_login_at    TIMESTAMPTZ,
    max_cards        INT         NOT NULL DEFAULT 5,
    is_guest         BOOLEAN     NOT NULL DEFAULT false,
    guest_token      VARCHAR(255) UNIQUE,
    firebase_uid     VARCHAR(255) UNIQUE,
    guest_expires_at TIMESTAMPTZ
);

CREATE TABLE music_themes (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    label      VARCHAR     NOT NULL,
    emoji      VARCHAR     NOT NULL,
    audio_url  TEXT        NOT NULL,
    category   VARCHAR     DEFAULT 'General',
    is_active  BOOLEAN     NOT NULL DEFAULT true,
    sort_order INT         NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE video_overlays (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    label         VARCHAR     NOT NULL,
    resource_url  TEXT        NOT NULL,
    thumbnail_url TEXT,
    overlay_type  VARCHAR     NOT NULL,
    category      VARCHAR     DEFAULT 'General',
    config        JSONB       NOT NULL DEFAULT '{}',
    is_active     BOOLEAN     NOT NULL DEFAULT true,
    sort_order    INT         NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE video_cards (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    organizer_id             UUID        REFERENCES users(id) ON DELETE CASCADE,
    title                    VARCHAR(255) NOT NULL,
    occasion                 VARCHAR(50)  NOT NULL,
    description              TEXT,
    share_token              VARCHAR(10)  NOT NULL UNIQUE,
    status                   card_status  NOT NULL DEFAULT 'collecting',
    music_theme_legacy       TEXT,
    music_theme_id           UUID        REFERENCES music_themes(id),
    video_overlay_id         UUID        REFERENCES video_overlays(id),
    max_contributors         INT         NOT NULL DEFAULT 50,
    is_locked                BOOLEAN     NOT NULL DEFAULT false,
    final_video_url          TEXT,
    preview_url              TEXT,
    preview_status           VARCHAR     NOT NULL DEFAULT 'idle',
    last_preview_generated_at TIMESTAMPTZ,
    final_hls_playlist_url   TEXT,
    preview_hls_playlist_url TEXT,
    thumbnail_url            TEXT,
    final_duration           FLOAT,
    is_archived              BOOLEAN     NOT NULL DEFAULT false,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at             TIMESTAMPTZ,
    expires_at               TIMESTAMPTZ,
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Cover frame shown first in the compiled video (migration 20260929).
    -- *_url columns hold bare S3 keys, like the other media columns.
    cover_photo_url          TEXT,
    cover_image_url          TEXT,
    cover_config             JSONB
);

CREATE TABLE video_contributions (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    video_card_id       UUID        REFERENCES video_cards(id) ON DELETE CASCADE,
    contributor_user_id UUID,
    contributor_name    VARCHAR(100) NOT NULL,
    contributor_email   VARCHAR(255),
    video_url           TEXT        NOT NULL,
    thumbnail_url       TEXT,
    duration_seconds    FLOAT,
    status              VARCHAR(20)  NOT NULL DEFAULT 'included',
    display_order       INT         NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    original_url        TEXT,
    hls_playlist_url    TEXT
);

CREATE TABLE video_processing_jobs (
    id            UUID       PRIMARY KEY DEFAULT gen_random_uuid(),
    video_card_id UUID       REFERENCES video_cards(id) ON DELETE CASCADE,
    status        job_status NOT NULL DEFAULT 'queued',
    error_message TEXT,
    attempts      INT        NOT NULL DEFAULT 0,
    metadata      JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    retried_job_id UUID      REFERENCES video_processing_jobs(id) ON DELETE SET NULL
);

CREATE TABLE occasions (
    id               VARCHAR     PRIMARY KEY,
    label            VARCHAR     NOT NULL,
    contributor_tips JSONB       NOT NULL DEFAULT '[]',
    message_template VARCHAR     NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    emoji            VARCHAR     DEFAULT '✨',
    sort_order       INT         NOT NULL DEFAULT 0
);

CREATE TABLE video_transcoding_jobs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_type TEXT        NOT NULL,
    resource_id   TEXT        NOT NULL,
    job_type      TEXT        NOT NULL,
    video_card_id UUID,
    status        TEXT        NOT NULL DEFAULT 'queued',
    attempts      INT         NOT NULL DEFAULT 0,
    error_message TEXT,
    metadata      JSONB       NOT NULL DEFAULT '{}',
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE video_card_collaborators (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    video_card_id UUID        NOT NULL REFERENCES video_cards(id) ON DELETE CASCADE,
    email         VARCHAR(255) NOT NULL,
    name          VARCHAR(100),
    user_id       UUID,
    invited_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (video_card_id, email)
);
