# 模型映射 (Model Mapping)

> 控制账号支持哪些模型，以及请求模型如何映射到上游实际模型。

## OpenAI Claude-GPT Bridge Mapping

For the OpenAI account-side Claude-GPT bridge, the existing
`account.credentials.model_mapping` is the only mapping source. A bridge account
becomes eligible for an Antigravity `/v1/messages` Claude request only when:

- the account platform is OpenAI;
- `extra.openai_claude_gpt_bridge_enabled=true`;
- `model_mapping` explicitly matches the requested Claude model, including the
  existing exact and suffix-wildcard matching rules;
- the mapped upstream model is non-empty and different from the requested Claude
  model.

Example:

```json
{
  "claude-opus-4-8": "gpt-5.5"
}
```

This mapping is account-global and is edited in the OpenAI account form. It is
not edited on Antigravity groups. Whitelist-style self mappings such as
`"claude-opus-4-8": "claude-opus-4-8"` do not enable the bridge, because they
would not hide a distinct GPT upstream model.

## Model Config UI Provider Editing (2026-07-03)

The admin model configuration page must treat provider as an editable platform
dimension, not as an Antigravity-only assumption:

- `modelPricingOptions.ts` is the shared frontend provider vocabulary for
  Anthropic, OpenAI, Gemini, and Antigravity. Use it from pricing rows, mapping
  popovers, detail dialogs, inline quick edits, and test dialogs.
- Model test account loading is provider-scoped. Changing the provider in the
  test dialog clears the selected account and reloads active schedulable accounts
  for that provider.
- Global model pricing detail and inline quick edit both persist `provider` and
  `billing_mode`. The pricing list/detail service must prefer a non-empty global
  override provider over the LiteLLM provider so provider edits are visible and
  filterable immediately.
- Image billing's flat fallback field continues to use `per_request_price`,
  matching the existing detail dialog and backend image billing resolver.

## Default Mapping Billing Object (2026-07-03)

The model configuration page has one editable billing selector for platform
default mappings: `计费对象` / `Billing Object`.

- Valid values are `requested` and `mapped`.
- `requested` means pricing lookup uses the client-requested model name. This is
  the default when no override is saved, preserving existing production
  behavior.
- `mapped` means pricing lookup uses the model name after the platform default
  mapping has rewritten the request.
- This is not channel `billing_model_source`, and it is not token-vs-image
  billing mode. Channel billing source, account-level `credentials.model_mapping`,
  and usage billing mode keep their existing behavior.

Settings KV stores the override maps by platform:

- `antigravity_default_model_mapping_billing_object`
- `anthropic_default_model_mapping_billing_object`
- `openai_default_model_mapping_billing_object`
- `gemini_default_model_mapping_billing_object`

Platform default mapping settings are saved as the full effective mapping table.
For Antigravity, no saved setting means the built-in table is used; once the
admin UI saves the table, the saved table replaces the built-in table. This is
required so deleting a built-in default mapping persists instead of being
reintroduced by runtime merge.

Admin APIs:

- `GET /api/v1/admin/accounts/default-model-mapping-billing-objects/:platform`
- `PUT /api/v1/admin/accounts/default-model-mapping-billing-objects/:platform`

The model-pricing list returns two separate editability flags:

- `mapping_key`: the request-model key to edit/delete or use when saving the
  billing object. Frontend operations must use this value, not the row's pricing
  model, because `upstream_only` rows are keyed by the mapped target model.
- `mapping_billing_objects`: optional per-source billing-object overrides for
  rows whose `related_models` contains multiple mapping keys.
- `billing_object_editable`: the row represents a mapping key whose billing
  object can be changed.
- `mapping_editable`: the row represents an effective platform default mapping
  key and can be edited or deleted.

Built-in platform defaults and LiteLLM-discovered rows that represent an
effective mapping key should expose edit/delete and billing-object controls. If
the row only has pricing data and no mapping key, it remains a normal pricing
row.

The frontend must render one row per effective mapping relationship. Do not hide
additional mapping sources behind a `+N` aggregate if those hidden sources need
edit, delete, or billing-object controls.

