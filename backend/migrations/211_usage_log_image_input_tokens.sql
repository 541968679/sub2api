-- Standing replica window-1: record image-modality input tokens separately from text/cache.
-- Renumbered from catchup 201 / old-sync 198.
-- Does not change actual_cost; additive column. Must coexist with true_first_token_ms and true_cost.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS image_input_tokens INT NOT NULL DEFAULT 0;

COMMENT ON COLUMN usage_logs.image_input_tokens IS 'Image-modality portion of input tokens when reported by upstream input_tokens_details; independent of cache_read_tokens';
