-- Migration: 244_channel_monitor_v2_refresh_5m
-- Remap of freeze 201.

UPDATE channel_monitor_v2_config
SET refresh_interval_seconds = 300,
    updated_at = NOW()
WHERE id = 1
  AND version = 1
  AND updated_by IS NULL
  AND refresh_interval_seconds = 60;
