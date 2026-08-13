-- Split display first-token (first_token_ms) from scheduling/account-quality
-- first-token (true_first_token_ms). Native /v1/responses stamps display on the
-- first SSE frame and true on the first useful/non-preamble event.

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS true_first_token_ms INTEGER;
