ALTER TABLE users
    ADD COLUMN IF NOT EXISTS downstream_usage_token_mode TEXT NOT NULL DEFAULT 'real';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_downstream_usage_token_mode_check'
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_downstream_usage_token_mode_check
            CHECK (downstream_usage_token_mode IN ('real', 'display'));
    END IF;
END $$;
