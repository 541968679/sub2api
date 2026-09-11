-- Migration: 239_channel_monitor_v2_ignored_error_categories
-- Remap of freeze 196.

ALTER TABLE channel_monitor_v2_config
    ADD COLUMN IF NOT EXISTS ignored_error_categories TEXT[] NOT NULL DEFAULT '{}';
