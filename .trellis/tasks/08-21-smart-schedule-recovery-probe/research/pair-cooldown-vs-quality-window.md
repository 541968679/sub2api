# 调研：账号质量窗口 vs 账号–用户冷却判定

调研日期：2026-08-21。只读代码与规格，不改实现。

## 三件分开说（当前值）

| 项 | 当前 | 含义 |
| --- | --- | --- |
| 计算窗口 | 15 分钟 | 每次算出的数，用最近 15 分钟已完成请求 |
| 计算间隔 | 5 分钟 | 多久重算一次并写入 Redis |
| 冷却判断间隔 | 每条请求 | 选号碰到该账号就拿**当前这份数**比门槛；冷却中/宽限中会短路，不比 |

第一刀只改计算间隔：窗口仍 15 分钟，判断仍每条，计算改为每完成一条。

---

## 15 分钟不是「必须攒满 15 分钟」

「账号 15m live」= 回看最近 15 分钟里，**这个账号上所有用户已经完成**的请求，聚成一份 p50 / 成功率。

不是：

- 必须等满 15 分钟才能冷却
- 必须有连续 15 分钟的样本
- 只统计当前这个用户

能冷却的条件是：**这份回看结果里，已完成样本数够门槛**（默认 TTFT≥10 或 成功+失败≥20），并且指标越界。2 分钟内打完 20 个也可以判；15 分钟里只有 3 个就不判。

「套这个用户的平台门槛」= 用该用户智能调度里填的 p50 / 成功率 / 样本下限 / or·and，去比上面那份 **账号** 数字。门槛是用户的，数字是账号的。

---

## 结论

质量窗口的**计算**和账号–用户冷却的**判定**是两套逻辑，只在热路径上拼在一起。

- 窗口：按 **账号**（或展示用的 **用户**）聚合最近 15 分钟已完成日志。没有 `(account_id, user_id)` 配对窗口。
- 判定：用这对用户自己的门槛 / 宽限 / 冷却 HASH，去读上面那份 **账号窗口**。
- 配对冷却本身是墙钟锁（`now + cooldown_minutes`），不是滚动质量窗口。一旦写入，窗口变好也不会提前解开。

「判断样本数」约束的是**已完成且已进窗口的行数**，不约束**正在飞的并发**。冷却前已经准入的请求可以远多于样本门槛。

## 1. 质量窗口怎么算

| 项 | 值 |
| --- | --- |
| 长度 | `AccountQualityWindow = 15m`（`account_quality.go` L12–16） |
| 刷新 | 维护 `RunTick` 每 `5m`（L21–22, `account_quality_maintenance.go` L136） |
| 候选 | `ListRecentTrafficAccountIDs`：15 分钟内 `usage_logs` ∪ `ops_error_logs` 出现过的 `account_id` |
| SQL | `usage_logs` / `ops_error_logs` 的 `created_at >= now-15m`，`GROUP BY account_id`（`usage_log_repo.go` L2378–2505） |
| 热路径存储 | `account-quality:live:{accountID}` JSON，TTL 20m。只 `Replace`，选号只 `Get` |
| 计入 | 已落库的成功 usage + 调度错误。进行中的请求不算 |
| 口径 | 默认 terminal（客户端 ≥400）；`schedule_use_failover_error_rate` 才把 Recovered hop 算进门槛 |

另有一条 **用户维** 15 分钟：`GROUP BY user_id`，给用户列表 / 智能调度页头。`user_id IS NULL` 的错误丢掉。这条 **不进入** 选号或配对冷却。

**不存在** `GROUP BY (account_id, user_id)` 的调度窗口。

管理端池内质量格走 `POST /admin/accounts/quality-stats/batch`，现查 SQL 15m + 30s 进程缓存，**不是** 热路径那份 5 分钟 Redis。芯片用的 `resume_users` 在 batch 里本来没有，靠切换后的本地补丁。

## 2. 账号–用户判定完整链

入口：所有选号 / sticky 清针走 `admitsScheduleUser`（`account_user_schedule.go` L230–264）。

