-- 106_add_user_model_pricing_overrides.sql
-- 用户级模型定价覆盖表：支持管理员为特定用户设置模型的真实计费价格和展示价格覆盖。
-- 优先级：用户级 > 渠道级 > 全局级 > LiteLLM/Fallback

CREATE TABLE IF NOT EXISTS user_model_pricing_overrides (
    id                      BIGSERIAL PRIMARY KEY,
    user_id                 BIGINT NOT NULL,
    model                   VARCHAR(255) NOT NULL,

    -- 真实计费覆盖（NULL = 不覆盖，使用上层链路的值）
    input_price             DOUBLE PRECISION,
    output_price            DOUBLE PRECISION,
    cache_write_price       DOUBLE PRECISION,
    cache_read_price        DOUBLE PRECISION,

    -- 展示覆盖（仅影响用户看到的 usage log，不影响计费）
    display_input_price     DOUBLE PRECISION,
    display_output_price    DOUBLE PRECISION,
    display_rate_multiplier DOUBLE PRECISION,
    cache_transfer_ratio    DOUBLE PRECISION,

    enabled                 BOOLEAN NOT NULL DEFAULT TRUE,
    notes                   VARCHAR(500) DEFAULT '',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_user_model_pricing_user_model UNIQUE(user_id, model)
);

CREATE INDEX IF NOT EXISTS idx_umpo_user_id ON user_model_pricing_overrides(user_id);
CREATE INDEX IF NOT EXISTS idx_umpo_model ON user_model_pricing_overrides(model);
