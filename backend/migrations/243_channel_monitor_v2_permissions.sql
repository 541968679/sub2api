-- Migration: 243_channel_monitor_v2_permissions
-- Remap of freeze 200+202. Grant current role access to v2 tables.

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE
    channel_monitor_v2_config,
    channel_monitor_v2_watermarks,
    channel_monitor_v2_metrics_1m,
    channel_monitor_v2_user_metrics_1m,
    channel_monitor_v2_error_metrics_1m,
    channel_monitor_v2_latency_histograms_1m,
    channel_monitor_v2_metrics_rollup,
    channel_monitor_v2_user_metrics_rollup,
    channel_monitor_v2_error_metrics_rollup,
    channel_monitor_v2_latency_histograms_rollup
TO CURRENT_USER;
