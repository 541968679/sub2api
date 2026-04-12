# 模型映射 (Model Mapping)

> 控制账号支持哪些模型，以及请求模型如何映射到上游实际模型。

## 数据模型

| 实体/字段 | 位置 | 说明 |
|-----------|------|------|
| model_mapping (JSONB) | `account.credentials["model_mapping"]` | `{ "请求模型": "上游模型" }` 键值对 |
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
| **Service** | `backend/internal/service/antigravity_gateway_service.go:959-992` | mapAntigravityModel() — 严格白名单模式 |
| **Service** | `backend/internal/service/gateway_service.go:8065-8071` | resolveAccountUpstreamModel() — 按平台分发 |
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

### 默认映射回退链

```
account.GetModelMapping():
  1. credentials["model_mapping"] 存在 → 使用自定义
  2. Antigravity 平台 → DefaultAntigravityModelMapping
  3. Bedrock 平台 → DefaultBedrockModelMapping
  4. 其他平台 → nil（无映射，透传）
```

## 重要机制

| 机制 | 说明 | 相关文件 |
|------|------|---------|
| 映射缓存 | GetModelMapping() 基于 credentials 指针 + FNV64a 哈希缓存，避免重复 JSON 解析 | `account.go:392-423` |
| 通配符匹配 | `*` 只能在末尾，如 `claude-*` 匹配 `claude-opus-4-5` | `account.go:571-600` |
| Antigravity 严格模式 | 不在映射中的模型返回空字符串，账号被排除调度 | `antigravity_gateway_service.go:959-992` |
| Bedrock 区域前缀 | `us.anthropic.claude-*` 中的 `us.` 会根据 aws_region 动态替换 | `domain/constants.go:121-139` |
| 模型迁移 | 默认映射中旧模型自动指向新模型（如 opus-4-5 → opus-4-6-thinking） | `domain/constants.go:72-115` |

## 已知陷阱

- **Antigravity 默认映射更新滞后**：上游新增模型时，`DefaultAntigravityModelMapping` 可能未及时更新，需手动添加映射
- **白名单 vs 映射混淆**：白名单模式本质是映射到自身，前端 `buildModelMappingObject` 统一输出为映射格式
- **通配符贪婪匹配**：`claude-*` 会匹配所有 claude 开头的模型，可能匹配到不期望的模型
