-- +goose Up
ALTER TABLE user_smart_schedule_policies
    ADD COLUMN IF NOT EXISTS probe_latency_v2 BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE user_smart_schedule_policies
    DROP COLUMN IF EXISTS probe_latency_v2;
