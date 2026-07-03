-- Optional proxy binding for custom image generation monitors.

ALTER TABLE image_channel_monitors
    ADD COLUMN IF NOT EXISTS proxy_id BIGINT;

ALTER TABLE image_channel_monitors
    ADD COLUMN IF NOT EXISTS proxy_name VARCHAR(200) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_image_channel_monitors_proxy_id
    ON image_channel_monitors (proxy_id);
