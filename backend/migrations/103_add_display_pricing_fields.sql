-- Add display pricing fields to global_model_pricing.
-- These fields only affect user-facing usage log display, not actual billing.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE global_model_pricing
    ADD COLUMN IF NOT EXISTS display_input_price     NUMERIC(20,12),
    ADD COLUMN IF NOT EXISTS display_output_price    NUMERIC(20,12),
    ADD COLUMN IF NOT EXISTS display_rate_multiplier NUMERIC(10,4),
    ADD COLUMN IF NOT EXISTS cache_transfer_ratio    NUMERIC(5,4);

COMMENT ON COLUMN global_model_pricing.display_input_price IS '展示给用户的输入单价（NULL=用真实价）';
COMMENT ON COLUMN global_model_pricing.display_output_price IS '展示给用户的输出单价（NULL=用真实价）';
COMMENT ON COLUMN global_model_pricing.display_rate_multiplier IS '展示给用户的费率倍数（NULL=用真实倍率）';
COMMENT ON COLUMN global_model_pricing.cache_transfer_ratio IS '缓存token转移到输入token的比例 0~1（NULL或0=不转移）';
