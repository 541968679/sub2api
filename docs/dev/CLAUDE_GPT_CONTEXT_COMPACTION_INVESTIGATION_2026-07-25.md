# Claude-GPT 上下文压缩兼容性调查（2026-07-25）

## 1. 问题范围

Claude Code（下文简称 CC）以带 1M 上下文语义的 Claude 模型访问
Sub2API Claude-GPT bridge 时，客户端按自己的 1M 窗口决定何时发起
compact；实际映射到 GPT/Codex OAuth 上游后，上游会更早拒绝过长输入。
生产实测的失败形态为：

```text
API Error: 400 Your input exceeds the context window of this model.
Please adjust your input and try again.
```

本调查不再推断或校准上游的精确 token 硬边界。对产品修复有决定意义的
事实是：**CC 所依据的上下文窗口大于当前 GPT 上游实际可接受的窗口，且
GPT 上游会在 CC 的预防性 compact 之前拒绝请求。**

## 2. 必须区分的两条 compact 路径

### 2.1 客户端主动 compact

CC 判断会话接近其上下文窗口后，会发起一条隐藏 compact 请求。客户端知道
自己正在压缩，因此能显示/等待 compact 状态，并在摘要成功后继续原会话。
Sub2API 已有这类请求的 Claude -> GPT 转换、`/responses/compact` 调用、失败
恢复、分块摘要、usage 合并和 keepalive 支持。

### 2.2 桥接服务预生成 auto-compact

fork 在 2026-07-24 加入了 `maybeAutoCompactAnthropicBridge`：普通生成请求在
发往 GPT 前，如果序列化 `input` 达到默认 512 KiB，会由桥接服务同步调用
`/responses/compact`。它是 Sub2API 私有机制，不是上游 Claude-GPT bridge
原生路径。

该机制的主要产品问题是 CC 完全不知道服务端插入了 compact：用户只看到
首字时间异常增长，使用记录也表现为一次很长的请求。它还可能在无状态历史
重放时重复发生。因此生产已于 2026-07-25 通过以下配置关闭：

```yaml
gateway:
  anthropic_bridge_auto_compact_enabled: false
```

关闭的只是预生成 auto-compact；客户端主动 compact 及其桥接恢复逻辑没有
关闭。

## 3. 当前 400 为什么没有触发 CC 自己压缩

当前 Claude-GPT bridge 在收到 OpenAI Responses 终态
`response.failed` / `context_length_exceeded` 后，会把上游消息作为 Anthropic
`invalid_request_error` 原样返回，HTTP 状态为 400：

```text
Your input exceeds the context window of this model.
```

本机 Claude Code 2.1.220 的行为验证表明，它不是仅凭“HTTP 400”判断
prompt-too-long，而会识别特定错误语义/文案，包括：

```text
Prompt is too long
input is too long for requested model
input length and `max_tokens` exceed context limit
```

用本地 mock 返回以下响应时，CC 将终止原因分类为 `prompt_too_long`，并在
有足够多可压缩消息组的历史中进入 `status: compacting`：

```http
HTTP/1.1 413 Payload Too Large
Content-Type: application/json

{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Prompt is too long: 271533 tokens > 200000"
  }
}
```

因此当前失败不是“CC 完全没有超限恢复能力”，而是桥接把 GPT 的超限错误
转换成了 CC 不识别的普通 400 文案，CC 无法进入 reactive compact/retry
路径。

## 4. 已验证、未验证与限制

### 已验证

- 生产隐藏预生成 auto-compact 已关闭，服务健康。
- 生产/人工长会话能复现 GPT 上游先于 CC 主动 compact 返回上下文超限。
- 当前代码对 `context_length_exceeded` 返回 HTTP 400，并保留上游文案。
- CC 2.1.220 对 `Prompt is too long` 形态会分类为 `prompt_too_long`。
- 在多消息 mock 中，CC 收到该错误后能进入 `compacting` 状态。
- 单条巨大输入或只有一个可压缩 exchange 时，CC 会报告
  `A single-exchange conversation cannot be compacted`；错误转换无法消除
  这一客户端固有限制。

### 尚未完成

- 尚未用“真实 CC + 本地修复版 Sub2API + GPT 上游 + 多轮长历史”完成从首次
  超限、客户端 compact、摘要成功、自动重试到最终成功的全链路验证。
