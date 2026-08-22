# Design: AG 池容纳 OpenAI 账号 + 桥接 lookup + 两池独立

决策 A–G 已拍板（2026-08-22）。Brandon：按推荐 + **openai / AG 池完全独立**（覆盖下文已删除的「Redis 仍按 (account,user) 共享」拟议）。

## Boundaries

| 在范围内 | 不在范围内 |
|---|---|
| `sanitizePoolMembers` AG tab 例外 | 改 `ResolveClaudeGPTBridgeModel` |
| 前端 AG 候选 + 桥接列 | 空流 converter / failover（F=否） |
| 热路径 lookup 平台（准入 + pair + unpooled + Observe） | 计费 / display / `actual_cost` |
| migration 211 + Ent PK + pause 按 platform | 整仓改 `account.Platform` |
| Redis 占用/冷却/probe/pin/pair-quality 按 platform 分片 | 独立风险审查 |

## Current data flow

```text
Admin AG tab add OpenAI
  -> PUT /admin/users/:id/smart-schedule/antigravity
  -> sanitizePoolMembers: acc.Platform != antigravity
  -> 400 SMART_SCHEDULE_PLATFORM_MISMATCH

Group 15 /v1/messages (AG group, Claude-GPT)
  -> ClaudeGPTBridgeRoute ready
  -> SelectAccountWithSchedulerForClaudeGPTBridge
       platform := openai  (no override)
  -> candidates: group-bound OpenAI + ResolveClaudeGPTBridgeModel
  -> admitsScheduleUser -> lookupEnabledSmartPolicy(user, account.Platform=openai)
  -> user 12: openai closed pool (shared with group 19)
  -> pair slot / cheaper-tier also keyed by openai

Group 19 /v1/responses (OpenAI group)
  -> same openai policy + same pair HASH
```

冲突：产品要把桥接放进 **AG 池**，代码却用 **account.Platform** 当策略 key。本任务显式改 lookup key，而不是假装 tab 平台等于账号平台。

## Proposed data flow

```text
Admin AG tab add OpenAI (bridge on or off)
  -> sanitize: platform==antigravity && acc.IsOpenAI() => allow
  -> row.platform='antigravity', account_id=OpenAI id
  -> PK (user_id, platform, account_id); openai 行保持不动

Group 15 bridge
  -> 选号平台仍是 openai（候选必须是 OpenAI 号 + ResolveClaudeGPTBridgeModel）
  -> 智能调度 lookup：AG 开且有成员 → antigravity；否则 openai
  -> 闭池命中 => 忽略账号侧 allow/deny/gate/cap
  -> Redis 占用/冷却/probe/pin/quality 用同一 lookup platform
  -> 桥关 => Resolve 失败，选不中（R3）

Group 15 native AG (not_configured only)
  -> forcePlatform=antigravity, 候选是 AG 号
  -> lookup antigravity
  -> 池里的 OpenAI 号因平台过滤不会进原生 AG 选号

Group 19 native GPT
  -> lookup openai；Redis 用 platform=openai
```

## Lookup（C，已锁定）

单一 helper，避免漏改：

```text
smartScheduleLookupPlatform(account, request, bundle) string
  若 request.RequireClaudeGPTBridge || request.GroupPlatform==antigravity 且 account.IsOpenAI():
      若 bundle.EnabledPolicy(antigravity) != nil:
          return antigravity
      return openai
  否则:
      return account.Platform
```

请求侧 hint 从 context 读：`ctxkey.Group.Platform` / `ctxkey.ForcePlatform` / `ctxkey.RequireClaudeGPTBridge`（桥接选号入口写入）。

必须接到：

- `admitsScheduleUser`
- `resolvePairSlotAcquire` / `lookupEnabledSmartPolicy` 在 `account_user_concurrency.go`
- `shouldEscapeSessionStickyForCheaperTier` / `isUnpooledScheduleUser`
- `ObservePairCompletion`：按当次请求的 lookup 平台选策略，禁止 map 遍历第一个 `HasAccount`

`SelectAccountWithSchedulerForClaudeGPTBridge` **不要** 把 scheduler `platform` 改成 antigravity：`isOpenAIAccountEligibleForScheduleRequest` 要求 `account.Platform == platform`，改了会选不中任何号。变的是智能调度策略 key，不是选号平台。

### C 未启用 AG 策略时（Brandon 2026-08-22 改判）

| 选项 | 分组 15 行为 | 分组 19 |
|---|---|---|
| 继续查 openai（锁定） | 与今天生产路径相同；关开关 = no-op | 不变 |

AG 关 / 缺失 / 空池 **不是** fail-open 到账号侧。只有 AG `EnabledPolicy != nil` 时才切到 antigravity，且此后禁止回落 openai。独立性只在 AG 开着时成立。

## 主键 / 已在 openai 池（A + B，已锁定）

`(user_id, platform, account_id)` + 共存：

