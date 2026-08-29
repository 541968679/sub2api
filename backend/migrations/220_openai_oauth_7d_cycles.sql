-- Closed Codex 7-day windows for OpenAI OAuth LiteLLM cycle cost.
-- Current-cycle cost is computed on the fly; only closed cycles are persisted.
-- UNIQUE(account_id, window_end) makes probe/header retries idempotent.

CREATE TABLE IF NOT EXISTS openai_oauth_7d_cycles (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    litellm_cost NUMERIC(20, 10) NOT NULL DEFAULT 0,
    used_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    closed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS openai_oauth_7d_cycles_account_id_window_end
    ON openai_oauth_7d_cycles (account_id, window_end);

CREATE INDEX IF NOT EXISTS openai_oauth_7d_cycles_account_id
    ON openai_oauth_7d_cycles (account_id);
