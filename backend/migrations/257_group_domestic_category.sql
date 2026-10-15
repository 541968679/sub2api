-- 257: display category for 国产模型 groups.
-- Additive. Empty means unset. Gateway routing still uses groups.platform.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS domestic_category VARCHAR(20) NOT NULL DEFAULT '';

COMMENT ON COLUMN groups.domestic_category IS
    'Display category for 国产模型 groups: mixed, deepseek, kimi, glm, qwen. Empty means unset. Not a gateway platform.';
