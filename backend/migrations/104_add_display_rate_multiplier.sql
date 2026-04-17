-- 用户专属分组倍率表增加展示倍率字段
-- 允许管理员为用户设置展示给用户的倍率，与真实计费倍率分离

-- 新增展示倍率列（NULL 表示不覆盖展示，用户看到真实倍率）
ALTER TABLE user_group_rate_multipliers
ADD COLUMN IF NOT EXISTS display_rate_multiplier DOUBLE PRECISION DEFAULT NULL;

COMMENT ON COLUMN user_group_rate_multipliers.display_rate_multiplier
IS '展示倍率（用户可见），NULL表示不覆盖展示';

-- 将 rate_multiplier 改为 NULLABLE，允许行仅设展示倍率而不覆盖真实倍率
-- NULL = 使用分组默认倍率，非 NULL = 专属覆盖
ALTER TABLE user_group_rate_multipliers
ALTER COLUMN rate_multiplier DROP NOT NULL;
