-- 为已持久化的 Antigravity 模型映射补 claude-fable-5 同名映射。
--
-- 平台级默认映射一旦在模型配置页保存过，保存的表会整体替换内置
-- DefaultAntigravityModelMapping；在 claude-fable-5 加入内置表之前保存过的
-- 部署因此缺这一条，导致 Antigravity 账号无法调度 claude-fable-5，
-- 模型配置页的映射表里也看不到该请求模型。
-- 未保存过设置的部署直接使用内置表，不需要回填。

UPDATE settings
SET value = (value::jsonb || '{"claude-fable-5":"claude-fable-5"}'::jsonb)::text
WHERE key = 'antigravity_default_model_mapping'
  AND jsonb_typeof(value::jsonb) = 'object'
  AND value::jsonb->>'claude-fable-5' IS NULL;

-- 同样回填已持久化 model_mapping 的 Antigravity 账号（与 146 回填 opus-4-8 的
-- 模式一致）：账号级映射存在时优先于平台默认映射，缺同名条目会让严格白名单
-- 漏调度 claude-fable-5。
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{model_mapping,claude-fable-5}',
    '"claude-fable-5"'::jsonb,
    true
)
WHERE platform = 'antigravity'
  AND deleted_at IS NULL
  AND jsonb_typeof(credentials->'model_mapping') = 'object'
  AND credentials->'model_mapping'->>'claude-fable-5' IS NULL;
