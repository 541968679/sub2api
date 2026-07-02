-- 172_add_cache_write_1h_price.sql
-- 为全局模型定价添加 1 小时缓存创建（写入）单价字段。
-- 背景：Anthropic 缓存创建分 5m（1.25×输入价）与 1h（2×输入价）两档，上游中转按档区分扣费；
-- 此前全局覆盖只有单一 cache_write_price 且会同写两档，导致 1h 溢价无法计入真实成本。
-- NULL = 沿用现状（1h 与 5m 同价）；配置后 1h 缓存创建按此价计费并启用分档计费。

ALTER TABLE global_model_pricing ADD COLUMN IF NOT EXISTS cache_write_1h_price DOUBLE PRECISION;

COMMENT ON COLUMN global_model_pricing.cache_write_1h_price IS '1 小时缓存创建单价（USD/token）；NULL = 与 cache_write_price 同价';
