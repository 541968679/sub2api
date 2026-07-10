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

## OpenAI GPT-5.6 Model Mapping (2026-07-10)

OpenAI model lists and presets include `gpt-5.6-sol`, `gpt-5.6-terra`, and
`gpt-5.6-luna` as normal OpenAI targets. They can be used in explicit account
`model_mapping` entries or admin UI presets, but the built-in
`defaultOpenAIClaudeGPTBridgePresetMappings` stays on the existing
`gpt-5.5`/`gpt-5.4` targets. Do not silently migrate Claude-GPT bridge defaults
to GPT-5.6 during upstream sync; that would change production bridge behavior
and billing expectations without an admin decision.

Backend model normalization treats these as first-class known OpenAI/Codex
models before the broad legacy `gpt-5 -> gpt-5.4` fallback. This covers compact,
date-suffixed, `openai/...`, and reasoning-effort variants such as
`gpt-5.6-terra-high`.

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

## Model Config Row Contract (2026-07-04)

The model-pricing list emits complete per-platform mapping roles; the frontend
derives rows from them without any cross-item expansion or dedup:

- `billing_basis_hints[]`: one hint per platform where the model participates
  in the default mapping (provider filter → only that platform; "All" view →
  every matching platform). The singular `billing_basis_hint` stays populated
  (provider-matched or first) for legacy consumers such as
  `resolveModelPricingProvider`.
- Per hint, roles never collapse: `mapping_target` is set when the model itself
  is a mapping key (including same-name entries), and `mapped_from` lists other
  request names mapping to it. A model can carry both simultaneously (e.g.
  `claude-opus-4-7 -> claude-opus-4-6` while `claude-opus-4-6 ->
  claude-opus-4-6-thinking`). The old implementation collapsed roles by
  `same_name > upstream_only > requested_only` priority, which silently dropped
  the model's own mapping and made saved targets render as the request name
  again.
- `mapping_editable` / `billing_object_editable` are true only when the model
  itself is a mapping key; rows for mapping sources are owned by the source
  models' own items (the list stubs every mapping key, so those items always
  exist).

Frontend row derivation (`modelPricingRows.ts`):

- A hint with `mapping_target` renders one editable/deletable mapping row per
  platform, keyed `platform:request-model`.
- Everything else renders a pass-through row (request = upstream = model). The
  edit action on pass-through rows opens the add-mapping popover prefilled with
  `from = to = model`, so any row can be turned into a real mapping entry; no
  delete button because there is no entry to delete. A `+N` badge with tooltip
  lists `mapped_from` sources.
- Mapping rows are derived only from the mapping key's own item — never
  expanded from the target item — so rows can no longer overwrite each other.
- Saving a mapping to a platform other than the active provider tab switches
  the tab to that platform so the new row is immediately visible.

Every row is deletable:

- Mapping rows delete the platform default mapping entry (existing behavior).
- Pass-through rows delete by hiding the model from the config list. The hidden
  set persists in Settings KV `model_pricing_hidden_models` (JSON array of
  lowercase model names, admin APIs `GET/PUT
  /api/v1/admin/model-pricing/hidden-models`). Hiding only affects this list —
  billing and request forwarding are untouched. The source filter gains a
  "hidden" view (`source=hidden`) that lists hidden models — including stale
  names no longer in the catalog — with a restore action. A hidden model that
  is (or later becomes) an effective mapping key stays visible as its mapping
  row; hiding never swallows real mapping entries.

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

## 测试连接模型列表 (2026-07-04)

`GET /api/v1/admin/accounts/:id/models`（账号管理 → 测试连接的模型下拉）在原有
账号级 `credentials.model_mapping` / 默认模型集的基础上，统一并入平台级默认映射
的请求模型名，保证模型配置页新增的映射能被选中测试：

- Antigravity 非透传账号：可测模型 = 生效映射表的请求模型名（账号自定义映射
  优先，否则 `ResolveAntigravityDefaultMapping()`），Claude 模型追加 [1m]/[2m]
  变体（`antigravity.ModelsForMappingKeys`）。不再使用滞后的静态
  `antigravity.DefaultModels()`。
- Claude / Gemini / OpenAI 账号：默认模型集或账号映射键 ∪ 对应平台默认映射键。
- OpenAI 自动透传与 Kiro 反代透传保持原行为（透传绕过映射改写）。

## 已知陷阱

- **保存过的平台默认映射会滞后于内置表 (2026-07-04)**：管理员在模型配置页保存
  过映射表后，保存表整体替换内置表；之后 fork 同步新增的内置模型（如
  claude-fable-5）不会自动出现，Antigravity 严格白名单会漏调度。补救模式是
  回填 migration（`177_add_fable5_to_default_model_mapping.sql` 同时回填
  settings 表和账号级 model_mapping，参照 146 的 opus-4-8 模式）。新增官方模型
  时检查是否需要同类回填。
- **Sonnet 5 production-only sync (2026-07-02)**：`claude-sonnet-5` 通过 Claude 默认模型列表和前端白名单预设暴露。Bedrock 默认映射为 `us.anthropic.claude-sonnet-5-v1`，再由 `ResolveBedrockModelID` 按账号 `aws_region` 替换区域前缀。默认 `context-1m-2025-08-07` beta 策略只放行 Sonnet 5 direct/Vertex/Bedrock ID，Sonnet 4.x、Opus、Haiku、legacy Sonnet 仍会过滤该 beta。
- **Antigravity 默认映射更新滞后**：上游新增模型时，`DefaultAntigravityModelMapping` 可能未及时更新，需手动添加映射
- **迁移编号分叉**：合并上游模型回填迁移时，先检查本 fork 最新 migration 编号；如上游编号已被二开占用，保留 SQL 逻辑并改用本地下一编号。
- **白名单 vs 映射混淆**：白名单模式本质是映射到自身，前端 `buildModelMappingObject` 统一输出为映射格式
- **通配符贪婪匹配**：`claude-*` 会匹配所有 claude 开头的模型，可能匹配到不期望的模型
