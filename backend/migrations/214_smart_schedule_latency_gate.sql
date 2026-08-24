-- +goose Up
ALTER TABLE user_smart_schedule_policies
    ADD COLUMN IF NOT EXISTS quality_max_slow_in_window INT NULL,
    ADD COLUMN IF NOT EXISTS quality_max_consecutive_slow INT NULL,
    ADD COLUMN IF NOT EXISTS quality_max_p50_duration_ms INT NULL;

UPDATE user_smart_schedule_policies p
SET quality_max_p50_duration_ms = 80000
FROM users u
WHERE p.user_id = u.id
  AND u.email = 'zuoge85@gmail.com'
  AND p.quality_max_p50_duration_ms IS NULL;

-- +goose Down
ALTER TABLE user_smart_schedule_policies
    DROP COLUMN IF EXISTS quality_max_slow_in_window,
    DROP COLUMN IF EXISTS quality_max_consecutive_slow,
    DROP COLUMN IF EXISTS quality_max_p50_duration_ms;
