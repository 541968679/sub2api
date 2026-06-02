-- Prevent user-level model pricing overrides from writing invalid numeric values.
-- NOT VALID avoids scanning historical rows during startup while still enforcing
-- the constraints for all new inserts and updates.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'user_model_pricing_overrides'::regclass
          AND conname = 'user_model_pricing_prices_non_negative_check'
    ) THEN
        ALTER TABLE user_model_pricing_overrides
            ADD CONSTRAINT user_model_pricing_prices_non_negative_check
            CHECK (
                (input_price IS NULL OR (input_price >= 0 AND input_price < 'Infinity'::double precision)) AND
                (output_price IS NULL OR (output_price >= 0 AND output_price < 'Infinity'::double precision)) AND
                (cache_write_price IS NULL OR (cache_write_price >= 0 AND cache_write_price < 'Infinity'::double precision)) AND
                (cache_read_price IS NULL OR (cache_read_price >= 0 AND cache_read_price < 'Infinity'::double precision)) AND
                (display_input_price IS NULL OR (display_input_price >= 0 AND display_input_price < 'Infinity'::double precision)) AND
                (display_output_price IS NULL OR (display_output_price >= 0 AND display_output_price < 'Infinity'::double precision)) AND
                (display_cache_read_price IS NULL OR (display_cache_read_price >= 0 AND display_cache_read_price < 'Infinity'::double precision))
            ) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'user_model_pricing_overrides'::regclass
          AND conname = 'user_model_pricing_display_rate_positive_check'
    ) THEN
        ALTER TABLE user_model_pricing_overrides
            ADD CONSTRAINT user_model_pricing_display_rate_positive_check
            CHECK (
                display_rate_multiplier IS NULL OR
                (display_rate_multiplier > 0 AND display_rate_multiplier < 'Infinity'::double precision)
            ) NOT VALID;
    END IF;
END
$$;
