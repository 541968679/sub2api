-- Migration: 241_channel_monitor_v2_health_thresholds
-- Remap of freeze 198.

ALTER TABLE channel_monitor_v2_config
    ADD COLUMN IF NOT EXISTS health_thresholds JSONB NOT NULL DEFAULT '{
      "minimum_sample": 50,
      "warning_error_rate": 0.05,
      "critical_error_rate": 0.20,
      "target_ttft_ms": 3000,
      "warning_ttft_ms": 8000,
      "critical_ttft_ms": 20000,
      "warning_cache_rate": 0,
      "critical_cache_rate": 0,
      "error_weight": 0.60,
      "ttft_weight": 0.20,
      "cache_weight": 0.20
    }'::jsonb;
