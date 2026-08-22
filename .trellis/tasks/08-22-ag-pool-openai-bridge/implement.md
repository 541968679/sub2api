# Implement: AG 池允许 OpenAI 账号

Review gate 已满足（2026-08-22）：Brandon 按推荐锁定 A–G + 两池完全独立。可 `task.py start`。

占用 checkout 保持 `main`。只改本任务文件，不 revert / 不覆盖无关 dirty 文件。Phase 3 wrap 已获 Brandon 批准：可 commit 本任务文件。不 push / 不 deploy。

## 前置

1. A–G 已确认。F=否：不要打开 `openai_gateway_messages.go` 空完成路径。
2. 当前 `main` migration max = **210** → `211_user_smart_schedule_account_pk.sql`。

## Ordered checklist

### 0. 测试钉

- [x] `sanitizePoolMembers`：OpenAI → AG tab，无桥开关，允许。
- [x] OpenAI → anthropic tab 仍 `SMART_SCHEDULE_PLATFORM_MISMATCH`。
- [x] AG 号 → AG tab 仍允许。
- [x] 桥关 OpenAI 号：`ResolveClaudeGPTBridgeModel` 仍 false。
- [x] lookup helper：AG 关/空/缺失时桥接/AG 分组 + OAI → openai；AG 开且有成员 → antigravity；原生 GPT → openai。
- [x] AG 策略 disabled / 空池 / 缺失：继续走 openai 闭池，不 fail-open 账号侧。
- [x] 双池 persist：同一 account 两行。
- [x] 冷却/占用跨 platform 隔离。

### 1. 写路径（R1）

- [x] `sanitizePoolMembers`：`platform==antigravity && acc.IsOpenAI()` 放行；其它跨平台仍拒。
- [x] 成员行 `platform` 仍是 **tab**（antigravity）。
- [x] B=共存：加入 AG 不删 openai 行。
- [x] `SetMemberPaused` / sort-order / resume / `ApplyMemberPaused` WHERE 带 platform。

### 2. 主键（A）

- [x] `backend/migrations/211_user_smart_schedule_account_pk.sql`：drop `(account_id, user_id)`，add `(user_id, platform, account_id)`。idempotent。
- [x] Ent schema `user_smart_schedule_account.go`（`go generate ./ent` 仍为已知残余，本任务不强制重跑）。
- [x] Repo `ReplacePlatform` 已按 platform 删插；同一 `account_id` 可多行。
- [x] 账号删除仍删该 `account_id` 的全部成员行，并在任一 (user, platform) 池空时 disable。

### 3. 热路径 lookup（R4 / C）

- [x] `SmartScheduleLookupPlatform`：读 `EnabledPolicy(antigravity)`；AG 未激活则 OAI+桥接/AG 分组 lookup 仍是 openai。
- [x] 接到 `admitsScheduleUser`、`resolvePairSlotAcquire`、unpooled cheaper-tier、Observe、hydrate/stamp。
- [x] `ObservePairCompletion` 按请求 lookup 平台选策略（与准入同一规则）。
- [x] **不要** 把 `SelectAccountWithSchedulerForClaudeGPTBridge` 的 scheduler platform 改成 antigravity。
- [x] 测试：AG 关走 openai 闭池；AG 开只走 AG；原生 GPT 只吃 openai；pair/cooldown/resume 同分片。

### 4. Redis 完全独立

- [x] cooldown / probe / pin / pair-quality / occupancy key 带 `{platform}`。
- [x] 全部读/写/clear/hydrate/delete + 测试更新。
- [x] 禁止读旧 key 回落。
- [x] Admin hydrate 按 tab platform 分别取数（同一 account 两池可不同冷却）。
- [x] 池页 resume 传 tab platform。账号删除清所有 platform 的 pair 状态。

### 5. 前端（R2 / E / R9）

- [x] `loadCandidates`：AG tab 合并 antigravity + openai lite 列表。
- [x] 筛选添加：AG tab 可在 antigravity/openai 间筛；其它 tab 仍锁。
- [x] `allPoolColumns` 增 `claude_gpt_bridge`；`DEFAULT_COLUMN_WIDTHS` + 布局 merge。
- [x] 单元格只读：开/关/非 OAI 为 `—`。
- [x] typeahead 显示平台 + 桥状态。
- [x] `zh.ts` + `en.ts` 同步。
- [x] resume API 带 `platform`。
- [x] specs：AG tab 能加 OAI；列可见。

### 6. 文档

- [x] `docs/dev/codebase/account.md`：AG 池可含 OpenAI；lookup 例外；PK；Redis key 形。
- [x] `docs/dev/codebase/gateway.md`：桥接准入用 AG 策略。
- [x] `.trellis/spec/backend/account-user-schedule.md`：AG 例外 + Selection key + Redis platform 分片。
- [x] `docs/dev/CHANGELOG_CUSTOM.md`。

### 7. 不做

- [x] 不改空流 / `newOpenAIEmptyVisibleOutputError`。
- [x] 不改 billing / display。
- [x] 不做独立风险审查。
- [x] 不 push / deploy（commit 仅本任务，Brandon 2026-08-22 批准）。

## Validation commands

```powershell
cd backend
go test -tags=unit ./internal/service -count=1 -run "SmartSchedule|sanitizePool|ClaudeGPTBridge|Unpooled|admitsScheduleUser|lookupEnabledSmartPolicy|LookupPlatform"
go test -tags=unit ./internal/repository -count=1 -run "SmartSchedule|PairQuality|Cooldown"

pnpm --dir frontend exec vitest run src/views/admin/__tests__/UserSmartScheduleView.spec.ts src/composables/__tests__/useUserSmartScheduleEditor.spec.ts src/composables/__tests__/useSmartSchedulePoolColumnLayout.spec.ts
```

不要为验证跑 `scripts/dev-stack.ps1`。

## Risky files

| 文件 | 风险 |
|---|---|
| `user_smart_schedule_service.go` sanitize | 例外写太宽会让任意平台进 AG |
| `account_user_schedule.go` / concurrency / unpooled | 只改一处会混用两套策略 |
| Redis cache keys | 漏改一条读写会串池 |
| `openai_account_scheduler.go` | 误改 scheduler platform → 桥接 0 候选 |
| migration 211 + Ent PK | PK / Ent 不一致 |
| `useUserSmartScheduleEditor.ts` | 双平台拉候选漏页或污染其它 tab |

## Rollback points

1. 仅前端：回滚候选合并与列。
2. 仅 sanitize：已有 OAI 行仍在 DB。
3. Lookup helper：回滚后桥接再吃 openai 池。
4. Redis key：回滚后两池再共享。
5. Migration 211：有双池行时 down 会失败，先删重复。
