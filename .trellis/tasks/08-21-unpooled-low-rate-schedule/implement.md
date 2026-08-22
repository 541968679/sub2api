# Implement：有空位打便宜，满了直接打贵号

未 `task.py start` 前不改产品代码。

## 顺序

1. Helper：`accountCheaperThenPreferred`；`shouldEscapeSessionStickyForCheaperTier` 必须用 `lookupEnabledSmartPolicy(..., sticky.Platform)`，禁止用分组 platform。
2. Gateway / OpenAI 负载感知 / 模型路由：只加 session 粘性逃逸。不要改 Layer 2「满了打贵号」。
3. OpenAI 高级调度无池：占槽顺序仍低倍率优先；**有更高档空位时禁止对最低档发 WaitPlan**，改为继续 acquire 下一档。池用户不改。补 session 逃逸。
4. `isBetterAccount`、`selectAccountForModelWithPlatform`、`selectAccountWithMixedScheduling` 走 helper。
5. 单测 AC1–AC10。AC2 必须断言「不是 WaitPlan 在便宜号上」。
6. 文档只记行为；禁止运营清单。`CHANGELOG_CUSTOM.md` 一笔。

## 验证

```powershell
go test -tags=unit ./internal/service -run "Unpooled|BetterAccount|StickyEscape|FallbackOnly|UpstreamRate|WaitPlan" -count=1
```

在 `backend/` 下跑。现有倍率 / 计费测试必须仍过。

## 风险点

| 点 | 风险 | 防法 |
|---|---|---|
| 误加档内 WaitPlan | 违反「不能等」 | 不实现旧 design 的升档循环等待 |
| 用分组 platform 判断无池 | mixed / bridge 拆掉 AG 或 OpenAI 池粘性 | 跟 `admitsScheduleUser` 用 `account.Platform`；AC8b |
| 拆 previous_response | 多轮断 | 逃逸只挂 session |
| 便宜无空位仍拆贵号粘性 | 多一次排队 | AC5：无空位保持钉 |

## 回滚点

第 2–3 步可 revert。第 4 步是比较一致性，可留。

## start 前

- Open Question 1 已锁：不能等，直接打。
- 用户看过终稿或明确说可以 `task.py start`。
- `implement.jsonl` / `check.jsonl` 写入真实 spec，不能只留 `_example`。
