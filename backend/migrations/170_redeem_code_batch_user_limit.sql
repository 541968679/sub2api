ALTER TABLE redeem_codes
ADD COLUMN IF NOT EXISTS batch_id VARCHAR(64);

ALTER TABLE redeem_codes
ADD COLUMN IF NOT EXISTS batch_redeem_limit_per_user BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX IF NOT EXISTS redeemcode_batch_id_used_by
ON redeem_codes (batch_id, used_by)
WHERE batch_id IS NOT NULL
  AND used_by IS NOT NULL
  AND status = 'used'
  AND batch_redeem_limit_per_user = TRUE;
