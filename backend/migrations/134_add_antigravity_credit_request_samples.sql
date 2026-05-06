-- 134_add_antigravity_credit_request_samples.sql
-- Request-level diagnostic samples for Antigravity AI Credits.
-- This table is written only when the diagnostic environment switch is enabled.

CREATE TABLE IF NOT EXISTS antigravity_credit_request_samples (
    id BIGSERIAL PRIMARY KEY,
    usage_log_id BIGINT REFERENCES usage_logs(id) ON DELETE SET NULL,
    request_id VARCHAR(255),
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    email VARCHAR(255),
    credit_type VARCHAR(50) NOT NULL,
    before_amount DECIMAL(20, 6),
    after_amount DECIMAL(20, 6),
    delta_amount DECIMAL(20, 6),
    before_captured_at TIMESTAMPTZ,
    after_captured_at TIMESTAMPTZ,
    confidence VARCHAR(20) NOT NULL DEFAULT 'unknown',
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ag_credit_samples_usage_log_id
    ON antigravity_credit_request_samples(usage_log_id);

CREATE INDEX IF NOT EXISTS idx_ag_credit_samples_account_time
    ON antigravity_credit_request_samples(account_id, created_at);

CREATE INDEX IF NOT EXISTS idx_ag_credit_samples_api_key_time
    ON antigravity_credit_request_samples(api_key_id, created_at);

CREATE INDEX IF NOT EXISTS idx_ag_credit_samples_email_time
    ON antigravity_credit_request_samples(email, created_at);