```text
取 ctx userID
  │
  ├─ 智能调度 EnabledPolicy(platform) 非空
  │     不在池 → 拒绝
  │     paused → 拒绝（不写冷却）
  │     CooldownActive(account, user, now) → 拒绝
  │     未配 p50/成功率 → 放行
  │     UserQualityResumeActive(u: 或 w: 未过期) → 放行（不写冷却）
  │     EvaluateAccountQualityHardClose(账号 live, 平台门槛)
  │        欠样本 / nil stats / 未越界 → 放行
  │        越界 → StartCooldown HSETNX → 拒绝
  │
  └─ 未开智能调度（旧 account_schedule_users）
        allow/deny
        同一套账号 live + 该用户门槛
        越界 → 只排除这对，不写配对冷却，下次请求重评
        窗口变好 → 立刻可再选
```

`EvaluateAccountQualityHardClose`（`account_quality_hard_close.go` L329–371）：

- 未配的指标不判。
- TTFT：`TTFTSamples >= min`（默认 10）且 p50 存在才判。
- 成功率：`success+error >= min`（默认 20）才判。
- 一个都判不了 → 不拦截。
- `or` / `and` 只在已判决的指标上。

配对冷却 Redis（`user_smart_schedule_cache.go` L74–106）：

- key `smart-schedule:cooldown:{accountID}` HASH field `u:{userID}=untilUnix`
- 热路径 `HSETNX`，不续期
- `CooldownActive`：`until > now`；过期会 `HDEL` 后返回 false
- 到期后没有「待考察」标记，下一请求按可调度走

宽限 overlay（`account-quality:resume:{accountID}`）：

| 动作 | 字段 | 效果 |
| --- | --- | --- |
| `resumed` / 立即恢复 | `u:=now+15m`, `w:=now+30m` | 最长 30 分钟门槛 fail-open |
| `selectable` | 删 `u:`, `w:=now+15m` | 15 分钟 fail-open |
| `paused` / `cooling` | 删 `u:` `w:` | 无宽限 |

这是墙钟豁免，不是新的采样窗口。宽限期满后读的仍是账号 15m live。

## 3. 时间窗口分别管什么

| 窗口 | 作用对象 | 用途 |
| --- | --- | --- |
| 15m 滚动（账号） | 全账号已完成请求 | 要不要 **开始** 冷却 / 旧门槛排除 |
| 5m tick | Redis live | 热路径最早何时看见新样本 |
| 15m `u:` + 最长 30m `w:` | 这对 user×account | 已恢复期间不判门槛 |
| `cooldown_minutes` 墙钟 | 这对 user×account | 已经冷却后的锁。与质量窗口脱钩 |
| 用户维 15m | 该用户所有账号 | 仅展示 |

账号硬关闭是第三条：同一份账号窗口，tick 里 `SetTempUnschedulable` 整号，不走配对冷却。

## 4. 为什么样本门槛挡不住打满

1. 选号看的是 **上一拍 Redis**，不是当前 in-flight。
2. 窗口只含 **已完成** 日志。并发打出去的请求要等结束才可能进 SQL，再等最多 5 分钟进 live。
3. 欠样本 fail-open：冷却期没流量 → 窗口空 → 到期后继续放行，直到凑齐 10/20。
4. 门槛齐样之前，配对 cap（或无限）已经决定能同时进多少。样本=10、cap=50 时，冷却触发前 in-flight 可以是 50，不是 10。
5. `resumed` 宽限里即使窗口已越界也不写冷却。

## 5. 对后续设计的含义（只记事实，不定案）

- 继续用账号 15m 做考察升/降：会吃到别人的流量，冷却清空后仍欠样本 fail-open，且默认仍等 tick。
- 配对冷却「有没有时间窗口」：锁本身没有；窗口只出现在 **触发** 那一步。
- 夹紧考察 cap 才能从根上限制「冷却前已经飞出去的请求数」。只改样本数或 tick 不够。