The request model name is the primary key for the model mapping table. The UI
must not render two rows with the same request model name; when a pricing row and
an upstream aggregate both describe the same request model, keep one editable
row keyed by that request model.

Anthropic includes LiteLLM alias defaults for common old naming schemes, for
example `claude-4-sonnet-20250514 -> claude-sonnet-4-20250514`, so those aliases
are editable mapping rows rather than plain pricing rows.

## 数据模型

| 实体/字段 | 位置 | 说明 |
|-----------|------|------|
| model_mapping (JSONB) | `account.credentials["model_mapping"]` | `{ "请求模型": "上游模型" }` 键值对 |
| *_default_model_mapping | Settings KV | 平台级默认映射，当前支持 anthropic/openai/gemini/antigravity |
| DefaultAntigravityModelMapping | `backend/internal/domain/constants.go:72-115` | Antigravity 平台内置默认映射 |
| DefaultBedrockModelMapping | `backend/internal/domain/constants.go:121-139` | Bedrock 平台内置默认映射 |

### 映射存储格式

```json
{
  "claude-opus-4-6": "claude-opus-4-6-thinking",
  "claude-*": "claude-sonnet-4-5",
  "gemini-2.5-flash": "gemini-2.5-flash"
}
```

- 支持**精确匹配**和**通配符**（`*` 只能在末尾）
- 白名单模式 = 模型映射到自身（`"model": "model"`）

## 关键文件

| 层级 | 文件 | 职责 |
|------|------|------|
| **Domain** | `backend/internal/domain/constants.go` | DefaultAntigravityModelMapping, DefaultBedrockModelMapping |
| **Service** | `backend/internal/service/account.go:392-600` | GetModelMapping(), GetMappedModel(), IsModelSupported() |
| **Service** | `backend/internal/service/setting_service.go` | 平台级默认映射 Settings KV 读写 |
| **Service** | `backend/internal/service/antigravity_gateway_service.go:959-992` | mapAntigravityModel() — 严格白名单模式 |
| **Service** | `backend/internal/service/gateway_service.go:8065-8071` | resolveAccountUpstreamModel() — 按平台分发 |
| **Handler** | `backend/internal/handler/admin/account_handler.go` | 默认映射管理 API |
| **Frontend Component** | `frontend/src/components/admin/model-pricing/ModelMappingInlinePopover.vue` | 模型配置页新增/编辑平台级默认映射 |
| **Frontend Composable** | `frontend/src/composables/useModelWhitelist.ts` | 模型列表、预设映射、buildModelMappingObject() |
| **Frontend Component** | `frontend/src/components/account/CreateAccountModal.vue` | 创建时的模型配置 UI |
| **Frontend Component** | `frontend/src/components/account/EditAccountModal.vue` | 编辑时的模型配置 UI |
| **Usage Log** | `backend/ent/schema/usage_log.go:41-57` | model, requested_model, upstream_model, model_mapping_chain |

## 核心流程

### 请求时模型解析

```
客户端请求 model="claude-opus-4-5"
  → gateway_service.go: resolveAccountUpstreamModel(account, model)
    ├─ Antigravity 平台:
    │   → antigravity_gateway_service.go: mapAntigravityModel()
    │     → account.GetMappedModel("claude-opus-4-5")
    │       → 查 credentials["model_mapping"]（有缓存）
    │       → 精确匹配 → 通配符匹配 → 未找到返回原名
    │     → 如果映射后 == 原名，检查 IsModelSupported()
    │     → 不支持 → 返回 "" → 该账号被排除调度
    │
    └─ 其他平台:
        → account.GetMappedModel("claude-opus-4-5")
        → 未找到 → 透传原模型名（不阻止）
```

### 管理员配置模型映射

```
CreateAccountModal.vue / EditAccountModal.vue
  → useModelWhitelist.ts: buildModelMappingObject(mode, models/mappings)
    ├─ mode='whitelist': { "model1": "model1", "model2": "model2" }
    └─ mode='mapping': { "源模型": "目标模型", ... }
  → 写入 credentials.model_mapping
  → POST/PUT /api/v1/admin/accounts → 保存到 DB
```

