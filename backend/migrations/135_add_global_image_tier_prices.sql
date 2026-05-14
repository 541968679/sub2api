-- Add per-size image pricing fields to global_model_pricing.
-- Enables size-tiered billing (1K/2K/4K) for image generation models
-- at the global override level, matching channel-level capabilities.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE global_model_pricing
    ADD COLUMN IF NOT EXISTS image_price_1k NUMERIC(20,12),
    ADD COLUMN IF NOT EXISTS image_price_2k NUMERIC(20,12),
    ADD COLUMN IF NOT EXISTS image_price_4k NUMERIC(20,12);
