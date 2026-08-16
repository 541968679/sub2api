-- User-centric per-platform smart schedule (closed account pool + quality + pair cooldown).
-- Independent of account_schedule_users. Empty tables = legacy admission for everyone.

CREATE TABLE IF NOT EXISTS user_smart_schedule_policies (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(32) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    quality_max_p50_ttft_ms INTEGER,
    quality_min_success_rate DOUBLE PRECISION,
    quality_min_success_samples INTEGER,
    quality_min_ttft_samples INTEGER,
    quality_condition VARCHAR(8),
    cooldown_minutes INTEGER NOT NULL DEFAULT 15,
    CONSTRAINT user_smart_schedule_policies_user_platform_key UNIQUE (user_id, platform)
);

CREATE INDEX IF NOT EXISTS idx_user_smart_schedule_policies_user_id
    ON user_smart_schedule_policies (user_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_policies_platform_check'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD CONSTRAINT user_smart_schedule_policies_platform_check
            CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok'));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_policies_cooldown_minutes_check'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD CONSTRAINT user_smart_schedule_policies_cooldown_minutes_check
            CHECK (cooldown_minutes >= 1 AND cooldown_minutes <= 1440);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_policies_quality_max_p50_ttft_ms_check'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD CONSTRAINT user_smart_schedule_policies_quality_max_p50_ttft_ms_check
            CHECK (quality_max_p50_ttft_ms IS NULL OR quality_max_p50_ttft_ms >= 1);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_policies_quality_min_success_rate_check'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD CONSTRAINT user_smart_schedule_policies_quality_min_success_rate_check
            CHECK (quality_min_success_rate IS NULL OR (quality_min_success_rate > 0 AND quality_min_success_rate <= 1));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_policies_quality_min_success_samples_check'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD CONSTRAINT user_smart_schedule_policies_quality_min_success_samples_check
            CHECK (quality_min_success_samples IS NULL OR quality_min_success_samples >= 1);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_policies_quality_min_ttft_samples_check'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD CONSTRAINT user_smart_schedule_policies_quality_min_ttft_samples_check
            CHECK (quality_min_ttft_samples IS NULL OR quality_min_ttft_samples >= 1);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_policies_quality_condition_check'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD CONSTRAINT user_smart_schedule_policies_quality_condition_check
            CHECK (quality_condition IS NULL OR quality_condition IN ('or', 'and'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS user_smart_schedule_accounts (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    platform VARCHAR(32) NOT NULL,
    max_concurrency INTEGER,
    PRIMARY KEY (account_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_smart_schedule_accounts_user_platform
    ON user_smart_schedule_accounts (user_id, platform);

CREATE INDEX IF NOT EXISTS idx_user_smart_schedule_accounts_account_id
    ON user_smart_schedule_accounts (account_id);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_accounts_platform_check'
    ) THEN
        ALTER TABLE user_smart_schedule_accounts
            ADD CONSTRAINT user_smart_schedule_accounts_platform_check
            CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok'));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_accounts_max_concurrency_check'
    ) THEN
        ALTER TABLE user_smart_schedule_accounts
            ADD CONSTRAINT user_smart_schedule_accounts_max_concurrency_check
            CHECK (max_concurrency IS NULL OR max_concurrency >= 1);
    END IF;
END $$;
