-- Independent allow/deny/pair-concurrency columns on account_schedule_users.
-- Backfill from the leftover exclusive user_schedule_mode so existing admission
-- results stay the same. Pair caps start unset.

ALTER TABLE account_schedule_users
    ADD COLUMN IF NOT EXISTS allow BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE account_schedule_users
    ADD COLUMN IF NOT EXISTS deny BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE account_schedule_users
    ADD COLUMN IF NOT EXISTS max_concurrency INTEGER;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'account_schedule_users_max_concurrency_check'
    ) THEN
        ALTER TABLE account_schedule_users
            ADD CONSTRAINT account_schedule_users_max_concurrency_check
            CHECK (max_concurrency IS NULL OR max_concurrency >= 1);
    END IF;
END $$;

UPDATE account_schedule_users AS asu
SET allow = TRUE
FROM accounts AS a
WHERE asu.account_id = a.id
  AND a.user_schedule_mode = 'allow'
  AND asu.allow = FALSE;

UPDATE account_schedule_users AS asu
SET deny = TRUE
FROM accounts AS a
WHERE asu.account_id = a.id
  AND a.user_schedule_mode = 'deny'
  AND asu.deny = FALSE;
