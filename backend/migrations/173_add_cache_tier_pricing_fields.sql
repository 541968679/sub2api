-- 173_add_cache_tier_pricing_fields.sql
-- 补全 5m/1h 缓存创建分档在用户级真实价与全局/用户级展示价上的字段：
-- ① user_model_pricing_overrides.cache_write_1h_price —— 用户级真实 1h 缓存写入价（对齐全局 172）；
-- ② global_model_pricing.display_cache_creation_1h_price —— 全局展示 1h 缓存创建价；
-- ③ user_model_pricing_overrides.display_cache_creation_1h_price —— 用户级展示 1h 缓存创建价。
-- 语义：既有 display_cache_creation_price 为 5m 档展示价；1h 展示价 NULL 时回退 5m 档展示价。
-- 展示字段仅影响用户可见 usage log 与下游 display 模式响应，不影响真实计费。

ALTER TABLE user_model_pricing_overrides ADD COLUMN IF NOT EXISTS cache_write_1h_price DOUBLE PRECISION;
ALTER TABLE global_model_pricing ADD COLUMN IF NOT EXISTS display_cache_creation_1h_price DOUBLE PRECISION;
ALTER TABLE user_model_pricing_overrides ADD COLUMN IF NOT EXISTS display_cache_creation_1h_price DOUBLE PRECISION;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'user_model_pricing_overrides'::regclass
          AND conname = 'user_model_pricing_cache_tier_non_negative_check'
    ) THEN
        ALTER TABLE user_model_pricing_overrides
            ADD CONSTRAINT user_model_pricing_cache_tier_non_negative_check
            CHECK (
                (cache_write_1h_price IS NULL OR (cache_write_1h_price >= 0 AND cache_write_1h_price < 'Infinity'::double precision)) AND
                (display_cache_creation_1h_price IS NULL OR (display_cache_creation_1h_price >= 0 AND display_cache_creation_1h_price < 'Infinity'::double precision))
            ) NOT VALID;
    END IF;
END
$$;

COMMENT ON COLUMN user_model_pricing_overrides.cache_write_1h_price IS '用户级 1h 缓存写入价（USD/token）；NULL = 与 cache_write_price 同价';
COMMENT ON COLUMN global_model_pricing.display_cache_creation_1h_price IS '展示给用户的 1h 缓存创建单价（USD/token）；NULL = 回退 display_cache_creation_price';
COMMENT ON COLUMN user_model_pricing_overrides.display_cache_creation_1h_price IS '展示给用户的 1h 缓存创建单价（USD/token）；NULL = 回退 display_cache_creation_price';
