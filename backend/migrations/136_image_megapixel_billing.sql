-- Add global image billing strategies and usage log image quality metadata.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE global_model_pricing
    ADD COLUMN IF NOT EXISTS image_billing_strategy VARCHAR(20) NOT NULL DEFAULT 'tier',
    ADD COLUMN IF NOT EXISTS image_megapixel_price NUMERIC(20,12),
    ADD COLUMN IF NOT EXISTS image_quality_prices JSONB,
    ADD COLUMN IF NOT EXISTS image_tier_rules JSONB;

ALTER TABLE usage_logs
    ALTER COLUMN image_size TYPE VARCHAR(32),
    ADD COLUMN IF NOT EXISTS image_quality VARCHAR(20);
