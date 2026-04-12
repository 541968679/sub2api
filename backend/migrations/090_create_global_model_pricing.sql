-- Global model pricing overrides.
-- Allows admins to set platform-wide custom prices that override LiteLLM defaults.
-- Resolution order: Channel Override → Global Override → LiteLLM → Hardcoded Fallback.
-- Table is opt-in: only consulted when an entry exists and enabled=true.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS global_model_pricing (
    id                 BIGSERIAL      PRIMARY KEY,
    model              VARCHAR(255)   NOT NULL,
    provider           VARCHAR(50)    NOT NULL DEFAULT '',
    billing_mode       VARCHAR(20)    NOT NULL DEFAULT 'token',
    input_price        NUMERIC(20,12),
    output_price       NUMERIC(20,12),
    cache_write_price  NUMERIC(20,12),
    cache_read_price   NUMERIC(20,12),
    image_output_price NUMERIC(20,12),
    per_request_price  NUMERIC(20,12),
    enabled            BOOLEAN        NOT NULL DEFAULT true,
    notes              TEXT           DEFAULT '',
    created_at         TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

-- Case-insensitive unique constraint on model name
CREATE UNIQUE INDEX IF NOT EXISTS idx_global_model_pricing_model
    ON global_model_pricing (LOWER(model));

-- Index for listing enabled entries (resolver hot path)
CREATE INDEX IF NOT EXISTS idx_global_model_pricing_enabled
    ON global_model_pricing (enabled) WHERE enabled = true;

COMMENT ON TABLE global_model_pricing IS '全局模型定价覆盖：管理员设定的平台级自定义价格，覆盖 LiteLLM 默认值';
COMMENT ON COLUMN global_model_pricing.model IS '模型名称（唯一，大小写不敏感）';
COMMENT ON COLUMN global_model_pricing.provider IS '平台标识（anthropic/openai/gemini/antigravity）';
COMMENT ON COLUMN global_model_pricing.billing_mode IS '计费模式：token / per_request / image';
COMMENT ON COLUMN global_model_pricing.enabled IS '是否启用此覆盖（false 时回退到 LiteLLM）';
COMMENT ON COLUMN global_model_pricing.notes IS '管理员备注';
