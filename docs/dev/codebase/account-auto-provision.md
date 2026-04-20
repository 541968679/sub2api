# 自动上号 (Account Auto Provision)

> 监控指定分组的健康账号数、并发利用率和 AI Credits 阈值；当任一条件触发时，自动把一个未分组备用账号补进目标分组，并按模板或“最后健康账号快照”应用配置。

## 数据模型

| 实体/字段 | 位置 | 说明 |
|-----------|------|------|
| `account_auto_provision_settings` | `settings` 表 JSON | 自动上号配置：全局开关、巡检间隔、规则列表、模板 |
| `account_auto_provision_state` | `settings` 表 JSON | 运行时状态：每个分组的最后健康模板快照、规则冷却时间 |
| `AccountAutoProvisionSettings` | `backend/internal/service/account_auto_provision_settings.go` | 规则配置结构 |
| `AccountAutoProvisionState` | `backend/internal/service/account_auto_provision_settings.go` | 运行时状态结构 |
| `AccountAutoProvisionTemplate` | `backend/internal/service/account_auto_provision_settings.go` | 上号模板：`proxy_id / concurrency / priority / load_factor / schedulable / allow_overages` |

## 关键文件

| 层级 | 文件 | 职责 |
|------|------|------|
| **Service** | `backend/internal/service/account_auto_provision_service.go` | 巡检 worker、触发判断、候选账号选择、补号执行 |
| **Service** | `backend/internal/service/setting_service_account_auto_provision.go` | 配置/状态的 DB 读写与校验 |
| **Handler** | `backend/internal/handler/admin/setting_handler_account_auto_provision.go` | 管理后台配置接口 |
| **Route** | `backend/internal/server/routes/admin.go` | `/admin/settings/account-auto-provision` 路由 |
| **Frontend View** | `frontend/src/views/admin/AccountAutoProvisionView.vue` | 自动上号配置页 |
| **Frontend API** | `frontend/src/api/admin/settings.ts` | 自动上号设置的前端调用封装 |
| **Navigation** | `frontend/src/router/index.ts` | 后台路由入口 |
| **Navigation** | `frontend/src/components/layout/AppSidebar.vue` | 后台菜单入口 |

## 核心流程

### 1. 加载与保存配置

```
AccountAutoProvisionView.vue
  → GET /api/v1/admin/settings/account-auto-provision
    → SettingHandler.GetAccountAutoProvisionSettings()
      → SettingService.GetAccountAutoProvisionSettings()
      → SettingService.GetAccountAutoProvisionState()

点击保存
  → PUT /api/v1/admin/settings/account-auto-provision
    → SettingHandler.UpdateAccountAutoProvisionSettings()
      → SettingService.SetAccountAutoProvisionSettings()
```

### 2. 后台巡检

```
ProvideAccountAutoProvisionService()
  → NewAccountAutoProvisionService(..., 30s)
  → Start()
    → ticker 周期触发 runOnce()
      → 读取 settings/state
      → 若未到 check_interval_seconds 则跳过
      → 逐条规则、逐个 group 检查触发条件
```

### 3. 任一触发条件命中后补号

```
runOnce()
  → ListByGroup(groupID)
  → 过滤出满足 group 要求的健康账号
    - account.IsSchedulable()
    - require_oauth_only
    - require_privacy_set
  → 判断是否命中任一条件
    - normal_account_count_below
    - concurrency_utilization_above
    - ai_credits_below
  → 冷却检查
  → ListSchedulableUngroupedByPlatform(group.Platform)
  → 过滤不满足分组要求的候选
  → 选择一个候选账号
  → 按 provision_mode 取模板
    - `template`
    - `clone_last_healthy`（无快照时回退显式模板）
  → AdminService.UpdateAccount()
  → AdminService.SetAccountSchedulable()
  → 回写 state.last_triggered
```

## 重要机制

| 机制 | 说明 | 相关文件 |
|------|------|---------|
| 任一条件触发 | 三类阈值是 OR 关系，只要任一命中就尝试补号 | `account_auto_provision_service.go` |
| 最后健康快照 | 从当前健康账号里选“最近使用”的那个，保存模板快照，供 clone 模式使用 | `buildLastHealthySnapshot` |
| 模板回退 | 选择 `clone_last_healthy` 时如果当前没有快照，回退到显式模板 | `evaluateRuleForGroup` |
| 候选约束 | 只从“未分组 + 同平台 + 健康”的账号中挑；额外遵守 `require_oauth_only / require_privacy_set` | `selectUngroupedCandidate` |
| Credits 查询节流 | `ai_credits_check_interval_minutes` 控制每条规则/分组的 Credits 刷新频率，避免高频远程查询 | `getGroupAICredits` |
| 运行时状态持久化 | 冷却时间和健康快照写入 `settings` 表，重启后仍可继续使用 | `setting_service_account_auto_provision.go` |

## 已知陷阱

- **AI Credits 只适用于 Antigravity**：当前阈值判断只对 `group.platform == antigravity` 生效，其它平台不会触发 Credits 条件。
- **快照不是完整账号复制**：只复制调度相关模板字段，不复制 credentials/token 等敏感信息。
- **显式模板是 clone 模式兜底**：如果分组从未出现过健康账号，clone 模式会使用显式模板，因此模板也要填可用值。
- **并发阈值看的是利用率**：按“当前并发 / 健康账号并发上限”计算，而不是绝对并发数。
- **配置生效依赖后台 worker**：仅保存配置不会立即补号，需等待巡检周期或手动触发后续验证请求。
