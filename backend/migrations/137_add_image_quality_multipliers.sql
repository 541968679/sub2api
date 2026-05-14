-- Add tier image billing quality multipliers.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE global_model_pricing
    ADD COLUMN IF NOT EXISTS image_quality_multipliers JSONB;
