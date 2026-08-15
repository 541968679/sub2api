-- Admin user-list pin. NULL = unpinned; non-NULL = pinned, newer first.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS pinned_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_users_pinned_at
    ON users (pinned_at DESC NULLS LAST)
    WHERE deleted_at IS NULL AND pinned_at IS NOT NULL;
