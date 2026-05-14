CREATE TABLE IF NOT EXISTS distribution_agents (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    contact TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    admin_note TEXT NOT NULL DEFAULT '',
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_distribution_agents_user_id
    ON distribution_agents(user_id);
CREATE INDEX IF NOT EXISTS idx_distribution_agents_status
    ON distribution_agents(status);
CREATE INDEX IF NOT EXISTS idx_distribution_agents_created_at
    ON distribution_agents(created_at);

CREATE TABLE IF NOT EXISTS distribution_wallets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id BIGINT NOT NULL REFERENCES distribution_agents(id) ON DELETE CASCADE,
    balance DECIMAL(20,8) NOT NULL DEFAULT 0,
    total_recharged DECIMAL(20,8) NOT NULL DEFAULT 0,
    total_spent DECIMAL(20,8) NOT NULL DEFAULT 0,
    total_rebate DECIMAL(20,8) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_distribution_wallets_user_id
    ON distribution_wallets(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_distribution_wallets_agent_id
    ON distribution_wallets(agent_id);
CREATE INDEX IF NOT EXISTS idx_distribution_wallets_status
    ON distribution_wallets(status);

CREATE TABLE IF NOT EXISTS distribution_wallet_ledger (
    id BIGSERIAL PRIMARY KEY,
    wallet_id BIGINT NOT NULL REFERENCES distribution_wallets(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action VARCHAR(30) NOT NULL,
    amount DECIMAL(20,8) NOT NULL,
    balance_after DECIMAL(20,8) NOT NULL,
    reference_type VARCHAR(40) NOT NULL DEFAULT '',
    reference_id VARCHAR(80) NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_distribution_wallet_ledger_wallet_id
    ON distribution_wallet_ledger(wallet_id);
CREATE INDEX IF NOT EXISTS idx_distribution_wallet_ledger_user_id
    ON distribution_wallet_ledger(user_id);
CREATE INDEX IF NOT EXISTS idx_distribution_wallet_ledger_action
    ON distribution_wallet_ledger(action);
CREATE INDEX IF NOT EXISTS idx_distribution_wallet_ledger_created_at
    ON distribution_wallet_ledger(created_at);

COMMENT ON TABLE distribution_agents IS 'Distribution agent applications and status';
COMMENT ON COLUMN distribution_agents.status IS 'pending|approved|rejected|frozen';
COMMENT ON TABLE distribution_wallets IS 'Independent distribution account wallet';
COMMENT ON TABLE distribution_wallet_ledger IS 'Distribution wallet balance ledger';
