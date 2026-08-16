-- Pool display order for one user×platform smart-schedule membership.
-- Independent of accounts.priority (scheduling weight). NULL = unset; keep relative id order.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'user_smart_schedule_accounts'
          AND column_name = 'sort_order'
    ) THEN
        ALTER TABLE user_smart_schedule_accounts
            ADD COLUMN sort_order INTEGER;
    END IF;
END $$;
