-- Migration: 236_channel_monitor_quota_mode
-- Remap of freeze 226. Quota mode for channel monitors:
--   check_mode probe|quota|quota_probe, optional account_id, history quota JSONB.
-- Provider CHECK expands to grok/antigravity/kimi/zhipu/deepseek/minimax.
-- Default show-quota setting is off.

DO $$
DECLARE
    monitor_constraint_def TEXT;
    template_constraint_def TEXT;
BEGIN
    SELECT pg_get_constraintdef(c.oid)
      INTO monitor_constraint_def
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
     WHERE t.relname = 'channel_monitors'
       AND c.conname = 'channel_monitors_provider_check';

    IF monitor_constraint_def IS NULL OR position('kimi' IN monitor_constraint_def) = 0 THEN
        ALTER TABLE channel_monitors
            DROP CONSTRAINT IF EXISTS channel_monitors_provider_check;
        ALTER TABLE channel_monitors
            ADD CONSTRAINT channel_monitors_provider_check
            CHECK (provider IN ('openai', 'anthropic', 'gemini', 'grok',
                                'antigravity', 'kimi', 'zhipu', 'deepseek', 'minimax'));
    END IF;

    SELECT pg_get_constraintdef(c.oid)
      INTO template_constraint_def
      FROM pg_constraint c
      JOIN pg_class t ON t.oid = c.conrelid
     WHERE t.relname = 'channel_monitor_request_templates'
       AND c.conname = 'channel_monitor_request_templates_provider_check';

    IF template_constraint_def IS NULL OR position('kimi' IN template_constraint_def) = 0 THEN
        ALTER TABLE channel_monitor_request_templates
            DROP CONSTRAINT IF EXISTS channel_monitor_request_templates_provider_check;
        ALTER TABLE channel_monitor_request_templates
            ADD CONSTRAINT channel_monitor_request_templates_provider_check
            CHECK (provider IN ('openai', 'anthropic', 'gemini', 'grok',
                                'antigravity', 'kimi', 'zhipu', 'deepseek', 'minimax'));
    END IF;
END $$;

ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS check_mode VARCHAR(32) NOT NULL DEFAULT 'probe';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'channel_monitors_check_mode_check'
    ) THEN
        ALTER TABLE channel_monitors
            ADD CONSTRAINT channel_monitors_check_mode_check
            CHECK (check_mode IN ('probe', 'quota', 'quota_probe'));
    END IF;
END $$;

ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_channel_monitors_account_id ON channel_monitors(account_id);

ALTER TABLE channel_monitor_histories
    ADD COLUMN IF NOT EXISTS quota JSONB;

INSERT INTO settings (key, value)
VALUES ('channel_monitor_show_quota', 'false')
ON CONFLICT (key) DO NOTHING;
