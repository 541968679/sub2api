-- Durable user×account pause: stay in the smart-schedule pool, skip this pair on the hot path.
-- Independent of pair cooldown HASH and accounts.schedulable.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'user_smart_schedule_accounts'
          AND column_name = 'paused'
    ) THEN
        ALTER TABLE user_smart_schedule_accounts
            ADD COLUMN paused BOOLEAN NOT NULL DEFAULT false;
    END IF;
END $$;
