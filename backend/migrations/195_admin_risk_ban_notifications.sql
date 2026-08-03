-- Admin inbox for risk-control auto-ban events.
-- Notifications are global events; read state is per admin.
-- Clear/delete removes the notification row for all admins (CASCADE clears reads).

CREATE TABLE IF NOT EXISTS admin_risk_ban_notifications (
    id                  BIGSERIAL PRIMARY KEY,
    user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_email          VARCHAR(255) NOT NULL DEFAULT '',
    moderation_log_id   BIGINT REFERENCES content_moderation_logs(id) ON DELETE SET NULL,
    highest_category    VARCHAR(64) NOT NULL DEFAULT '',
    highest_score       DECIMAL(8, 6) NOT NULL DEFAULT 0,
    violation_count     INT NOT NULL DEFAULT 0,
    ban_threshold       INT NOT NULL DEFAULT 0,
    group_name          VARCHAR(255) NOT NULL DEFAULT '',
    model               VARCHAR(255) NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_admin_risk_ban_notifications_created_at
    ON admin_risk_ban_notifications (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_admin_risk_ban_notifications_user_id
    ON admin_risk_ban_notifications (user_id);

CREATE TABLE IF NOT EXISTS admin_risk_ban_notification_reads (
    admin_user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_id  BIGINT NOT NULL REFERENCES admin_risk_ban_notifications(id) ON DELETE CASCADE,
    read_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (admin_user_id, notification_id)
);

CREATE INDEX IF NOT EXISTS idx_admin_risk_ban_notification_reads_admin
    ON admin_risk_ban_notification_reads (admin_user_id, read_at DESC);

COMMENT ON TABLE admin_risk_ban_notifications IS 'Admin-facing risk-control auto-ban events';
COMMENT ON TABLE admin_risk_ban_notification_reads IS 'Per-admin read receipts for ban notifications';
