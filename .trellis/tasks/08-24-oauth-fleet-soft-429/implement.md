# Implement：OAuth fleet 软429

工作区：占用 checkout `E:\cursor project\api2sub`。不要开隔离 worktree。不要把 upstream-sync worktree 规则抄进本文或后续提交说明。

本轮 **只写规划**。实现须等 Brandon 评审本目录制品并另行批准（`task.py start` 不在本轮）。

不要把当前工作区里无关的 sticky / unpooled 脏改卷进本任务。若必须改已脏的 `gateway_service.go` / `openai_gateway_service.go` / `openai_account_scheduler.go` / `gateway_cache.go`，只提交软 429 过滤挂钩，与那些 hunk 分开评审。

## 顺序（实现阶段才写代码）

1. **Settings 合约**（先于热路径）：`SettingKeyOAuthFleetSoft429Settings`、`OAuthFleetSoft429Settings`、`DefaultOAuthFleetSoft429Settings`（**`Enabled: false`**）、`Get`/`Set` 校验（TTL 5–300、政策枚举、平台白名单、v1 状态码只许 429、body 码最多 32）。缺省 / 坏 JSON → Default\*（空 KV = 关）。单测可抄 `overload_cooldown_test.go` 的回落/校验形状，**不要抄 `TestDefaultOverloadCooldownSettings` 的 `Enabled: true`**。
2. **Admin API**：`setting_handler.go` GET/PUT、`dto/settings.go`、`routes/admin.go` `/oauth-fleet-soft-429`。**禁止**写进 `GetPublicSettings` / injection；加 `NotExposedOnPublicSettings` 单测。
3. **分类 helper**（纯函数）：`classifyOAuth429(account, settings, status, headers, body) → soft|hard|not_applicable`。内置硬 > hard 码表 > soft 码表 > `long_reset_policy`。Codex 非 100% + 长 reset：policy=`soft` 时 soft。
4. **启用谓词**：`oauthFleetSoft429Applies(account, settings)` 严格按 design §5.6。覆盖只读 `extra.oauth_fleet_soft_429`（`boolOverrideFromMap`）。**禁止**调用或放宽 `IsPoolMode()`。**禁止**读 `credentials` 里的同名键。
5. **`HandleUpstreamError` 顺序**：Anthropic 官方窗口 / 其它硬窗口保持最前 → applies 且 soft 则写 Redis（TTL=`ttl_seconds`）并 return → 其余原路径。
6. **Layer-2 Redis**：`oauth-soft-429:{accountID}`。调度 `Select*`：无硬亲和时并入排除集。读写失败 fail-open。影子写凭据账号 id（design §9）。
7. **Layer-1**：软 429 仍进 `failedAccountIDs`；`RetryableOnSameAccount` 保持 false。
8. **Admin UI**：Gateway tab 卡片（529 后、质量硬关闭前），自存按钮，`data-test="oauth-fleet-soft-429-card"`。`frontend/src/api/admin/settings.ts` 一对 get/update。`SettingsView.spec.ts` 对齐质量硬关闭：页级 save 不写本卡片；本卡片 save 只打本 API。
9. **账号三态**：`EditAccountModal.vue` 仅 oauth / setup-token；inherit / forceOn / forceOff → extra 真/假/删键。大表单 merge extra，不得抹该键。不要出现在 Pool Mode 区。
10. **i18n**：`frontend/src/i18n/locales/zh.ts` 与 `en.ts` 同时加 design §5.7 全部键。
11. **回归锁**：OAuth 401 + invalidate（时长仍 `oauth_401_cooldown_minutes`）；`pool_mode` 早退 / 同账号重试 / 硬驱逐；Fable / image 模型范围；pair / quality 不进 `IsSchedulable`；sticky / `previous_response` 不被 layer-2 拆掉。
12. **文档**：`account.md`、`gateway.md` 调度节、`CHANGELOG_CUSTOM.md`（`git add -f`）。需要时再加 `.trellis/spec/backend/` 短规约（含 Settings 七段合约）。

## 文件提示（实现时）

| 层 | 文件 |
|---|---|
| KV key | `backend/internal/service/domain_constants.go` |
| 结构 / Default | `backend/internal/service/settings_view.go` |
| Get/Set | `backend/internal/service/setting_service.go` |
| 谓词 / 分类 | `backend/internal/service` 新小文件或 `ratelimit_service.go` 旁 |
| 热路径 | `backend/internal/service/ratelimit_service.go` |
| 调度排除 | `gateway_service.go` / openai 选号（只挂钩，隔离脏 hunk） |
| Redis | 现有 `repository/*_cache.go` 风格，键名见 design |
| handler / DTO / 路由 | `setting_handler.go`、`dto/settings.go`、`routes/admin.go` |
| FE API / 页 / 账号 / 测 | `frontend/src/api/admin/settings.ts`、`SettingsView.vue`、`SettingsView.spec.ts`、`EditAccountModal.vue` |
| i18n | `frontend/src/i18n/locales/{zh,en}.ts` |

## i18n 键清单（实现时必须 zh+en）

