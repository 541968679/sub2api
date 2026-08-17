-- Non-transactional: CREATE INDEX CONCURRENTLY cannot run inside a transaction.
-- usage_logs is a hot append table; a regular CREATE INDEX would block writes.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_true_cost_user_account_created
    ON usage_logs (user_id, account_id, created_at)
    WHERE true_cost IS NOT NULL;
