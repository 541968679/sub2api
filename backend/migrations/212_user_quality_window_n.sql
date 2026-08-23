-- Per-user last-N window for user-global quality Q_u. NULL = inherit site account_quality_window_n.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS quality_window_n INTEGER DEFAULT NULL;

COMMENT ON COLUMN users.quality_window_n IS
    'User override for Q_u last-N window (NULL = inherit site account_quality_window_n; app clamps 1-100)';
