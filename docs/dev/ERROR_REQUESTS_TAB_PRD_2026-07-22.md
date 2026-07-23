# 管理员「错误请求」完整 Tab + 筛选范围错误率 — PRD

日期：2026-07-22  
状态：需求已收敛（brainstorm 完成，待 design / implement）  
范围：管理员端 only

## 1. Goal

把管理员「使用记录」页的「错误请求」从半成品附属列表，升级为**完整独立 tab**：可筛、字段够用、并在**当前筛选范围内**给出可信的**终局错误率**，支撑 Claude-GPT bridge 等链路的运营排查。

## 2. Background（现状）

- 入口：`UsageView` 已有 `usage | ranking | errors` tab，但切到 errors 时上方 usage 图表/筛选/导出仍混杂，体感不是完整页。
- 列表：复用 `OpsErrorLogTable` + `/admin/ops/errors`；数据侧已有 user/model mapping 等字段，但 Usage 页几乎未接筛选（仅日期 + group/account）。
- 后端 `OpsErrorLogFilter` 已支持更多条件（user、platform、model、status_codes、q、view…），能力未在 Usage 错误 tab 暴露。
- 数据特性：
  - 一次客户端失败常对应多条 `ops_error_logs`（账号 failover）。
  - 许多失败（如空响应）**没有**对应成功 `usage_logs` 行。
- 运维 Dashboard 已有全局 `error_rate`，不满足按用户/模型/bridge/错误码下钻。

## 3. Decisions（已拍板）

| # | 决策 | 结论 |
|---|------|------|
| D1 | 错误率主口径 | **Request 级去重终局错误率** |
| D2 | 入口 | 使用记录页内 **完整独立 tab（方案 A）** |
| D3 | 筛选 MVP | 见 §4.1；**错误码多选为 P0** |
| D4 | 桥接 | MVP **启发式** + 列/筛选；落库打标为加固项 |
| D5 | 统计展示 | 顶部统计卡 + Top 分解（方案 A） |
| D6 | 列表字段 | P0 列含用户/桥接/请求→上游模型/状态码等（方案 A） |
| D7 | 范围 | **仅管理员端**；用户端错误 tab 本轮不动 |
| D8 | 成功分母 | 对话/token 类成功 usage；与错误同业务筛选对齐 |
| D9 | 错误码与分母 | **S1 条件错误率**：错误码主要收窄分子/错误列表；分母=业务范围内全部请求 |

## 4. Requirements

### 4.1 信息架构

- R1. 「错误请求」为使用记录页的完整 tab。
- R2. 激活该 tab 时主内容区**只展示**错误相关 UI：
  - 错误专用筛选条
  - 错误率统计卡（+ Top 分解）
  - 错误列表 + 分页
  - 详情弹窗
- R3. **不展示**：使用记录表、用户排行、仅服务 usage 的导出/清理、以及默认不展示仅服务 usage 的大图表（可整体隐藏或移出该 tab）。
- R4. 页级**时间范围**可与错误 tab 共用。

### 4.2 筛选（P0）

同一套 filter 驱动：**列表 + 统计卡 + Top 分解**。

| 字段 | 行为 |
|------|------|
| 时间范围 | 共用页顶 |
| 用户 | user_id / email 搜索 |
| 分组 | group_id |
| 账号 | account_id |
| 平台 | platform |
| 请求模型 | requested_model / model |
| 上游模型 | upstream_model |
| 是否桥接 | 全部 / 仅桥接 / 非桥接 |
| **HTTP 错误码** | **多选**；快捷含 429、529、502、503、400、其他 5xx |
| 错误类型 / phase | 可选 |
| 消息关键字 | `q`（P1 可同迭代） |
| 含/不含业务限流 | 默认 `view=errors` 排除 429/529；**用户显式选 429/529 时按其选择查询并计入** |

### 4.3 桥接判定（MVP 启发式）

```text
is_bridge ≈
  入站平台语义为 Claude/Antigravity（platform ∈ {antigravity, anthropic} 或等价）
  AND upstream_model 匹配 GPT 系（如 gpt-*）
  AND（若有 account）账号侧为 OpenAI bridge 路径
```

- 列表列：桥接 是/否  
- 筛选：全部 / 仅桥接 / 非桥接  
- 加固（同迭代或紧随）：写入 `ops_error_logs` 时打标 `is_claude_gpt_bridge`（路由层已有 bridge 上下文时）

### 4.4 错误率统计口径

#### 主指标：终局错误率（request-level）

在**业务筛选范围** \(F_{biz}\)（时间、用户、分组、账号、平台、请求/上游模型、桥接）内：

\[
\text{error\_rate} =
\frac{|\text{Distinct terminal error requests in } F_{biz} \cap F_{err}|}
{|\text{Success usage requests in } F_{biz}| + |\text{Distinct terminal error requests in } F_{biz}|}
\]

