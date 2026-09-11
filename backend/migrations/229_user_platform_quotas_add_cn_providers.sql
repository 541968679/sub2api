-- Allow Kimi / Zhipu / DeepSeek on user_platform_quotas.platform CHECK.
-- Remapped from upstream 224_user_platform_quotas_add_cn_providers.sql.
ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok',
                        'kimi', 'zhipu', 'deepseek'));
