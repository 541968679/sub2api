-- Standing replica window-1: subscription plan currency label (default empty = legacy display).
-- Renumbered from catchup 200 / old-sync 197. main already used 209 for user_quality_snapshots.
ALTER TABLE subscription_plans
    ADD COLUMN IF NOT EXISTS currency VARCHAR(16) NOT NULL DEFAULT '';

COMMENT ON COLUMN subscription_plans.currency IS 'Display currency code/label for plan prices (e.g. USD, CNY); empty keeps historical UI';
