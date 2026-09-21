# loadtest（tools/kimi-loadtest）

OpenAI 兼容 `/v1/chat/completions` 高并发压测。默认流量形状来自生产用户 `363`（`78496398@qq.com`），模型可换成 `glm-5.3` 等；并统计客户 SLA 表上的 TTFT / TPOT，以及空 `model` 等协议字段问题。

生产 inbound 画像（2026-09-21 16:34 CST，48h）：

| 项 | 值 |
|---|---|
| User-Agent | `Go-http-client/2.0`（Go stdlib HTTP/2 默认，本工具不覆盖） |
| 路径 | `POST /v1/chat/completions` |
| 模型 | `kimi-k3` ~97% |
| 短同步 | 57%，p50 上下文 434 tok，p50 耗时 8.1s |
| 长大流 | 43%，p50 上下文 52k（cache ~32k + input ~5k），p50 真首字 10.7s，p50 耗时 17s |
| 并发 | p50 in-flight ~24，p95 ~55，峰值 ~86 |
| 稳定性坑 | 503 无号、413 超 1MB、502 空流、524、400 prompt too long / invalid temperature |

本工具用 Go `net/http` 发请求，因此默认 UA 就是 `Go-http-client/2.0`。对 `https://` 上游会协商 HTTP/2。

## 用法

不要把 key 写进命令行（会进进程列表）。用环境变量：

```powershell
cd tools/kimi-loadtest
$env:LOADTEST_KEY = "sk-..."   # 或 KIMI_LOADTEST_KEY / SUB2API_KEY
go run . -base-url https://YOUR-UPSTREAM -model glm-5.3 -profile user363-sla -concurrency 50 -total 100 -yes
```

多模型轮询：

```powershell
go run . -base-url https://YOUR-UPSTREAM -models glm-5.3,kimi-k3 -profile user363-stream -concurrency 50 -total 100 -yes
```

冒烟（不发请求）：

```powershell
go run . -dry-run -profile user363
go run . -print-fingerprint
```

建议顺序：

1. `-profile smoke -concurrency 1 -total 1` 确认鉴权与模型名
2. `-profile user363-sync` 短请求基线
3. `-profile user363-stream -concurrency 50` 按现网 48h 长大流画像
4. `-profile user363-sla -concurrency 50 -yes` **按客户 SLA 表构造**：输入 p50/p90/p99 = 50K/160K/380K，输出 0.2K/1.3K/7K，并判定 TTFT/TPOT
5. 过 Sub2API 时加 `-strict-contract`，空 `model` 直接失败（对应 `expected "kimi-k3", got ""`）

## 常用参数

| 参数 | 默认 | 说明 |
|---|---|---|
| `-model` | `kimi-k3` | 单个模型，例如 `glm-5.3` |
| `-models` | 空 | 逗号分隔，轮询覆盖 `-model` |
| `-profile` | `user363` | `user363` 混合 / `user363-stream` / `user363-sync` / `user363-sla`（客户 SLA 分布） / `smoke` |
| `-sla` | false | 按客户 TTFT/TPOT 门槛判定（`user363-sla` 自动打开） |
| `-strict-contract` | false | `model` 缺失/空/不一致则进程失败 |
| `-concurrency` | 50 | 同时 in-flight 数 |
| `-total` | 100 | 总请求数 |
| `-duration` | 0 | 持续加压；>0 时忽略 `-total` 的截断，按时间一直补请求 |
| `-size-cap` | 80000 | 单请求上下文 token 上限；`0` 按画像抽到 200k+ |
| `-max-tokens` | 256 | 限制输出，压稳定性时够用；`0` 不传该字段 |
| `-tools` | `auto` | 流式带 coding tools + `tool_choice=none`；同步不带 |
| `-timeout` | 180s | 单请求总超时（客户 p95 首字曾到 46s） |
| `-temperature` | 空 | 默认不传。生产里 kimi-k3 出现过 `invalid temperature` |
| `-yes` | false | 估算输入 >2M token 时必须加，防止误烧额度 |
| `-abort-after-first-content` | false | 只测首字，读到第一条 content 就断开 |
| `-out` | `tmp/kimi-loadtest-runs/<ts>` | `run.json` + `requests.jsonl` + `summary.md` |

Key：`LOADTEST_KEY` / `KIMI_LOADTEST_KEY` / `SUB2API_KEY`。

## 客户 SLA（截图表）

`user363-sla` 按客户「请求数据分布」构造压测，summary 里按同一口径出表：

| 指标 | 要求 |
|---|---|
| 输入 tokens | p50 50K / p90 160K / p99 380K / avg 80K |
| 输出 tokens | p50 0.2K / p90 1.3K / p99 7K / avg 0.6K |
| TTFT | p50&lt;4s / p75&lt;8s / p90&lt;12s / p99&lt;30s |
| TPOT | p50&gt;70 tok/s；慢尾（表上 p99）&gt;40 tok/s |

TTFT 取流式第一条 **content**（role-only 不计）。TPOT = `output_tokens / (总耗时 − TTFT)`。

## 通过口径（给新渠道用）

`user363-sla`、50 并发、至少 100 请求：

- 成功率 ≥ 98%，无 empty/truncated stream
- 上表 TTFT / TPOT 全 PASS
- 直连上游允许缺 `model`（记入 contract）；过 Sub2API 时 `-strict-contract` 下应为 0
- 413=0；50 in-flight 不被 429 打崩；HTTPS 协商 HTTP/2

## 安全

- 不把 API key 写入 jsonl / summary / 日志
- 请求体是合成填充文本，不含客户原文
- 大上下文很费额度：先 smoke，再加 `-yes`
