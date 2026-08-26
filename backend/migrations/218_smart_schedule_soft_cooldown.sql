-- +goose Up
ALTER TABLE user_smart_schedule_policies
    ADD COLUMN IF NOT EXISTS soft_cooldown BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE user_smart_schedule_policies
    DROP COLUMN IF EXISTS soft_cooldown;
