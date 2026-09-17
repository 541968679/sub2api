-- Migration: 245_channel_monitor_v2_privacy_defaults
-- Remap of freeze 203+204+205+206. Factory ignore list + hide-throughput default on.

UPDATE channel_monitor_v2_config
SET ignored_error_categories = ARRAY[
    'authentication',
    'client_cancelled',
    'content_policy',
    'context_limit',
    'group_access',
    'model_unsupported',
    'not_found',
    'quota_or_balance'
]::text[]
WHERE id = 1
  AND COALESCE(cardinality(ignored_error_categories), 0) = 0;

INSERT INTO settings (key, value)
VALUES ('channel_monitor_hide_throughput', 'true')
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value)
VALUES ('channel_monitor_hide_user_ranking', 'false')
ON CONFLICT (key) DO NOTHING;
