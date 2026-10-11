-- Adds the cover frame: a photo of the recipient with an editable greeting,
-- rendered by the app and shown as the first frame of the compiled video.
--
-- Apply to the database BEFORE deploying the API/worker that reads these columns
-- (their generated queries select them explicitly). The currently deployed
-- code is unaffected by the extra columns.
--
-- cover_photo_url: S3 key of the original photo (kept so the cover can be re-edited)
-- cover_image_url: S3 key of the rendered 1080x1920 JPEG used by the worker
-- cover_config:    editor state (headline, subline, style) for re-editing

ALTER TABLE video_cards
    ADD COLUMN IF NOT EXISTS cover_photo_url TEXT,
    ADD COLUMN IF NOT EXISTS cover_image_url TEXT,
    ADD COLUMN IF NOT EXISTS cover_config    JSONB;