- 同一 OAI 号可同时在 openai 池（分组 19）和 AG 池（分组 15）。
- 加入 AG **不** 从 openai 删行。
- Migration **211**（main max=210）：
  - `ALTER TABLE user_smart_schedule_accounts DROP CONSTRAINT user_smart_schedule_accounts_pkey`
  - `ADD PRIMARY KEY (user_id, platform, account_id)`
  - 保留 `(account_id)` 索引（账号删除仍要扫成员）
  - Ent `field.ID` 改为 `user_id, platform, account_id`
  - `SetMemberPaused` / `ApplyMemberPaused` / `SetPairAdmission` WHERE 加上 `platform`
- 存量行：`platform` 已有，只需换主键，无需 backfill 账号。
- Copy-from-platform 仍只拷策略不拷成员。
- 账号删除仍删该 `account_id` 的**全部**成员行，并在任一 `(user, platform)` 池空时 disable。

## Redis 完全独立（覆盖原共享拟议）

**必须实现，不能只写注释。** 今天 Redis 是 `(account, user)`：

```text
smart-schedule:cooldown:{accountID}          HASH field u:{userID}
smart-schedule:probe:{accountID}             HASH field u:{userID}
smart-schedule:pinned:{accountID}            HASH field u:{userID}
smart-schedule:pair-quality:{accountID}      HASH field u:{userID}
smart-schedule:pair-quality-trend:{accountID}:{userID}
smart-schedule:pair-quality-events:{accountID}:{userID}
concurrency:account_user:{accountID}:{userID}
```

双池若共用这些 key，AG 冷却的 HASH TTL 会切到 openai 同号剩余冷却（已有「勿缩短兄弟 field TTL」坑）。因此 **platform 进 KEY**，field 仍 `u:{userID}`：

```text
smart-schedule:cooldown:{platform}:{accountID}          HASH u:{userID}
smart-schedule:probe:{platform}:{accountID}             HASH u:{userID}
smart-schedule:pinned:{platform}:{accountID}            HASH u:{userID}
smart-schedule:pair-quality:{platform}:{accountID}      HASH u:{userID}
smart-schedule:pair-quality-trend:{platform}:{accountID}:{userID}
smart-schedule:pair-quality-events:{platform}:{accountID}:{userID}
concurrency:account_user:{accountID}:{userID}:{platform}
```

`smart-schedule:user:{userID}` 用户策略缓存不变（JSON 已按 platform 分政策）。

`account-quality:resume:{accountID}` 仍是账号质量轨 `(account, user)`，本任务不改字段形。豁免期 overlay 跨池共享记为残余风险。

规则：

- 所有读/写/clear/hydrate/delete 走新 key。禁止读旧 key 回落混用。
- pair cap：桥接用 AG 成员 cap + AG 占用 zset；原生 GPT 用 openai 成员 cap + openai 占用 zset。两套计数互不影响。
- `ObservePairCompletion.Platform` 必须是请求 lookup 平台。空且双池命中多于一条 → **不 ingest**（禁止第一个 HasAccount）。
- Admin 池页 `SetPairAdmission` / resume 必须带 tab `platform`。账号页省略 platform 时只操作 `account.Platform` 那一行，不波及 AG 双池行。
- 账号删除：对该 (account,user) **所有 platform** 清 cooldown/probe/pin/pair-quality。

发版后旧 Redis key 成为孤儿，靠 TTL 消失。在飞占用/冷却会重置——残余风险，不双读旧 key。

As-built（check 后补齐，未改产品决策）：智能调度豁免期从共享 `account-quality:resume` 拆到 `smart-schedule:resume:{platform}:{accountID}`。池页 pair-quality batch/detail 带 tab platform。账号页 resume 省略 platform 时只动 `account.Platform` 那一行。

## 池列（D，已锁定）

只读：

- 列 key：`claude_gpt_bridge`
- 数据：`account.extra.openai_claude_gpt_bridge_enabled === true`
- AG tab 显示；非 OpenAI 号画 `—`
- 不写 PUT body，不进 `user_smart_schedule_accounts`

## 候选列表（E，已锁定）

AG tab `loadCandidates`：

1. 现有 `platform=antigravity&lite=1` 分页
2. 再走 `platform=openai&lite=1` 分页
3. 合并去重；`addableAccounts` 排除已在**当前 AG 草稿**里的 id

筛选添加：AG tab 可在 `antigravity | openai | 全部（本 tab）` 间筛。其它 tab 仍锁。不要在 AG tab 拉 anthropic/gemini/grok。

## Compatibility

- 无 AG 池 / 开关关：分组 15 继续吃该用户的 openai 闭池（与今天一致）。关开关上线是 no-op。
- AG 开且有成员：分组 15 只吃 AG 闭池；池未命中即拒，不回落 openai。
- 只改 sanitize、不改 lookup：能写入但不生效。Lookup 必须同任务交付。
- 原生 AG / 其它平台 tab：不变。原生 OpenAI 分组始终 `account.Platform=openai`。
- 桥接预检不读智能调度池。

## Rollback

1. 无 migration：回滚 sanitize 例外 + lookup helper + Redis key 形 + 前端。
2. 有 211：准备 down（恢复 `(account_id, user_id)`）。已有双池行时 down 前删每个 (account,user) 的重复 platform 行（默认保留 `platform=account.Platform`）。
3. 不回滚桥接资格函数。
