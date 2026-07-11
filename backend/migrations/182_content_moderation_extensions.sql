ALTER TABLE content_moderation_logs
    ADD COLUMN IF NOT EXISTS matched_keyword VARCHAR(255) NOT NULL DEFAULT '';

ALTER TABLE usage_logs
    DROP CONSTRAINT IF EXISTS usage_logs_request_type_check;

ALTER TABLE usage_logs
    ADD CONSTRAINT usage_logs_request_type_check
    CHECK (request_type IN (0, 1, 2, 3, 4)) NOT VALID;

INSERT INTO settings (key, value)
VALUES
    ('cyber_session_block_enabled', 'false'),
    ('cyber_session_block_ttl_seconds', '3600')
ON CONFLICT (key) DO NOTHING;
