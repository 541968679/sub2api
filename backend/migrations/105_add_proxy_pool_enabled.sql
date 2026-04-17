-- 代理表增加代理池启用标记
-- 用于控制代理是否参与自动分配代理池
ALTER TABLE proxies
ADD COLUMN IF NOT EXISTS pool_enabled BOOLEAN NOT NULL DEFAULT true;

COMMENT ON COLUMN proxies.pool_enabled IS '是否启用代理池（参与自动分配）';
