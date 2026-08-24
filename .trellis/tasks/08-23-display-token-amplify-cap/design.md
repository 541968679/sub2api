# Design：input+cache 联合展示 cap

## 白话

现有放大先跑完（L1 展示价 + M/α，再 L2 展示倍率）。然后：

```text
S = display_input + display_cache
C = jitter(configured_joint_cap, request_id)    # 92%–100% × 配置值

S ≤ C  →  input/cache 不动
S > C  →  按封顶前比例缩小，使 in' + cache' = C
          每个分量成本 = 新 token × 该分量展示单价
          少掉的钱丢掉，不折进 output/input/cache

output 另有独立 cap + 自己的 jitter（不进合计）
新请求 actual_cost = (in'×P_in + cache'×P_cache + out'×P_out + other) × display_rate
旧行不套这套 cap
```

代码默认两个 cap 都是 0（关）。推荐运营值：联合 **1_000_000**，output **80_000**。

## 研究结论（user_id=16，展示层，7 天）

方法、直方图、近似误差见 `research/display-token-distribution.md`。

| 量 | p50 | p90 | p95 | p99 | max | >1M |
|---|---:|---:|---:|---:|---:|---:|
| display input | 30 621 | 218 354 | 322 339 | 537 433 | 2 874 142 | — |
| display cache | 5 376 | 107 162 | 136 909 | 184 218 | 1 918 265 | — |
| **joint S** | **58 677** | **235 707** | **335 990** | **546 637** | **2 882 851** | **431 (0.13%)** |
| display output | 2 145 | 8 151 | 9 971 | 30 881 | 204 965 | — |

S>800k：916 行（0.28%）。1M 在 p99 之上约 1.83×，只切极端尾，同时对齐「GPT 上下文 ~1M」。**不是没看分布就写的营销整数。**

## 1. Joint budget

在 **完整 L1+L2 之后**（含现有 M：`display_cache ≤ billing_real_cache × M_eff`）：

1. `S = display_input + display_cache`（整数 token）。
2. `C = JitteredSumCap(configured, seed)`。`configured≤0` → 跳过联合 cap。
3. `S ≤ C`：两分量原样返回。
4. `S > C`：按 §2 分配，满足 `in' + cache' = C`，且 `0 ≤ in' ≤ in`、`0 ≤ cache' ≤ cache`。

禁止：先各自 jitter 再相加（合计可以再次 >C）。

## 2. 合计必须缩小的分配策略

### 主规则：按封顶前比例缩小（proportional）

```text
in'    = round(in * C / S)
cache' = C - in'          # 用减法保证合计恰好 C
if cache' > cache:        # 取整偶发越界
  cache' = cache
  in'    = C - cache'
if in' > in:
  in'    = in
  cache' = C - in'
# 仍保证 in'+cache'=C 且都不高于封顶前
```

一边为 0 时：另一边 `min(自身, C)`，余量无法填的部分就是「丢掉的 token」（本来另一边就是 0）。

**为什么像真的 GPT 窗口**

同一请求的 input:cache 形状保留，只是被塞进更小窗口。用户仍看见「有前缀 cache + 有新 input」，比例与放大后一致。

**扣费副作用**

`Δcost = (in-in')×P_in + (cache-cache')×P_cache`（再乘展示倍率）。比例切 token 时，**贵的分量少得更多钱**。user 16 的 input 展示价约为 cache 的 10×（sol 4e-6 vs 4e-7），所以同样少 1 token，input 少扣约 10 倍。这是「保形状」的代价，不按价优化。

分量仍可解释：`in' × P_in`、`cache' × P_cache`。单价不改。

### 否决的备选

| 策略 | 否决原因 |
|---|---|
| **保 cache、砍 input** | cache 可吃完整预算 → input=0，像空回合 + 巨大前缀，不像一次真实 completion。 |
| **保 input、砍 cache** | 明显有 cache 的请求会变成 cache=0，破坏 prefix-cache 观感。 |
| **Water-fill：先砍较大分量** | 尾部行 cache 或 input 单边很大时，会把大的一方削到只剩「C−小的」，比例跳变；两端都大时不像「同一段对话被窗口卡住」。 |
| **按价切（price-aware）** | 优化少扣/少露，不是窗口故事。改展示价就会改 input:cache 比，用户能从 usage 反推策略。 |

不在主规则里做 price-aware 微调。

## 3. Jitter（只打在合计 cap）

