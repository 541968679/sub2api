-- 251: groups CCS Codex import model picker (opt-in).
-- Additive. Default off so existing groups keep one-click CCS import.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS ccs_import_model_picker_enabled BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS ccs_import_default_model VARCHAR(100) NOT NULL DEFAULT '';

COMMENT ON COLUMN groups.ccs_import_model_picker_enabled IS
    'When true, user CCS Codex import shows a model picker. Default false.';

COMMENT ON COLUMN groups.ccs_import_default_model IS
    'Default Codex model written into CCS deeplink model= when the picker is enabled. Required if picker is enabled.';
