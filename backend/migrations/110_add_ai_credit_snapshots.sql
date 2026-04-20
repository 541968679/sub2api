-- 110_add_ai_credit_snapshots.sql
-- 新增 ai_credit_snapshots 表：定时采样 Antigravity AI Credits 余额，
-- 用于管理员使用记录页计算"时间窗内 credits 消耗量 / 每 credit 对应额度与调用次数"。

CREATE TABLE IF NOT EXISTS ai_credit_snapshots (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    credit_type VARCHAR(50) NOT NULL,
    amount DECIMAL(20, 6) NOT NULL,
    captured_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_credit_snapshots_email_captured_at
    ON ai_credit_snapshots(email, captured_at);

CREATE INDEX IF NOT EXISTS idx_ai_credit_snapshots_captured_at
    ON ai_credit_snapshots(captured_at);
