# Implement：每用户 \(Q_u\) N + 用户格子 P95

工作区：占用 checkout `E:\cursor project\api2sub`。不要开隔离 worktree。不 commit / push / deploy。

## 顺序

1. Migration `212_user_quality_window_n.sql` + Ent `User.quality_window_n` + `go generate ./ent`。
2. `ResolveUserQualityWindowN`；`AccountQualityLastN.OverrideN`；encode/decode `override_n`。
3. `userWindowN` + `ObserveAccountCompletion` 拆 Q_a/Q_u；`GetUserLastNStatsBatch` / snapshot 按用户 stamp；`ResizeUserLastN` / `ApplyUserQualityWindowN`。
4. Repo：读映射 + `GetQualityWindowN` / batch；Update 用 SQL 写列（对齐 `display_cache_token_max_mult`）。
5. `UpdateUserInput` + handler + AdminUser DTO；保存后 resize Redis。
6. `UserQualityDialog` 编辑 N；`AccountQualityCell` 用户 combined 显示 P95；i18n。
7. 测单：后端 last-N / maintenance / update；前端 cell + dialog。
8. 改 `.trellis/spec/backend/user-quality-last-n.md`；`docs/dev/codebase/account.md` 用户质量句；`CHANGELOG_CUSTOM.md`。

## 测试必须红的形状

- 用户 A N=10、用户 B 继承 20：ingest / batch stamp 不同
- 改全站 N：覆盖用户不跟、继承用户跟
- 保存 8：Redis FIFO ≤8，stats.window_n=8
- 清除覆盖：生效回到全站
- 用户 combined 格子有 `p95`；账号 combined 没有新 P95 行

## 验证

```powershell
go test -tags=unit ./internal/service -run "UserQuality|AccountQualityLastN|NormalizeAccountQualityWindowN|GetUserLastN" -count=1
go test -tags=unit ./internal/handler/admin -run "UserHandler_.*Quality|UpdateUser" -count=1
go test -tags=unit ./internal/repository -run "UserQualityLastN|UserRepo" -count=1
pnpm --dir frontend exec vitest run src/components/account/__tests__/AccountQualityCell.spec.ts src/components/admin/user/__tests__/UserQualityDialog.spec.ts
```

## 回滚点

migration 212 + OverrideN JSON + dialog/cell。无 Settings 新键。
