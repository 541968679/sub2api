# Implement: pair-full wait bypass

Q1 closed: **sticky 不并**. OpenAI `sticky_escape` / `previous_response` 再绑定本轮不改。

## Checklist

1. Anthropic `gateway_service` 模型路由等待 + Layer 3：本轮 `tryAcquireAccountAndPairSlot` 返回 `pairFull` 的号不得进入 `AccountWaitPlan`。有替代则改选；全满则 `ErrNoAvailableAccounts`。
2. OpenAI `openai_gateway_service` Layer 3（含 advanced scheduler 关闭时的 load-aware 回退）：同上，禁止用 Layer 2 过滤前的陈旧 `candidates` 排队。
3. OpenAI `openai_account_scheduler` waitOrder：本轮 acquire `pairFull` 的号也跳过（fresh Redis 检查保留作兜底）。
4. WaitPlan 醒来：各 handler 在 `AcquireAccountSlotWithWaitTimeout` / 立即 `TryAcquireAccountSlot` 成功后走 `AttachPairSlotAfterAccountWait`（已持账号槽 + `AcquireAccountUserSlot`）。`pairFull` 则释放账号槽、排除再选，不得只持账号槽转发。
5. cap=0 / 未设 / UI 999：不得 `pair_full`。配对满额不清 pin。
6. 单测：快照未满但 acquire `pairFull` 不得 WaitPlan；全满无 WaitPlan；醒来 pairFull 释放账号槽；cap=0/999 回归。
7. `docs/dev/CHANGELOG_CUSTOM.md`；必要时补 `docs/dev/codebase/gateway.md` 与 spec 反例。不 commit / push / deploy。

## Validation

```powershell
cd backend
go test -tags=unit ./internal/service -run "PairFull|PairConcurrency|UserSchedulePair|AttachPairSlot" -count=1
go test -tags=unit ./internal/handler -run "PairFull|AcquireResponsesAccountSlot" -count=1
```

## Risky files

| Area | Risk | Guard |
| --- | --- | --- |
| Layer 3 仍遍历 Layer 2 的 `candidates` | 本轮 acquire 已 pairFull 仍 WaitPlan | 本轮 `pairFullIDs` 排除后再排队 |
| 路由等待用陈旧 `routingPairCounts` | 快照未满、acquire 已满仍排队 | acquire `pairFull` 记入 skip 集 |
| 醒来只拿账号槽 | 配对已满仍转发 | `AttachPairSlotAfterAccountWait` |
| 把 Recovered 当 pair_full | 口径错 | 不读 ops_error_logs；只比 Redis cap |
| 并进 sticky_escape | Q1 已关 | 不改 `shouldEscapeStickyAccount` 交接 |

## Review gates before `task.py start`

- [x] PRD Q1 关闭为 sticky 不并
- [x] design.md 删除 sticky 专章
- [x] implement.md / jsonl 已写
- [x] 代码已确认 Layer 3 / 路由等待 / handler 醒来三处洞；OpenAI scheduler waitOrder 已有 fresh 检查，只补本轮 pairFull skip + 回归
