-- 108_add_user_model_pricing_display_cache_read.sql
-- 为用户模型定价覆盖表添加展示缓存读取价格字段。

ALTER TABLE user_model_pricing_overrides ADD COLUMN IF NOT EXISTS display_cache_read_price DOUBLE PRECISION;
