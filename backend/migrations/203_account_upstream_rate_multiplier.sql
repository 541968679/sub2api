-- Scheduling-only upstream rate. Independent of accounts.rate_multiplier (billing).
-- Column default is 1.0; oauth/apikey existing rows are backfilled to 0.15 once.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'accounts'
          AND column_name = 'upstream_rate_multiplier'
    ) THEN
        ALTER TABLE accounts
            ADD COLUMN upstream_rate_multiplier DECIMAL(10, 4) NOT NULL DEFAULT 1.0;

        UPDATE accounts
        SET upstream_rate_multiplier = 0.15
        WHERE type IN ('oauth', 'apikey');
    END IF;
END $$;
