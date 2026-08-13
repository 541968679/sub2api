-- Account-level user schedule allow/deny lists.
-- unrestricted (default) keeps existing group-only scheduling.

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS user_schedule_mode VARCHAR(16) NOT NULL DEFAULT 'unrestricted';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'accounts_user_schedule_mode_check'
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT accounts_user_schedule_mode_check
            CHECK (user_schedule_mode IN ('unrestricted', 'allow', 'deny'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS account_schedule_users (
    account_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_account_schedule_users_user_id ON account_schedule_users(user_id);
