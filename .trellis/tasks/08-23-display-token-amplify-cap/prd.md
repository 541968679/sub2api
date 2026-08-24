# 展示 token 放大上限（input+cache 联合预算）

## Goal

给展示层加一层 **绝对 token 上限**：在现有 L1+L2 放大（含 M/α）之后，把用户看见的 **input+cache 合计**压进一个像 GPT 上下文窗口的预算；output 另有独立上限。新请求按封顶后的展示账 **真正少扣钱**。历史行冻结。

## User value

展示数字不再涨到几百万；账单跟「展示 token × 展示单价 × 展示倍率」走，封顶的请求少扣。旧记录不改、不退。

## Background

- 现有机制：`display_cache_token_max_mult`（M）+ `display_output_residual_growth_ratio`（α）。cache 放大受 `real×M` 约束，残差折入 output 再 input。本任务 **不替换** 这条路径。
- 新 cap 叠在 L1+L2 **之后**。封顶后的「少掉的」展示成本 **丢弃**，不再折进任何分量。
- GPT 上下文通常是 **input + cache < ~1M**。两个独立绝对上限会让合计仍 >1M，所以联合预算是产品约束，不是实现偷懒。
- 研究用户 user_id=16（邮箱仅用于解析 id）。窗口 **2026-08-17 → 2026-08-24（7 天）**，n=322175。方法见 `research/display-token-distribution.md`。

## Locked decisions

1. 新请求：`展示token × 展示单价 × 展示倍率 = 最终扣费`（`actual_cost` / quota）。封顶后账单 **低于** 未封顶路径。
2. 封顶后 **禁止** 把少掉的成本折进 output / input / cache（与 M/α 残差折入不同）。
3. 允许打破：`真实token × 真实价 = 展示token × 展示价`。
4. **联合预算**：`S = display_input + display_cache`。`S ≤ jittered_cap` 则两分量不动；否则压到 `in' + cache' = jittered_cap`。
5. **Output 独立封顶**，不进 1M 窗口。
6. **历史冻结**：不改、不退已写 `usage_logs` 与已扣费。读路径不得给旧行发明更低账单。
7. Jitter 打在 **合计 cap** 上，再分配；禁止对 input/cache 各自 jitter。
8. Settings 默认 0 = 关闭（行为与今天相同）。推荐配置值见研究，**不**写进代码默认。
9. 不碰 sibling 的 `openai_long_context_billing_enabled`。
10. 占用 checkout 写代码。不 commit / push / deploy，除非 Brandon 另批。

## Requirements

- R1. 写路径（新请求）：L1+L2 之后套联合 cap（+ 可选 output cap）+ 合计 jitter；用封顶后分量重算展示成本；`actual_cost` / 余额 / 订阅 / API Key quota 跟新的更低账走。
- R2. 封顶后不残差折入。`display_*_cost = capped_tokens × 该分量展示单价`。
- R3. 计费实数 token 列（含 billing-real `cache_read_tokens`）不因本 cap 被改写。
- R4. 旧行：读路径不做本 cap。新行：展示数字与当时扣费一致（需能区分新/旧行）。
- R5. 下游 display-mode 新响应与写路径同一套 cap（新请求）。
- R6. Admin：一个 **input+cache 合计** 上限 + 一个 **output** 上限。0/空 = 关。校验 ≥0 整数。
- R7. Jitter：稳定（`request_id`，缺则 usage id）；合计预算落在配置 cap 的 92%–100%；同一行刷新不变。
- R8. 单测：联合压预算、output 独立、0=关、jitter 稳定、新账更低、旧行不受读路径 cap、M/α 残差仍只服务旧放大。

## Acceptance Criteria

- [ ] AC1. `S ≤ jittered_cap`：input/cache 与 L1+L2 结果相同。映射 R1。
- [ ] AC2. `S > jittered_cap`：`in' + cache' = jittered_cap`，两分量都不高于封顶前。映射 R1、Locked 4。
- [ ] AC3. 封顶后 `actual_cost_new ≤ actual_cost_uncapped`（有 cap 绑定时严格更小，忽略取整）。映射 R1、Locked 1。
- [ ] AC4. 封顶后无残差折入：output 不因联合 cap 变大。映射 R2。
- [ ] AC5. 仅 output cap 绑定时：input/cache 不动，output 降，账降。映射 Locked 5。
- [ ] AC6. 默认 0：写路径/读路径与现网一致。映射 Locked 8。
- [ ] AC7. 旧 `usage_logs` 读出来的展示账与上线前相同（不套新 cap）。映射 R4。
- [ ] AC8. 同一 `request_id` 两次 jittered_cap 相同；不同 id 在 92%–100% 带内散开。映射 R7。
- [ ] AC9. i18n zh+en；Settings 加在现有「展示层」M/α 旁；不覆盖 long-context 开关。映射 R6、Locked 9。

## Out of scope

- commit / push / 生产部署
- 回写或退还历史扣费
- 用独立的 input 绝对 cap + cache 绝对 cap（会破坏 1M 合计故事）
- 改 M/α 语义，或用 M 残差补回本 cap 砍掉的钱
- 封顶 cache-creation（仍走现有独立反算）
- 客户端改协议 / stream / body

## Open questions

无。分配策略、jitter 带、推荐数字在 `design.md` / `research/display-token-distribution.md`。