- HTTP 400 与 413 中哪一个在受支持 CC 版本范围内兼容性最佳，仍需结合上游
  代码/历史和真实客户端回归后决定。
- 若上游错误未提供可信的 token limit，桥接不应伪造精确上限数字；文案可只
  使用稳定的 `Prompt is too long` 前缀并保留经过清洗的上游详情。

## 5. 已否决的产品方案

以下方式可以作为单机诊断手段，但不能作为 Sub2API 的最终产品方案：

- 要求用户移除模型的 `[1m]` 后缀。
- 要求用户改用裸 `opus`。
- 要求所有用户配置 `CLAUDE_CODE_AUTO_COMPACT_WINDOW` 或修改 CC 设置。
- 重新启用桥接侧隐藏预生成 auto-compact。

原因是服务端不能要求所有客户端协调修改；隐藏 compact 又会恢复不可见长
等待、首字时间膨胀和额外上游调用问题。

## 6. 当前候选修复（方案二）

Sub2API 单方面可实施的最小方案是：

1. 只捕获 GPT 上游明确的上下文超限，例如
   `error.code == context_length_exceeded`，并保留现有文案特征匹配作为兼容
   兜底。
2. 将 Anthropic 下游错误规范化为 CC 可识别的 prompt-too-long 契约，候选为
   HTTP 413 + `invalid_request_error` + 以 `Prompt is too long` 开头的消息。
3. 不将这类请求切换到其他同窗口 GPT 账号，不重放已产生的可见输出，不记为
   成功 usage。
4. 由 CC 自己进入 reactive compact，发起它可见、可等待的客户端 compact
   请求；Sub2API 继续使用已有 compact bridge/recovery 路径处理该请求。
5. 对 `response.failed/context_length_exceeded`、非超限 400、流式/非流式响应、
   failover 和 Anthropic JSON/SSE 错误形态增加回归测试，再做真实长会话闭环。

该方案是**首次超限后的恢复**，不是提前预防。GPT 上游仍会先拒绝一次，但
用户可见的压缩状态、等待机制和会话重试重新归 CC 所有，不再由桥接静默代办。

## 7. 上游复核结果

### 7.1 复核基线

2026-07-25 已刷新 `upstream/main` 到 `2e2638c01`，并同时检查 GitHub 代码、
Issue、已合并 PR 和开放 PR。结论是：**上游没有一个已经合并的修复，会把
OpenAI `context_length_exceeded` 转成 CC 可识别的 `Prompt is too long` 错误，
也没有已合并的 reactive compact 触发机制。**

### 7.2 上游已合并修复解决的是错误归类和透传

相关上游历史如下：

