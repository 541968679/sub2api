# 修复智能调度配对并发满额后仍被调度

## Goal

有真实配对上限（N≥1）的用户×账号，占用达到上限后，这次请求不得再打到该账号：不能选中、不能 WaitPlan、等待醒来后也不能只拿账号槽就转发。有替代则改选；没有可替换账号时走现有「无可用账号」失败。不因配对满额单独 429。客户端入站协议不变。

## Background

管理员看到池内某号配对并发已满，请求却还打过去并报错。现网契约：配对打满排除再选，禁止为此发 WaitPlan / 429（`.trellis/spec/backend/account-user-schedule.md`；`08-14-account-user-concurrency` D6、`08-16-advanced-user-account-pool-scheduling` R9）。实现只在快照已满时跳过，等待和醒来两条路径没有守住同一条契约。

Q1 已关闭：OpenAI sticky_escape 不并进。不能用 Recovered 条数代替质量口径。

## Confirmed Facts

- 热路径上限：`isPairConcurrencyFull` / `tryAcquireAccountAndPairSlot`（`account_user_concurrency.go`）。智能调度开启时看池成员 `PairCap`；未开启看 `Account.UserConcurrency`。`0` / 未设 / UI 999 都不是真实上限。
- 选号入口：`gateway_service.go`、`openai_gateway_service.go`、`openai_account_scheduler.go`、`openai_ws_forwarder.go`、`gemini_messages_compat_service.go`。
- 已有单测覆盖「快照已满则改选、粘性不清 pin、WaitPlan 为 nil」。
- 配对漏洞 1：同一请求内快照未满、`Acquire` 已满时，Anthropic Layer 3、模型路由等待、`openai_gateway_service` Layer 3 仍用旧候选列表发 `AccountWaitPlan`。OpenAI 专用 scheduler 的 wait 会再查一次，这条兜底不会。
- 配对漏洞 2：WaitPlan 醒来后各 handler 只走 `AcquireAccountSlotWithWaitTimeout`，不再拿 `concurrency:account_user:{accountID}:{userID}`。
- Q1 关闭：OpenAI `sticky_escape` / `previous_response` 再绑定不在本任务。逃逸用进程内 EWMA，不是 15 分钟质量窗；Recovered 不是调度错误率，不能当「质量不健康」证据。
- 现网质量口径只作边界（不改）：terminal `status>=400`；Recovered 不进当前调度错误率（`schedule_use_failover_error_rate` 默认 false）；Claude→GPT 桥接进 `bridge_*`。

## Requirements

- R1. 真实配对上限 N≥1 且该对占用 `>= N` 时，本次请求不得选中该账号，也不得对该账号发 WaitPlan。映射 D1。
- R2. 同一请求里快照未满但 `tryAcquireAccountAndPairSlot` 返回 `pairFull` 时，必须排除该账号再选；禁止用旧候选列表对该账号排队。映射 D1。
- R3. WaitPlan 只表示等账号槽。醒来后必须再走配对槽获取；此时 `pairFull` 则排除再选，不得只拿账号槽就转发。映射 D2。
- R4. 有其他合规账号时改选到其他账号，不因配对满额单独 429。映射 D3。
- R5. 没有可替换账号时走现有「无可用账号」失败，不得把已满配对当成排队目标。映射 D3。
- R6. 未设 / 0 / 智能调度未开启且无旧 cap / UI 999：行为与现网一致，不得误判 `pair_full`。映射 D4。
- R7. Anthropic、OpenAI（含 scheduler 与 gateway fallback）、Gemini 兼容入口的配对满额行为一致。映射 D5。
- R8. 不改计费、扣费、`actual_cost`、显示价格变换；不折进 `IsSchedulable()`；不改「配对满额不清 pin、质量拦截才清 pin」语义。
- R9. 回归测试锁住 R1–R5：快照未满但 acquire 已满不得 WaitPlan 回同一号；WaitPlan 醒来后 pairing 已满不得只拿账号槽转发。

## Acceptance Criteria

- [ ] AC1. 快照已显示该对满额且另有合规账号：选到其他账号，WaitPlan 为 nil。映射 R1、R4。
- [ ] AC2. 快照未满、acquire 返回 `pairFull`、另有合规账号：选到其他账号，不得对该满额账号发 WaitPlan。映射 R2、R4。
- [ ] AC3. 全部合规账号都配对满额：返回无可用账号，WaitPlan 为 nil。映射 R5。
- [ ] AC4. 因账号槽满而 WaitPlan 的请求，醒来后必须再获取配对槽；此时已满则改选或无可用账号，不得只持有账号槽就转发。映射 R3。
- [ ] AC5. 粘性钉在配对已满账号上：本次跳过该号并改选，不清 pin。映射 R1、R8。
- [ ] AC6. 未设上限或 cap=0 的封闭池成员：不出现 `pair_full`，占用仍可计数。映射 R6。
- [ ] AC7. Anthropic / OpenAI / Gemini 三条热路径都满足 AC1–AC4。映射 R7。
- [ ] AC8. 计费与显示价格路径不变。映射 R8。

## Decisions

- D1. 配对满额只能排除再选，不能当 WaitPlan 理由。同一请求内 acquire 刚判满，也按满额处理。
- D2. 账号槽等待和配对槽是两件事。醒来后必须重新获取配对槽。
- D3. 有替代则改选；无替代则无可用账号。不因配对满额单独 429。
- D4. 999 只是后台展示分母；`0` / null 不是上限。
- D5. 配对修复覆盖所有会发 WaitPlan 或醒来只拿账号槽的网关入口。

## Out of Scope

- OpenAI sticky_escape 再绑定（Q1 未批准前不改）。
- 用 Recovered / 桥接失败条数改调度错误率或门槛。
- 改生产账号 1724 的 `upstream_rate_multiplier` 或人工下线该号。
- 重写 OpenAI 打分权重 / TopK。
- 新增「等待配对槽释放」队列。
- 改配对上限配置 UI、999 展示语义、未保存草稿是否生效。
- 改质量门槛、冷却、paused、整号硬关闭。
- 改计费 / 显示价格 / 客户端协议（含要求客户端改 `stream` 或换端点）。

## Open Questions

- Q1. OpenAI sticky_escape 再绑定是否并进本任务？  
  **关闭：不并。** Recovered 不是调度错误率；逃逸 EWMA 与 15 分钟质量窗不是同一口径。本任务只修 pair-full WaitPlan / 醒来绕过。
