# 无池用户：有空位打便宜号，满了立刻打贵号

## Goal

没开智能调度池的用户，**有空位时打最低上游倍率号**；便宜号并发满了就**立刻打有空位的贵号，不等**。  
会话若钉在贵号、便宜号现在有空位，代码拆开 session 粘性换过去。

不靠运营改倍率，不给每人建池，不改账单，不在便宜档排队。

## User value

无池用户今天在「两边都有空位」时，热路径已经会选便宜号。真正漏掉的是：

- 会话钉在贵号上，即使后来便宜号空了也不换
- 少数降级选号路径还在比 priority，没比倍率
- OpenAI 高级调度占不到便宜号时会在便宜档 WaitPlan，无池用户按本次决定应立刻改打贵号

满了还排队已被否决：不能等，直接打。

## Background

- `08-16-upstream-rate-schedule`：有空位的候选里先比 `upstream_rate_multiplier`。
- `GatewayService` Layer 2：先丢掉 `LoadRate >= 100`，再在空闲集合取最低倍率。便宜号全满 → 立刻选空闲贵号。见 `gateway_service.go` 约 2147–2170 行。这条与本次「满了直接打」一致，**不改**。
- OpenAI 负载感知同样先滤空位（`openai_gateway_service.go` 约 2149–2167 行）。
- OpenAI 高级调度：倍率第一键；占槽失败后 WaitPlan 仍按倍率，会停在满载便宜号上。无池用户本次改为占不到便宜号就立刻试贵号空位。
- 粘性：仍准入不拆钉。无池 + 便宜档有空位时改为拆 session 粘性。`previous_response` 不拆。
- 降级路径 `isBetterAccount` / 按平台逐个比较：priority + LRU，无倍率。
- 无池判定：`lookupEnabledSmartPolicy(...) == nil`（userID<=0 也是）。
- 同倍率号之间没有第二套便宜信号。

## Locked decisions

1. 只改代码。不做运营手册，不把「调高贵号倍率」当验收。
2. 不自动建池，不铺冷却 / 考察 / 置顶。
3. 只对未开**该账号 platform** 智能调度的用户改粘性逃逸和 OpenAI 高级调度的「满了立刻升档」。判定必须跟 `admitsScheduleUser` 一样用 `account.Platform`，禁止用分组 platform 一刀切（mixed / Claude-GPT bridge 会错）。池内用户保持现网。
4. 不改 `actual_cost`、扣费、展示变换、`rate_multiplier`。
5. `fallback_only` 仍先硬分区；分区内有空位先低倍率。
6. `previous_response` 不拆。只动 session sticky。
7. **便宜档并发满、更高档有空位：立刻占更高档。禁止为此在便宜档 WaitPlan。**（Brandon 2026-08-22：不能等，直接打。）
8. 全部号都满了才走现有 Layer 3 WaitPlan（这时没有「空闲贵号」可直接打）。

## Requirements

- R1. 有空位时先选最低 `EffectiveUpstreamRate()`。热路径已有；补齐 `isBetterAccount`、单平台 / mixed 降级比较。
- R2. 无池用户：最低档占不到槽、更高档有空位 → 立刻占更高档。不得返回便宜档 WaitPlan。Gateway 负载感知已符合，保持。OpenAI 高级调度无池路径要改到符合。
- R3. 无池 session 粘性钉在更高倍率号，且更低档有即时空位 → 清 pin 并选便宜号。低档无空位 → 保持原粘性（直接打当前钉住的号，不再为换号排队）。
- R4. 已开智能调度的用户×**该账号 platform**：不走 R2 的升档改动、不走 R3 逃逸。mixed 请求里只对「该号 platform 无池」的粘性逃逸。
- R5. `isBetterAccount` / Gemini 同等比较只作用于已经 `admitsScheduleUser` 之后的集合；池过滤仍先发生。
- R6. 回归测试覆盖 R1–R5，并包含：池用户不逃逸、mixed 下只开了 AG 池时 AG 粘性不逃逸、ctx 无 UserID 时与现网一样回退普通调度。文档只记代码行为。

## Acceptance Criteria

- [ ] AC1. 无池、便宜号与贵号都有空位：选便宜号。映射 R1。
- [ ] AC2. 无池、便宜号并发满、贵号空闲：立刻占贵号，不是便宜档 WaitPlan。映射 R2、Locked 7。
- [ ] AC3. 无池、便宜号全部不可调度、贵号可调度：选贵号。映射 R2。
- [ ] AC4. 无池、粘性钉贵号、便宜号有空位：清 session sticky 并选便宜号。映射 R3。
- [ ] AC5. 无池、粘性钉贵号、便宜号无空位：保持贵号，立刻占或走该号现有粘性等待（不另开便宜档等待）。映射 R3、Locked 7。
- [ ] AC6. OpenAI `previous_response` 钉在贵号：不因 R3 拆开。映射 Locked 6。
- [ ] AC7. 有主号时不选 `fallback_only`。映射 Locked 5。
- [ ] AC8. 用户在 OpenAI 开池：session 钉在池内贵号、池外或池内更便宜号有空位 → **不**清 pin、不套 R2。映射 R4。
- [ ] AC8b. Anthropic 分组 mixed：用户只开了 Antigravity 池，sticky 是 AG 号 → 即使分组是 anthropic 也不逃逸。映射 R4、Locked 3。
- [ ] AC8c. ctx 无 UserID：与现网相同，不当成开池；不额外发明失败策略。映射 R6。
- [ ] AC9. 降级路径两个空闲号，0.15 赢过 1.0。映射 R1。
- [ ] AC10. 单测证明不读调度倍率去算 `actual_cost`。映射 Locked 4。

## Out of Scope

- 运营改倍率 / 标兜底 / 操作手册
- 便宜档短等再溢（已否决）
- 自动灌全组进智能调度池
- 全站 pair 冷却 / 考察 / 置顶
- Anthropic 质量打分
- 改计费、展示价、cache-read
- 让客户端改 `stream` / 端点
- 生产 push / 部署