### 管理员配置平台级默认映射

```
ModelPricingTab.vue
  → 新增/编辑映射时选择供应商
  → ModelMappingInlinePopover.vue
  → GET/PUT /api/v1/admin/accounts/default-model-mapping/:platform
  → setting_service.go 写入对应 Settings KV
  → account.ResolveMappedModel() 调度时读取平台默认映射
```

- 支持平台：`anthropic`、`openai`、`gemini`、`antigravity`
- Antigravity 仍保留内置严格白名单，运行时会合并管理员覆盖映射。
- OpenAI/Anthropic/Gemini 的平台默认映射只负责把已配置请求模型改写到上游模型；未配置模型继续按兼容平台透传，不会因为存在默认映射而变成全局白名单。
- 当账号自身 `credentials.model_mapping` 没命中时，非 Antigravity 账号会继续尝试平台级默认映射，确保在模型配置页新增的供应商映射能参与账号调度和请求改写。

### 默认映射回退链

```
account.GetModelMapping():
  1. credentials["model_mapping"] 存在 → 使用自定义
  2. Antigravity 平台 → DefaultAntigravityModelMapping
  3. Bedrock 平台 → DefaultBedrockModelMapping
  4. 其他平台 → nil（无映射，透传）

account.ResolveMappedModel():
  1. credentials["model_mapping"] 命中 → 使用账号级映射
  2. 非 Antigravity 账号未命中 → 尝试平台级默认映射
  3. 仍未命中 → 透传原模型名
```

## 重要机制

| 机制 | 说明 | 相关文件 |
|------|------|---------|
| 映射缓存 | GetModelMapping() 基于 credentials 指针 + FNV64a 哈希缓存，避免重复 JSON 解析 | `account.go:392-423` |
| 平台级默认映射 | 管理员可在模型配置页为 anthropic/openai/gemini/antigravity 配置默认映射 | `setting_service.go`, `account_handler.go`, `ModelMappingInlinePopover.vue` |
| 通配符匹配 | `*` 只能在末尾，如 `claude-*` 匹配 `claude-opus-4-5` | `account.go:571-600` |
| Antigravity 严格模式 | 不在映射中的模型返回空字符串，账号被排除调度 | `antigravity_gateway_service.go:959-992` |
| Bedrock 区域前缀 | `us.anthropic.claude-*` 中的 `us.` 会根据 aws_region 动态替换 | `domain/constants.go:121-139` |
| 模型迁移 | 默认映射中旧模型自动指向新模型（如 opus-4-5 → opus-4-6-thinking） | `domain/constants.go:72-115` |
| 持久化映射回填 | 新增官方模型时，已有 `credentials.model_mapping` 账号需要 migration 补同名映射，避免严格模式漏调度 | `backend/migrations/146_add_opus48_to_model_mapping.sql` |

## 已知陷阱

- **Sonnet 5 production-only sync (2026-07-02)**：`claude-sonnet-5` 通过 Claude 默认模型列表和前端白名单预设暴露。Bedrock 默认映射为 `us.anthropic.claude-sonnet-5-v1`，再由 `ResolveBedrockModelID` 按账号 `aws_region` 替换区域前缀。默认 `context-1m-2025-08-07` beta 策略只放行 Sonnet 5 direct/Vertex/Bedrock ID，Sonnet 4.x、Opus、Haiku、legacy Sonnet 仍会过滤该 beta。
- **Antigravity 默认映射更新滞后**：上游新增模型时，`DefaultAntigravityModelMapping` 可能未及时更新，需手动添加映射
- **迁移编号分叉**：合并上游模型回填迁移时，先检查本 fork 最新 migration 编号；如上游编号已被二开占用，保留 SQL 逻辑并改用本地下一编号。
- **白名单 vs 映射混淆**：白名单模式本质是映射到自身，前端 `buildModelMappingObject` 统一输出为映射格式
- **通配符贪婪匹配**：`claude-*` 会匹配所有 claude 开头的模型，可能匹配到不期望的模型
