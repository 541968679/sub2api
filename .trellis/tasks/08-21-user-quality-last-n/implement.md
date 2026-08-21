# 实现清单：用户质量格 last-N

## 顺序

1. Redis `user-quality:last-n` + 观察者（复用账号 FIFO 数学）。单测：用户 A/B 隔离、N 来自全局设置。
2. 完成挂钩：gateway / OpenAI RecordUsage + 计入 ops 错误传入 `user_id`。failover 计入与账号同一 `CountedInAccountScheduleRate`。
3. `POST /admin/users/quality-stats/batch` 改读 last-N；盖 N；Apply schedule caliber。单测：batch 不走 15m SQL、failover 非 0。
4. 表 `user_quality_snapshots` + 5 分钟 tick 快照 + `GET /admin/users/:id/quality-history`。
5. 前端：用户列表 / 页头 combined 格 + 点击历史弹窗；hint 改为 last-N。
6. 规格 + `CHANGELOG_CUSTOM.md` + `research/user-quality-api-contract.md`。

## 校验

```text
go test -tags=unit ./internal/service -count=1 -run "UserQuality|ObserveUser|GetUserLastN"
go test -tags=unit ./internal/repository -count=1 -run "UserQuality"
go test -tags=unit ./internal/handler/admin -count=1 -run "UserHandler_GetBatchQuality|UserQualityHistory"
pnpm --dir frontend exec vitest run src/views/admin/__tests__/UsersView.spec.ts src/components/admin/user/__tests__/AdminUserListRowTable.spec.ts src/components/user/__tests__/UserQualityHistoryDialog.spec.ts
```

补：A/B 隔离；failover 入 \(W_{ok}\)；列表 batch 非 15m；N=`account_quality_window_n`。

## 风险文件

- `account_quality_maintenance.go`（用户 tick 不得改账号 last-N / 硬关闭）
- `ops_service.go` / `gateway_service.go` / `openai_gateway_service.go` 挂钩（账号观察者行为不变）
- `user_handler.go` batch（勿再调用 `GetUserQualityStatsBatch` 15m）
- `usage_log_repo.go` 用户 15m SQL（可留，列表不用）

回滚：回旧二进制；可留 `user-quality:last-n:*` 与 `user_quality_snapshots` 脏数据。
