-- Configurable probing in-flight cap on user×platform smart-schedule policy.
-- follow_n (NULL/empty/'follow_n') uses the policy window N.
-- custom uses probe_concurrency (1–100). Member max_concurrency remains a hard ceiling.
-- This is not account_quality_window_n.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'user_smart_schedule_policies'
          AND column_name = 'probe_concurrency_mode'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD COLUMN probe_concurrency_mode VARCHAR(16);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'user_smart_schedule_policies'
          AND column_name = 'probe_concurrency'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD COLUMN probe_concurrency INTEGER;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_policies_probe_concurrency_mode_check'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD CONSTRAINT user_smart_schedule_policies_probe_concurrency_mode_check
            CHECK (probe_concurrency_mode IS NULL OR probe_concurrency_mode IN ('follow_n', 'custom'));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_policies_probe_concurrency_check'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD CONSTRAINT user_smart_schedule_policies_probe_concurrency_check
            CHECK (probe_concurrency IS NULL OR (probe_concurrency >= 1 AND probe_concurrency <= 100));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'user_smart_schedule_policies_probe_concurrency_pair_check'
    ) THEN
        ALTER TABLE user_smart_schedule_policies
            ADD CONSTRAINT user_smart_schedule_policies_probe_concurrency_pair_check
            CHECK (probe_concurrency_mode IS DISTINCT FROM 'custom' OR probe_concurrency IS NOT NULL);
    END IF;
END $$;
