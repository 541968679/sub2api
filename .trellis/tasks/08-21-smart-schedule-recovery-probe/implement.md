# 实现清单：配对 N 条窗冷却

未 `task.py start` 前不改业务代码。

## 顺序

1. 策略：N 字段（默认 10，1–100）。PUT/GET/copy 把旧两个最少样本收敛成 N。单测 normalize。
2. 配对 live Redis：两窗 FIFO、入窗规则、清零、重算 p50/成功率。单测失败/无首字/failover/跨用户隔离。
3. 完成路径：usage + 计入错误后更新配对窗。冷却/暂停不入窗；豁免期入窗不判断。
4. `admitsScheduleUser`：智能调度冷却只读 \(Q_{a,u}\)。`selectable` 不再 `MarkUserQualityWindow`。到期清零再攒。单测 + 现有 synthesis 改编。
5. 趋势点 + 冷却/豁免/可调度事件（详情用）。
6. 前端：N 表单；「豁免期」文案（zh/en）；配对质量列 + 详情对话框。账号质量列不改口径。`will_cool` 改看配对窗。
7. 规格：`.trellis/spec/backend/account-user-schedule.md` 补配对轨。`CHANGELOG_CUSTOM.md`。

## 校验

```text
go test -tags=unit ./internal/service -count=1
go test -tags=unit ./internal/repository -count=1
pnpm --dir frontend exec vitest run src/views/admin/__tests__/UserSmartScheduleView.spec.ts src/composables/__tests__/smartSchedulePoolAdmission.spec.ts
```

补：未满 N 不冷却；豁免期内入窗期满可立刻冷；可调度清零；跨用户不串窗；账号 15m 单测不回归。

## 风险文件

- `account_user_schedule.go` `admitsScheduleUser`
- `user_smart_schedule_service.go` `SetPairAdmission`
- `account_quality_live_cache.go`（豁免 HASH 仍给豁免期用，不要误删账号轨）
- usage / ops_error 完成挂钩（别拖慢热路径）
- `UserSmartScheduleView.vue` / `smartSchedulePoolAdmission.ts` / zh.ts+en.ts

回滚：回旧二进制；可留 Redis 脏 key。

## `start` 前

- [x] 核心合同已决（N=10，账号×用户，两窗，豁免期/可调度）
- [x] 用户确认范围 1–100（默认仍 10）
- [x] 用户评审本 design / implement 后才 `task.py start`
- [x] `implement.jsonl` / `check.jsonl` 写入真实 spec/research 行
