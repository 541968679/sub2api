# 实现清单：账号质量 last-N（后端）

## 顺序

1. Settings：`account_quality_window_n`，默认 20，1–100；GET 回填旧样本字段。单测 normalize / echo。
2. last-N 数学：两窗 FIFO、入窗规则、投影 `AccountQualityStats`（含 `n` 别名）。单测失败 / 无首字 / 有首字。
3. Redis：`account-quality:last-n` + 投影 live；`Get` / batch 读 last-N。单测 ingest 与 batch。
4. 完成挂钩：usage + 计入错误 → 账号观察者（全用户）。入窗后硬关闭读同一 live。
5. 格子 `accounts/quality-stats/batch` 改读 last-N。用户维 batch 保持 15 分钟 SQL。
6. tick：禁止 15 分钟 SQL `Replace`；改为扫描 last-N 做历史快照 + 可选硬关闭兜底。
7. 规格 + `CHANGELOG_CUSTOM.md` + `research/account-quality-api-contract.md`。

## 校验

```text
go test -tags=unit ./internal/service -count=1 -run "AccountQuality|QualityHardClose|QualityStats|ObserveAccount"
go test -tags=unit ./internal/repository -count=1 -run "AccountQuality"
go test -tags=unit ./internal/handler/admin -count=1 -run "QualityHardClose|QualityStats|QualityHistory"
```

补：未满 N 不硬关闭；全用户入窗；格子与 live 同口径；tick 不再 SQL 覆盖 live。

## 风险文件

- `account_quality_live_cache.go`（不要 SCAN 删 last-N；resume HASH 保留）
- `account_quality_maintenance.go` `RunTick`
- `account_quality_hard_close.go` 样本地板 = N
- `ops_service.go` / `gateway_service.go` / `openai_gateway_service.go` 挂钩
- `account_usage_service.go` 账号 batch（勿改用户 batch）

回滚：回旧二进制；可留 `account-quality:last-n:*` 脏 key。
