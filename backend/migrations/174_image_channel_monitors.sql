-- Image generation channel monitoring.

CREATE TABLE IF NOT EXISTS image_channel_monitors (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    source_type VARCHAR(20) NOT NULL DEFAULT 'custom',
    endpoint VARCHAR(500) NOT NULL DEFAULT '',
    api_key_encrypted TEXT NOT NULL DEFAULT '',
    account_id BIGINT,
    account_name VARCHAR(200) NOT NULL DEFAULT '',
    model VARCHAR(200) NOT NULL,
    prompt VARCHAR(2000) NOT NULL,
    size VARCHAR(32) NOT NULL DEFAULT '1024x1024',
    quality VARCHAR(32) NOT NULL DEFAULT 'auto',
    n INTEGER NOT NULL DEFAULT 1,
    download_image BOOLEAN NOT NULL DEFAULT TRUE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    interval_seconds INTEGER NOT NULL DEFAULT 300,
    timeout_seconds INTEGER NOT NULL DEFAULT 300,
    last_checked_at TIMESTAMPTZ,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT image_channel_monitors_source_type_check
        CHECK (source_type IN ('custom', 'account')),
    CONSTRAINT image_channel_monitors_source_payload_check
        CHECK (
            (source_type = 'custom' AND endpoint <> '' AND api_key_encrypted <> '' AND account_id IS NULL)
            OR
            (source_type = 'account' AND account_id IS NOT NULL)
        ),
    CONSTRAINT image_channel_monitors_n_check CHECK (n BETWEEN 1 AND 10),
    CONSTRAINT image_channel_monitors_interval_check CHECK (interval_seconds BETWEEN 15 AND 3600),
    CONSTRAINT image_channel_monitors_timeout_check CHECK (timeout_seconds BETWEEN 30 AND 600)
);

CREATE INDEX IF NOT EXISTS idx_image_channel_monitors_enabled_checked
    ON image_channel_monitors (enabled, last_checked_at);

CREATE INDEX IF NOT EXISTS idx_image_channel_monitors_account_id
    ON image_channel_monitors (account_id);

CREATE TABLE IF NOT EXISTS image_channel_monitor_histories (
    id BIGSERIAL PRIMARY KEY,
    monitor_id BIGINT NOT NULL REFERENCES image_channel_monitors(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'error',
    http_status INTEGER,
    api_header_ms INTEGER,
    api_body_ms INTEGER,
    api_total_ms INTEGER,
    json_bytes INTEGER,
    has_url BOOLEAN NOT NULL DEFAULT FALSE,
    has_b64_json BOOLEAN NOT NULL DEFAULT FALSE,
    image_url_host VARCHAR(255) NOT NULL DEFAULT '',
    image_first_byte_ms INTEGER,
    image_download_ms INTEGER,
    image_bytes BIGINT,
    image_content_type VARCHAR(100) NOT NULL DEFAULT '',
    image_width INTEGER,
    image_height INTEGER,
    error_stage VARCHAR(64) NOT NULL DEFAULT '',
    message VARCHAR(500) NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT image_channel_monitor_histories_status_check
        CHECK (status IN ('operational', 'degraded', 'failed', 'error'))
);

CREATE INDEX IF NOT EXISTS idx_image_channel_monitor_histories_monitor_checked
    ON image_channel_monitor_histories (monitor_id, checked_at DESC);

CREATE INDEX IF NOT EXISTS idx_image_channel_monitor_histories_checked
    ON image_channel_monitor_histories (checked_at);
