-- Dedicated pager for routing / group-model / protocol rows that no longer
-- enter pair or account schedule ErrorCount. ON CONFLICT keeps an operator
-- rename of the same rule.

INSERT INTO ops_alert_rules (
    name, description, enabled, metric_type, operator, threshold,
    window_minutes, sustained_minutes, severity, notify_email, cooldown_minutes,
    created_at, updated_at
) VALUES (
    '需运维：组模型/路由/协议',
    '窗口内需运维错误（组内无号、路由 503、协议错配）大于 0 时触发。不计入配对冷却。',
    true, 'ops_attention_count', '>', 0,
    15, 5, 'P2', true, 30, NOW(), NOW()
) ON CONFLICT (name) DO NOTHING;
