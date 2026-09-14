-- Allow Composite model routes to target CN providers when the table exists.
-- Remapped from upstream 227_composite_routes_add_cn_providers.sql.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = current_schema() AND table_name = 'composite_model_routes'
    ) THEN
        ALTER TABLE composite_model_routes
            DROP CONSTRAINT IF EXISTS composite_model_routes_target_platform_check;
        ALTER TABLE composite_model_routes
            ADD CONSTRAINT composite_model_routes_target_platform_check
            CHECK (target_platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok',
                                       'kimi', 'zhipu', 'deepseek'));
    END IF;
END $$;
