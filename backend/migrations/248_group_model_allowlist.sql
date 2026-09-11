-- 248: groups.model_allowlist
-- Remap of freeze 235. Do NOT rename or drop models_list_config (listing-only).
-- New JSONB column is additive. Gateway request-admission stays default-off
-- (see 249_group_model_allowlist_enforce.sql). Empty object = allowlist disabled.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS model_allowlist JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN groups.model_allowlist IS
    'Group model allowlist config (enabled + models). Listing may filter when enabled; gateway request admission is gated by group_model_allowlist_enforce (default off).';
