-- Optional per-account-per-user quality gates on account_schedule_users.
-- Any non-null quality column means the gate is enabled for that pair.
-- Unconfigured metrics stay null and are not judged.

ALTER TABLE account_schedule_users
    ADD COLUMN IF NOT EXISTS quality_max_p50_ttft_ms INTEGER;

ALTER TABLE account_schedule_users
    ADD COLUMN IF NOT EXISTS quality_min_success_rate DOUBLE PRECISION;

ALTER TABLE account_schedule_users
    ADD COLUMN IF NOT EXISTS quality_min_success_samples INTEGER;

ALTER TABLE account_schedule_users
    ADD COLUMN IF NOT EXISTS quality_min_ttft_samples INTEGER;

ALTER TABLE account_schedule_users
    ADD COLUMN IF NOT EXISTS quality_condition VARCHAR(8);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'account_schedule_users_quality_max_p50_ttft_ms_check'
    ) THEN
        ALTER TABLE account_schedule_users
            ADD CONSTRAINT account_schedule_users_quality_max_p50_ttft_ms_check
            CHECK (quality_max_p50_ttft_ms IS NULL OR quality_max_p50_ttft_ms >= 1);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'account_schedule_users_quality_min_success_rate_check'
    ) THEN
        ALTER TABLE account_schedule_users
            ADD CONSTRAINT account_schedule_users_quality_min_success_rate_check
            CHECK (quality_min_success_rate IS NULL OR (quality_min_success_rate > 0 AND quality_min_success_rate <= 1));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'account_schedule_users_quality_min_success_samples_check'
    ) THEN
        ALTER TABLE account_schedule_users
            ADD CONSTRAINT account_schedule_users_quality_min_success_samples_check
            CHECK (quality_min_success_samples IS NULL OR quality_min_success_samples >= 1);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'account_schedule_users_quality_min_ttft_samples_check'
    ) THEN
        ALTER TABLE account_schedule_users
            ADD CONSTRAINT account_schedule_users_quality_min_ttft_samples_check
            CHECK (quality_min_ttft_samples IS NULL OR quality_min_ttft_samples >= 1);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'account_schedule_users_quality_condition_check'
    ) THEN
        ALTER TABLE account_schedule_users
            ADD CONSTRAINT account_schedule_users_quality_condition_check
            CHECK (quality_condition IS NULL OR quality_condition IN ('or', 'and'));
    END IF;
END $$;
