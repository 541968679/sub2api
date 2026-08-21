# 设计：配对 N 条窗冷却（账号质量轨不变）

已决合同见 `prd.md`、`research/pair-quality-n-window.md`。考察期不做。

## 边界

| 轨 | 窗口 | 间隔 | 谁用 |
| --- | --- | --- | --- |
| \(Q_a\) 账号 | 15 分钟 | 5 分钟 | 账号格、硬关闭、快照。不喂智能调度冷却 |
| \(Q_{a,u}\) 配对 | 一个 N、两份 FIFO | 每条入窗完成即算 | 仅智能调度这对冷却 + 新「配对质量」列 |

N：该用户该平台策略一个整数，默认 10，范围 **1–100**。原 `quality_min_success_samples` / `quality_min_ttft_samples` 读写都变成这一个 N（GET 两字段可回填同一值以免旧前端空）。

样本只计 `(account_id, user_id)`。

## 两窗

**\(W_{ttft}\)**：成功且有首字（`true_first_token_ms` 或 `first_token_ms`）。失败、无首字同步成功不进。满 N 才允许判 p50。

**\(W_{ok}\)**：计入口径的完成。失败是否计入受 `schedule_use_failover_error_rate`。满 N 才允许判成功率。

一条请求：失败 → 只 \(W_{ok}\)；同步成功无首字 → 只 \(W_{ok}\)；流式成功有首字 → 两窗都进。

or/and 与现网相同：未满的指标不进入。

## 状态（界面中文 / API 英文不变）

| 界面 | API | 冷却 | 两窗 | 判断 |
| --- | --- | --- | --- | --- |
| 豁免期 | `resumed` | 清 | 进入时清零；期内入窗 | 现网时间豁免内不判断（`u:` 15m + `w:` 至 30m）。期满再判断 |
| 可调度 | `selectable` | 清 | 清零再攒 N | 不写 `w:`。未满 N 不冷却 |
| 冷却到期 | — | 过期 | 同可调度 | 无时间豁免 |
| 暂停 / 配对冷却 | `paused` / `cooling` | 见现网 | 不入窗 | 不比 \(Q_{a,u}\) |

禁止保留冷却前窗口来判断。

`will_cool` 只看配对窗 + 门槛，不看账号 15m。

## 数据流

```text
请求完成（usage / 计入的错误）
  → 配对在冷却或暂停？丢弃
  → 按规则 append W_ttft / W_ok（FIFO 截断 N）
  → 重算 p50 / 成功率
  → 写配对 live + 趋势点
  → 豁免期未满？结束
  → 对应窗满 N 且门槛越界？HSETNX 冷却 + 写事件
选号 admitsScheduleUser
  → paused / cooldown HASH → 拒
  → 豁免期 u:/w: → 放行（不写冷却）
  → 只读 Q_{a,u}，不再 Get account-quality:live
```

建议 Redis：

- `smart-schedule:pair-quality:{accountID}` HASH `u:{userID}` = 两窗样本 + 当前 p50/成功率
- 趋势：同前缀或 list，每次重算 append 一个点，TTL 24h，详情图用
- 事件：冷却开始/结束、进入豁免期、可调度、到期清零。至少 7 天，详情「冷却 / 恢复记录」用。可 SQL 小表或 Redis 再落库。

完成路径挂钩：现有 RecordUsage / ops_error 写入之后（与 failover 口径同一开关）。不要每条选号重算。

## 前端

- 参数区：两个最少样本改成一个「窗口样本数 N」。
- 池表：保留账号质量列；**新增配对质量列**（p50、成功率、两窗条数/N）。点击打开配对详情（趋势 + 事件），不复用账号 `quality-history`。
- 「已恢复」文案全部改为「豁免期」。`state` 仍传 `resumed`。

## 兼容 / 回滚

- 旧策略行：两个最少样本都缺 → N=10；只填了一个 → 用那个值 clamp 到 1–100。
- 回滚二进制：忽略配对 live，冷却会退回读账号 15m（旧代码）。新 key / 新列不影响账号轨。
- 不改客户端协议、计费、`IsSchedulable()`。

## 残留

进行中请求仍不进窗。N=10 且并发 30，仍要 10 个完成才第一次能冷。夹并发 / 考察期本阶段不做。

## 已实现 JSON 字段

见同目录 `research/backend-api-contract.md`。权威字段是 `quality_window_samples`、`p50_ttft_ms`、`ttft_count`、`ok_count`、`will_cool`、详情 `live`/`ts`。为已接线的前端同时回填 `quality_window_n`、`ttft_p50_ms`、`ttft_samples`、`ok_samples`、`current`、`captured_at`、`at`。
