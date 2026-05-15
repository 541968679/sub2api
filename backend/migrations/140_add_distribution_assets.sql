CREATE TABLE IF NOT EXISTS distribution_assets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    wallet_id BIGINT NOT NULL REFERENCES distribution_wallets(id) ON DELETE CASCADE,
    asset_type VARCHAR(30) NOT NULL,
    reference_type VARCHAR(40) NOT NULL,
    reference_id VARCHAR(80) NOT NULL,
    display_value TEXT NOT NULL DEFAULT '',
    package_url TEXT NOT NULL DEFAULT '',
    face_value DECIMAL(20,8) NOT NULL DEFAULT 0,
    cost_rmb DECIMAL(20,8) NOT NULL DEFAULT 0,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    validity_days INT NOT NULL DEFAULT 0,
    quota_usd DECIMAL(20,8) NOT NULL DEFAULT 0,
    status VARCHAR(30) NOT NULL DEFAULT 'active',
    customer_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_distribution_assets_reference
    ON distribution_assets(reference_type, reference_id);
CREATE INDEX IF NOT EXISTS idx_distribution_assets_user_id
    ON distribution_assets(user_id);
CREATE INDEX IF NOT EXISTS idx_distribution_assets_asset_type
    ON distribution_assets(asset_type);
CREATE INDEX IF NOT EXISTS idx_distribution_assets_status
    ON distribution_assets(status);
CREATE INDEX IF NOT EXISTS idx_distribution_assets_group_id
    ON distribution_assets(group_id);
CREATE INDEX IF NOT EXISTS idx_distribution_assets_created_at
    ON distribution_assets(created_at);

COMMENT ON TABLE distribution_assets IS 'Generated distribution redeem codes and API key packages';
COMMENT ON COLUMN distribution_assets.asset_type IS 'balance_redeem_code|subscription_redeem_code|api_key';
