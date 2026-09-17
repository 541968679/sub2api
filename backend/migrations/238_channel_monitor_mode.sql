-- Migration: 238_channel_monitor_mode
-- Remap of freeze 195. Exclusive v1 (active probes) or v2 (passive). Default v1.

INSERT INTO settings (key, value)
VALUES ('channel_monitor_mode', 'v1')
ON CONFLICT (key) DO NOTHING;
