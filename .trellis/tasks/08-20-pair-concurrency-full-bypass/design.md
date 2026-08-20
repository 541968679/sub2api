# Design: pair-full wait bypass

## Boundaries

- **In**: Anthropic `gateway_service` 路由等待 / Layer 3、OpenAI `openai_gateway_service` Layer 3（含 advanced scheduler 关闭时的 load-aware 回退）、WaitPlan 醒来后的配对槽（`gateway_handler.go`、`openai_gateway_handler.go`、`gemini_v1beta_handler.go`、chat/completions 与 responses 分册、OpenAI WS 立即抢槽）。OpenAI 专用 scheduler waitOrder 已有 fresh Redis 配对检查，补本轮 `pairFull` skip 与回归，不改排队语义。
- **Out**: OpenAI `sticky_escape` / `previous_response` 再绑定（Q1 关闭）。改账号 `upstream_rate_multiplier`、打分权重、智能调度 UI、质量/冷却/paused、计费、客户端协议。不把 Recovered / ops 错误条数当 pair_full 或质量证据。
- **Reuse**: `isPairConcurrencyFull`、`tryAcquireAccountAndPairSlot`、`AcquireAccountUserSlot`、`ExcludedIDs` / failover `FailedAccountIDs`。

## Data flow

```text
选号
  -> pair 快照过滤
  -> tryAcquireAccountAndPairSlot
       pairFull -> 记入本轮 pairFullIDs，禁止 WaitPlan，继续下一候选
       账号槽满 -> WaitPlan（仅账号槽）
  -> handler 若 !Acquired
       等账号槽（或立即 TryAcquire）
       -> AttachPairSlotAfterAccountWait（已持账号槽 + AcquireAccountUserSlot）
            pairFull / 未抢到配对槽 -> 释放账号槽，排除再选或无可用账号
            成功 -> 转发（ReleaseFunc 含账号+配对）
```

## Pair-full

### 选号

Layer 2 acquire 返回 `pairFull` 后，该账号不得进入随后的 WaitPlan 循环。实现上 A 为主、B 为兜底：

- A. WaitPlan 循环跳过本轮 `pairFullIDs`，不要把刚判满的号交给陈旧 `candidates`。
- B. WaitPlan 前对剩余候选再用现有 `isPairConcurrencyFull`（快照或 fresh Redis）。OpenAI scheduler waitOrder 已有 B；再补 A。

模型路由等待循环同样：acquire 刚 `pairFull` 的号不能用同一份陈旧 `routingPairCounts` 排队。

### 醒来

各 `AcquireAccountSlotWithWaitTimeout` / 立即 `TryAcquireAccountSlot` 成功后，必须再拿配对槽（已持有账号槽 + `AcquireAccountUserSlot`）。不能先放掉刚等到的账号槽再整段重抢，除非重抢失败时把账号槽也释放。

`pairFull` 或配对获取失败：释放账号槽，按现有 failover / `FailedAccountIDs` 排除再选，或无可用账号。不把该号当成功选中，不 BindSticky。

Wake 路径拿不到完整 `Account` 时，用 `scheduleUserIDFromContext` + `smartScheduleCache` 解析 cap，与热路径同一套 `resolvePairSlotAcquire`。

## Compatibility

- 入站仍是同步/流式原契约；不要求客户端改 `stream` 或换端点。
- 配对满额不清 pin；质量拦截清 pin。本轮不改 sticky_escape。
- 无迁移、无 Settings KV 新键。

## Rollback

回退调度文件与 handler 等待后的二次获取即可。Redis 槽语义不变。

## Tests

- 快照未满 + acquire `pairFull` + 另有号 → 选另一号，WaitPlan nil（Anthropic 路由/Layer3、OpenAI gateway Layer3 / scheduler）。
- 全部合规账号 acquire `pairFull` → 无可用账号，WaitPlan nil。
- WaitPlan 醒来后配对已满 → 不转发，释放账号槽。
- cap=0 / 999 展示：不 `pair_full`。
- 粘性钉在配对已满账号上：本次跳过该号并改选，不清 pin（已有回归保留）。
