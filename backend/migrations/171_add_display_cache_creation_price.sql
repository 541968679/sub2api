-- 171_add_display_cache_creation_price.sql
-- 为全局模型定价与用户模型定价覆盖添加展示缓存创建价格字段。
-- 该字段仅影响用户侧 usage log 的展示换算（缓存创建 token 按展示价直接反算放大），
-- 不影响真实计费（actual_cost 不变）。防止用户通过 usage log 反算出真实缓存写入价格。

ALTER TABLE global_model_pricing ADD COLUMN IF NOT EXISTS display_cache_creation_price DOUBLE PRECISION;

ALTER TABLE user_model_pricing_overrides ADD COLUMN IF NOT EXISTS display_cache_creation_price DOUBLE PRECISION;

-- 与 147 同型的 NOT VALID 约束（147 已应用，不可原地扩展，按名新建独立约束）。
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'user_model_pricing_overrides'::regclass
          AND conname = 'user_model_pricing_display_cache_creation_non_negative_check'
    ) THEN
        ALTER TABLE user_model_pricing_overrides
            ADD CONSTRAINT user_model_pricing_display_cache_creation_non_negative_check
            CHECK (
                display_cache_creation_price IS NULL OR
                (display_cache_creation_price >= 0 AND display_cache_creation_price < 'Infinity'::double precision)
            ) NOT VALID;
    END IF;
END
$$;

COMMENT ON COLUMN global_model_pricing.display_cache_creation_price IS '展示给用户的缓存创建单价（USD/token），仅影响 usage log 展示，不影响真实计费';
COMMENT ON COLUMN user_model_pricing_overrides.display_cache_creation_price IS '展示给用户的缓存创建单价（USD/token），仅影响 usage log 展示，不影响真实计费';
