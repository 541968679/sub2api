-- Persist the configured upstream request identifier on usage_logs.
-- Remapped from upstream 232_add_usage_log_upstream_request_id.sql. Additive; does not change actual_cost.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS upstream_request_id VARCHAR(128);

COMMENT ON COLUMN usage_logs.upstream_request_id IS 'Upstream request identifier read from the account-configured response header; NULL when unconfigured or WS turns';
