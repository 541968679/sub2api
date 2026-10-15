-- Semi-open invite rebate. Existing users stay unauthorized.
-- Rebate ledger rows gain a source key so the same recharge cannot accrue twice
-- and a refund can reverse that exact accrual. Historical rows are not backfilled.

ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS aff_invite_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS aff_rebate_per_invitee_cap DECIMAL(20,8) NULL,
    ADD COLUMN IF NOT EXISTS aff_rebate_freeze_hours INT NULL,
    ADD COLUMN IF NOT EXISTS aff_rebate_duration_days INT NULL;

COMMENT ON COLUMN user_affiliates.aff_invite_enabled IS 'Admin grant. false means this user cannot invite and new recharges from existing invitees do not accrue';
COMMENT ON COLUMN user_affiliates.aff_rebate_per_invitee_cap IS 'Per-invitee cumulative rebate cap override. NULL inherits global. 0 means unlimited';
COMMENT ON COLUMN user_affiliates.aff_rebate_freeze_hours IS 'Freeze hours override. NULL inherits global. 0 means new rebate is settleable immediately';
COMMENT ON COLUMN user_affiliates.aff_rebate_duration_days IS 'Rebate duration days override, counted from the invitee bind time. NULL inherits global. 0 means forever';

ALTER TABLE user_affiliate_ledger
    ADD COLUMN IF NOT EXISTS source_kind VARCHAR(32) NULL,
    ADD COLUMN IF NOT EXISTS source_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS reverse_seq BIGINT NULL,
    ADD COLUMN IF NOT EXISTS note TEXT NULL;

COMMENT ON COLUMN user_affiliate_ledger.action IS 'accrue|reverse|settle|transfer';
COMMENT ON COLUMN user_affiliate_ledger.source_kind IS 'payment_order|redeem_code|admin_balance. NULL on historical rows';
COMMENT ON COLUMN user_affiliate_ledger.source_id IS 'Primary key of the recharge fact. Payment orders also keep source_order_id';
COMMENT ON COLUMN user_affiliate_ledger.reverse_seq IS 'Idempotency key for one reverse of a source. Payment refunds use credited micro-units; admin debits use the adjustment record id';
COMMENT ON COLUMN user_affiliate_ledger.note IS 'Admin settlement note. NULL for accrue and reverse';

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_affiliate_ledger_accrue_source
    ON user_affiliate_ledger (source_kind, source_id)
    WHERE action = 'accrue' AND source_kind IS NOT NULL AND source_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_affiliate_ledger_reverse_source
    ON user_affiliate_ledger (source_kind, source_id, reverse_seq)
    WHERE action = 'reverse' AND source_kind IS NOT NULL AND source_id IS NOT NULL AND reverse_seq IS NOT NULL;
