-- Snapshot true upstream cost for admin smart-schedule PnL.
-- Historical rows stay NULL (no backfill). ADD COLUMN on the usage_logs parent
-- is partition-safe: existing partitions inherit the new columns.
-- Do not CREATE INDEX here: usage_logs is a hot write table. The partial
-- index is 206_usage_log_true_cost_index_notx.sql (CONCURRENTLY).

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS true_cost NUMERIC(20, 10),
    ADD COLUMN IF NOT EXISTS true_cost_rate NUMERIC(10, 4);
