# Design：有空位打便宜，满了立刻打贵号

## 白话结构

```text
没开智能调度池：
  有空位的号里，先打最低倍率
  便宜号满了、贵号有空位 → 立刻打贵号（不等）
  session 钉在贵号、便宜号现在有空位 → 拆钉换便宜号
  previous_response 钉住 → 不拆

开了池：
  现网不动
```

不新建调度器，不新建表，不加等待。

## 无池判定（必须跟准入同一把尺子）

```go
// 跟 admitsScheduleUser 一样：看这张账号的 platform，不是分组 platform。
unpooledForAccount := lookupEnabledSmartPolicy(ctx, lookup, userID, account.Platform) == nil
```

错误写法：用分组 `anthropic` 判断「整次请求无池」。用户只开了 Antigravity 池、请求走 mixed 时，会把 AG 粘性当成无池拆掉。

API Key 中间件会把 `ctxkey.UserID` 写进 context。部分 handler 传 `sub2apiUserID=0`，`withScheduleUserID` 会回落到这个值。userID<=0 时现网智能调度本身就不生效（fail-open 到普通规则），本次逃逸也会生效——这是旧行为，不是新开的洞。

## 算法（无池）

占槽（有空位集合 `available`，已按倍率→priority→负载→LRU）：

```text
# 与现网 Gateway Layer 2 相同，保持：
available = candidates where LoadRate < 100
pick = min rate, then priority, then load, then LRU
acquire immediately
# 便宜满、贵号在 available 里 → 直接 acquire 贵号
```

OpenAI 高级调度无池差异（现网会在便宜档 WaitPlan，要改掉）：

```text
先按倍率对有空位的号 acquire
占不到任何有空位的号 → 才 Layer 3 WaitPlan（全员都满）
禁止：便宜号在候选里但已满时，对便宜号发 WaitPlan，而旁边贵号还有空位
```

Session 粘性逃逸：

```text
if unpooled
   and sticky.rate > minRate(among schedulable candidates)
   and some minRate account has LoadRate < 100:
  delete session sticky
  fall through to available-set pick  # 会打到便宜号
else:
  keep current sticky behavior
```

`previous_response` 不进入逃逸。

## 改哪些路径

| 路径 | 改什么 |
|---|---|
| `GatewayService` Layer 2/3 占槽 / 满了升档 | **不改**（已是直接打贵号） |
| `GatewayService` Layer 1.5 session sticky | 无池贵号钉 + 便宜档有空位 → 清 pin |
| Anthropic 模型路由里的 sticky | 同样逃逸 |
| `OpenAIGatewayService` 负载感知 sticky | 同样逃逸 |
| `defaultOpenAIAccountScheduler.selectByLoadBalance` | 无池：有更高档空位时不要 WaitPlan 在便宜档；补 session 逃逸 |
| `isBetterAccount` + 单平台 / mixed 逐个比较 | 先 `compareUpstreamRate` |

不改：池准入、计费、`sort_order`、Gateway「满了打贵号」本身。

## 524 / 超时

不增加任何等待。满了直接打空闲贵号。全部满了才用现成 Layer 3 WaitPlan。粘性逃逸只在便宜档有空位时发生。

## 对智能调度的影响（结论：准入/冷却/池不动）

| 智能调度部件 | 会被碰到吗 |
|---|---|
| 封闭池 / `admitsScheduleUser` | 否。不改这个函数 |
| pair 冷却 / 考察 / 置顶 / 暂停 | 否 |
| 池内 WaitPlan（高级调度） | 否。R2 只在 `unpooledForAccount` |
| 池内 session 粘性 | 否。逃逸前先查该号 platform 的 EnabledPolicy |
| 池内倍率排序 | `isBetter*` 会先比倍率，但候选已是池内；与现网热路径叠层一致 |
| 池表 sort_order | 否 |

会间接变热的：无池用户从贵号粘性逃到便宜号后，便宜号占用上升。池用户若和他们共用同一批号，会感到更挤。这是共享库存，不是逻辑串味。

## 其他风险

- **Prompt cache / 多轮**：拆 session 粘性会丢该会话在贵号上的 cache，下一次在便宜号冷启动。`previous_response` 不拆。
- **占槽竞态**：load 显示便宜号有空位 → 清 pin → acquire 失败 → 立刻打贵号（符合「不能等」），sticky 已丢。
- **Gemini `isBetterGeminiAccount`**：与 OpenAI `isBetterAccount` 同类，改比较会作用到所有走降级路径的人，但只在准入之后；漏改则 Gemini 降级仍不比倍率。
- **Codex `/v1/models` 选号**也走 `isBetterAccount`：可能更多打到低倍率 OAuth 去拉清单，不改池。

## 兼容

- 候选同倍率：逃逸条件 `sticky.rate > minRate` 为假，粘性不拆；占槽与现网相同。
- 池用户：`unpooled==false`，高级调度继续可以在池内便宜号上 WaitPlan。
- 缺倍率字段：继续类型默认。

## 回滚

无迁移。回滚 = revert。行为面主要是 sticky 逃逸 + OpenAI 无池不再便宜档干等。

## Trade-offs

| 做法 | 结论 |
|---|---|
| 便宜满了短等 | 已否决 |
| 立刻打空闲贵号 | 采用；Gateway 已如此 |
| 自动建池 | 拒绝 |
| 拆 previous_response | 拒绝 |
