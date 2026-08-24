-- +goose Up
ALTER TABLE user_smart_schedule_policies
    ADD COLUMN IF NOT EXISTS quality_sched_window_n INT NULL,
    ADD COLUMN IF NOT EXISTS quality_sched_max_slow_in_window INT NULL,
    ADD COLUMN IF NOT EXISTS quality_sched_max_consecutive_slow INT NULL;

-- +goose Down
ALTER TABLE user_smart_schedule_policies
    DROP COLUMN IF EXISTS quality_sched_window_n,
    DROP COLUMN IF EXISTS quality_sched_max_slow_in_window,
    DROP COLUMN IF EXISTS quality_sched_max_consecutive_slow;
