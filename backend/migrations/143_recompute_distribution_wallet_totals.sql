-- Recompute distribution wallet lifetime counters from ledger actions.
-- Asset refunds are positive balance movements, but they are not recharges.
UPDATE distribution_wallets dw
SET total_recharged = COALESCE(ledger.total_recharged, 0),
    total_spent = COALESCE(ledger.total_spent, 0),
    updated_at = NOW()
FROM (
    SELECT
        wallet_id,
        SUM(CASE WHEN action = 'admin_adjust' AND amount > 0 THEN amount ELSE 0 END) AS total_recharged,
        SUM(CASE WHEN action IN ('generate_redeem_code', 'generate_subscription', 'generate_api_key') AND amount < 0 THEN -amount ELSE 0 END) AS total_spent
    FROM distribution_wallet_ledger
    GROUP BY wallet_id
) ledger
WHERE dw.id = ledger.wallet_id
  AND (dw.total_recharged IS DISTINCT FROM COALESCE(ledger.total_recharged, 0)
       OR dw.total_spent IS DISTINCT FROM COALESCE(ledger.total_spent, 0));

UPDATE distribution_wallets dw
SET total_recharged = 0,
    total_spent = 0,
    updated_at = NOW()
WHERE NOT EXISTS (
    SELECT 1 FROM distribution_wallet_ledger l WHERE l.wallet_id = dw.id
)
  AND (dw.total_recharged <> 0 OR dw.total_spent <> 0);

-- Backfill refunds for historical distribution API-key assets whose underlying
-- API key was already disabled/deleted outside the distribution void flow.
WITH refundable_assets AS (
    SELECT
        da.id,
        da.wallet_id,
        da.user_id,
        da.cost_rmb,
        SUM(da.cost_rmb) OVER (PARTITION BY da.wallet_id ORDER BY da.id) AS running_refund
    FROM distribution_assets da
    JOIN api_keys ak ON da.reference_type = 'api_key' AND ak.id::text = da.reference_id
    WHERE da.asset_type = 'api_key'
      AND da.refunded_at IS NULL
      AND da.refunded_rmb = 0
      AND da.cost_rmb > 0
      AND COALESCE(ak.quota_used, 0) = 0
      AND (ak.deleted_at IS NOT NULL OR ak.status = 'disabled')
),
updated_assets AS (
    UPDATE distribution_assets da
    SET status = 'disabled',
        refunded_at = NOW(),
        refunded_rmb = da.cost_rmb,
        updated_at = NOW()
    FROM refundable_assets ra
    WHERE da.id = ra.id
    RETURNING da.id, da.wallet_id, da.user_id, da.cost_rmb
),
wallet_refunds AS (
    SELECT wallet_id, SUM(cost_rmb) AS refund_total
    FROM updated_assets
    GROUP BY wallet_id
),
updated_wallets AS (
    UPDATE distribution_wallets dw
    SET balance = balance + wr.refund_total,
        updated_at = NOW()
    FROM wallet_refunds wr
    WHERE dw.id = wr.wallet_id
    RETURNING dw.id AS wallet_id, dw.user_id, dw.balance
)
INSERT INTO distribution_wallet_ledger (wallet_id, user_id, action, amount, balance_after, reference_type, reference_id, note, created_at)
SELECT
    ua.wallet_id,
    ua.user_id,
    'asset_refund',
    ua.cost_rmb,
    uw.balance - (wr.refund_total - ra.running_refund),
    'distribution_asset',
    ua.id::text,
    'backfill voided api key asset refund',
    NOW()
FROM updated_assets ua
JOIN refundable_assets ra ON ra.id = ua.id
JOIN wallet_refunds wr ON wr.wallet_id = ua.wallet_id
JOIN updated_wallets uw ON uw.wallet_id = ua.wallet_id
WHERE NOT EXISTS (
    SELECT 1
    FROM distribution_wallet_ledger l
    WHERE l.action = 'asset_refund'
      AND l.reference_type = 'distribution_asset'
      AND l.reference_id = ua.id::text
);