| 符号 | 定义 |
|------|------|
| 终局错误请求 | `ops_error_logs` 中客户端可见失败；按 `request_id` 去重，缺则 `client_request_id`；同一请求多账号 failover **计 1 次** |
| 成功 usage | 同 \(F_{biz}\) 下对话/token 类 `usage_logs`；优先 `COUNT` 与错误侧一致的请求粒度（有 request_id 则按 id） |
| \(F_{err}\) | 错误专属条件：错误码、错误类型、消息关键字等 |
| 默认排除 | 业务限流 429/529（`view=errors`）；显式勾选错误码时覆盖 |

#### 错误码筛选语义（S1）

- 错误码/类型/**主要**作用于：错误列表、分子中的错误子集、以及「该码占错误比例」。
- **分母**始终为 \(F_{biz}\) 下「成功 + 全部终局错误」，回答：「在该业务范围内，有多少请求最终以该类错误失败」。
- 另可展示：该错误码占**终局错误数**的百分比（错误构成）。

#### 副指标

- 原始 `ops_error_logs` 行数（未去重，观察 failover 放大）。
- （可选）上游尝试失败相关计数，不作为主错误率。

#### 统计卡 UI（MVP）

| 卡片 | 内容 |
|------|------|
| 总请求数 | 成功 + 终局错误 |
| 终局错误数 | 去重后 |
| 错误率 | 百分比 + 阈值色（建议 >5% 警告、>20% 危险，具体可配置） |
| 原始错误行数 | 未去重 |

Top 分解（当前 filter 内）：

- 按错误码 Top
- 按请求模型 / 上游模型 Top

### 4.5 列表与详情

#### 列表默认列（P0）

时间、用户、分组、账号、平台、桥接、请求模型→上游模型、状态码、错误类型/phase、错误摘要、操作（详情）

#### 详情

沿用/增强 Ops 错误详情：request ids、完整 message/body、upstream 尝试链、endpoint、延迟字段、bridge 与映射链。

### 4.6 非目标（本轮）

- 用户端错误请求 tab 升级
- 与告警规则/邮件报表深度联动
- 完整趋势大盘/热力图
- 列自定义配置器（可二期，对齐 usage 列设置）

## 5. Acceptance Criteria

1. 切到「错误请求」tab 时，主区域不再同时展示使用记录表与排行；错误相关筛选/统计/列表完整可用。
2. 可按用户、分组、账号、平台、请求/上游模型、桥接、**错误码（含 429/529 多选）** 筛选，列表结果与筛选一致。
3. 统计卡错误率随**同一业务筛选**刷新；公式为 request 去重终局错误率；failover 多行不重复计入分子。
4. 仅筛 502 时：列表为 502；错误率分母为业务范围内全部请求（S1）；能看到 502 终局错误数与占比。
5. 桥接列与「仅桥接」筛选对 Claude→GPT 路径可用（启发式可接受边界在设计中写明）。
6. 列表默认可见用户、桥接、请求→上游模型、状态码。
7. 不影响用户端错误请求现有行为。
8. 有回归测试：filter 映射、去重计数、S1 错误码分母语义、bridge 启发式（单测/API 测）。

## 6. Technical Notes（设计阶段展开）

- 复用/扩展 `/admin/ops/errors` 与 `OpsErrorLogFilter`；新增 **stats 聚合 API**（或同接口 `include_stats=1`）避免前端拉全量算率。
- 成功 usage 计数需与错误侧 filter 映射文档化（字段对照表）。
- 大数据量下 stats 必须 SQL 聚合 + 合理时间窗上限；禁止前端扫全表。
- `view=errors` 与显式 status_codes 的优先级写进 API 契约。
- 与现有 Ops 监控页并存：本 tab 负责可筛选下钻；Dashboard 保留全局健康。

## 7. Open Questions（不阻塞 MVP，设计时可定默认）

| ID | 问题 | 建议默认 |
|----|------|----------|
| O1 | 错误率阈值色是否做成设置项 | 先写死 5%/20% |
| O2 | 图片/视频 usage 精确 request_type 排除列表 | 设计时对照 `usage_logs.request_type` 枚举 |
| O3 | bridge 落库打标是否同 PR 完成 | 若改动面小则同迭代，否则 P0.5 |
| O4 | 无 request_id 的错误如何去重 | client_request_id → 再退化为按 id 计 1 |

## 8. Next Steps

1. `design.md`：API 契约、filter 映射表、SQL 去重方案、UI 线框级结构、bridge 启发式精确规则。  
2. `implement.md`：后端 stats → 前端 tab 改造 → 测试清单。  
3. 用户确认 design 后开始实现。
