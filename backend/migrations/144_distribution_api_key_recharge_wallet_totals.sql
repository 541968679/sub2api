-- Include API-key recharges in distribution wallet lifetime spend counters.
UPDATE distribution_wallets dw
SET total_spent = COALESCE(ledger.total_spent, 0),
    updated_at = NOW()
FROM (
    SELECT
        wallet_id,
        SUM(CASE WHEN action IN ('generate_redeem_code', 'generate_subscription', 'generate_api_key', 'recharge_api_key') AND amount < 0 THEN -amount ELSE 0 END) AS total_spent
    FROM distribution_wallet_ledger
    GROUP BY wallet_id
) ledger
WHERE dw.id = ledger.wallet_id
  AND dw.total_spent IS DISTINCT FROM COALESCE(ledger.total_spent, 0);

UPDATE distribution_wallets dw
SET total_spent = 0,
    updated_at = NOW()
WHERE NOT EXISTS (
    SELECT 1 FROM distribution_wallet_ledger l WHERE l.wallet_id = dw.id
)
  AND dw.total_spent <> 0;
