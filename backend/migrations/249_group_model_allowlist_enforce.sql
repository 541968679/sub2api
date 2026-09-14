-- 249: converge groups.model_allowlist and seed enforce-off.
-- Remap of freeze 236. Never rename models_list_config.
-- Missing or non-true group_model_allowlist_enforce means request admission is not enforced.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS model_allowlist JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE groups SET model_allowlist = '{}'::jsonb WHERE model_allowlist IS NULL;

ALTER TABLE groups ALTER COLUMN model_allowlist SET DEFAULT '{}'::jsonb;
ALTER TABLE groups ALTER COLUMN model_allowlist SET NOT NULL;

COMMENT ON COLUMN groups.model_allowlist IS
    'Group model allowlist config (enabled + models). Listing may filter when enabled; gateway request admission is gated by group_model_allowlist_enforce (default off).';

INSERT INTO settings (key, value)
VALUES ('group_model_allowlist_enforce', 'false')
ON CONFLICT (key) DO NOTHING;
