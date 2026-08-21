# 账号质量 last-N：前端契约

配对 `quality_window_n`（默认 10）与本轨无关。本文件只描述 \(Q_a\)。

## Settings KV

- Key：`quality_hard_close_settings`（`GET/PUT /api/v1/admin/settings/quality-hard-close`）
- 规范 N：`account_quality_window_n`（integer，默认 **20**，clamp **1–100**）
- 读入别名：`window_n`、`n`
- 兼容回填（GET 始终同值）：`min_success_samples`、`min_ttft_samples`
- 解析：显式 N → `min_success_samples` → `min_ttft_samples` → 20。不要 `min(旧成功, 旧 TTFT)`。
- 其余字段不变：`enabled`、`max_p50_ttft_ms`、`min_success_rate`、`pause_minutes`、`condition`、`schedule_use_failover_error_rate`

账号 overlay `GET/PUT /admin/accounts/:id/quality-hard-close`：

- `resolved.account_quality_window_n` / `resolved.min_success_samples` / `resolved.min_ttft_samples` = **全局 N**
- overlay 里的旧样本 / `account_quality_window_n` 不单独改窗

## `POST /api/v1/admin/accounts/quality-stats/batch`

Body 不变：`{ "account_ids": number[] }`。

`stats[id]` 尽量保持旧字段，语义改为 last-N：

| 字段 | 语义 |
| --- | --- |
| `n` / `window_n` / `account_quality_window_n` | 全站 N，格子用 `k/N` |
| `success_count` / `error_count` | \(W_{ok}\) 成败条数（`k = success+error`） |
| `success_rate` / `error_rate` | 由 \(W_{ok}\) 算出；`k==0` 为 null |
| `ttft_samples` | \(W_{ttft}\) 条数 |
| `p50_ttft_ms` / `avg_ttft_ms` / `p95_ttft_ms` / `max_ttft_ms` | 仅来自 \(W_{ttft}\)；0 条为 null |
| `window_seconds` | 可仍为 900；展示以 N 为准 |
| `schedule_use_failover_error_rate` | 入窗时的调度开关回声 |
| `terminal_error_count` / `failover_error_count` | 只投影当前调度口径（见 design） |
| `bridge_*` | last-N 不维护，可为 0 / null |

未满 N：**不要**因该指标硬关闭。格子应用 `ttft_samples < N` / `k < N` 显示样本不足。

## 硬关闭

- 仍 `EvaluateAccountQualityHardClose`；`min_*_samples` GET 已是 N。
- 读 last-N live，不是 15 分钟 SQL。
- `account_resume_until` 豁免不变。

## 不要读错

| 字段 | 轨 |
| --- | --- |
| `account_quality_window_n` | 账号 \(Q_a\)，默认 20 |
| `quality_window_n` / `quality_window_samples` | 智能调度配对 \(Q_{a,u}\)，默认 10 |
