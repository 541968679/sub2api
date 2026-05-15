ALTER TABLE distribution_agents
    ADD COLUMN IF NOT EXISTS rmb_per_usd_override DECIMAL(20,8),
    ADD COLUMN IF NOT EXISTS subscription_discount_override DECIMAL(20,8);

ALTER TABLE distribution_assets
    ADD COLUMN IF NOT EXISTS refunded_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS refunded_rmb DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refunded_by BIGINT REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_distribution_assets_refunded_at
    ON distribution_assets(refunded_at);

COMMENT ON COLUMN distribution_agents.rmb_per_usd_override IS 'Optional agent-specific RMB per 1 USD generation ratio';
COMMENT ON COLUMN distribution_agents.subscription_discount_override IS 'Optional agent-specific subscription generation discount';
COMMENT ON COLUMN distribution_assets.refunded_at IS 'Set when an asset is voided/disabled and its cost is refunded to distribution wallet';
