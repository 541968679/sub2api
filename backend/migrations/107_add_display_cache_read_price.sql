-- 107_add_display_cache_read_price.sql
-- 为全局模型定价添加展示缓存读取价格字段，防止用户通过 usage log 反算出真实缓存价格。

ALTER TABLE global_model_pricing ADD COLUMN IF NOT EXISTS display_cache_read_price DOUBLE PRECISION;