```
admin.settings.oauthFleetSoft429.title
admin.settings.oauthFleetSoft429.description
admin.settings.oauthFleetSoft429.enabled
admin.settings.oauthFleetSoft429.enabledHint
admin.settings.oauthFleetSoft429.ttlSeconds
admin.settings.oauthFleetSoft429.ttlSecondsHint
admin.settings.oauthFleetSoft429.longResetPolicy
admin.settings.oauthFleetSoft429.longResetSoft
admin.settings.oauthFleetSoft429.longResetHard
admin.settings.oauthFleetSoft429.longResetThreshold
admin.settings.oauthFleetSoft429.longResetThresholdSeconds
admin.settings.oauthFleetSoft429.longResetThresholdHint
admin.settings.oauthFleetSoft429.scope
admin.settings.oauthFleetSoft429.scopeAllOAuth
admin.settings.oauthFleetSoft429.scopeOptIn
admin.settings.oauthFleetSoft429.platforms
admin.settings.oauthFleetSoft429.includeSetupToken
admin.settings.oauthFleetSoft429.includeSetupTokenHint
admin.settings.oauthFleetSoft429.softBodyCodes
admin.settings.oauthFleetSoft429.hardBodyCodes
admin.settings.oauthFleetSoft429.codesHint
admin.settings.oauthFleetSoft429.invariantHint
admin.settings.oauthFleetSoft429.saved
admin.settings.oauthFleetSoft429.saveFailed
admin.accounts.oauthFleetSoft429.label
admin.accounts.oauthFleetSoft429.hint
admin.accounts.oauthFleetSoft429.inherit
admin.accounts.oauthFleetSoft429.forceOn
admin.accounts.oauthFleetSoft429.forceOff
```

## 校验命令（实现阶段）

在 `backend/`：

```powershell
go test -tags=unit ./internal/service -run "OAuthFleetSoft429|HandleUpstreamError|handle429|PoolMode|OAuth401|AnthropicWindow|CodexRateLimit|GetPublicSettings" -count=1
go test -tags=unit ./internal/handler -run "Failover|failedAccount|PoolMode|OAuthFleetSoft429" -count=1
```

在 `frontend/`：

```powershell
pnpm --dir frontend exec vitest run src/views/admin/__tests__/SettingsView.spec.ts -t "oauth fleet"
```

若 Redis 排除落在 repository，再加对应 `*_cache*` 包的单测。不要用「客户端改 stream」当验收。

## 测试必须红的形状

- OAuth `rate_limit_exceeded` / 无头 429：`SetRateLimited` / `SetTempUnschedulable` **0 次**；Redis SET 1 次且 TTL=settings；本请求排除该 id。
- Codex `used_percent < 100` + 很长 reset 头：`long_reset_policy=soft` → 仍 soft；`=hard` → `SetRateLimited`；`=threshold` 且时长 ≥ 阈值 → hard。
- Anthropic 5h/7d rejected、Codex 100%、`usage_limit_reached`：仍 `SetRateLimited`；把这些码写进 `soft_body_codes` 也不能变软。
- 账号规则 429 + keyword `rate limit`：OAuth 软路径 **不**写 `temp_unschedulable_until`。
- OAuth 401：仍 temp unsched + invalidate（分钟数 = 现有 oauth_401 配置）。
- `IsPoolMode()` API Key：仍早退、不写限流；`RetryableOnSameAccount` 仍 true（401/403/429）。
- `IsSchedulable()` 在软 429 后仍 true；短 TTL 内无硬亲和选号排除该 id；TTL 后可再选。
- sticky / `previous_response`：layer-2 不拆钉。
- 解析序：extra `false` → 现网 `handle429`；extra `true` + 全局 `enabled=false`（含空 KV）→ 仍 soft 路径；缺省 + 全局关（含空 KV）→ 现网 `handle429`；缺省 + `scope=opt_in` 且平台不在列表 → 现网；setup-token + `include_setup_token=false` + 缺省 → 现网。
- extra 覆盖在模拟 OAuth refresh / `UpdateCredentials` 之后仍在。
- Get 空 KV / 坏 JSON → Default\* 且 **`enabled=false`**（钉死：不得等于 529 的 Default\* `Enabled: true`）；Set TTL=4 或 301 且 enabled → 错；Set `soft_status_codes` 含 529 → 错；Set 未知平台 → 错。
- `GetPublicSettings` JSON 不含 `oauth_fleet` / 本 key。
- Settings 页级 save 不调用 update OAuth-fleet API；卡片 save 只打该 API。

## 评审门

- [ ] Brandon 已评 `prd.md` / `design.md` / `implement.md`（本轮结束条件）。
- [ ] 未跑 `task.py start`（实现批准另开）。
- [ ] `implement.jsonl` / `check.jsonl` 已有真实 spec/research 行（非仅 `_example`）。
- [ ] 实现时不碰展示计费 / `actual_cost` / pair→`IsSchedulable` / `IsPoolMode` 扩面。
- [ ] 实现 diff 不含无关 sticky / unpooled。
- [ ] 配置面按 design 落地：KV JSON + 网关卡片 + extra 三态；Default\* `enabled=false`（空 KV=关）；TTL/政策/范围不藏在热路径魔法数里。

## 回滚点

分类 helper + `HandleUpstreamError` 挂钩 + Redis 过滤 + Settings 卡片 / Get-Set。Revert 即回现网。Redis 键按当时 TTL 过期（≤300s）。无 migration 可回滚。KV 行可留。

## 碰撞

| 文件 / 主题 | 怎么处理 |
|---|---|
| 已脏的 gateway / scheduler / sticky / unpooled | 不并进本任务；软排除挂钩单独 hunk |
| `pool-mode-hard-eviction` spec | 只读；不改 `IsPoolMode()` |
| `account-user-schedule` spec | pair / quality 仍在 admission |
| display-token / billing | 不改 |
| 529 / 流式超时 / quality hard-close | v1 不做；只抄它们的 Settings 卡片模式 |
| OAuth refresh 凭据整表写 | 覆盖只放 extra |