| 上游变更 | 实际解决的问题 | 不包含的能力 |
|---|---|---|
| [#3548](https://github.com/Wei-Shaw/sub2api/pull/3548) | 识别上下文窗口错误，禁止把确定性的超限请求切换到其他 OpenAI 账号 | 不改成 `Prompt is too long`，不触发 CC compact |
| [#3859](https://github.com/Wei-Shaw/sub2api/pull/3859) | 修复 `/v1/messages` 吞掉 `response.failed` 并返回 HTTP 200 空消息 | 默认仍是网关错误语义，不负责客户端 compact |
| [#3868](https://github.com/Wei-Shaw/sub2api/pull/3868)、[#3870](https://github.com/Wei-Shaw/sub2api/pull/3870)、[#3873](https://github.com/Wei-Shaw/sub2api/pull/3873) | 让 Responses、Chat 和 Messages 的 `response.failed` 能命中管理员错误透传规则；上下文错误可配置成 400 并保留原文 | 没有内置默认透传规则；没有 `Prompt is too long` 规范化；没有 reactive compact |

上游最新代码中的 `isOpenAIContextWindowError` 主要用于禁止账号 failover、区分
HTTP 413 请求体大小限制，并在配置了错误透传规则时提供语义状态。仓库没有
为 `context_length_exceeded` 预置规则；测试使用的是临时绑定规则。因此直接
使用上游默认行为并不能保证 CC 进入 `prompt_too_long`。

### 7.3 上游开放 PR 也没有提供客户端可见恢复

- [#3808](https://github.com/Wei-Shaw/sub2api/pull/3808) 处理 Messages 空流、
  客户端 4xx 和 compact 失败恢复，但截至复核时仍为 `OPEN / CONFLICTING`。
  fork 后续提交 `c67c1ff7e` 已吸收并显著扩展了其中的 compact 恢复部分。
  该代码把超限返回为非重试 4xx，仍保留上游 `context window` 文案，不会诱发
  CC reactive compact。
- [#4756](https://github.com/Wei-Shaw/sub2api/pull/4756) 正是“生成前由适配器
  隐藏调用 `/responses/compact`”的方案。截至复核时仍为开放 PR，且上游设计
  为默认关闭。fork 的 `8ca41688f` 在 2026-07-24 移植并改成默认开启，生产又
  于 2026-07-25 明确关闭。它与本调查已否决的隐藏 auto-compact 是同一类方案，
  不是方案二。

### 7.4 本地与上游的真实偏差

本地并不是简单“落后到缺少上游修复”，而是形成了不同的组合：

1. fork 已有比上游 `main` 更强的客户端主动 compact 失败恢复，包括完整历史
   快照、分块/递归摘要、fallback model、usage 合并、keepalive 和 continuation
   清理。`docs/dev/UPSTREAM_SYNC.md` 关于 `c67c1ff7e` 覆盖 compact failure
   recovery 的判断在这个范围内成立。
2. fork 的 `openAIMessagesTerminalFailureError` 会把普通生成阶段的
   `context_length_exceeded` 直接写成 HTTP 400 / `invalid_request_error`，但
   保留上游原文。这是当前生产 400 的直接代码来源；该 helper 来自 fork 的
   `c67c1ff7e`，并非漏同步某个上游 prompt-too-long 修复。
3. fork 额外引入了预生成 auto-compact（`8ca41688f`），而上游同类 #4756 尚未
   合并且默认关闭。生产关闭该机制后，才暴露出第 2 点的错误契约不足。
4. fork 还有 Opus 4.8/5 的强制 1M Anthropic beta 策略（`fc543d150`）。该策略
   作用于原生 Anthropic/Antigravity/Bedrock/Vertex 上游请求，不会让映射后的
   GPT Responses 上游获得 1M 能力，因此不能修复 Claude-GPT 窗口不匹配。

严格的“Antigravity 分组存在映射意图时改走 OpenAI Claude-GPT bridge”也是
fork 层；底层 Anthropic Messages -> OpenAI Responses 转换则来自上游。当前
缺口恰好位于两层的出口契约：上游识别了错误，本地也能稳定返回客户端 4xx，
但双方都没有把它翻译成 CC reactive compact 所需的提示。

### 7.5 其他上游能力为什么不是本问题的修复

上游/本地都存在 Antigravity `PromptTooLongError` 和“invalid request fallback
group”。它用于原生 Antigravity 请求在 prompt-too-long 后切到一个 Anthropic
分组，会更换实际供应路径；它不作用于已经进入 OpenAI Claude-GPT bridge 的
`response.failed/context_length_exceeded`，也不是 CC 自己压缩后重试。

管理员错误透传规则理论上可以把已识别的上下文错误改成指定状态码和自定义
消息，因此可作为方案二的快速实验入口。但当前规则会把错误类型固定成
`upstream_error`，其完整 CC 版本兼容性尚未验证，不能替代代码级、带回归测试
的稳定错误契约。

## 8. 最终判断

- 上游并非“完全没处理”：它已修复吞错、错误 failover 和可配置透传。
- 上游也确实**没有**处理本次产品目标：让 1M 语义的 CC 在较小 GPT 窗口先
  超限时，自动进入客户端可见 compact/retry。
- 本地偏差很大，但当前问题不是因为漏同步一个现成的上游修复。相反，当前
  400 来自 fork 为 compact recovery 新增的确定性客户端错误分支；它只差最后
  一步 CC 错误语义转换。
- 因此仍应实施方案二：在 Claude-GPT bridge 出口将明确的上下文超限规范化为
  CC 可识别的 prompt-too-long 契约，并用真实多轮历史验证 compact + retry
  闭环。没有可直接 cherry-pick 的上游已合并方案。
