-- Retire the account user allow list.
-- The allow column stays so old rows and Ent keep decoding.
-- Deny, pair caps, and quality gates are left in place.
-- Idempotent: a second run matches zero rows.

UPDATE account_schedule_users
SET allow = false
WHERE allow IS TRUE;

DELETE FROM account_schedule_users
WHERE allow = false
  AND deny = false
  AND max_concurrency IS NULL
  AND quality_max_p50_ttft_ms IS NULL
  AND quality_min_success_rate IS NULL;

UPDATE accounts
SET user_schedule_mode = 'unrestricted',
    updated_at = NOW()
WHERE user_schedule_mode = 'allow';
