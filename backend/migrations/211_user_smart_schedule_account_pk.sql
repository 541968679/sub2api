-- Allow the same account in multiple user smart-schedule platform pools.
-- PK (account_id, user_id) → (user_id, platform, account_id). Additive only.
-- Do not edit 202 / 204 / 207 / 208.

DO $$
DECLARE
    pk_cols text[];
BEGIN
    SELECT array_agg(a.attname::text ORDER BY array_position(i.indkey, a.attnum))
    INTO pk_cols
    FROM pg_index i
    JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY (i.indkey)
    WHERE i.indrelid = 'user_smart_schedule_accounts'::regclass
      AND i.indisprimary;

    IF pk_cols = ARRAY['account_id', 'user_id'] THEN
        ALTER TABLE user_smart_schedule_accounts
            DROP CONSTRAINT user_smart_schedule_accounts_pkey;
        ALTER TABLE user_smart_schedule_accounts
            ADD CONSTRAINT user_smart_schedule_accounts_pkey
            PRIMARY KEY (user_id, platform, account_id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_user_smart_schedule_accounts_account_id
    ON user_smart_schedule_accounts (account_id);
