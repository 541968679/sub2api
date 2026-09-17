-- Non-transactional: CREATE INDEX CONCURRENTLY cannot run inside a transaction.
-- Remapped from upstream 233_add_usage_log_upstream_request_id_index_notx.sql.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_upstream_request_id
    ON usage_logs (upstream_request_id)
    WHERE upstream_request_id IS NOT NULL;
