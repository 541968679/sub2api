-- Two-level master agent. The master's cut is a slice of the direct
-- inviter's rebate, not an extra payout. Historical ledger rows keep a
-- NULL accrual_role and stay outside the new partial unique indexes.

ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS sales_parent_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS sales_code VARCHAR(32) NULL,
    ADD COLUMN IF NOT EXISTS is_master BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS master_cut_percent DECIMAL(20,8) NULL;

COMMENT ON COLUMN user_affiliates.sales_parent_id IS 'Master who recruited this salesperson. Written only at registration.';
COMMENT ON COLUMN user_affiliates.sales_code IS 'Recruit code owned by a master. Kept when is_master is turned off.';
COMMENT ON COLUMN user_affiliates.is_master IS 'Admin grant. false stops the sales code and new cuts. Accrued quota stays.';
COMMENT ON COLUMN user_affiliates.master_cut_percent IS 'Percent of a subordinate salesperson rebate taken by the master. NULL means 0.';

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_affiliates_sales_code
    ON user_affiliates (sales_code)
    WHERE sales_code IS NOT NULL;

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS accrual_role VARCHAR(16) NULL,
    ADD COLUMN IF NOT EXISTS cut_percent_snapshot DECIMAL(20,8) NULL,
    ADD COLUMN IF NOT EXISTS direct_inviter_id BIGINT NULL;

COMMENT ON COLUMN user_affiliate_ledger.accrual_role IS 'direct|master_cut. NULL on historical rows, which remain the direct rebate.';
COMMENT ON COLUMN user_affiliate_ledger.cut_percent_snapshot IS 'Master cut percent used for this event. NULL when the event was not split.';
COMMENT ON COLUMN user_affiliate_ledger.direct_inviter_id IS 'Salesperson who owns the customer invite. Equals user_id on a direct row.';

DROP INDEX IF EXISTS uq_user_affiliate_ledger_accrue_source;
DROP INDEX IF EXISTS uq_user_affiliate_ledger_reverse_source;

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_affiliate_ledger_accrue_source
    ON user_affiliate_ledger (source_kind, source_id, accrual_role)
    WHERE action = 'accrue'
      AND source_kind IS NOT NULL
      AND source_id IS NOT NULL
      AND accrual_role IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_affiliate_ledger_reverse_source
    ON user_affiliate_ledger (source_kind, source_id, reverse_seq, accrual_role)
    WHERE action = 'reverse'
      AND source_kind IS NOT NULL
      AND source_id IS NOT NULL
      AND reverse_seq IS NOT NULL
      AND accrual_role IS NOT NULL;