```text
seed = request_id if nonempty else fmt.Sprint(usage_log_id)
span = floor(configured * 0.08)          # 8% → 92%–100%
off  = hash64(seed + "|joint") % (span+1)
C    = configured - off                  # [0.92C, C]
```

- 1M 配置 → C ∈ [920000, 1000000]。user 16 的 p99=547k，常态行碰不到这条带。
- 同一 seed 恒等；不同请求在带内散开，避免张张都是 1000000。
- **先**得到 C，**再**比例分配。禁止对 in/cache 二次 jitter。
- Output 用另一条 lane：`hash64(seed + "|output")`，同样 92%–100%，互不影响。
- Jitter 比例固定在代码里（0.08），不当第三个运营旋钮。

## 4. Output cap（独立）

- `configured_out ≤ 0`：output 不动。
- 否则 `out' = min(out, JitteredCap(configured_out, seed, "output"))`。
- 成本 `out' × P_out`。不因 output 被砍而把差额折进 input/cache。
- 与联合预算无加减关系。

## 5. 扣费（仅新请求写路径）

封顶与重算展示分量成本之后：

```text
display_total' = in'×P_in + cache'×P_cache + out'×P_out
                 + cache_creation_display + image/other
actual_cost_new = display_total' × display_rate
```

`display_rate` 即用户看见的倍率（L2 之后的 `RateMultiplier`）。无展示价的分量保持 L1+L2 后的成本（other）。

不变量：

- 任一 cap 绑定 ⇒ `actual_cost_new ≤ actual_cost_uncapped`（取整除外应严格 <）。
- 写路径用 `actual_cost_new` 扣余额 / 订阅 / API Key quota。
- 落库：**计费实数 token 列不变**；`actual_cost` 写成新低值；见 §历史。

## 6. Admin Settings

| key | 含义 | 默认 |
|---|---|---|
| `display_context_token_max` | 展示层 **input+cache 合计** 上限（token）。0/空=关 | 0 |
| `display_output_token_max` | 展示层 output 上限。0/空=关 | 0 |

不要做成三个互相独立的 input/cache/output 绝对 cap。放在现有「展示层」M/α 旁边。校验：整数 ≥0；过大可夹到合理上限（如 1e9）防溢出。

推荐运营填法（**不是**代码默认）：联合 `1000000`，output `80000`。

i18n zh+en。加法合并 Settings，不覆盖 `openai_long_context_billing_enabled`。

## 7. 历史冻结

读路径若对 **所有** 行套当前 cap，旧行会显示更低账，违反「已经扣过的不能动」。

写路径改 `actual_cost` 后，读路径若不再套 cap，新行会显示未封顶 token，恒等式破裂。

因此新行必须能被认出，并按 **当时** 的 cap 重放（设置日后改了也不改旧新行的展示）。

最小落库（migration = 当时 `main` max+1，additive）：

| 列 | 旧行 | 新行（cap 关） | 新行（cap 开并绑定或已应用） |
|---|---|---|---|
| `display_token_cap_applied` | false | false | true |
| `display_context_token_max_used` | 0 | 0 | 当时联合 cap（配置值，jitter 用 request_id 重放） |
| `display_output_token_max_used` | 0 | 0 | 当时 output cap |

读路径：

- `applied=false`：只跑今天的 L1+L2（M/α），**不**套本绝对 cap。
- `applied=true`：L1+L2 后用 **行内 used 值** + `request_id` 重放 jitter/分配。

计费实数 token、旧 `actual_cost` 不 UPDATE。无历史 backfill。

## 8. 与 M/α 的边界

1. L1 `AllocateDisplayTokens`（M + α 残差）—— 不变。
2. L2 `ApplyUserDisplayRateWithCap`（倍率；cache 仍 ≤ real×M）—— 不变。
3. **本 cap**：只看第 2 步之后的展示 token；砍掉的成本 **不** 再进 α 残差池。

下游 `computeSeparatedDisplayUsage` 在 rate-scale 之后走同一 helper（新请求）。

## 9. 写路径挂钩

在 `CalculateCostUnified` / `buildRecordUsageLog` 之后、扣费之前：用与 usage 读路径相同的展示价 + 展示倍率做 L1+L2，再套 cap，替换 `cost.ActualCost`。Claude `GatewayService` 与 OpenAI `recordUsage` 共用一个 helper，避免两套账。

非 token 模式（按次 / 图）不套本 cap。

## 10. 回滚

Settings 改回 0：新请求不再降账、不再打 `applied`。已 `applied=true` 的行仍按 used 值展示（冻结）。完整回滚代码 = revert。无 kill-switch。
