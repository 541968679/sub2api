-- Snapshot display-layer joint/output token caps on usage_logs when a cap binds.
-- Historical rows stay applied=false / used=0 (no backfill). ADD COLUMN on the
-- usage_logs parent is partition-safe: existing partitions inherit the columns.

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS display_token_cap_applied BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS display_context_token_max_used BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS display_output_token_max_used BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN usage_logs.display_token_cap_applied IS
    'True when the display-layer joint/output absolute cap bound this row at write time';
COMMENT ON COLUMN usage_logs.display_context_token_max_used IS
    'Configured joint input+cache cap snapshotted on bind (jitter replays from request_id)';
COMMENT ON COLUMN usage_logs.display_output_token_max_used IS
    'Configured output cap snapshotted on bind (jitter replays from request_id)';
