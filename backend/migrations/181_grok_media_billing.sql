ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS video_rate_independent BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS video_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS video_price_480p DECIMAL(20,8),
    ADD COLUMN IF NOT EXISTS video_price_720p DECIMAL(20,8),
    ADD COLUMN IF NOT EXISTS video_price_1080p DECIMAL(20,8);

UPDATE groups
SET allow_image_generation = TRUE
WHERE platform = 'grok' AND allow_image_generation = FALSE;

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS video_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS video_resolution VARCHAR(10),
    ADD COLUMN IF NOT EXISTS video_duration_seconds INTEGER;

ALTER TABLE usage_logs DROP CONSTRAINT IF EXISTS usage_logs_image_billing_size_check;
ALTER TABLE usage_logs
    ADD CONSTRAINT usage_logs_image_billing_size_check CHECK (
        image_count <= 0
        OR billing_mode = 'video'
        OR COALESCE(video_count, 0) > 0
        OR image_size IN ('1K', '2K', '4K')
    ) NOT VALID;

COMMENT ON COLUMN groups.video_price_480p IS 'Video generation price in USD per second at 480p';
COMMENT ON COLUMN groups.video_price_720p IS 'Video generation price in USD per second at 720p';
COMMENT ON COLUMN groups.video_price_1080p IS 'Video generation price in USD per second at 1080p';
COMMENT ON COLUMN usage_logs.video_duration_seconds IS 'Billable duration per generated video in seconds';
